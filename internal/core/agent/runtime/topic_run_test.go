package runtime

import (
	"context"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/chentianyu/celestia/internal/models"
)

type capturedWeComMessage struct {
	toUser string
	text   string
}

type topicTestOutput struct {
	messages []capturedWeComMessage
}

func (t *topicTestOutput) SendWeComText(_ context.Context, toUser string, text string) error {
	t.messages = append(t.messages, capturedWeComMessage{toUser: toUser, text: text})
	return nil
}

type topicTestTransport struct {
	t *testing.T
}

func (t topicTestTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	switch req.URL.Host {
	case "rss.test":
		return response(http.StatusOK, "application/xml", `<?xml version="1.0" encoding="UTF-8"?>
<rss><channel>
  <item>
    <title>Celestia workflow launch</title>
    <link>https://example.com/topics/launch</link>
    <description>Workflow-driven topic summary is now available.</description>
  </item>
</channel></rss>`), nil
	case "llm.test":
		if got := req.Header.Get("Authorization"); got != "Bearer secret-token" {
			t.t.Fatalf("authorization header = %q, want bearer secret-token", got)
		}
		return response(http.StatusOK, "application/json", `{"choices":[{"message":{"role":"assistant","content":"Daily digest ready."}}]}`), nil
	default:
		t.t.Fatalf("unexpected request host %q", req.URL.Host)
		return nil, nil
	}
}

func TestRunTopicSummaryWorkflowExecutesRSSLLMAndWeComOutput(t *testing.T) {
	ctx := context.Background()
	svc, _ := newAgentPersistenceTestService(t)
	output := &topicTestOutput{}
	svc.SetTopicOutputRuntime(output)
	previousTransport := http.DefaultClient.Transport
	http.DefaultClient.Transport = topicTestTransport{t: t}
	defer func() {
		http.DefaultClient.Transport = previousTransport
	}()

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

	workflow := models.AgentTopicWorkflow{
		ID:   "topic-summary-workflow",
		Name: "Topic Summary Workflow",
		Nodes: []models.AgentTopicNode{
			{
				ID:    "rss-main",
				Type:  topicNodeTypeRSSSources,
				Label: "RSS Sources",
				Position: models.AgentNodePoint{
					X: 80,
					Y: 80,
				},
				Data: map[string]any{
					"sources": []models.AgentTopicSource{{
						ID:       "feed-main",
						Name:     "Main Feed",
						Category: "news",
						FeedURL:  "https://rss.test/feed",
						Weight:   1,
						Enabled:  true,
					}},
				},
			},
			{
				ID:    "prompt-main",
				Type:  topicNodeTypePromptUnit,
				Label: "Prompt Unit",
				Position: models.AgentNodePoint{
					X: 280,
					Y: 80,
				},
				Data: map[string]any{
					"prompt": "Summarize the incoming RSS items into a concise operator digest.",
				},
			},
			{
				ID:    "llm-main",
				Type:  topicNodeTypeLLM,
				Label: "LLM",
				Position: models.AgentNodePoint{
					X: 480,
					Y: 80,
				},
				Data: map[string]any{
					"provider_id": "topic-llm",
					"user_prompt": "Focus on why the update matters.",
				},
			},
			{
				ID:    "wecom-main",
				Type:  topicNodeTypeWeComOutput,
				Label: "WeCom Output",
				Position: models.AgentNodePoint{
					X: 700,
					Y: 80,
				},
				Data: map[string]any{
					"to_user": "alice",
				},
			},
		},
		Edges: []models.AgentTopicEdge{
			{ID: "edge-rss-llm", Source: "rss-main", SourceHandle: "content", Target: "llm-main", TargetHandle: "context"},
			{ID: "edge-prompt-llm", Source: "prompt-main", SourceHandle: "prompt", Target: "llm-main", TargetHandle: "prompt"},
			{ID: "edge-llm-wecom", Source: "llm-main", SourceHandle: "text", Target: "wecom-main", TargetHandle: "text"},
		},
	}
	if _, err := svc.SaveTopic(ctx, models.AgentTopicSnapshot{
		ActiveWorkflowID: workflow.ID,
		Workflows:        []models.AgentTopicWorkflow{workflow},
	}); err != nil {
		t.Fatalf("SaveTopic() error = %v", err)
	}

	run, err := svc.RunTopicSummary(ctx, workflow.ID)
	if err != nil {
		t.Fatalf("RunTopicSummary() error = %v", err)
	}
	if run.Status != "succeeded" {
		t.Fatalf("run status = %q, want succeeded", run.Status)
	}
	if len(run.Items) != 1 {
		t.Fatalf("run items = %d, want 1", len(run.Items))
	}
	if got := strings.TrimSpace(run.OutputText); got != "Daily digest ready." {
		t.Fatalf("run output = %q, want Daily digest ready.", got)
	}
	if len(output.messages) != 1 {
		t.Fatalf("wecom messages = %d, want 1", len(output.messages))
	}
	if output.messages[0].toUser != "alice" || strings.TrimSpace(output.messages[0].text) != "Daily digest ready." {
		t.Fatalf("wecom message = %+v", output.messages[0])
	}

	updated, err := svc.Snapshot(ctx)
	if err != nil {
		t.Fatalf("Snapshot() after run error = %v", err)
	}
	if len(updated.TopicSummary.SentLog) != 1 {
		t.Fatalf("sent log count = %d, want 1", len(updated.TopicSummary.SentLog))
	}
	if updated.TopicSummary.Runs[0].WorkflowID != workflow.ID {
		t.Fatalf("latest run workflow_id = %q, want %q", updated.TopicSummary.Runs[0].WorkflowID, workflow.ID)
	}
}

func response(status int, contentType string, body string) *http.Response {
	return &http.Response{
		StatusCode: status,
		Status:     http.StatusText(status),
		Header: http.Header{
			"Content-Type": []string{contentType},
		},
		Body: io.NopCloser(strings.NewReader(body)),
	}
}
