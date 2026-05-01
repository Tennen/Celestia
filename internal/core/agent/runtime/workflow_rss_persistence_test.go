package runtime

import (
	"context"
	"net/http"
	"testing"

	"github.com/chentianyu/celestia/internal/models"
)

func TestRunWorkflowPersistsRSSStateWhenDownstreamFails(t *testing.T) {
	ctx := context.Background()
	svc, _ := newAgentPersistenceTestService(t)
	workflow := models.AgentWorkflow{
		ID:   "workflow-rss-degraded-state",
		Name: "RSS State On Failure",
		Nodes: []models.AgentWorkflowNode{
			{
				ID:       "rss-main",
				Type:     workflowNodeTypeRSSSources,
				Label:    "RSS Sources",
				Position: models.AgentNodePoint{X: 80, Y: 80},
				Data: map[string]any{"sources": []models.AgentWorkflowSource{{
					ID: "feed-main", Name: "Main Feed", Category: "news", FeedURL: "https://rss.test/feed", Weight: 1, Enabled: true,
				}}},
			},
			{
				ID:       "text-main",
				Type:     workflowNodeTypeText,
				Label:    "Text",
				Position: models.AgentNodePoint{X: 320, Y: 20},
				Data:     map[string]any{"text": "Summarize the RSS items."},
			},
			{
				ID:       "llm-main",
				Type:     workflowNodeTypeLLM,
				Label:    "LLM",
				Position: models.AgentNodePoint{X: 320, Y: 80},
				Data:     map[string]any{"provider_id": "missing-provider", "user_prompt": "Keep it brief."},
			},
		},
		Edges: []models.AgentWorkflowEdge{
			{ID: "edge-rss-llm", Source: "rss-main", SourceHandle: "content", Target: "llm-main", TargetHandle: "context"},
			{ID: "edge-text-llm", Source: "text-main", SourceHandle: "text", Target: "llm-main", TargetHandle: "prompt"},
		},
	}
	if _, err := svc.SaveWorkflow(ctx, models.AgentWorkflowSnapshot{
		ActiveWorkflowID: workflow.ID,
		Workflows:        []models.AgentWorkflow{workflow},
	}); err != nil {
		t.Fatalf("SaveWorkflow() error = %v", err)
	}

	body := `<?xml version="1.0" encoding="UTF-8"?>
<rss><channel>
  <item>
    <title>OpenAI News</title>
    <link>https://openai.com/index/sample-post</link>
    <guid isPermaLink="true">https://openai.com/index/sample-post</guid>
    <description>Repeatable RSS body without Last-Modified.</description>
    <pubDate>Wed, 29 Apr 2026 20:00:00 GMT</pubDate>
  </item>
</channel></rss>`
	transport := &workflowFeedTransport{
		t: t,
		responses: []workflowFeedResponse{
			{status: http.StatusOK, body: body},
			{status: http.StatusOK, body: body},
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
	if firstRun.Status != "degraded" {
		t.Fatalf("first run status = %q, want degraded", firstRun.Status)
	}
	if got := workflowResultMetadataInt(t, firstRun, "rss-main", "item_count"); got != 1 {
		t.Fatalf("first rss item_count = %d, want 1", got)
	}

	snapshot, err := svc.Snapshot(ctx)
	if err != nil {
		t.Fatalf("Snapshot() after first run error = %v", err)
	}
	if len(snapshot.Workflow.SourceStates) != 1 {
		t.Fatalf("source states after first run = %d, want 1", len(snapshot.Workflow.SourceStates))
	}
	if snapshot.Workflow.SourceStates[0].LastResponseBody == "" {
		t.Fatal("last response body should persist even when downstream nodes fail")
	}

	secondRun, err := svc.RunWorkflow(ctx, workflow.ID)
	if err != nil {
		t.Fatalf("RunWorkflow() second error = %v", err)
	}
	if secondRun.Status != "succeeded" {
		t.Fatalf("second run status = %q, want succeeded", secondRun.Status)
	}
	if got := workflowResultMetadataInt(t, secondRun, "rss-main", "item_count"); got != 0 {
		t.Fatalf("second rss item_count = %d, want 0", got)
	}
	if !workflowResultMetadataBool(t, secondRun, "rss-main", "blocked_by_upstream") {
		t.Fatal("second rss run should block downstream when no new items are available")
	}
	llmResult := workflowNodeResult(t, secondRun, "llm-main")
	if llmResult.Summary != "LLM waiting for upstream context" {
		t.Fatalf("llm summary = %q, want LLM waiting for upstream context", llmResult.Summary)
	}
	if len(transport.requests) != 2 {
		t.Fatalf("RSS requests = %d, want 2", len(transport.requests))
	}
	if got := transport.requests[1].Header.Get("If-Modified-Since"); got == "" {
		t.Fatal("second request should send If-Modified-Since based on the persisted request time")
	}
}
