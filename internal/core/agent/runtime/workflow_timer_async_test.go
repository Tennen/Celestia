package runtime

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/chentianyu/celestia/internal/models"
)

type sharedLLMWorkflowTransport struct {
	t               *testing.T
	mu              sync.Mutex
	llmBodies       []string
	feedBodiesByURL map[string]string
	llmStarted      chan struct{}
	releaseFirstLLM chan struct{}
}

func (t *sharedLLMWorkflowTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	switch req.URL.Host {
	case "rss.test":
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

func TestRunWorkflowAggregatesSharedLLMWithoutTimer(t *testing.T) {
	ctx := context.Background()
	svc, _ := newAgentPersistenceTestService(t)
	output := &workflowTestOutput{}
	svc.SetWorkflowOutputRuntime(output)
	configureSharedLLMWorkflowProvider(t, ctx, svc)

	transport := &sharedLLMWorkflowTransport{
		t: t,
		feedBodiesByURL: map[string]string{
			"https://rss.test/feed-a": workflowRSSBody("Alpha headline", "guid-alpha", "https://example.com/a"),
			"https://rss.test/feed-b": workflowRSSBody("Beta headline", "guid-beta", "https://example.com/b"),
		},
	}
	previousTransport := http.DefaultClient.Transport
	http.DefaultClient.Transport = transport
	defer func() {
		http.DefaultClient.Transport = previousTransport
	}()

	workflow := sharedLLMWorkflowDefinition(false)
	if _, err := svc.SaveWorkflow(ctx, models.AgentWorkflowSnapshot{
		ActiveWorkflowID: workflow.ID,
		Workflows:        []models.AgentWorkflow{workflow},
	}); err != nil {
		t.Fatalf("SaveWorkflow() error = %v", err)
	}

	run, err := svc.RunWorkflow(ctx, workflow.ID)
	if err != nil {
		t.Fatalf("RunWorkflow() error = %v", err)
	}
	if run.Status != "succeeded" {
		t.Fatalf("run status = %q, want succeeded", run.Status)
	}
	llmBodies := transport.llmBodiesSnapshot()
	if len(llmBodies) != 1 {
		t.Fatalf("LLM requests = %d, want 1", len(llmBodies))
	}
	if !strings.Contains(llmBodies[0], "Alpha headline") || !strings.Contains(llmBodies[0], "Beta headline") {
		t.Fatalf("aggregated LLM body missing RSS inputs: %s", llmBodies[0])
	}
	if len(output.messages) != 1 {
		t.Fatalf("wecom messages = %d, want 1", len(output.messages))
	}
}

func TestWorkflowTimeSchedulerQueuesSharedLLMPerTimer(t *testing.T) {
	ctx := context.Background()
	svc, _ := newAgentPersistenceTestService(t)
	output := &workflowTestOutput{}
	svc.SetWorkflowOutputRuntime(output)
	configureSharedLLMWorkflowProvider(t, ctx, svc)

	transport := &sharedLLMWorkflowTransport{
		t: t,
		feedBodiesByURL: map[string]string{
			"https://rss.test/feed-a": workflowRSSBody("Alpha headline", "guid-alpha", "https://example.com/a"),
			"https://rss.test/feed-b": workflowRSSBody("Beta headline", "guid-beta", "https://example.com/b"),
		},
	}
	previousTransport := http.DefaultClient.Transport
	http.DefaultClient.Transport = transport
	defer func() {
		http.DefaultClient.Transport = previousTransport
	}()

	workflow := sharedLLMWorkflowDefinition(true)
	if _, err := svc.SaveWorkflow(ctx, models.AgentWorkflowSnapshot{
		ActiveWorkflowID: workflow.ID,
		Workflows:        []models.AgentWorkflow{workflow},
	}); err != nil {
		t.Fatalf("SaveWorkflow() error = %v", err)
	}

	now := time.Date(2026, 4, 28, 8, 30, 0, 0, time.UTC)
	svc.handleWorkflowTimeTick(now)

	llmBodies := transport.llmBodiesSnapshot()
	if len(llmBodies) != 2 {
		t.Fatalf("LLM requests = %d, want 2", len(llmBodies))
	}
	if !strings.Contains(llmBodies[0], "Alpha headline") || strings.Contains(llmBodies[0], "Beta headline") {
		t.Fatalf("first LLM body should contain only timer A RSS input: %s", llmBodies[0])
	}
	if !strings.Contains(llmBodies[1], "Beta headline") || strings.Contains(llmBodies[1], "Alpha headline") {
		t.Fatalf("second LLM body should contain only timer B RSS input: %s", llmBodies[1])
	}
	if len(output.messages) != 2 {
		t.Fatalf("wecom messages = %d, want 2", len(output.messages))
	}

	snapshot, err := svc.Snapshot(ctx)
	if err != nil {
		t.Fatalf("Snapshot() error = %v", err)
	}
	if len(snapshot.Workflow.Runs) != 2 {
		t.Fatalf("workflow runs = %d, want 2", len(snapshot.Workflow.Runs))
	}
	if len(snapshot.Workflow.TimerStates) != 2 {
		t.Fatalf("timer states = %d, want 2", len(snapshot.Workflow.TimerStates))
	}
}

func TestRunWorkflowDoesNotActivateTimerDrivenSharedLLM(t *testing.T) {
	ctx := context.Background()
	svc, _ := newAgentPersistenceTestService(t)
	output := &workflowTestOutput{}
	svc.SetWorkflowOutputRuntime(output)
	configureSharedLLMWorkflowProvider(t, ctx, svc)

	transport := &sharedLLMWorkflowTransport{
		t: t,
		feedBodiesByURL: map[string]string{
			"https://rss.test/feed-a": workflowRSSBody("Alpha headline", "guid-alpha", "https://example.com/a"),
			"https://rss.test/feed-b": workflowRSSBody("Beta headline", "guid-beta", "https://example.com/b"),
		},
	}
	previousTransport := http.DefaultClient.Transport
	http.DefaultClient.Transport = transport
	defer func() {
		http.DefaultClient.Transport = previousTransport
	}()

	workflow := sharedLLMWorkflowDefinition(true)
	if _, err := svc.SaveWorkflow(ctx, models.AgentWorkflowSnapshot{
		ActiveWorkflowID: workflow.ID,
		Workflows:        []models.AgentWorkflow{workflow},
	}); err != nil {
		t.Fatalf("SaveWorkflow() error = %v", err)
	}

	run, err := svc.RunWorkflow(ctx, workflow.ID)
	if err != nil {
		t.Fatalf("RunWorkflow() error = %v", err)
	}
	if run.Status != "succeeded" {
		t.Fatalf("run status = %q, want succeeded", run.Status)
	}
	if llmRequests := len(transport.llmBodiesSnapshot()); llmRequests != 0 {
		t.Fatalf("LLM requests = %d, want 0", llmRequests)
	}
	if len(output.messages) != 0 {
		t.Fatalf("wecom messages = %d, want 0", len(output.messages))
	}
}

func TestWorkflowTimeSchedulerTriggersOnlyMatchingTimerForSharedLLM(t *testing.T) {
	ctx := context.Background()
	svc, _ := newAgentPersistenceTestService(t)
	output := &workflowTestOutput{}
	svc.SetWorkflowOutputRuntime(output)
	configureSharedLLMWorkflowProvider(t, ctx, svc)

	transport := &sharedLLMWorkflowTransport{
		t: t,
		feedBodiesByURL: map[string]string{
			"https://rss.test/feed-a": workflowRSSBody("Alpha headline", "guid-alpha", "https://example.com/a"),
			"https://rss.test/feed-b": workflowRSSBody("Beta headline", "guid-beta", "https://example.com/b"),
		},
	}
	previousTransport := http.DefaultClient.Transport
	http.DefaultClient.Transport = transport
	defer func() {
		http.DefaultClient.Transport = previousTransport
	}()

	workflow := sharedLLMWorkflowDefinitionWithTimerTimes("08:30", "18:00")
	if _, err := svc.SaveWorkflow(ctx, models.AgentWorkflowSnapshot{
		ActiveWorkflowID: workflow.ID,
		Workflows:        []models.AgentWorkflow{workflow},
	}); err != nil {
		t.Fatalf("SaveWorkflow() error = %v", err)
	}

	now := time.Date(2026, 4, 28, 8, 30, 0, 0, time.UTC)
	svc.handleWorkflowTimeTick(now)

	llmBodies := transport.llmBodiesSnapshot()
	if len(llmBodies) != 1 {
		t.Fatalf("LLM requests = %d, want 1", len(llmBodies))
	}
	if !strings.Contains(llmBodies[0], "Alpha headline") || strings.Contains(llmBodies[0], "Beta headline") {
		t.Fatalf("LLM body should contain only timer A RSS input: %s", llmBodies[0])
	}
	if len(output.messages) != 1 {
		t.Fatalf("wecom messages = %d, want 1", len(output.messages))
	}

	snapshot, err := svc.Snapshot(ctx)
	if err != nil {
		t.Fatalf("Snapshot() error = %v", err)
	}
	if len(snapshot.Workflow.Runs) != 1 {
		t.Fatalf("workflow runs = %d, want 1", len(snapshot.Workflow.Runs))
	}
	if len(snapshot.Workflow.TimerStates) != 1 {
		t.Fatalf("timer states = %d, want 1", len(snapshot.Workflow.TimerStates))
	}
	if got := snapshot.Workflow.TimerStates[0].NodeID; got != "timer-a" {
		t.Fatalf("triggered timer node = %q, want timer-a", got)
	}
}

func TestWorkflowTimeSchedulerClaimsTimerBeforeExecutingRun(t *testing.T) {
	ctx := context.Background()
	svc, _ := newAgentPersistenceTestService(t)
	output := &workflowTestOutput{}
	svc.SetWorkflowOutputRuntime(output)
	configureSharedLLMWorkflowProvider(t, ctx, svc)

	transport := &sharedLLMWorkflowTransport{
		t:               t,
		llmStarted:      make(chan struct{}),
		releaseFirstLLM: make(chan struct{}),
		feedBodiesByURL: map[string]string{
			"https://rss.test/feed-a": workflowRSSBody("Alpha headline", "guid-alpha", "https://example.com/a"),
			"https://rss.test/feed-b": workflowRSSBody("Beta headline", "guid-beta", "https://example.com/b"),
		},
	}
	previousTransport := http.DefaultClient.Transport
	http.DefaultClient.Transport = transport
	defer func() {
		http.DefaultClient.Transport = previousTransport
	}()

	workflow := sharedLLMWorkflowDefinitionWithTimerTimes("08:30", "18:00")
	if _, err := svc.SaveWorkflow(ctx, models.AgentWorkflowSnapshot{
		ActiveWorkflowID: workflow.ID,
		Workflows:        []models.AgentWorkflow{workflow},
	}); err != nil {
		t.Fatalf("SaveWorkflow() error = %v", err)
	}

	now := time.Date(2026, 4, 28, 8, 30, 0, 0, time.UTC)
	done := make(chan struct{})
	go func() {
		defer close(done)
		svc.handleWorkflowTimeTick(now)
	}()

	<-transport.llmStarted
	svc.handleWorkflowTimeTick(now)

	if llmRequests := len(transport.llmBodiesSnapshot()); llmRequests != 1 {
		t.Fatalf("LLM requests before releasing first run = %d, want 1", llmRequests)
	}

	close(transport.releaseFirstLLM)
	<-done

	if llmRequests := len(transport.llmBodiesSnapshot()); llmRequests != 1 {
		t.Fatalf("LLM requests after first run completes = %d, want 1", llmRequests)
	}
	if len(output.messages) != 1 {
		t.Fatalf("wecom messages = %d, want 1", len(output.messages))
	}

	snapshot, err := svc.Snapshot(ctx)
	if err != nil {
		t.Fatalf("Snapshot() error = %v", err)
	}
	if len(snapshot.Workflow.Runs) != 1 {
		t.Fatalf("workflow runs = %d, want 1", len(snapshot.Workflow.Runs))
	}
}

func configureSharedLLMWorkflowProvider(t *testing.T, ctx context.Context, svc *Service) {
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
	if _, err := svc.SaveSettings(ctx, settings); err != nil {
		t.Fatalf("SaveSettings() error = %v", err)
	}
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
