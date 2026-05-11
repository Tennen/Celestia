package slash

import (
	"context"
	"testing"

	"github.com/chentianyu/celestia/internal/models"
)

func TestRunKnowledgeNewWithQuestionForcesFreshCodexSession(t *testing.T) {
	ctx := context.Background()
	agent := &fakeAgentRuntime{}
	svc := New(nil, agent)

	_, handled, err := svc.Run(ctx, models.ProjectInputRequest{Input: `/kb new reset context`, SessionID: "alice"})
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if !handled {
		t.Fatal("Run() handled = false, want true")
	}
	if len(agent.knowledgeReqs) != 1 {
		t.Fatalf("knowledge reqs = %d, want 1", len(agent.knowledgeReqs))
	}
	if !agent.knowledgeReqs[0].NewSession || agent.knowledgeReqs[0].Question != "reset context" {
		t.Fatalf("knowledge req = %+v", agent.knowledgeReqs[0])
	}
}
