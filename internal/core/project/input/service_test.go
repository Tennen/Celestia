package input

import (
	"context"
	"testing"
	"time"

	"github.com/chentianyu/celestia/internal/models"
)

type fakeInputAgent struct {
	recordedResponse string
}

func (f *fakeInputAgent) Converse(context.Context, models.AgentConversationRequest) (models.AgentConversation, error) {
	return models.AgentConversation{}, nil
}

func (f *fakeInputAgent) RecordConversationResult(_ context.Context, _ models.AgentConversationRequest, _ string, response string, status string, _ map[string]any) (models.AgentConversation, error) {
	f.recordedResponse = response
	return models.AgentConversation{ID: "conv", Response: response, Status: status}, nil
}

type fakeImageSlash struct{}

func (fakeImageSlash) Run(context.Context, models.ProjectInputRequest) (models.SlashCommandResult, bool, error) {
	return models.SlashCommandResult{
		Command: "kb",
		Output:  "text must not be returned",
		Images:  []models.ProjectOutputImage{{Path: "/tmp/answer.png", ContentType: "image/png"}},
		Metadata: map[string]any{
			"reply_kind": "image",
		},
		ExecutedAt: time.Now().UTC(),
	}, true, nil
}

func TestHandleInputSuppressesTextForImageSlashResult(t *testing.T) {
	agent := &fakeInputAgent{}
	svc := New(agent, fakeImageSlash{})

	result, err := svc.HandleInput(context.Background(), models.ProjectInputRequest{Input: "/kb ask x"})
	if err != nil {
		t.Fatalf("HandleInput() error = %v", err)
	}
	if result.ResponseText != "" || agent.recordedResponse != "" {
		t.Fatalf("response text = %q recorded = %q, want both empty", result.ResponseText, agent.recordedResponse)
	}
	if len(result.Images) != 1 {
		t.Fatalf("images = %+v, want one image", result.Images)
	}
}
