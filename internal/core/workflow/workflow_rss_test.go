package workflow

import (
	"context"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/chentianyu/celestia/internal/models"
)

type workflowFeedResponse struct {
	status       int
	lastModified string
	body         string
}

type workflowFeedTransport struct {
	t         *testing.T
	responses []workflowFeedResponse
	requests  []*http.Request
}

func (t *workflowFeedTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	if req.URL.Host != "rss.test" {
		t.t.Fatalf("unexpected request host %q", req.URL.Host)
	}
	if len(t.responses) == 0 {
		t.t.Fatalf("unexpected RSS request for %s", req.URL.String())
	}
	t.requests = append(t.requests, req.Clone(req.Context()))
	next := t.responses[0]
	t.responses = t.responses[1:]
	resp := response(next.status, "application/xml", next.body)
	if next.lastModified != "" {
		resp.Header.Set("Last-Modified", next.lastModified)
	}
	return resp, nil
}

func TestRunWorkflowRSSReturnsOnlyNewGUIDItems(t *testing.T) {
	ctx := context.Background()
	svc, _ := newWorkflowPersistenceTestService(t)
	workflow := workflowRSSOnlyDefinition()
	if _, err := svc.SaveWorkflow(ctx, models.AgentWorkflowSnapshot{
		ActiveWorkflowID: workflow.ID,
		Workflows:        []models.AgentWorkflow{workflow},
	}); err != nil {
		t.Fatalf("SaveWorkflow() error = %v", err)
	}

	transport := &workflowFeedTransport{
		t: t,
		responses: []workflowFeedResponse{
			{
				status:       http.StatusOK,
				lastModified: "Tue, 28 Apr 2026 10:00:00 GMT",
				body: `<?xml version="1.0" encoding="UTF-8"?>
<rss><channel>
  <item>
    <title>Alpha</title>
    <link>https://example.com/topic?id=1</link>
    <guid>guid-alpha</guid>
    <description>Initial item.</description>
  </item>
</channel></rss>`,
			},
			{
				status:       http.StatusOK,
				lastModified: "Tue, 28 Apr 2026 10:05:00 GMT",
				body: `<?xml version="1.0" encoding="UTF-8"?>
<rss><channel>
  <item>
    <title>Alpha Updated URL</title>
    <link>https://example.com/topic?id=1&amp;rev=2</link>
    <guid>guid-alpha</guid>
    <description>Same guid, changed link.</description>
  </item>
  <item>
    <title>Beta</title>
    <link>https://example.com/topic?id=2</link>
    <guid>guid-beta</guid>
    <description>New item.</description>
  </item>
</channel></rss>`,
			},
		},
	}
	previousTransport := http.DefaultClient.Transport
	http.DefaultClient.Transport = transport
	defer func() {
		http.DefaultClient.Transport = previousTransport
	}()

	firstRun, err := svc.RunWorkflow(ctx, workflow.ID)
	if err != nil {
		t.Fatalf("RunWorkflow() first error = %v", err)
	}
	if len(firstRun.Items) != 1 {
		t.Fatalf("first run items = %d, want 1", len(firstRun.Items))
	}
	if guid := firstRun.Items[0].GUID; guid != "guid-alpha" {
		t.Fatalf("first run guid = %q, want guid-alpha", guid)
	}

	secondRun, err := svc.RunWorkflow(ctx, workflow.ID)
	if err != nil {
		t.Fatalf("RunWorkflow() second error = %v", err)
	}
	if len(secondRun.Items) != 1 {
		t.Fatalf("second run items = %d, want 1", len(secondRun.Items))
	}
	if guid := secondRun.Items[0].GUID; guid != "guid-beta" {
		t.Fatalf("second run guid = %q, want guid-beta", guid)
	}
	if len(transport.requests) != 2 {
		t.Fatalf("RSS requests = %d, want 2", len(transport.requests))
	}
	if got := transport.requests[1].Header.Get("If-Modified-Since"); got != "Tue, 28 Apr 2026 10:00:00 GMT" {
		t.Fatalf("If-Modified-Since = %q, want first Last-Modified header", got)
	}

	snapshot, err := svc.Snapshot(ctx)
	if err != nil {
		t.Fatalf("Snapshot() error = %v", err)
	}
	if len(snapshot.Workflow.SourceStates) != 1 {
		t.Fatalf("source states = %d, want 1", len(snapshot.Workflow.SourceStates))
	}
	if !strings.Contains(snapshot.Workflow.SourceStates[0].LastResponseBody, "guid-beta") {
		t.Fatalf("last response body was not updated: %q", snapshot.Workflow.SourceStates[0].LastResponseBody)
	}
}

func TestRunWorkflowRSSHandlesNotModifiedResponses(t *testing.T) {
	ctx := context.Background()
	svc, _ := newWorkflowPersistenceTestService(t)
	workflow := workflowRSSOnlyDefinition()
	if _, err := svc.SaveWorkflow(ctx, models.AgentWorkflowSnapshot{
		ActiveWorkflowID: workflow.ID,
		Workflows:        []models.AgentWorkflow{workflow},
	}); err != nil {
		t.Fatalf("SaveWorkflow() error = %v", err)
	}
	seedRequestedAt := time.Now().UTC().Add(-2 * time.Hour)
	seedBody := `<?xml version="1.0" encoding="UTF-8"?><rss><channel><item><title>Alpha</title><link>https://example.com/topic?id=1</link><guid>guid-alpha</guid></item></channel></rss>`
	if _, err := svc.update(ctx, func(snapshot *models.AgentSnapshot) error {
		snapshot.Workflow.SourceStates = []models.AgentWorkflowSourceState{{
			WorkflowID:       workflow.ID,
			NodeID:           "rss-main",
			SourceID:         "feed-main",
			FeedURL:          "https://rss.test/feed",
			LastRequestedAt:  seedRequestedAt,
			LastModified:     "Tue, 28 Apr 2026 10:00:00 GMT",
			LastResponseBody: seedBody,
			UpdatedAt:        seedRequestedAt,
		}}
		return nil
	}); err != nil {
		t.Fatalf("update() error = %v", err)
	}

	transport := &workflowFeedTransport{
		t: t,
		responses: []workflowFeedResponse{{
			status:       http.StatusNotModified,
			lastModified: "Tue, 28 Apr 2026 10:00:00 GMT",
		}},
	}
	previousTransport := http.DefaultClient.Transport
	http.DefaultClient.Transport = transport
	defer func() {
		http.DefaultClient.Transport = previousTransport
	}()

	run, err := svc.RunWorkflow(ctx, workflow.ID)
	if err != nil {
		t.Fatalf("RunWorkflow() error = %v", err)
	}
	if len(run.Items) != 0 {
		t.Fatalf("run items = %d, want 0", len(run.Items))
	}
	if len(transport.requests) != 1 {
		t.Fatalf("RSS requests = %d, want 1", len(transport.requests))
	}
	if got := transport.requests[0].Header.Get("If-Modified-Since"); got != "Tue, 28 Apr 2026 10:00:00 GMT" {
		t.Fatalf("If-Modified-Since = %q, want stored Last-Modified", got)
	}
	if got := workflowResultMetadataInt(t, run, "rss-main", "not_modified_count"); got != 1 {
		t.Fatalf("not_modified_count = %d, want 1", got)
	}

	snapshot, err := svc.Snapshot(ctx)
	if err != nil {
		t.Fatalf("Snapshot() error = %v", err)
	}
	if len(snapshot.Workflow.SourceStates) != 1 {
		t.Fatalf("source states = %d, want 1", len(snapshot.Workflow.SourceStates))
	}
	state := snapshot.Workflow.SourceStates[0]
	if state.LastResponseBody != seedBody {
		t.Fatalf("last response body changed on 304: %q", state.LastResponseBody)
	}
	if !state.LastRequestedAt.After(seedRequestedAt) {
		t.Fatalf("last requested at = %s, want after %s", state.LastRequestedAt, seedRequestedAt)
	}
}

func TestRunWorkflowManualDoesNotActivateTimerNode(t *testing.T) {
	ctx := context.Background()
	svc, _ := newWorkflowPersistenceTestService(t)
	workflow := workflowTimerRSSDefinition("23:59", "UTC")
	if _, err := svc.SaveWorkflow(ctx, models.AgentWorkflowSnapshot{
		ActiveWorkflowID: workflow.ID,
		Workflows:        []models.AgentWorkflow{workflow},
	}); err != nil {
		t.Fatalf("SaveWorkflow() error = %v", err)
	}

	transport := &workflowFeedTransport{
		t: t,
		responses: []workflowFeedResponse{{
			status:       http.StatusOK,
			lastModified: "Tue, 28 Apr 2026 10:00:00 GMT",
			body:         `<?xml version="1.0" encoding="UTF-8"?><rss><channel><item><title>Alpha</title><link>https://example.com/topic?id=1</link><guid>guid-alpha</guid></item></channel></rss>`,
		}},
	}
	previousTransport := http.DefaultClient.Transport
	http.DefaultClient.Transport = transport
	defer func() {
		http.DefaultClient.Transport = previousTransport
	}()

	run, err := svc.RunWorkflow(ctx, workflow.ID)
	if err != nil {
		t.Fatalf("RunWorkflow() error = %v", err)
	}
	if len(run.Items) != 0 {
		t.Fatalf("run items = %d, want 0", len(run.Items))
	}
	if len(transport.requests) != 0 {
		t.Fatalf("RSS requests = %d, want 0", len(transport.requests))
	}
}

func TestWorkflowTimeSchedulerTriggersTimerConnectedRSS(t *testing.T) {
	ctx := context.Background()
	svc, _ := newWorkflowPersistenceTestService(t)
	now := time.Date(2026, 4, 28, 8, 30, 0, 0, time.UTC)
	workflow := workflowTimerRSSDefinition("08:30", "UTC")
	if _, err := svc.SaveWorkflow(ctx, models.AgentWorkflowSnapshot{
		ActiveWorkflowID: workflow.ID,
		Workflows:        []models.AgentWorkflow{workflow},
	}); err != nil {
		t.Fatalf("SaveWorkflow() error = %v", err)
	}

	transport := &workflowFeedTransport{
		t: t,
		responses: []workflowFeedResponse{{
			status:       http.StatusOK,
			lastModified: "Tue, 28 Apr 2026 08:30:00 GMT",
			body:         `<?xml version="1.0" encoding="UTF-8"?><rss><channel><item><title>Alpha</title><link>https://example.com/topic?id=1</link><guid>guid-alpha</guid></item></channel></rss>`,
		}},
	}
	previousTransport := http.DefaultClient.Transport
	http.DefaultClient.Transport = transport
	defer func() {
		http.DefaultClient.Transport = previousTransport
	}()

	svc.handleWorkflowTimeTick(now)
	waitForWorkflowTestCondition(t, 2*time.Second, "expected timer-connected RSS run to complete", func() bool {
		return len(transport.requests) == 1
	})
	if len(transport.requests) != 1 {
		t.Fatalf("RSS requests after first tick = %d, want 1", len(transport.requests))
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
	if !snapshot.Workflow.TimerStates[0].LastTriggeredAt.Equal(now) {
		t.Fatalf("last triggered at = %s, want %s", snapshot.Workflow.TimerStates[0].LastTriggeredAt, now)
	}

	svc.handleWorkflowTimeTick(now.Add(20 * time.Second))
	if len(transport.requests) != 1 {
		t.Fatalf("RSS requests after duplicate tick = %d, want still 1", len(transport.requests))
	}
	updated, err := svc.Snapshot(ctx)
	if err != nil {
		t.Fatalf("Snapshot() second error = %v", err)
	}
	if len(updated.Workflow.Runs) != 1 {
		t.Fatalf("workflow runs after duplicate tick = %d, want 1", len(updated.Workflow.Runs))
	}
}

func TestWorkflowTimeSchedulerTriggersIntervalTimerWithinWindow(t *testing.T) {
	ctx := context.Background()
	svc, _ := newWorkflowPersistenceTestService(t)
	workflow := workflowIntervalTimerRSSDefinition("08:00", "18:00", 600, "UTC")
	if _, err := svc.SaveWorkflow(ctx, models.AgentWorkflowSnapshot{
		ActiveWorkflowID: workflow.ID,
		Workflows:        []models.AgentWorkflow{workflow},
	}); err != nil {
		t.Fatalf("SaveWorkflow() error = %v", err)
	}

	transport := &workflowFeedTransport{
		t: t,
		responses: []workflowFeedResponse{
			{
				status:       http.StatusOK,
				lastModified: "Tue, 28 Apr 2026 08:00:00 GMT",
				body:         `<?xml version="1.0" encoding="UTF-8"?><rss><channel><item><title>Alpha</title><link>https://example.com/topic?id=1</link><guid>guid-alpha</guid></item></channel></rss>`,
			},
			{
				status:       http.StatusOK,
				lastModified: "Tue, 28 Apr 2026 08:10:00 GMT",
				body:         `<?xml version="1.0" encoding="UTF-8"?><rss><channel><item><title>Beta</title><link>https://example.com/topic?id=2</link><guid>guid-beta</guid></item></channel></rss>`,
			},
		},
	}
	previousTransport := http.DefaultClient.Transport
	http.DefaultClient.Transport = transport
	defer func() {
		http.DefaultClient.Transport = previousTransport
	}()

	windowStart := time.Date(2026, 4, 28, 8, 0, 0, 0, time.UTC)
	svc.handleWorkflowTimeTick(windowStart)
	waitForWorkflowTestCondition(t, 2*time.Second, "expected first interval RSS request to complete", func() bool {
		return len(transport.requests) == 1
	})
	if len(transport.requests) != 1 {
		t.Fatalf("RSS requests after window start = %d, want 1", len(transport.requests))
	}

	svc.handleWorkflowTimeTick(windowStart.Add(5 * time.Minute))
	if len(transport.requests) != 1 {
		t.Fatalf("RSS requests inside same slot = %d, want still 1", len(transport.requests))
	}

	nextSlot := windowStart.Add(10 * time.Minute)
	svc.handleWorkflowTimeTick(nextSlot)
	waitForWorkflowTestCondition(t, 2*time.Second, "expected second interval RSS request to complete", func() bool {
		return len(transport.requests) == 2
	})
	if len(transport.requests) != 2 {
		t.Fatalf("RSS requests after next slot = %d, want 2", len(transport.requests))
	}

	svc.handleWorkflowTimeTick(time.Date(2026, 4, 28, 18, 0, 0, 0, time.UTC))
	if len(transport.requests) != 2 {
		t.Fatalf("RSS requests at window end = %d, want still 2", len(transport.requests))
	}

	snapshot, err := svc.Snapshot(ctx)
	if err != nil {
		t.Fatalf("Snapshot() error = %v", err)
	}
	if len(snapshot.Workflow.Runs) != 2 {
		t.Fatalf("workflow runs = %d, want 2", len(snapshot.Workflow.Runs))
	}
	if len(snapshot.Workflow.TimerStates) != 1 {
		t.Fatalf("timer states = %d, want 1", len(snapshot.Workflow.TimerStates))
	}
	if !snapshot.Workflow.TimerStates[0].LastTriggeredAt.Equal(nextSlot) {
		t.Fatalf("last triggered at = %s, want %s", snapshot.Workflow.TimerStates[0].LastTriggeredAt, nextSlot)
	}
}
