package slash

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/chentianyu/celestia/internal/models"
)

func (s *Service) runWorkflow(ctx context.Context, args []string) (string, map[string]any, error) {
	if s.workflow == nil {
		return "", map[string]any{"domain": "workflow"}, errors.New("workflow runtime is not available")
	}
	action := "status"
	if len(args) > 0 {
		action = strings.ToLower(strings.TrimSpace(args[0]))
	}
	switch action {
	case "", "status", "list":
		snapshot, err := s.workflow.Snapshot(ctx)
		if err != nil {
			return "", map[string]any{"domain": "workflow", "action": "status"}, err
		}
		return formatWorkflowList(snapshot.Workflow.ActiveWorkflowID, snapshot.Workflow.Workflows), map[string]any{
			"domain":             "workflow",
			"action":             "status",
			"active_workflow_id": snapshot.Workflow.ActiveWorkflowID,
			"workflows":          len(snapshot.Workflow.Workflows),
			"runs":               len(snapshot.Workflow.Runs),
		}, nil
	case "run":
		workflowID := ""
		if len(args) > 1 {
			workflowID = strings.TrimSpace(args[1])
		}
		run, err := s.workflow.RunWorkflow(ctx, workflowID)
		metadata := map[string]any{"domain": "workflow", "action": "run", "workflow_id": run.WorkflowID, "run_id": run.ID}
		if err != nil {
			return "", metadata, err
		}
		return formatWorkflowRun(run), metadata, nil
	case "runs":
		snapshot, err := s.workflow.Snapshot(ctx)
		if err != nil {
			return "", map[string]any{"domain": "workflow", "action": "runs"}, err
		}
		return formatWorkflowRuns(snapshot.Workflow.Runs), map[string]any{"domain": "workflow", "action": "runs", "runs": len(snapshot.Workflow.Runs)}, nil
	default:
		return "", map[string]any{"domain": "workflow"}, fmt.Errorf("unknown workflow command %q", action)
	}
}

func formatWorkflowList(activeID string, workflows []models.AgentWorkflow) string {
	if len(workflows) == 0 {
		return "No workflows configured."
	}
	lines := []string{"Workflows:"}
	for _, workflow := range workflows {
		marker := " "
		if workflow.ID == activeID {
			marker = "*"
		}
		lines = append(lines, fmt.Sprintf("- %s %s (%s)", marker, workflow.Name, workflow.ID))
	}
	return strings.Join(lines, "\n")
}

func formatWorkflowRun(run models.AgentWorkflowRun) string {
	output := strings.TrimSpace(run.OutputText)
	if output == "" {
		output = strings.TrimSpace(run.Summary)
	}
	return strings.TrimSpace(fmt.Sprintf("Workflow %s %s.\n%s", run.WorkflowName, run.Status, output))
}

func formatWorkflowRuns(runs []models.AgentWorkflowRun) string {
	if len(runs) == 0 {
		return "No workflow runs recorded."
	}
	lines := []string{"Recent workflow runs:"}
	for idx, run := range runs {
		if idx >= 10 {
			break
		}
		lines = append(lines, fmt.Sprintf("- %s %s %s", run.WorkflowName, run.Status, run.CreatedAt.Format("2006-01-02 15:04:05")))
	}
	return strings.Join(lines, "\n")
}
