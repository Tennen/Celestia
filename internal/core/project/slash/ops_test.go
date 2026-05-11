package slash

import (
	"context"
	"strings"
	"testing"

	"github.com/chentianyu/celestia/internal/models"
)

func TestRunEvolutionQueueDispatchesAgentRuntime(t *testing.T) {
	ctx := context.Background()
	agent := &fakeAgentRuntime{}
	svc := New(nil, agent)

	result, handled, err := svc.Run(ctx, models.ProjectInputRequest{Input: `/evolution queue ship remote ops commit_message=remote-ops`})
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if !handled {
		t.Fatal("Run() handled = false, want true")
	}
	if len(agent.evolutionGoals) != 1 || agent.evolutionGoals[0].Goal != "ship remote ops" {
		t.Fatalf("evolution goals = %+v", agent.evolutionGoals)
	}
	if result.Metadata["goal_id"] != "goal-1" {
		t.Fatalf("metadata goal_id = %#v, want goal-1", result.Metadata["goal_id"])
	}
}

func TestRunScreenshotReturnsImage(t *testing.T) {
	ctx := context.Background()
	agent := &fakeAgentRuntime{}
	svc := New(nil, agent)

	result, handled, err := svc.Run(ctx, models.ProjectInputRequest{Input: `/screenshot http://localhost:3000 width=390 height=844 full_page=true`})
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if !handled {
		t.Fatal("Run() handled = false, want true")
	}
	if len(agent.screenshots) != 1 || agent.screenshots[0].Width != 390 || !agent.screenshots[0].FullPage {
		t.Fatalf("screenshots = %+v", agent.screenshots)
	}
	if len(result.Images) != 1 || result.Images[0].Path != "/tmp/screenshot.png" {
		t.Fatalf("Run() images = %+v, want screenshot image", result.Images)
	}
}

func TestRunServiceLogsDispatchesServiceOperation(t *testing.T) {
	ctx := context.Background()
	agent := &fakeAgentRuntime{}
	svc := New(nil, agent)

	result, handled, err := svc.Run(ctx, models.ProjectInputRequest{Input: `/service logs lines=50`})
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if !handled {
		t.Fatal("Run() handled = false, want true")
	}
	if len(agent.serviceOps) != 1 || agent.serviceOps[0].Action != "logs" || agent.serviceOps[0].Lines != 50 {
		t.Fatalf("service ops = %+v", agent.serviceOps)
	}
	if !strings.Contains(result.Output, "running pid=123") {
		t.Fatalf("Run() output = %q, want service output", result.Output)
	}
}
