package workflow

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/chentianyu/celestia/internal/models"
)

const workflowRSSBodyLimit = 2 << 20

type rssNodeConfig struct {
	Sources []models.AgentWorkflowSource `json:"sources"`
}

type workflowFeedFetchResult struct {
	Items           []models.AgentWorkflowItem
	RawBody         string
	LastModified    string
	IfModifiedSince string
	RequestedAt     time.Time
	NotModified     bool
	StatusCode      int
}

func (e *workflowExecutor) executeRSSNode(node models.AgentWorkflowNode) (workflowNodeValue, string, map[string]any, error) {
	config, err := decodeWorkflowNodeData[rssNodeConfig](node.Data)
	if err != nil {
		return workflowNodeValue{}, "", nil, err
	}
	triggerEdges := e.incomingByHandle(node.ID, "trigger")
	if len(triggerEdges) > 0 {
		triggerInputs, inputErr := e.collect(node.ID, "trigger")
		if inputErr != nil {
			return workflowNodeValue{}, "", nil, inputErr
		}
		if triggerInputs.hasBlockingWindow() {
			return workflowNodeValue{Text: "", Items: nil, Blocked: true, BlockedByWindow: true}, "RSS outside time window", map[string]any{
				"item_count":          0,
				"trigger_input_count": len(triggerEdges),
				"blocked_by_window":   true,
			}, nil
		}
		if triggerInputs.triggers == 0 {
			return workflowNodeValue{Text: "", Items: nil, Blocked: true, BlockedByTimer: true}, "RSS waiting for timer trigger", map[string]any{
				"item_count":          0,
				"trigger_input_count": len(triggerEdges),
				"triggered":           false,
			}, nil
		}
	}
	now := time.Now().UTC()
	items := make([]models.AgentWorkflowItem, 0, len(config.Sources)*4)
	enabledCount := 0
	requestedCount := 0
	notModifiedCount := 0
	errorCount := 0
	for _, source := range config.Sources {
		if !source.Enabled {
			continue
		}
		enabledCount++
		state, stateKey := e.sourceStateFor(node.ID, source)
		requestedCount++
		result, fetchErr := pollWorkflowFeed(e.ctx, source, state, now)
		if fetchErr != nil {
			errorCount++
			e.fetchErrors = append(e.fetchErrors, models.AgentRunError{Target: firstNonEmpty(source.ID, source.Name, source.FeedURL), Error: fetchErr.Error()})
			continue
		}
		e.stageSourceStateUpdate(buildWorkflowSourceStateUpdate(stateKey, state, node.ID, e.workflow.ID, source, result))
		if result.NotModified {
			notModifiedCount++
			continue
		}
		items = append(items, workflowNewFeedItems(result.Items, state.LastResponseBody, source)...)
	}
	items = truncateWorkflowItems(items, 30)
	metadata := map[string]any{
		"item_count":             len(items),
		"enabled_source_count":   enabledCount,
		"requested_source_count": requestedCount,
		"not_modified_count":     notModifiedCount,
		"error_count":            errorCount,
	}
	if len(items) == 0 {
		metadata["blocked_by_upstream"] = true
		return workflowNodeValue{Blocked: true}, "No new RSS items", metadata, nil
	}
	return workflowNodeValue{Text: workflowItemsContextJSON(items), Items: items}, fmt.Sprintf("%d new items from %d sources", len(items), enabledCount), metadata, nil
}

func pollWorkflowFeed(ctx context.Context, source models.AgentWorkflowSource, state models.AgentWorkflowSourceState, now time.Time) (workflowFeedFetchResult, error) {
	if strings.TrimSpace(source.FeedURL) == "" {
		return workflowFeedFetchResult{}, errors.New("feed_url is required")
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, source.FeedURL, nil)
	if err != nil {
		return workflowFeedFetchResult{}, err
	}
	ifModifiedSince := workflowFeedIfModifiedSince(state)
	if ifModifiedSince != "" {
		req.Header.Set("If-Modified-Since", ifModifiedSince)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return workflowFeedFetchResult{}, err
	}
	defer resp.Body.Close()
	result := workflowFeedFetchResult{
		IfModifiedSince: ifModifiedSince,
		LastModified:    strings.TrimSpace(resp.Header.Get("Last-Modified")),
		RequestedAt:     now,
		StatusCode:      resp.StatusCode,
	}
	if resp.StatusCode == http.StatusNotModified {
		result.NotModified = true
		return result, nil
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return workflowFeedFetchResult{}, errors.New("feed request failed with " + resp.Status)
	}
	raw, err := io.ReadAll(io.LimitReader(resp.Body, workflowRSSBodyLimit))
	if err != nil {
		return workflowFeedFetchResult{}, err
	}
	result.RawBody = string(raw)
	result.Items = parseFeedItems(raw, source)
	return result, nil
}

func workflowFeedIfModifiedSince(state models.AgentWorkflowSourceState) string {
	if lastModified := strings.TrimSpace(state.LastModified); lastModified != "" {
		return lastModified
	}
	if !state.LastRequestedAt.IsZero() {
		return state.LastRequestedAt.UTC().Format(http.TimeFormat)
	}
	return ""
}

func buildWorkflowSourceStateUpdate(key string, current models.AgentWorkflowSourceState, nodeID string, workflowID string, source models.AgentWorkflowSource, result workflowFeedFetchResult) workflowSourceStateUpdate {
	requestState := current
	requestState.WorkflowID = strings.TrimSpace(workflowID)
	requestState.NodeID = strings.TrimSpace(nodeID)
	requestState.SourceID = strings.TrimSpace(source.ID)
	requestState.FeedURL = strings.TrimSpace(source.FeedURL)
	requestState.LastRequestedAt = result.RequestedAt.UTC()
	requestState.UpdatedAt = result.RequestedAt.UTC()

	commitState := requestState
	if result.NotModified {
		if result.LastModified != "" {
			commitState.LastModified = result.LastModified
		}
	} else {
		commitState.LastModified = ""
		if result.LastModified != "" {
			commitState.LastModified = result.LastModified
		}
		commitState.LastResponseBody = result.RawBody
	}
	return workflowSourceStateUpdate{
		Key:          key,
		RequestState: requestState,
		CommitState:  commitState,
	}
}

func workflowNewFeedItems(current []models.AgentWorkflowItem, previousBody string, source models.AgentWorkflowSource) []models.AgentWorkflowItem {
	previousItems := parseFeedItems([]byte(previousBody), source)
	previousIDs := workflowFeedItemIdentitySet(previousItems)
	out := make([]models.AgentWorkflowItem, 0, len(current))
	currentIDs := map[string]struct{}{}
	for _, item := range current {
		identity := workflowFeedItemIdentity(item)
		if identity != "" {
			if _, exists := previousIDs[identity]; exists {
				continue
			}
			if _, exists := currentIDs[identity]; exists {
				continue
			}
			currentIDs[identity] = struct{}{}
		}
		out = append(out, item)
	}
	return out
}

func workflowFeedItemIdentitySet(items []models.AgentWorkflowItem) map[string]struct{} {
	out := make(map[string]struct{}, len(items))
	for _, item := range items {
		identity := workflowFeedItemIdentity(item)
		if identity == "" {
			continue
		}
		out[identity] = struct{}{}
	}
	return out
}

func workflowFeedItemIdentity(item models.AgentWorkflowItem) string {
	if guid := strings.TrimSpace(strings.ToLower(item.GUID)); guid != "" {
		return "guid:" + guid
	}
	if url := normalizeWorkflowURL(item.URL); url != "" {
		return "url:" + url
	}
	title := strings.TrimSpace(strings.ToLower(item.Title))
	publishedAt := strings.TrimSpace(strings.ToLower(item.PublishedAt))
	summary := strings.TrimSpace(strings.ToLower(item.Summary))
	if title == "" && publishedAt == "" && summary == "" {
		return ""
	}
	return "fallback:" + title + "\x1f" + publishedAt + "\x1f" + summary
}
