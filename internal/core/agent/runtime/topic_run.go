package runtime

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/chentianyu/celestia/internal/models"
	"github.com/google/uuid"
)

type topicNodeValue struct {
	Prompt string
	Text   string
	Items  []models.AgentTopicItem
	Search *models.AgentSearchResult
}

type topicExecutor struct {
	ctx            context.Context
	service        *Service
	workflow       models.AgentTopicWorkflow
	incoming       map[string][]models.AgentTopicEdge
	outgoing       map[string][]models.AgentTopicEdge
	nodes          map[string]models.AgentTopicNode
	cache          map[string]topicNodeValue
	active         map[string]bool
	nodeResults    map[string]models.AgentTopicNodeResult
	fetchErrors    []models.AgentRunError
	deliveryErrors []models.AgentRunError
	sentLog        map[string]struct{}
	sentItems      map[string]models.AgentTopicItem
}

type rssNodeConfig struct {
	Sources []models.AgentTopicSource `json:"sources"`
}

type promptNodeConfig struct {
	Prompt string `json:"prompt"`
}

type llmNodeConfig struct {
	ProviderID string `json:"provider_id,omitempty"`
	UserPrompt string `json:"user_prompt,omitempty"`
}

type searchNodeConfig struct {
	ProviderID string   `json:"provider_id,omitempty"`
	Query      string   `json:"query"`
	Recency    string   `json:"recency,omitempty"`
	MaxItems   int      `json:"max_items,omitempty"`
	Sites      []string `json:"sites,omitempty"`
}

type wecomOutputConfig struct {
	ToUser string `json:"to_user"`
}

func (s *Service) RunTopicSummary(ctx context.Context, workflowID string) (models.AgentTopicRun, error) {
	snapshot, err := s.Snapshot(ctx)
	if err != nil {
		return models.AgentTopicRun{}, err
	}
	workflow, ok := selectTopicWorkflow(snapshot.TopicSummary, workflowID)
	if !ok {
		return models.AgentTopicRun{}, errors.New("topic workflow not found")
	}
	executor := newTopicExecutor(ctx, s, workflow, snapshot.TopicSummary.SentLog)
	run := models.AgentTopicRun{
		ID:           uuid.NewString(),
		WorkflowID:   workflow.ID,
		WorkflowName: workflow.Name,
		ProfileID:    workflow.ID,
		CreatedAt:    time.Now().UTC(),
		StartedAt:    time.Now().UTC(),
		Status:       "running",
		Items:        []models.AgentTopicItem{},
		NodeResults:  []models.AgentTopicNodeResult{},
	}
	targets := executor.targetNodes()
	if len(targets) == 0 {
		return models.AgentTopicRun{}, errors.New("workflow has no executable target nodes")
	}
	outputs := make([]string, 0, len(targets))
	for _, nodeID := range targets {
		value, evalErr := executor.evaluate(nodeID)
		if evalErr != nil {
			continue
		}
		outputs = append(outputs, strings.TrimSpace(value.Text))
		run.Items = append(run.Items, value.Items...)
	}
	run.Items = truncateTopicItems(run.Items, 30)
	run.NodeResults = executor.results()
	run.FetchErrors = executor.fetchErrors
	run.DeliveryErrors = executor.deliveryErrors
	run.OutputText = strings.Join(uniqueTopicStrings(outputs), "\n\n")
	run.FinishedAt = time.Now().UTC()
	run.Status = topicRunStatus(run)
	run.Summary = topicRunSummary(workflow.Name, run)
	if err := s.appendTopicRun(ctx, run, executor.sentItemList()); err != nil {
		return models.AgentTopicRun{}, err
	}
	return run, nil
}

func newTopicExecutor(ctx context.Context, service *Service, workflow models.AgentTopicWorkflow, sentLog []models.AgentTopicSentLogItem) *topicExecutor {
	nodes := make(map[string]models.AgentTopicNode, len(workflow.Nodes))
	incoming := make(map[string][]models.AgentTopicEdge, len(workflow.Nodes))
	outgoing := make(map[string][]models.AgentTopicEdge, len(workflow.Nodes))
	for _, node := range workflow.Nodes {
		nodes[node.ID] = node
	}
	for _, edge := range workflow.Edges {
		incoming[edge.Target] = append(incoming[edge.Target], edge)
		outgoing[edge.Source] = append(outgoing[edge.Source], edge)
	}
	return &topicExecutor{
		ctx:         ctx,
		service:     service,
		workflow:    workflow,
		incoming:    incoming,
		outgoing:    outgoing,
		nodes:       nodes,
		cache:       map[string]topicNodeValue{},
		active:      map[string]bool{},
		nodeResults: map[string]models.AgentTopicNodeResult{},
		sentLog:     topicSentLogSet(sentLog),
		sentItems:   map[string]models.AgentTopicItem{},
	}
}

func (e *topicExecutor) targetNodes() []string {
	targets := make([]string, 0, len(e.workflow.Nodes))
	hasWeComOutput := false
	for _, node := range e.workflow.Nodes {
		if node.Type == topicNodeTypeWeComOutput {
			hasWeComOutput = true
			targets = append(targets, node.ID)
		}
	}
	if hasWeComOutput {
		return targets
	}
	targets = targets[:0]
	for _, node := range e.workflow.Nodes {
		if node.Type == topicNodeTypeGroup {
			continue
		}
		if len(e.outgoing[node.ID]) == 0 {
			targets = append(targets, node.ID)
		}
	}
	return targets
}

func (e *topicExecutor) evaluate(nodeID string) (topicNodeValue, error) {
	if value, ok := e.cache[nodeID]; ok {
		return value, nil
	}
	if e.active[nodeID] {
		err := errors.New("workflow graph contains a cycle")
		e.recordFailure(nodeID, err)
		return topicNodeValue{}, err
	}
	node, ok := e.nodes[nodeID]
	if !ok {
		err := fmt.Errorf("workflow node %q not found", nodeID)
		e.recordFailure(nodeID, err)
		return topicNodeValue{}, err
	}
	e.active[nodeID] = true
	defer delete(e.active, nodeID)
	value, summary, metadata, err := e.execute(node)
	if err != nil {
		e.recordFailure(nodeID, err)
		return topicNodeValue{}, err
	}
	e.cache[nodeID] = value
	e.nodeResults[nodeID] = models.AgentTopicNodeResult{
		NodeID:   node.ID,
		NodeType: node.Type,
		Status:   "succeeded",
		Summary:  summary,
		Metadata: metadata,
	}
	return value, nil
}

func (e *topicExecutor) execute(node models.AgentTopicNode) (topicNodeValue, string, map[string]any, error) {
	switch node.Type {
	case topicNodeTypeGroup:
		return topicNodeValue{}, "Group container", map[string]any{"children": len(e.groupChildren(node.ID))}, nil
	case topicNodeTypeRSSSources:
		return e.executeRSSNode(node)
	case topicNodeTypePromptUnit:
		return e.executePromptNode(node)
	case topicNodeTypeSearchProvider:
		return e.executeSearchNode(node)
	case topicNodeTypeLLM:
		return e.executeLLMNode(node)
	case topicNodeTypeWeComOutput:
		return e.executeWeComOutputNode(node)
	default:
		return topicNodeValue{}, "", nil, fmt.Errorf("unsupported workflow node type %q", node.Type)
	}
}

func (e *topicExecutor) executeRSSNode(node models.AgentTopicNode) (topicNodeValue, string, map[string]any, error) {
	config, err := decodeTopicNodeData[rssNodeConfig](node.Data)
	if err != nil {
		return topicNodeValue{}, "", nil, err
	}
	items := make([]models.AgentTopicItem, 0, len(config.Sources)*4)
	errorCount := 0
	for _, source := range config.Sources {
		if !source.Enabled {
			continue
		}
		fetched, fetchErr := fetchFeed(e.ctx, source)
		if fetchErr != nil {
			errorCount++
			e.fetchErrors = append(e.fetchErrors, models.AgentRunError{Target: firstNonEmpty(source.ID, source.Name, source.FeedURL), Error: fetchErr.Error()})
			continue
		}
		for _, item := range fetched {
			key := normalizeTopicURL(item.URL)
			if key != "" {
				if _, ok := e.sentLog[key]; ok {
					continue
				}
				e.sentLog[key] = struct{}{}
			}
			items = append(items, item)
		}
	}
	items = truncateTopicItems(items, 30)
	return topicNodeValue{Text: topicItemsText(items), Items: items}, fmt.Sprintf("%d items from %d sources", len(items), len(config.Sources)), map[string]any{
		"item_count":   len(items),
		"source_count": len(config.Sources),
		"error_count":  errorCount,
	}, nil
}

func (e *topicExecutor) executePromptNode(node models.AgentTopicNode) (topicNodeValue, string, map[string]any, error) {
	config, err := decodeTopicNodeData[promptNodeConfig](node.Data)
	if err != nil {
		return topicNodeValue{}, "", nil, err
	}
	prompt := strings.TrimSpace(config.Prompt)
	if prompt == "" {
		return topicNodeValue{}, "", nil, errors.New("prompt node requires prompt")
	}
	return topicNodeValue{Prompt: prompt, Text: prompt}, "Prompt ready", map[string]any{"chars": len(prompt)}, nil
}

func (e *topicExecutor) executeSearchNode(node models.AgentTopicNode) (topicNodeValue, string, map[string]any, error) {
	config, err := decodeTopicNodeData[searchNodeConfig](node.Data)
	if err != nil {
		return topicNodeValue{}, "", nil, err
	}
	query := strings.TrimSpace(config.Query)
	if query == "" {
		return topicNodeValue{}, "", nil, errors.New("search provider node requires query")
	}
	result, runErr := e.service.RunSearch(e.ctx, models.AgentSearchRequest{
		EngineSelector: strings.TrimSpace(config.ProviderID),
		MaxItems:       maxInt(config.MaxItems, 8),
		LogContext:     "topic_workflow:" + e.workflow.ID,
		Plans: []models.AgentSearchPlan{{
			Label:   node.Label,
			Query:   query,
			Recency: strings.TrimSpace(config.Recency),
			Sites:   append([]string{}, config.Sites...),
		}},
	})
	if runErr != nil {
		return topicNodeValue{}, "", nil, runErr
	}
	return topicNodeValue{
			Search: &result,
			Text:   topicSearchResultText(result),
		}, fmt.Sprintf("%d search results", len(result.Items)), map[string]any{
			"item_count":  len(result.Items),
			"error_count": len(result.Errors),
		}, nil
}

func (e *topicExecutor) executeLLMNode(node models.AgentTopicNode) (topicNodeValue, string, map[string]any, error) {
	config, err := decodeTopicNodeData[llmNodeConfig](node.Data)
	if err != nil {
		return topicNodeValue{}, "", nil, err
	}
	promptInputs, promptErr := e.collect(node.ID, "prompt")
	if promptErr != nil {
		return topicNodeValue{}, "", nil, promptErr
	}
	contextInputs, contextErr := e.collect(node.ID, "context")
	if contextErr != nil {
		return topicNodeValue{}, "", nil, contextErr
	}
	searchInputs, searchErr := e.collect(node.ID, "search")
	if searchErr != nil {
		return topicNodeValue{}, "", nil, searchErr
	}
	if len(e.incomingByHandle(node.ID, "tool")) > 0 {
		return topicNodeValue{}, "", nil, errors.New("llm tool handle is reserved but not executable yet")
	}
	if len(e.incomingByHandle(node.ID, "skill")) > 0 {
		return topicNodeValue{}, "", nil, errors.New("llm skill handle is reserved but not executable yet")
	}
	promptText := strings.Join(uniqueTopicStrings(promptInputs.prompts), "\n\n")
	contextText := strings.Join(uniqueTopicStrings(contextInputs.texts), "\n\n")
	searchText := strings.Join(uniqueTopicStrings(searchInputs.searches), "\n\n")
	finalPrompt := buildWorkflowLLMPrompt(promptText, strings.TrimSpace(config.UserPrompt), contextText, searchText)
	if strings.TrimSpace(finalPrompt) == "" {
		return topicNodeValue{}, "", nil, errors.New("llm node has no promptable input")
	}
	output, genErr := e.service.GenerateTextWithProvider(e.ctx, strings.TrimSpace(config.ProviderID), finalPrompt)
	if genErr != nil {
		return topicNodeValue{}, "", nil, genErr
	}
	return topicNodeValue{
			Text:  output,
			Items: append([]models.AgentTopicItem{}, contextInputs.items...),
		}, fmt.Sprintf("Generated %d chars", len(output)), map[string]any{
			"chars": len(output),
			"items": len(contextInputs.items),
		}, nil
}

func (e *topicExecutor) executeWeComOutputNode(node models.AgentTopicNode) (topicNodeValue, string, map[string]any, error) {
	config, err := decodeTopicNodeData[wecomOutputConfig](node.Data)
	if err != nil {
		return topicNodeValue{}, "", nil, err
	}
	if e.service.topicOutput == nil {
		return topicNodeValue{}, "", nil, errors.New("wecom output runtime is not configured")
	}
	inputs, inputErr := e.collect(node.ID, "")
	if inputErr != nil {
		return topicNodeValue{}, "", nil, inputErr
	}
	text := strings.Join(uniqueTopicStrings(inputs.texts), "\n\n")
	if strings.TrimSpace(text) == "" {
		return topicNodeValue{}, "", nil, errors.New("wecom output node requires text input")
	}
	toUser := strings.TrimSpace(config.ToUser)
	if toUser == "" {
		return topicNodeValue{}, "", nil, errors.New("wecom output node requires to_user")
	}
	if sendErr := e.service.topicOutput.SendWeComText(e.ctx, toUser, text); sendErr != nil {
		e.deliveryErrors = append(e.deliveryErrors, models.AgentRunError{Target: toUser, Error: sendErr.Error()})
		return topicNodeValue{}, "", nil, sendErr
	}
	for _, item := range inputs.items {
		key := normalizeTopicURL(item.URL)
		if key == "" {
			continue
		}
		e.sentItems[key] = item
	}
	return topicNodeValue{Text: text, Items: append([]models.AgentTopicItem{}, inputs.items...)}, "Delivered to WeCom", map[string]any{
		"to_user":    toUser,
		"text_chars": len(text),
		"item_count": len(inputs.items),
	}, nil
}

type collectedTopicInputs struct {
	prompts  []string
	texts    []string
	searches []string
	items    []models.AgentTopicItem
}

func (e *topicExecutor) collect(nodeID string, targetHandle string) (collectedTopicInputs, error) {
	out := collectedTopicInputs{}
	for _, edge := range e.incomingByHandle(nodeID, targetHandle) {
		value, err := e.evaluate(edge.Source)
		if err != nil {
			return out, err
		}
		if value.Prompt != "" {
			out.prompts = append(out.prompts, value.Prompt)
		}
		if value.Text != "" {
			out.texts = append(out.texts, value.Text)
		}
		if value.Search != nil {
			out.searches = append(out.searches, topicSearchResultText(*value.Search))
		}
		out.items = append(out.items, value.Items...)
	}
	return out, nil
}

func (e *topicExecutor) incomingByHandle(nodeID string, targetHandle string) []models.AgentTopicEdge {
	edges := e.incoming[nodeID]
	if strings.TrimSpace(targetHandle) == "" {
		return append([]models.AgentTopicEdge{}, edges...)
	}
	filtered := make([]models.AgentTopicEdge, 0, len(edges))
	for _, edge := range edges {
		if strings.TrimSpace(edge.TargetHandle) == strings.TrimSpace(targetHandle) {
			filtered = append(filtered, edge)
		}
	}
	return filtered
}

func (e *topicExecutor) groupChildren(groupID string) []models.AgentTopicNode {
	children := make([]models.AgentTopicNode, 0)
	for _, node := range e.workflow.Nodes {
		if node.ParentID == groupID {
			children = append(children, node)
		}
	}
	return children
}

func (e *topicExecutor) recordFailure(nodeID string, err error) {
	node := e.nodes[nodeID]
	e.nodeResults[nodeID] = models.AgentTopicNodeResult{
		NodeID:   nodeID,
		NodeType: node.Type,
		Status:   "failed",
		Summary:  err.Error(),
	}
}

func (e *topicExecutor) results() []models.AgentTopicNodeResult {
	results := make([]models.AgentTopicNodeResult, 0, len(e.nodeResults))
	for _, node := range e.workflow.Nodes {
		if result, ok := e.nodeResults[node.ID]; ok {
			results = append(results, result)
		}
	}
	return results
}

func (e *topicExecutor) sentItemList() []models.AgentTopicItem {
	items := make([]models.AgentTopicItem, 0, len(e.sentItems))
	for _, item := range e.sentItems {
		items = append(items, item)
	}
	return items
}

func buildWorkflowLLMPrompt(promptText string, userPrompt string, contextText string, searchText string) string {
	sections := make([]string, 0, 4)
	if strings.TrimSpace(promptText) != "" {
		sections = append(sections, "Prompt Unit:\n"+strings.TrimSpace(promptText))
	}
	if strings.TrimSpace(userPrompt) != "" {
		sections = append(sections, "User Prompt:\n"+strings.TrimSpace(userPrompt))
	}
	if strings.TrimSpace(contextText) != "" {
		sections = append(sections, "Workflow Context:\n"+strings.TrimSpace(contextText))
	}
	if strings.TrimSpace(searchText) != "" {
		sections = append(sections, "Search Results:\n"+strings.TrimSpace(searchText))
	}
	return strings.Join(sections, "\n\n")
}

func topicSearchResultText(result models.AgentSearchResult) string {
	lines := make([]string, 0, len(result.Items))
	for idx, item := range result.Items {
		line := fmt.Sprintf("%d. %s", idx+1, firstNonEmpty(strings.TrimSpace(item.Title), "(untitled)"))
		if link := strings.TrimSpace(item.Link); link != "" {
			line += "\n" + link
		}
		if snippet := strings.TrimSpace(item.Snippet); snippet != "" {
			line += "\n" + snippet
		}
		lines = append(lines, line)
	}
	if len(result.Errors) > 0 {
		lines = append(lines, "Errors: "+strings.Join(result.Errors, "; "))
	}
	return strings.Join(lines, "\n\n")
}

func topicRunStatus(run models.AgentTopicRun) string {
	failures := 0
	for _, result := range run.NodeResults {
		if result.Status == "failed" {
			failures++
		}
	}
	switch {
	case failures == len(run.NodeResults) && failures > 0:
		return "failed"
	case failures > 0 || len(run.FetchErrors) > 0 || len(run.DeliveryErrors) > 0:
		return "degraded"
	default:
		return "succeeded"
	}
}

func topicRunSummary(workflowName string, run models.AgentTopicRun) string {
	switch run.Status {
	case "succeeded":
		return fmt.Sprintf("Workflow %s completed with %d items.", workflowName, len(run.Items))
	case "degraded":
		return fmt.Sprintf("Workflow %s completed with issues across %d nodes.", workflowName, len(run.NodeResults))
	default:
		return fmt.Sprintf("Workflow %s failed.", workflowName)
	}
}

func truncateTopicItems(items []models.AgentTopicItem, limit int) []models.AgentTopicItem {
	if limit <= 0 || len(items) <= limit {
		return items
	}
	return items[:limit]
}
