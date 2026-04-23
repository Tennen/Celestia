package slash

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	coreagent "github.com/chentianyu/celestia/internal/core/agent"
	"github.com/chentianyu/celestia/internal/core/audit"
	"github.com/chentianyu/celestia/internal/core/control"
	"github.com/chentianyu/celestia/internal/core/pluginmgr"
	"github.com/chentianyu/celestia/internal/core/policy"
	"github.com/chentianyu/celestia/internal/core/registry"
	"github.com/chentianyu/celestia/internal/core/state"
	"github.com/chentianyu/celestia/internal/models"
	"github.com/chentianyu/celestia/internal/storage"
)

type AgentRuntime interface {
	Snapshot(context.Context) (models.AgentSnapshot, error)
	RunMarketAnalysis(context.Context, coreagent.MarketRunRequest) (models.AgentMarketRun, error)
	ImportMarketPortfolioCodes(context.Context, models.AgentMarketImportCodesRequest) (models.AgentMarketImportCodesResponse, error)
}

type CommandExecutor interface {
	ExecuteCommand(context.Context, models.Device, models.CommandRequest) (models.CommandResponse, error)
}

type Service struct {
	store    storage.Store
	registry *registry.Service
	state    *state.Service
	controls *control.Service
	policy   *policy.Service
	audit    *audit.Service
	executor CommandExecutor
	agent    AgentRuntime
}

func New(
	store storage.Store,
	registrySvc *registry.Service,
	stateSvc *state.Service,
	controls *control.Service,
	policySvc *policy.Service,
	auditSvc *audit.Service,
	pluginMgr *pluginmgr.Manager,
	agent AgentRuntime,
) *Service {
	return &Service{
		store:    store,
		registry: registrySvc,
		state:    stateSvc,
		controls: controls,
		policy:   policySvc,
		audit:    auditSvc,
		executor: pluginMgr,
		agent:    agent,
	}
}

func (s *Service) SetCommandExecutor(executor CommandExecutor) {
	s.executor = executor
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
