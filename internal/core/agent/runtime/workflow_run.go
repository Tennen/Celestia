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

type workflowNodeValue struct {
	Prompt string
	Text   string
	Items  []models.AgentWorkflowItem
	Search *models.AgentSearchResult
}

type workflowExecutor struct {
	ctx            context.Context
	service        *Service
	workflow       models.AgentWorkflow
	incoming       map[string][]models.AgentWorkflowEdge
	outgoing       map[string][]models.AgentWorkflowEdge
	nodes          map[string]models.AgentWorkflowNode
	cache          map[string]workflowNodeValue
	active         map[string]bool
	nodeResults    map[string]models.AgentWorkflowNodeResult
	fetchErrors    []models.AgentRunError
	deliveryErrors []models.AgentRunError
	sentLog        map[string]struct{}
	sentItems      map[string]models.AgentWorkflowItem
}

type rssNodeConfig struct {
	Sources []models.AgentWorkflowSource `json:"sources"`
}

type textNodeConfig struct {
	Text   string `json:"text,omitempty"`
	Prompt string `json:"prompt,omitempty"`
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

func (s *Service) RunWorkflow(ctx context.Context, workflowID string) (models.AgentWorkflowRun, error) {
	snapshot, err := s.Snapshot(ctx)
	if err != nil {
		return models.AgentWorkflowRun{}, err
	}
	workflow, ok := selectWorkflow(snapshot.Workflow, workflowID)
	if !ok {
		return models.AgentWorkflowRun{}, errors.New("workflow not found")
	}
	executor := newWorkflowExecutor(ctx, s, workflow, snapshot.Workflow.SentLog)
	run := models.AgentWorkflowRun{
		ID:           uuid.NewString(),
		WorkflowID:   workflow.ID,
		WorkflowName: workflow.Name,
		CreatedAt:    time.Now().UTC(),
		StartedAt:    time.Now().UTC(),
		Status:       "running",
		Items:        []models.AgentWorkflowItem{},
		NodeResults:  []models.AgentWorkflowNodeResult{},
	}
	targets := executor.targetNodes()
	if len(targets) == 0 {
		return models.AgentWorkflowRun{}, errors.New("workflow has no executable target nodes")
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
	run.Items = truncateWorkflowItems(run.Items, 30)
	run.NodeResults = executor.results()
	run.FetchErrors = executor.fetchErrors
	run.DeliveryErrors = executor.deliveryErrors
	run.OutputText = strings.Join(uniqueWorkflowStrings(outputs), "\n\n")
	run.FinishedAt = time.Now().UTC()
	run.Status = workflowRunStatus(run)
	run.Summary = workflowRunSummary(workflow.Name, run)
	if err := s.appendWorkflowRun(ctx, run, executor.sentItemList()); err != nil {
		return models.AgentWorkflowRun{}, err
	}
	return run, nil
}

func newWorkflowExecutor(ctx context.Context, service *Service, workflow models.AgentWorkflow, sentLog []models.AgentWorkflowSentLogItem) *workflowExecutor {
	nodes := make(map[string]models.AgentWorkflowNode, len(workflow.Nodes))
	incoming := make(map[string][]models.AgentWorkflowEdge, len(workflow.Nodes))
	outgoing := make(map[string][]models.AgentWorkflowEdge, len(workflow.Nodes))
	for _, node := range workflow.Nodes {
		nodes[node.ID] = node
	}
	for _, edge := range workflow.Edges {
		incoming[edge.Target] = append(incoming[edge.Target], edge)
		outgoing[edge.Source] = append(outgoing[edge.Source], edge)
	}
	return &workflowExecutor{
		ctx:         ctx,
		service:     service,
		workflow:    workflow,
		incoming:    incoming,
		outgoing:    outgoing,
		nodes:       nodes,
		cache:       map[string]workflowNodeValue{},
		active:      map[string]bool{},
		nodeResults: map[string]models.AgentWorkflowNodeResult{},
		sentLog:     workflowSentLogSet(sentLog),
		sentItems:   map[string]models.AgentWorkflowItem{},
	}
}

func (e *workflowExecutor) targetNodes() []string {
	targets := make([]string, 0, len(e.workflow.Nodes))
	hasWeComOutput := false
	for _, node := range e.workflow.Nodes {
		if node.Type == workflowNodeTypeWeComOutput {
			hasWeComOutput = true
			targets = append(targets, node.ID)
		}
	}
	if hasWeComOutput {
		return targets
	}
	targets = targets[:0]
	for _, node := range e.workflow.Nodes {
		if node.Type == workflowNodeTypeGroup {
			continue
		}
		if len(e.outgoing[node.ID]) == 0 {
			targets = append(targets, node.ID)
		}
	}
	return targets
}

func (e *workflowExecutor) evaluate(nodeID string) (workflowNodeValue, error) {
	if value, ok := e.cache[nodeID]; ok {
		return value, nil
	}
	if e.active[nodeID] {
		err := errors.New("workflow graph contains a cycle")
		e.recordFailure(nodeID, err)
		return workflowNodeValue{}, err
	}
	node, ok := e.nodes[nodeID]
	if !ok {
		err := fmt.Errorf("workflow node %q not found", nodeID)
		e.recordFailure(nodeID, err)
		return workflowNodeValue{}, err
	}
	e.active[nodeID] = true
	defer delete(e.active, nodeID)
	value, summary, metadata, err := e.execute(node)
	if err != nil {
		e.recordFailure(nodeID, err)
		return workflowNodeValue{}, err
	}
	e.cache[nodeID] = value
	e.nodeResults[nodeID] = models.AgentWorkflowNodeResult{
		NodeID:   node.ID,
		NodeType: node.Type,
		Status:   "succeeded",
		Summary:  summary,
		Metadata: metadata,
	}
	return value, nil
}

func (e *workflowExecutor) execute(node models.AgentWorkflowNode) (workflowNodeValue, string, map[string]any, error) {
	switch node.Type {
	case workflowNodeTypeGroup:
		return workflowNodeValue{}, "Group container", map[string]any{"children": len(e.groupChildren(node.ID))}, nil
	case workflowNodeTypeRSSSources:
		return e.executeRSSNode(node)
	case workflowNodeTypeText, legacyWorkflowNodeTypePrompt:
		return e.executeTextNode(node)
	case workflowNodeTypeSearchProvider:
		return e.executeSearchNode(node)
	case workflowNodeTypeLLM:
		return e.executeLLMNode(node)
	case workflowNodeTypeWeComOutput:
		return e.executeWeComOutputNode(node)
	default:
		return workflowNodeValue{}, "", nil, fmt.Errorf("unsupported workflow node type %q", node.Type)
	}
}

func (e *workflowExecutor) executeRSSNode(node models.AgentWorkflowNode) (workflowNodeValue, string, map[string]any, error) {
	config, err := decodeWorkflowNodeData[rssNodeConfig](node.Data)
	if err != nil {
		return workflowNodeValue{}, "", nil, err
	}
	items := make([]models.AgentWorkflowItem, 0, len(config.Sources)*4)
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
			key := normalizeWorkflowURL(item.URL)
			if key != "" {
				if _, ok := e.sentLog[key]; ok {
					continue
				}
				e.sentLog[key] = struct{}{}
			}
			items = append(items, item)
		}
	}
	items = truncateWorkflowItems(items, 30)
	return workflowNodeValue{Text: workflowItemsContextJSON(items), Items: items}, fmt.Sprintf("%d items from %d sources", len(items), len(config.Sources)), map[string]any{
		"item_count":   len(items),
		"source_count": len(config.Sources),
		"error_count":  errorCount,
	}, nil
}

func (e *workflowExecutor) executeTextNode(node models.AgentWorkflowNode) (workflowNodeValue, string, map[string]any, error) {
	config, err := decodeWorkflowNodeData[textNodeConfig](node.Data)
	if err != nil {
		return workflowNodeValue{}, "", nil, err
	}
	inputs, inputErr := e.collect(node.ID, "text")
	if inputErr != nil {
		return workflowNodeValue{}, "", nil, inputErr
	}
	textParts := append([]string{}, inputs.texts...)
	textParts = append(textParts, firstNonEmpty(strings.TrimSpace(config.Text), strings.TrimSpace(config.Prompt)))
	text := strings.Join(orderedWorkflowStrings(textParts), "\n\n")
	if text == "" {
		return workflowNodeValue{}, "", nil, errors.New("text node requires text content or upstream text input")
	}
	return workflowNodeValue{Prompt: text, Text: text, Items: append([]models.AgentWorkflowItem{}, inputs.items...)}, "Text ready", map[string]any{
		"chars":        len(text),
		"input_count":  len(inputs.texts),
		"item_count":   len(inputs.items),
		"local_chars":  len(strings.TrimSpace(firstNonEmpty(config.Text, config.Prompt))),
	}, nil
}

func (e *workflowExecutor) executeSearchNode(node models.AgentWorkflowNode) (workflowNodeValue, string, map[string]any, error) {
	config, err := decodeWorkflowNodeData[searchNodeConfig](node.Data)
	if err != nil {
		return workflowNodeValue{}, "", nil, err
	}
	query := strings.TrimSpace(config.Query)
	if query == "" {
		return workflowNodeValue{}, "", nil, errors.New("search provider node requires query")
	}
	result, runErr := e.service.RunSearch(e.ctx, models.AgentSearchRequest{
		EngineSelector: strings.TrimSpace(config.ProviderID),
		MaxItems:       maxInt(config.MaxItems, 8),
		LogContext:     "workflow:" + e.workflow.ID,
		Plans: []models.AgentSearchPlan{{
			Label:   node.Label,
			Query:   query,
			Recency: strings.TrimSpace(config.Recency),
			Sites:   append([]string{}, config.Sites...),
		}},
	})
	if runErr != nil {
		return workflowNodeValue{}, "", nil, runErr
	}
	return workflowNodeValue{
			Search: &result,
			Text:   workflowSearchResultText(result),
		}, fmt.Sprintf("%d search results", len(result.Items)), map[string]any{
			"item_count":  len(result.Items),
			"error_count": len(result.Errors),
		}, nil
}

func (e *workflowExecutor) executeLLMNode(node models.AgentWorkflowNode) (workflowNodeValue, string, map[string]any, error) {
	config, err := decodeWorkflowNodeData[llmNodeConfig](node.Data)
	if err != nil {
		return workflowNodeValue{}, "", nil, err
	}
	promptInputs, promptErr := e.collect(node.ID, "prompt")
	if promptErr != nil {
		return workflowNodeValue{}, "", nil, promptErr
	}
	contextInputs, contextErr := e.collect(node.ID, "context")
	if contextErr != nil {
		return workflowNodeValue{}, "", nil, contextErr
	}
	searchInputs, searchErr := e.collect(node.ID, "search")
	if searchErr != nil {
		return workflowNodeValue{}, "", nil, searchErr
	}
	if len(e.incomingByHandle(node.ID, "tool")) > 0 {
		return workflowNodeValue{}, "", nil, errors.New("llm tool handle is reserved but not executable yet")
	}
	if len(e.incomingByHandle(node.ID, "skill")) > 0 {
		return workflowNodeValue{}, "", nil, errors.New("llm skill handle is reserved but not executable yet")
	}
	promptText := strings.Join(orderedWorkflowStrings(promptInputs.prompts), "\n\n")
	contextTexts := append([]string{}, contextInputs.texts...)
	promptValues := workflowStringSet(promptInputs.prompts)
	for _, text := range promptInputs.texts {
		trimmed := strings.TrimSpace(text)
		if trimmed == "" {
			continue
		}
		if _, ok := promptValues[trimmed]; ok {
			continue
		}
		contextTexts = append(contextTexts, trimmed)
	}
	contextText := strings.Join(orderedWorkflowStrings(contextTexts), "\n\n")
	searchText := strings.Join(orderedWorkflowStrings(searchInputs.searches), "\n\n")
	finalPrompt := buildWorkflowLLMPrompt(promptText, strings.TrimSpace(config.UserPrompt), contextText, searchText)
	if strings.TrimSpace(finalPrompt) == "" {
		return workflowNodeValue{}, "", nil, errors.New("llm node has no promptable input")
	}
	output, genErr := e.service.GenerateTextWithProvider(e.ctx, strings.TrimSpace(config.ProviderID), finalPrompt)
	if genErr != nil {
		return workflowNodeValue{}, "", nil, genErr
	}
	contextItems := append([]models.AgentWorkflowItem{}, contextInputs.items...)
	contextItems = append(contextItems, promptInputs.items...)
	return workflowNodeValue{
			Text:  output,
			Items: contextItems,
		}, fmt.Sprintf("Generated %d chars", len(output)), map[string]any{
			"chars": len(output),
			"items": len(contextItems),
		}, nil
}

func (e *workflowExecutor) executeWeComOutputNode(node models.AgentWorkflowNode) (workflowNodeValue, string, map[string]any, error) {
	config, err := decodeWorkflowNodeData[wecomOutputConfig](node.Data)
	if err != nil {
		return workflowNodeValue{}, "", nil, err
	}
	if e.service.workflowOutput == nil {
		return workflowNodeValue{}, "", nil, errors.New("wecom output runtime is not configured")
	}
	inputs, inputErr := e.collect(node.ID, "")
	if inputErr != nil {
		return workflowNodeValue{}, "", nil, inputErr
	}
	text := strings.Join(orderedWorkflowStrings(inputs.texts), "\n\n")
	if strings.TrimSpace(text) == "" {
		return workflowNodeValue{}, "", nil, errors.New("wecom output node requires text input")
	}
	toUser := strings.TrimSpace(config.ToUser)
	if toUser == "" {
		return workflowNodeValue{}, "", nil, errors.New("wecom output node requires to_user")
	}
	if sendErr := e.service.workflowOutput.SendWeComText(e.ctx, toUser, text); sendErr != nil {
		e.deliveryErrors = append(e.deliveryErrors, models.AgentRunError{Target: toUser, Error: sendErr.Error()})
		return workflowNodeValue{}, "", nil, sendErr
	}
	for _, item := range inputs.items {
		key := normalizeWorkflowURL(item.URL)
		if key == "" {
			continue
		}
		e.sentItems[key] = item
	}
	return workflowNodeValue{Text: text, Items: append([]models.AgentWorkflowItem{}, inputs.items...)}, "Delivered to WeCom", map[string]any{
		"to_user":    toUser,
		"text_chars": len(text),
		"item_count": len(inputs.items),
	}, nil
}

type collectedWorkflowInputs struct {
	prompts  []string
	texts    []string
	searches []string
	items    []models.AgentWorkflowItem
}

func (e *workflowExecutor) collect(nodeID string, targetHandle string) (collectedWorkflowInputs, error) {
	out := collectedWorkflowInputs{}
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
			out.searches = append(out.searches, workflowSearchResultText(*value.Search))
		}
		out.items = append(out.items, value.Items...)
	}
	return out, nil
}

func (e *workflowExecutor) incomingByHandle(nodeID string, targetHandle string) []models.AgentWorkflowEdge {
	edges := e.incoming[nodeID]
	if strings.TrimSpace(targetHandle) == "" {
		return append([]models.AgentWorkflowEdge{}, edges...)
	}
	filtered := make([]models.AgentWorkflowEdge, 0, len(edges))
	for _, edge := range edges {
		if strings.TrimSpace(edge.TargetHandle) == strings.TrimSpace(targetHandle) {
			filtered = append(filtered, edge)
		}
	}
	return filtered
}

func (e *workflowExecutor) groupChildren(groupID string) []models.AgentWorkflowNode {
	children := make([]models.AgentWorkflowNode, 0)
	for _, node := range e.workflow.Nodes {
		if node.ParentID == groupID {
			children = append(children, node)
		}
	}
	return children
}

func (e *workflowExecutor) recordFailure(nodeID string, err error) {
	node := e.nodes[nodeID]
	e.nodeResults[nodeID] = models.AgentWorkflowNodeResult{
		NodeID:   nodeID,
		NodeType: node.Type,
		Status:   "failed",
		Summary:  err.Error(),
	}
}

func (e *workflowExecutor) results() []models.AgentWorkflowNodeResult {
	results := make([]models.AgentWorkflowNodeResult, 0, len(e.nodeResults))
	for _, node := range e.workflow.Nodes {
		if result, ok := e.nodeResults[node.ID]; ok {
			results = append(results, result)
		}
	}
	return results
}

func (e *workflowExecutor) sentItemList() []models.AgentWorkflowItem {
	items := make([]models.AgentWorkflowItem, 0, len(e.sentItems))
	for _, item := range e.sentItems {
		items = append(items, item)
	}
	return items
}
