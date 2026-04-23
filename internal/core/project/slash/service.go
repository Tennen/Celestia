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
}

type HomeRuntime interface {
	ListViews(context.Context, control.HomeFilter) ([]models.DeviceView, error)
	Execute(context.Context, control.HomeRequest) (control.HomeResult, error)
}

type Service struct {
	home  HomeRuntime
	agent AgentRuntime
}

func New(home HomeRuntime, agent AgentRuntime) *Service {
	return &Service{
		home:  home,
		agent: agent,
	}
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
	default:
		err = fmt.Errorf("unknown slash command %q", command)
	}
	result := models.SlashCommandResult{
		Command:    command,
		Args:       args,
		Output:     strings.TrimSpace(output),
		Metadata:   metadata,
		ExecutedAt: time.Now().UTC(),
	}
	return result, true, err
}
