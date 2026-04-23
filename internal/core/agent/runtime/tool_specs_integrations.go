package runtime

import (
	"context"
	"strings"

	"github.com/chentianyu/celestia/internal/models"
	einotool "github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/components/tool/utils"
)

type markdownToolInput struct {
	Markdown  string `json:"markdown" jsonschema:"required" jsonschema_description:"Markdown content to render."`
	Mode      string `json:"mode,omitempty" jsonschema_description:"long-image or multi-page."`
	OutputDir string `json:"output_dir,omitempty" jsonschema_description:"Optional output directory."`
}

func (s *Service) markdownToolSpec() agentToolSpec {
	desc := "Render Markdown into image assets through Celestia's markdown renderer."
	return agentToolSpec{
		Name:         "markdown_render",
		Description:  desc,
		Keywords:     []string{"markdown", "image", "md2img", "长图"},
		Params:       []string{"markdown", "mode", "output_dir"},
		PreferResult: true,
		NewTool: func(s *Service) (einotool.InvokableTool, error) {
			return utils.InferTool("markdown_render", desc, s.runMarkdownTool)
		},
		RequestToJSON: rawTextRequestJSON("markdown"),
	}
}

func (s *Service) runMarkdownTool(ctx context.Context, input markdownToolInput) (models.AgentMarkdownRenderResult, error) {
	return s.RunMarkdownRender(ctx, models.AgentMarkdownRenderRequest{
		Markdown:  input.Markdown,
		Mode:      input.Mode,
		OutputDir: input.OutputDir,
	})
}

type appleNotesToolInput struct {
	Args string `json:"args,omitempty" jsonschema_description:"Arguments after the memo executable. Use notes/list/search/add forms supported by memo."`
}

func (s *Service) appleNotesToolSpec() agentToolSpec {
	desc := "Manage Apple Notes on macOS through the memo CLI."
	return agentToolSpec{
		Name:        "apple_notes",
		Description: desc,
		Keywords:    []string{"apple notes", "memo", "备忘录", "笔记"},
		Params:      []string{"args"},
		Terminal:    true,
		Command:     "memo",
		Install:     "brew tap antoniorodr/memo && brew install antoniorodr/memo/memo",
		Detail: strings.TrimSpace(`
# Apple Notes

This tool invokes the memo CLI. Examples:
- {"args":"notes"}
- {"args":"notes -s query"}
- {"args":"notes -a \"Title\""}
`),
		NewTool: func(s *Service) (einotool.InvokableTool, error) {
			return utils.InferTool("apple_notes", desc, s.runAppleNotesTool)
		},
		RequestToJSON: func(req models.AgentToolRunRequest) (string, error) {
			return requestJSONOrDefault(req, map[string]any{"args": "notes"})
		},
	}
}

func (s *Service) runAppleNotesTool(ctx context.Context, input appleNotesToolInput) (models.AgentTerminalResult, error) {
	args := strings.TrimSpace(firstNonEmpty(input.Args, "notes"))
	if !strings.HasPrefix(args, "notes") {
		args = "notes " + args
	}
	return s.RunTerminal(ctx, models.AgentTerminalRequest{Command: "memo " + args})
}

type appleRemindersToolInput struct {
	Args string `json:"args,omitempty" jsonschema_description:"Arguments after the remindctl executable, such as today, upcoming, add, complete, or delete."`
}

func (s *Service) appleRemindersToolSpec() agentToolSpec {
	desc := "Manage Apple Reminders on macOS through remindctl."
	return agentToolSpec{
		Name:        "apple_reminders",
		Description: desc,
		Keywords:    []string{"apple reminders", "todo", "提醒", "待办"},
		Params:      []string{"args"},
		Terminal:    true,
		Command:     "remindctl",
		Install:     "brew install steipete/tap/remindctl",
		Detail: strings.TrimSpace(`
# Apple Reminders

This tool invokes the remindctl CLI. Examples:
- {"args":"today"}
- {"args":"upcoming --json"}
- {"args":"add \"Buy milk\""}
`),
		NewTool: func(s *Service) (einotool.InvokableTool, error) {
			return utils.InferTool("apple_reminders", desc, s.runAppleRemindersTool)
		},
		RequestToJSON: func(req models.AgentToolRunRequest) (string, error) {
			return requestJSONOrDefault(req, map[string]any{"args": "today"})
		},
	}
}

func (s *Service) runAppleRemindersTool(ctx context.Context, input appleRemindersToolInput) (models.AgentTerminalResult, error) {
	args := strings.TrimSpace(firstNonEmpty(input.Args, "today"))
	return s.RunTerminal(ctx, models.AgentTerminalRequest{Command: "remindctl " + args})
}
