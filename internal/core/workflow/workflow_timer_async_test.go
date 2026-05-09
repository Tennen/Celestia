package workflow

import (
	"context"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/chentianyu/celestia/internal/models"
)

func TestRunWorkflowAggregatesSharedLLMWithoutTimer(t *testing.T) {
	ctx := context.Background()
	svc, store := newWorkflowPersistenceTestService(t)
	output := &workflowTestOutput{}
	svc.SetWorkflowOutputRuntime(output)
	configureSharedLLMWorkflowProvider(t, ctx, store, svc)

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
	svc, store := newWorkflowPersistenceTestService(t)
	output := &workflowTestOutput{}
	svc.SetWorkflowOutputRuntime(output)
	configureSharedLLMWorkflowProvider(t, ctx, store, svc)

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

	waitForWorkflowTestCondition(t, 2*time.Second, "expected two queued LLM runs to finish", func() bool {
		return len(transport.llmBodiesSnapshot()) == 2 && len(output.messages) == 2
	})
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
	svc, store := newWorkflowPersistenceTestService(t)
	output := &workflowTestOutput{}
	svc.SetWorkflowOutputRuntime(output)
	configureSharedLLMWorkflowProvider(t, ctx, store, svc)

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
	svc, store := newWorkflowPersistenceTestService(t)
	output := &workflowTestOutput{}
	svc.SetWorkflowOutputRuntime(output)
	configureSharedLLMWorkflowProvider(t, ctx, store, svc)

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

	waitForWorkflowTestCondition(t, 2*time.Second, "expected the matching timer to reach the shared LLM", func() bool {
		return len(transport.llmBodiesSnapshot()) == 1 && len(output.messages) == 1
	})
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

func TestWorkflowTimeSchedulerDoesNotPullTimerIndependentRSSIntoSharedLLM(t *testing.T) {
	ctx := context.Background()
	svc, store := newWorkflowPersistenceTestService(t)
	output := &workflowTestOutput{}
	svc.SetWorkflowOutputRuntime(output)
	configureSharedLLMWorkflowProvider(t, ctx, store, svc)

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

	workflow := sharedLLMWorkflowDefinitionWithSingleTimer()
	if _, err := svc.SaveWorkflow(ctx, models.AgentWorkflowSnapshot{
		ActiveWorkflowID: workflow.ID,
		Workflows:        []models.AgentWorkflow{workflow},
	}); err != nil {
		t.Fatalf("SaveWorkflow() error = %v", err)
	}

	svc.handleWorkflowTimeTick(time.Date(2026, 4, 28, 8, 30, 0, 0, time.UTC))
	waitForWorkflowTestCondition(t, 2*time.Second, "expected only the timer-connected RSS branch to finish", func() bool {
		return len(transport.llmBodiesSnapshot()) == 1 && len(output.messages) == 1
	})

	llmBodies := transport.llmBodiesSnapshot()
	if len(llmBodies) != 1 {
		t.Fatalf("LLM requests = %d, want 1", len(llmBodies))
	}
	if !strings.Contains(llmBodies[0], "Alpha headline") || strings.Contains(llmBodies[0], "Beta headline") {
		t.Fatalf("LLM body should contain only timer-connected RSS input: %s", llmBodies[0])
	}
	rssURLs := transport.rssURLsSnapshot()
	if len(rssURLs) != 1 {
		t.Fatalf("RSS requests = %d, want 1", len(rssURLs))
	}
	if rssURLs[0] != "https://rss.test/feed-a" {
		t.Fatalf("RSS request URL = %q, want only feed-a", rssURLs[0])
	}
}

func TestWorkflowTimeSchedulerClaimsTimerBeforeExecutingRun(t *testing.T) {
	ctx := context.Background()
	svc, store := newWorkflowPersistenceTestService(t)
	output := &workflowTestOutput{}
	svc.SetWorkflowOutputRuntime(output)
	configureSharedLLMWorkflowProvider(t, ctx, store, svc)

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
	waitForWorkflowTestCondition(t, 2*time.Second, "expected first queued workflow run to complete", func() bool {
		return len(transport.llmBodiesSnapshot()) == 1 && len(output.messages) == 1
	})

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
