package agent

import (
	"context"
	"encoding/json"
	"errors"
	"sort"
	"strings"

	"github.com/chentianyu/celestia/internal/models"
	einotool "github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/components/tool/utils"
	"github.com/cloudwego/eino/schema"
)

type agentToolSpec struct {
	Name            string
	Description     string
	Keywords        []string
	Params          []string
	Terminal        bool
	Command         string
	Install         string
	Detail          string
	PreferResult    bool
	NewTool         func(*Service) (einotool.InvokableTool, error)
	RequestToJSON   func(models.AgentCapabilityRunRequest) (string, error)
	ReturnDirectly  bool
	RequiresConsent bool
}

type agentToolRuntime struct {
	spec agentToolSpec
	tool einotool.InvokableTool
	info *schema.ToolInfo
}

func (s *Service) buildAgentToolRuntimes(ctx context.Context) ([]agentToolRuntime, error) {
	specs := s.agentToolSpecs()
	out := make([]agentToolRuntime, 0, len(specs))
	for _, spec := range specs {
		item, err := spec.NewTool(s)
		if err != nil {
			return nil, err
		}
		info, err := item.Info(ctx)
		if err != nil {
			return nil, err
		}
		out = append(out, agentToolRuntime{spec: spec, tool: item, info: info})
	}
	return out, nil
}

func agentBaseTools(runtimes []agentToolRuntime) []einotool.BaseTool {
	out := make([]einotool.BaseTool, 0, len(runtimes))
	for _, item := range runtimes {
		out = append(out, item.tool)
	}
	return out
}

func agentReturnDirectly(runtimes []agentToolRuntime) map[string]struct{} {
	out := map[string]struct{}{}
	for _, item := range runtimes {
		if item.spec.ReturnDirectly {
			out[item.spec.Name] = struct{}{}
		}
	}
	return out
}

func (s *Service) agentToolSpecs() []agentToolSpec {
	return []agentToolSpec{
		s.searchToolSpec(),
		s.topicToolSpec(),
		s.writingToolSpec(),
		s.marketToolSpec(),
		s.evolutionToolSpec(),
		s.terminalToolSpec(),
		s.codexToolSpec(),
		s.markdownToolSpec(),
		s.appleNotesToolSpec(),
		s.appleRemindersToolSpec(),
		s.wecomSendToolSpec(),
	}
}

func inferAgentTool[T, D any](
	name string,
	description string,
	fn func(context.Context, T) (D, error),
) func(*Service) (einotool.InvokableTool, error) {
	return func(*Service) (einotool.InvokableTool, error) {
		return utils.InferTool(name, description, fn)
	}
}

func capabilityFromRuntime(runtime agentToolRuntime) models.AgentCapabilityInfo {
	return capabilityFromSpec(runtime.spec)
}

func capabilityFromSpec(spec agentToolSpec) models.AgentCapabilityInfo {
	return models.AgentCapabilityInfo{
		Name:             spec.Name,
		Description:      spec.Description,
		Terminal:         spec.Terminal,
		Command:          spec.Command,
		Install:          spec.Install,
		Keywords:         append([]string{}, spec.Keywords...),
		Tool:             spec.Name,
		Action:           "invoke",
		Params:           append([]string{}, spec.Params...),
		PreferToolResult: spec.PreferResult,
		Detail:           spec.Detail,
	}
}

func defaultAgentCapabilityInfos() []models.AgentCapabilityInfo {
	service := &Service{}
	specs := service.agentToolSpecs()
	out := make([]models.AgentCapabilityInfo, 0, len(specs))
	for _, spec := range specs {
		out = append(out, capabilityFromSpec(spec))
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out
}

func normalizeToolName(name string) string {
	value := strings.ToLower(strings.TrimSpace(name))
	value = strings.TrimPrefix(value, "agent.")
	value = strings.ReplaceAll(value, "-", "_")
	return value
}

func requestJSONOrDefault(req models.AgentCapabilityRunRequest, fallback map[string]any) (string, error) {
	text := strings.TrimSpace(req.Input)
	if text != "" && isJSONObject(text) {
		return text, nil
	}
	if req.Command != "" {
		fallback["command"] = strings.TrimSpace(req.Command)
	}
	if len(req.Args) > 0 {
		fallback["args"] = append([]string{}, req.Args...)
	}
	if text != "" {
		fallback["input"] = text
	}
	return marshalCompactJSON(fallback)
}

func marshalCompactJSON(value any) (string, error) {
	raw, err := json.Marshal(value)
	if err != nil {
		return "", err
	}
	return string(raw), nil
}

func marshalToolOutput(value any) string {
	raw, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return strings.TrimSpace(err.Error())
	}
	return strings.TrimSpace(string(raw))
}

func isJSONObject(text string) bool {
	trimmed := strings.TrimSpace(text)
	if !strings.HasPrefix(trimmed, "{") || !strings.HasSuffix(trimmed, "}") {
		return false
	}
	var payload map[string]any
	return json.Unmarshal([]byte(trimmed), &payload) == nil
}

func rawTextRequestJSON(field string) func(models.AgentCapabilityRunRequest) (string, error) {
	return func(req models.AgentCapabilityRunRequest) (string, error) {
		text := strings.TrimSpace(req.Input)
		if text != "" && isJSONObject(text) {
			return text, nil
		}
		if text == "" && len(req.Args) > 0 {
			text = strings.Join(req.Args, " ")
		}
		if text == "" && req.Command != "" {
			text = req.Command
		}
		if text == "" {
			return "", errors.New(field + " is required")
		}
		return marshalCompactJSON(map[string]any{field: text})
	}
}
