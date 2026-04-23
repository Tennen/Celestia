package agent

import (
	"context"
	"errors"
	"strings"

	"github.com/chentianyu/celestia/internal/models"
	einotool "github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/components/tool/utils"
)

type marketToolInput struct {
	Action   string  `json:"action,omitempty" jsonschema_description:"run, status, portfolio, or add_holding."`
	Phase    string  `json:"phase,omitempty" jsonschema_description:"midday or close when action is run."`
	Notes    string  `json:"notes,omitempty" jsonschema_description:"Optional notes for the market report."`
	Code     string  `json:"code,omitempty" jsonschema_description:"Fund, ETF, or stock code when adding a holding."`
	Name     string  `json:"name,omitempty" jsonschema_description:"Optional asset name when adding a holding."`
	Quantity float64 `json:"quantity,omitempty" jsonschema_description:"Holding quantity."`
	AvgCost  float64 `json:"avg_cost,omitempty" jsonschema_description:"Average cost."`
}

func (s *Service) marketToolSpec() agentToolSpec {
	desc := "Run A-share, ETF, and fund portfolio analysis or inspect/update the tracked portfolio."
	return agentToolSpec{
		Name:         "market_analysis",
		Description:  desc,
		Keywords:     []string{"market", "fund", "etf", "a股", "基金", "行情"},
		Params:       []string{"action", "phase", "notes", "code", "name", "quantity", "avg_cost"},
		PreferResult: true,
		NewTool: func(s *Service) (einotool.InvokableTool, error) {
			return utils.InferTool("market_analysis", desc, s.runMarketTool)
		},
		RequestToJSON: func(req models.AgentCapabilityRunRequest) (string, error) {
			text := strings.TrimSpace(req.Input)
			if text != "" && isJSONObject(text) {
				return text, nil
			}
			return marshalCompactJSON(map[string]any{"action": "run", "phase": firstNonEmpty(detectMarketPhase(text), text, "close")})
		},
	}
}

func (s *Service) runMarketTool(ctx context.Context, input marketToolInput) (any, error) {
	switch strings.ToLower(firstNonEmpty(input.Action, "run")) {
	case "run", "report", "analysis":
		return s.RunMarketAnalysis(ctx, MarketRunRequest{
			Phase: firstNonEmpty(detectMarketPhase(input.Phase), input.Phase, "close"),
			Notes: input.Notes,
		})
	case "status", "latest":
		snapshot, err := s.Snapshot(ctx)
		return snapshot.Market.Runs, err
	case "portfolio", "holdings":
		snapshot, err := s.Snapshot(ctx)
		return snapshot.Market.Portfolio, err
	case "add_holding":
		holding := models.AgentMarketHolding{
			Code:     strings.TrimSpace(input.Code),
			Name:     strings.TrimSpace(input.Name),
			Quantity: input.Quantity,
			AvgCost:  input.AvgCost,
		}
		snapshot, err := s.addMarketHolding(ctx, holding)
		return snapshot.Market.Portfolio, err
	default:
		return nil, errors.New("unsupported market action")
	}
}

type evolutionToolInput struct {
	Action          string `json:"action,omitempty" jsonschema_description:"queue, run, status, tick, set_codex_model, or set_codex_effort."`
	GoalID          string `json:"goal_id,omitempty" jsonschema_description:"Evolution goal id."`
	Goal            string `json:"goal,omitempty" jsonschema_description:"Goal text for queued implementation work."`
	CommitMessage   string `json:"commit_message,omitempty" jsonschema_description:"Optional commit message for a queued goal."`
	CodexModel      string `json:"codex_model,omitempty" jsonschema_description:"Codex model setting."`
	ReasoningEffort string `json:"reasoning_effort,omitempty" jsonschema_description:"Codex reasoning effort setting."`
}

func (s *Service) evolutionToolSpec() agentToolSpec {
	desc := "Queue, run, and inspect Celestia's local evolution/coding operator."
	return agentToolSpec{
		Name:         "evolution_operator",
		Description:  desc,
		Keywords:     []string{"evolution", "coding", "codex", "代码", "自进化"},
		Params:       []string{"action", "goal_id", "goal", "commit_message", "codex_model", "reasoning_effort"},
		PreferResult: true,
		NewTool: func(s *Service) (einotool.InvokableTool, error) {
			return utils.InferTool("evolution_operator", desc, s.runEvolutionTool)
		},
		RequestToJSON: func(req models.AgentCapabilityRunRequest) (string, error) {
			text := strings.TrimSpace(req.Input)
			if text != "" && isJSONObject(text) {
				return text, nil
			}
			return marshalCompactJSON(map[string]any{"action": "queue", "goal": text})
		},
	}
}

func (s *Service) runEvolutionTool(ctx context.Context, input evolutionToolInput) (any, error) {
	switch strings.ToLower(firstNonEmpty(input.Action, "status")) {
	case "queue", "add", "create":
		return s.CreateEvolutionGoal(ctx, EvolutionGoalRequest{Goal: input.Goal, CommitMessage: input.CommitMessage})
	case "run":
		return s.RunEvolutionGoal(ctx, input.GoalID)
	case "status":
		return s.evolutionStatus(ctx, input.GoalID)
	case "tick":
		return s.runNextEvolutionGoal(ctx)
	case "set_codex_model":
		return s.setCodexEvolutionOption(ctx, "model", input.CodexModel)
	case "set_codex_effort":
		return s.setCodexEvolutionOption(ctx, "effort", input.ReasoningEffort)
	default:
		return nil, errors.New("unsupported evolution action")
	}
}

type terminalToolInput struct {
	Command string `json:"command" jsonschema:"required" jsonschema_description:"Shell command to run. Use only for explicit operator requests."`
	CWD     string `json:"cwd,omitempty" jsonschema_description:"Optional working directory."`
}

func (s *Service) terminalToolSpec() agentToolSpec {
	desc := "Run an explicit shell command through Celestia terminal settings. Use only when the user explicitly asks for command execution."
	return agentToolSpec{
		Name:        "terminal_run",
		Description: desc,
		Keywords:    []string{"terminal", "shell", "command"},
		Params:      []string{"command", "cwd"},
		Terminal:    true,
		NewTool: func(s *Service) (einotool.InvokableTool, error) {
			return utils.InferTool("terminal_run", desc, s.runTerminalTool)
		},
		RequestToJSON: rawTextRequestJSON("command"),
	}
}

func (s *Service) runTerminalTool(ctx context.Context, input terminalToolInput) (models.AgentTerminalResult, error) {
	return s.RunTerminal(ctx, models.AgentTerminalRequest{Command: input.Command, CWD: input.CWD})
}

type codexToolInput struct {
	Prompt          string `json:"prompt" jsonschema:"required" jsonschema_description:"Task prompt for Codex."`
	CWD             string `json:"cwd,omitempty" jsonschema_description:"Optional working directory."`
	Model           string `json:"model,omitempty" jsonschema_description:"Optional Codex model."`
	ReasoningEffort string `json:"reasoning_effort,omitempty" jsonschema_description:"minimal, low, medium, high, or xhigh."`
	TimeoutMS       int    `json:"timeout_ms,omitempty" jsonschema_description:"Optional timeout in milliseconds."`
}

func (s *Service) codexToolSpec() agentToolSpec {
	desc := "Run Codex for an explicit local coding task."
	return agentToolSpec{
		Name:         "codex_runner",
		Description:  desc,
		Keywords:     []string{"codex", "coding", "code"},
		Params:       []string{"prompt", "cwd", "model", "reasoning_effort", "timeout_ms"},
		PreferResult: true,
		NewTool: func(s *Service) (einotool.InvokableTool, error) {
			return utils.InferTool("codex_runner", desc, s.runCodexTool)
		},
		RequestToJSON: rawTextRequestJSON("prompt"),
	}
}

func (s *Service) runCodexTool(ctx context.Context, input codexToolInput) (models.AgentCodexResult, error) {
	return s.RunCodex(ctx, models.AgentCodexRequest{
		Prompt:          input.Prompt,
		CWD:             input.CWD,
		Model:           input.Model,
		ReasoningEffort: input.ReasoningEffort,
		TimeoutMS:       input.TimeoutMS,
	})
}
