package slash

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	coreagent "github.com/chentianyu/celestia/internal/core/agent"
	"github.com/chentianyu/celestia/internal/core/control"
	"github.com/chentianyu/celestia/internal/models"
)

type AgentRuntime interface {
	Snapshot(context.Context) (models.AgentSnapshot, error)
	RunMarketAnalysis(context.Context, coreagent.MarketRunRequest) (models.AgentMarketRun, error)
	ImportMarketPortfolioCodes(context.Context, models.AgentMarketImportCodesRequest) (models.AgentMarketImportCodesResponse, error)
	CreateEvolutionGoal(context.Context, coreagent.EvolutionGoalRequest) (models.AgentEvolutionGoal, error)
	RunEvolutionGoal(context.Context, string) (models.AgentEvolutionGoal, error)
	RunEvolutionOperation(context.Context, coreagent.EvolutionOperationRequest) (models.AgentEvolutionTestResult, error)
	CreateApproval(context.Context, models.AgentApprovalCreateRequest) (models.AgentApprovalRequest, error)
	ApproveApproval(context.Context, string, models.AgentApprovalDecisionRequest) (models.AgentApprovalRequest, error)
	RejectApproval(context.Context, string, models.AgentApprovalDecisionRequest) (models.AgentApprovalRequest, error)
	RunScreenshot(context.Context, models.AgentScreenshotRequest) (models.AgentScreenshotResult, error)
	RunServiceOperation(context.Context, coreagent.ServiceOperationRequest) (models.AgentTerminalResult, error)
	StartKnowledgeSession(context.Context, models.AgentKnowledgeRequest) (models.AgentKnowledgeSession, error)
	RunKnowledge(context.Context, models.AgentKnowledgeRequest) (models.AgentKnowledgeResult, error)
	ListKnowledgeAnswers(context.Context, models.AgentKnowledgeAnswersRequest) ([]models.AgentKnowledgeAnswer, error)
	RenderKnowledgeAnswer(context.Context, models.AgentKnowledgeAnswerRequest) (models.AgentKnowledgeAnswerRenderResult, error)
}

type HomeRuntime interface {
	ListViews(context.Context, control.HomeFilter) ([]models.DeviceView, error)
	Execute(context.Context, control.HomeRequest) (control.HomeResult, error)
}

type WorkflowRuntime interface {
	Snapshot(context.Context) (models.AgentSnapshot, error)
	RunWorkflow(context.Context, string) (models.AgentWorkflowRun, error)
}

type Service struct {
	home     HomeRuntime
	agent    AgentRuntime
	workflow WorkflowRuntime
}

func New(home HomeRuntime, agent AgentRuntime, workflowRuntime ...WorkflowRuntime) *Service {
	svc := &Service{
		home:  home,
		agent: agent,
	}
	if len(workflowRuntime) > 0 {
		svc.workflow = workflowRuntime[0]
	}
	return svc
}

func (s *Service) Run(ctx context.Context, req models.ProjectInputRequest) (models.SlashCommandResult, bool, error) {
	input := strings.TrimSpace(req.Input)
	if !strings.HasPrefix(input, "/") {
		return models.SlashCommandResult{}, false, nil
	}
	fields, err := splitSlashFields(strings.TrimPrefix(input, "/"))
	if err != nil {
		return models.SlashCommandResult{}, true, err
	}
	if len(fields) == 0 {
		return models.SlashCommandResult{}, true, errors.New("slash command is empty")
	}
	command := strings.ToLower(strings.TrimSpace(fields[0]))
	args := append([]string{}, fields[1:]...)
	var output string
	var metadata map[string]any
	switch command {
	case "help":
		output = slashHelp()
		metadata = map[string]any{"domain": "help"}
	case "home", "device", "devices":
		output, metadata, err = s.runHome(ctx, req, args)
	case "market":
		output, metadata, err = s.runMarket(ctx, args)
	case "kb", "knowledge":
		output, metadata, err = s.runKnowledge(ctx, req, args)
	case "workflow":
		output, metadata, err = s.runWorkflow(ctx, args)
	case "evolution", "evolve":
		output, metadata, err = s.runEvolution(ctx, req, args)
	case "screenshot", "screen":
		output, metadata, err = s.runScreenshot(ctx, args)
	case "approve":
		output, metadata, err = s.runApproval(ctx, req, args, true)
	case "reject":
		output, metadata, err = s.runApproval(ctx, req, args, false)
	case "service", "gateway":
		output, metadata, err = s.runServiceOperation(ctx, args)
	default:
		err = fmt.Errorf("unknown slash command %q", command)
	}
	result := models.SlashCommandResult{
		Command:    command,
		Args:       args,
		Output:     strings.TrimSpace(output),
		Images:     slashImages(metadata),
		Metadata:   metadata,
		ExecutedAt: time.Now().UTC(),
	}
	return result, true, err
}

func slashImages(metadata map[string]any) []models.ProjectOutputImage {
	if len(metadata) == 0 {
		return nil
	}
	images, ok := metadata["images"].([]models.ProjectOutputImage)
	if !ok || len(images) == 0 {
		return nil
	}
	return images
}
