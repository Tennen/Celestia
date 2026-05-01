package runtime

import (
	"context"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/chentianyu/celestia/internal/models"
)

func TestWorkflowTimeSchedulerEnqueuesLaterTimerWhileFirstRunIsExecuting(t *testing.T) {
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

	workflow := sharedLLMWorkflowDefinitionWithTimerTimes("08:30", "08:31")
	if _, err := svc.SaveWorkflow(ctx, models.AgentWorkflowSnapshot{
		ActiveWorkflowID: workflow.ID,
		Workflows:        []models.AgentWorkflow{workflow},
	}); err != nil {
		t.Fatalf("SaveWorkflow() error = %v", err)
	}

	firstReturned := make(chan struct{})
	go func() {
		defer close(firstReturned)
		svc.handleWorkflowTimeTick(time.Date(2026, 4, 28, 8, 30, 0, 0, time.UTC))
	}()

	select {
	case <-firstReturned:
	case <-time.After(250 * time.Millisecond):
		t.Fatal("first scheduler tick should return quickly after enqueueing the workflow run")
	}

	select {
	case <-transport.llmStarted:
	case <-time.After(2 * time.Second):
		t.Fatal("first scheduled workflow run did not reach the LLM node")
	}

	svc.handleWorkflowTimeTick(time.Date(2026, 4, 28, 8, 31, 0, 0, time.UTC))
	if llmRequests := len(transport.llmBodiesSnapshot()); llmRequests != 1 {
		t.Fatalf("LLM requests before releasing first run = %d, want 1", llmRequests)
	}

	close(transport.releaseFirstLLM)

	deadline := time.Now().Add(2 * time.Second)
	for len(transport.llmBodiesSnapshot()) < 2 && time.Now().Before(deadline) {
		time.Sleep(10 * time.Millisecond)
	}
	llmBodies := transport.llmBodiesSnapshot()
	if len(llmBodies) != 2 {
		t.Fatalf("LLM requests after releasing first run = %d, want 2", len(llmBodies))
	}
	if !strings.Contains(llmBodies[1], "Beta headline") || strings.Contains(llmBodies[1], "Alpha headline") {
		t.Fatalf("second LLM body should contain only timer B RSS input: %s", llmBodies[1])
	}

	deadline = time.Now().Add(2 * time.Second)
	for len(output.messages) < 2 && time.Now().Before(deadline) {
		time.Sleep(10 * time.Millisecond)
	}
	if len(output.messages) != 2 {
		t.Fatalf("wecom messages = %d, want 2", len(output.messages))
	}
}

func TestWorkflowTimeSchedulerDropsQueuedTimerFromStaleWorkflowDefinition(t *testing.T) {
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

	workflow := sharedLLMWorkflowDefinitionWithTimerTimes("08:30", "08:31")
	if _, err := svc.SaveWorkflow(ctx, models.AgentWorkflowSnapshot{
		ActiveWorkflowID: workflow.ID,
		Workflows:        []models.AgentWorkflow{workflow},
	}); err != nil {
		t.Fatalf("SaveWorkflow() error = %v", err)
	}

	svc.handleWorkflowTimeTick(time.Date(2026, 4, 28, 8, 30, 0, 0, time.UTC))
	select {
	case <-transport.llmStarted:
	case <-time.After(2 * time.Second):
		t.Fatal("first scheduled workflow run did not reach the LLM node")
	}

	svc.handleWorkflowTimeTick(time.Date(2026, 4, 28, 8, 31, 0, 0, time.UTC))
	workflowWithoutTimerB := sharedLLMWorkflowDefinitionWithSingleTimer()
	workflowWithoutTimerB.ID = workflow.ID
	if _, err := svc.SaveWorkflow(ctx, models.AgentWorkflowSnapshot{
		ActiveWorkflowID: workflowWithoutTimerB.ID,
		Workflows:        []models.AgentWorkflow{workflowWithoutTimerB},
	}); err != nil {
		t.Fatalf("SaveWorkflow() removing timer-b error = %v", err)
	}

	close(transport.releaseFirstLLM)
	waitForWorkflowTestCondition(t, 2*time.Second, "expected only the first queued workflow run to complete", func() bool {
		return len(output.messages) == 1
	})
	time.Sleep(200 * time.Millisecond)

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
}
