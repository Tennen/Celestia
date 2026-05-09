package workflow

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"sync"
	"testing"

	"github.com/chentianyu/celestia/internal/models"
	sqlitestore "github.com/chentianyu/celestia/internal/storage/sqlite"
)

type sharedLLMWorkflowTransport struct {
	t               *testing.T
	mu              sync.Mutex
	llmBodies       []string
	rssURLs         []string
	feedBodiesByURL map[string]string
	llmStarted      chan struct{}
	releaseFirstLLM chan struct{}
}

func (t *sharedLLMWorkflowTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	switch req.URL.Host {
	case "rss.test":
		t.appendRSSURL(req.URL.String())
		body, ok := t.feedBodiesByURL[req.URL.String()]
		if !ok {
			t.t.Fatalf("unexpected RSS request URL %q", req.URL.String())
		}
		return response(http.StatusOK, "application/xml", body), nil
	case "llm.test":
		if got := req.Header.Get("Authorization"); got != "Bearer secret-token" {
			t.t.Fatalf("authorization header = %q, want bearer secret-token", got)
		}
		body, err := io.ReadAll(req.Body)
		if err != nil {
			t.t.Fatalf("read llm request body: %v", err)
		}
		count := t.appendLLMBody(string(body))
		if count == 1 {
			notifyWorkflowTransportChannel(t.llmStarted)
			if t.releaseFirstLLM != nil {
				<-t.releaseFirstLLM
			}
		}
		reply := fmt.Sprintf(`{"choices":[{"message":{"role":"assistant","content":"Digest %d"}}]}`, count)
		return response(http.StatusOK, "application/json", reply), nil
	default:
		t.t.Fatalf("unexpected request host %q", req.URL.Host)
		return nil, nil
	}
}

func (t *sharedLLMWorkflowTransport) appendLLMBody(body string) int {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.llmBodies = append(t.llmBodies, body)
	return len(t.llmBodies)
}

func (t *sharedLLMWorkflowTransport) llmBodiesSnapshot() []string {
	t.mu.Lock()
	defer t.mu.Unlock()
	return append([]string{}, t.llmBodies...)
}

func (t *sharedLLMWorkflowTransport) appendRSSURL(url string) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.rssURLs = append(t.rssURLs, url)
}

func (t *sharedLLMWorkflowTransport) rssURLsSnapshot() []string {
	t.mu.Lock()
	defer t.mu.Unlock()
	return append([]string{}, t.rssURLs...)
}

func notifyWorkflowTransportChannel(ch chan struct{}) {
	if ch == nil {
		return
	}
	select {
	case <-ch:
		return
	default:
		close(ch)
	}
}

func configureSharedLLMWorkflowProvider(t *testing.T, ctx context.Context, store *sqlitestore.Store, svc *Service) {
	t.Helper()
	snapshot, err := svc.Snapshot(ctx)
	if err != nil {
		t.Fatalf("Snapshot() error = %v", err)
	}
	settings := snapshot.Settings
	settings.DefaultLLMProviderID = "topic-llm"
	settings.LLMProviders = []models.AgentLLMProvider{{
		ID:       "topic-llm",
		Name:     "Topic LLM",
		Type:     "openai-like",
		BaseURL:  "https://llm.test",
		APIKey:   "secret-token",
		Model:    "gpt-4.1-mini",
		ChatPath: "/v1/chat/completions",
	}}
	saveWorkflowTestSettings(t, store, settings)
}

func sharedLLMWorkflowDefinition(withTimers bool) models.AgentWorkflow {
	workflowID := "workflow-shared-llm"
	if withTimers {
		workflowID = "workflow-shared-llm-timers"
	}
	nodes := []models.AgentWorkflowNode{
		{
			ID:       "rss-a",
			Type:     workflowNodeTypeRSSSources,
			Label:    "RSS A",
			Position: models.AgentNodePoint{X: 80, Y: 80},
			Data: map[string]any{"sources": []models.AgentWorkflowSource{{
				ID: "feed-a", Name: "Feed A", Category: "news", FeedURL: "https://rss.test/feed-a", Weight: 1, Enabled: true,
			}}},
		},
		{
			ID:       "rss-b",
			Type:     workflowNodeTypeRSSSources,
			Label:    "RSS B",
			Position: models.AgentNodePoint{X: 80, Y: 220},
			Data: map[string]any{"sources": []models.AgentWorkflowSource{{
				ID: "feed-b", Name: "Feed B", Category: "news", FeedURL: "https://rss.test/feed-b", Weight: 1, Enabled: true,
			}}},
		},
		{
			ID:       "text-main",
			Type:     workflowNodeTypeText,
			Label:    "Text",
			Position: models.AgentNodePoint{X: 320, Y: 150},
			Data:     map[string]any{"text": "Summarize the incoming RSS items."},
		},
		{
			ID:       "llm-main",
			Type:     workflowNodeTypeLLM,
			Label:    "LLM",
			Position: models.AgentNodePoint{X: 540, Y: 150},
			Data:     map[string]any{"provider_id": "topic-llm", "user_prompt": "Keep it short."},
		},
		{
			ID:       "wecom-main",
			Type:     workflowNodeTypeWeComOutput,
			Label:    "WeCom Output",
			Position: models.AgentNodePoint{X: 760, Y: 150},
			Data:     map[string]any{"to_user": "alice"},
		},
	}
	edges := []models.AgentWorkflowEdge{
		{ID: "edge-rss-a-llm", Source: "rss-a", SourceHandle: "content", Target: "llm-main", TargetHandle: "context"},
		{ID: "edge-rss-b-llm", Source: "rss-b", SourceHandle: "content", Target: "llm-main", TargetHandle: "context"},
		{ID: "edge-text-llm", Source: "text-main", SourceHandle: "text", Target: "llm-main", TargetHandle: "prompt"},
		{ID: "edge-llm-wecom", Source: "llm-main", SourceHandle: "text", Target: "wecom-main", TargetHandle: "text"},
	}
	if withTimers {
		nodes = append([]models.AgentWorkflowNode{
			{
				ID:       "timer-a",
				Type:     workflowNodeTypeTimer,
				Label:    "Timer A",
				Position: models.AgentNodePoint{X: 80, Y: 20},
				Data:     map[string]any{"schedule": "daily", "at": "08:30", "timezone": "UTC"},
			},
			{
				ID:       "timer-b",
				Type:     workflowNodeTypeTimer,
				Label:    "Timer B",
				Position: models.AgentNodePoint{X: 80, Y: 160},
				Data:     map[string]any{"schedule": "daily", "at": "08:30", "timezone": "UTC"},
			},
		}, nodes...)
		edges = append([]models.AgentWorkflowEdge{
			{ID: "edge-timer-a-rss", Source: "timer-a", SourceHandle: "trigger", Target: "rss-a", TargetHandle: "trigger"},
			{ID: "edge-timer-b-rss", Source: "timer-b", SourceHandle: "trigger", Target: "rss-b", TargetHandle: "trigger"},
		}, edges...)
	}
	return models.AgentWorkflow{
		ID:          workflowID,
		Name:        "Shared LLM Workflow",
		Description: "Exercise shared LLM fan-in semantics.",
		Nodes:       nodes,
		Edges:       edges,
	}
}

func sharedLLMWorkflowDefinitionWithTimerTimes(timerAAt string, timerBAt string) models.AgentWorkflow {
	workflow := sharedLLMWorkflowDefinition(true)
	for idx := range workflow.Nodes {
		switch workflow.Nodes[idx].ID {
		case "timer-a":
			workflow.Nodes[idx].Data = map[string]any{"schedule": "daily", "at": timerAAt, "timezone": "UTC"}
		case "timer-b":
			workflow.Nodes[idx].Data = map[string]any{"schedule": "daily", "at": timerBAt, "timezone": "UTC"}
		}
	}
	return workflow
}

func sharedLLMWorkflowDefinitionWithSingleTimer() models.AgentWorkflow {
	workflow := sharedLLMWorkflowDefinition(false)
	workflow.ID = "workflow-shared-llm-single-timer"
	workflow.Nodes = append([]models.AgentWorkflowNode{{
		ID:       "timer-a",
		Type:     workflowNodeTypeTimer,
		Label:    "Timer A",
		Position: models.AgentNodePoint{X: 80, Y: 20},
		Data:     map[string]any{"schedule": "daily", "at": "08:30", "timezone": "UTC"},
	}}, workflow.Nodes...)
	workflow.Edges = append([]models.AgentWorkflowEdge{{
		ID:           "edge-timer-a-rss",
		Source:       "timer-a",
		SourceHandle: "trigger",
		Target:       "rss-a",
		TargetHandle: "trigger",
	}}, workflow.Edges...)
	return workflow
}

func workflowRSSBody(title string, guid string, link string) string {
	return fmt.Sprintf(`<?xml version="1.0" encoding="UTF-8"?>
<rss><channel>
  <item>
    <title>%s</title>
    <link>%s</link>
    <guid>%s</guid>
    <description>%s description.</description>
  </item>
</channel></rss>`, title, link, guid, title)
}
