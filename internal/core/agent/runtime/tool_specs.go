package runtime

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/chentianyu/celestia/internal/models"
	einotool "github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/components/tool/utils"
)

type searchToolInput struct {
	Query          string   `json:"query" jsonschema:"required" jsonschema_description:"Search query."`
	EngineSelector string   `json:"engine_selector,omitempty" jsonschema_description:"Optional configured search engine id or type."`
	Recency        string   `json:"recency,omitempty" jsonschema_description:"Optional recency filter such as day, week, month, or year."`
	MaxItems       int      `json:"max_items,omitempty" jsonschema_description:"Maximum result count. Defaults to 8."`
	Sites          []string `json:"sites,omitempty" jsonschema_description:"Optional site filters."`
}

func (s *Service) searchToolSpec() agentToolSpec {
	desc := "Search the web through Celestia's configured search engine profiles. Use this for current external information."
	return agentToolSpec{
		Name:         "search_web",
		Description:  desc,
		Keywords:     []string{"search", "web", "news", "实时", "搜索"},
		Params:       []string{"query", "engine_selector", "recency", "max_items", "sites"},
		PreferResult: true,
		NewTool: func(s *Service) (einotool.InvokableTool, error) {
			return utils.InferTool("search_web", desc, s.runSearchTool)
		},
		RequestToJSON: func(req models.AgentToolRunRequest) (string, error) {
			text := strings.TrimSpace(req.Input)
			if text != "" && isJSONObject(text) {
				return text, nil
			}
			return marshalCompactJSON(map[string]any{"query": text, "max_items": 8})
		},
	}
}

func (s *Service) runSearchTool(ctx context.Context, input searchToolInput) (models.AgentSearchResult, error) {
	if strings.TrimSpace(input.Query) == "" {
		return models.AgentSearchResult{}, errors.New("query is required")
	}
	return s.RunSearch(ctx, models.AgentSearchRequest{
		EngineSelector: input.EngineSelector,
		MaxItems:       maxInt(input.MaxItems, 8),
		LogContext:     "conversation:search_web",
		Plans: []models.AgentSearchPlan{{
			Label:   "agent",
			Query:   strings.TrimSpace(input.Query),
			Recency: input.Recency,
			Sites:   append([]string{}, input.Sites...),
		}},
	})
}

type workflowToolInput struct {
	Action     string `json:"action,omitempty" jsonschema_description:"run, state, list_workflows, get_workflow, use_workflow, or list_runs."`
	WorkflowID string `json:"workflow_id,omitempty" jsonschema_description:"Workflow id."`
}

func (s *Service) workflowToolSpec() agentToolSpec {
	desc := "Run and inspect Celestia workflows composed from modular nodes such as RSS, prompt, LLM, search, and WeCom output."
	return agentToolSpec{
		Name:         "workflow",
		Description:  desc,
		Keywords:     []string{"workflow", "rss", "automation", "digest", "日报", "新闻摘要"},
		Params:       []string{"action", "workflow_id"},
		PreferResult: true,
		NewTool: func(s *Service) (einotool.InvokableTool, error) {
			return utils.InferTool("workflow", desc, s.runWorkflowTool)
		},
		RequestToJSON: func(req models.AgentToolRunRequest) (string, error) {
			text := strings.TrimSpace(req.Input)
			if text != "" && isJSONObject(text) {
				return text, nil
			}
			return marshalCompactJSON(map[string]any{"action": "run", "workflow_id": text})
		},
	}
}

func (s *Service) runWorkflowTool(ctx context.Context, input workflowToolInput) (any, error) {
	action := strings.ToLower(firstNonEmpty(input.Action, "run"))
	switch action {
	case "run", "digest", "summary":
		return s.RunWorkflow(ctx, input.WorkflowID)
	case "state", "status":
		snapshot, err := s.Snapshot(ctx)
		if err != nil {
			return nil, err
		}
		return map[string]any{
			"active_workflow_id": snapshot.Workflow.ActiveWorkflowID,
			"workflows":          len(snapshot.Workflow.Workflows),
			"runs":               len(snapshot.Workflow.Runs),
			"sent_log":           len(snapshot.Workflow.SentLog),
		}, nil
	case "list_workflows":
		snapshot, err := s.Snapshot(ctx)
		return snapshot.Workflow.Workflows, err
	case "get_workflow":
		snapshot, err := s.Snapshot(ctx)
		if err != nil {
			return nil, err
		}
		workflow, ok := selectWorkflow(snapshot.Workflow, input.WorkflowID)
		if !ok {
			return nil, errors.New("workflow not found")
		}
		return workflow, nil
	case "use_workflow":
		snapshot, err := s.update(ctx, func(snapshot *models.AgentSnapshot) error {
			workflow, ok := selectWorkflow(snapshot.Workflow, input.WorkflowID)
			if !ok {
				return errors.New("workflow not found")
			}
			now := time.Now().UTC()
			snapshot.Workflow.ActiveWorkflowID = workflow.ID
			snapshot.Workflow.UpdatedAt = now
			snapshot.UpdatedAt = now
			return nil
		})
		if err != nil {
			return nil, err
		}
		return snapshot.Workflow, nil
	case "list_runs":
		snapshot, err := s.Snapshot(ctx)
		return snapshot.Workflow.Runs, err
	default:
		return nil, errors.New("unsupported workflow action")
	}
}

type writingToolInput struct {
	Action  string `json:"action,omitempty" jsonschema_description:"list, show, create, append, summarize, restore, or set."`
	TopicID string `json:"topic_id,omitempty" jsonschema_description:"Writing topic id."`
	Title   string `json:"title,omitempty" jsonschema_description:"Topic or material title."`
	Content string `json:"content,omitempty" jsonschema_description:"Material content or section content."`
	Section string `json:"section,omitempty" jsonschema_description:"summary, outline, or draft when action is set."`
}

func (s *Service) writingToolSpec() agentToolSpec {
	desc := "Organize fragmented writing inputs into topics, materials, summaries, outlines, and drafts."
	return agentToolSpec{
		Name:         "writing_organizer",
		Description:  desc,
		Keywords:     []string{"writing", "draft", "summary", "写作", "整理"},
		Params:       []string{"action", "topic_id", "title", "content", "section"},
		PreferResult: true,
		NewTool: func(s *Service) (einotool.InvokableTool, error) {
			return utils.InferTool("writing_organizer", desc, s.runWritingTool)
		},
		RequestToJSON: func(req models.AgentToolRunRequest) (string, error) {
			return requestJSONOrDefault(req, map[string]any{"action": "list"})
		},
	}
}

func (s *Service) runWritingTool(ctx context.Context, input writingToolInput) (any, error) {
	switch strings.ToLower(firstNonEmpty(input.Action, "list")) {
	case "list":
		snapshot, err := s.Snapshot(ctx)
		return snapshot.Writing.Topics, err
	case "show":
		snapshot, err := s.Snapshot(ctx)
		if err != nil {
			return nil, err
		}
		topic, ok := findWritingTopic(snapshot.Writing.Topics, input.TopicID)
		if !ok {
			return nil, errors.New("writing topic not found")
		}
		return topic, nil
	case "create":
		return s.SaveWritingTopic(ctx, WritingTopicRequest{Title: input.Title})
	case "append":
		return s.AddWritingMaterial(ctx, input.TopicID, WritingMaterialRequest{Title: input.Title, Content: input.Content})
	case "summarize":
		return s.SummarizeWritingTopic(ctx, input.TopicID)
	case "restore":
		return s.RestoreWritingTopic(ctx, input.TopicID)
	case "set":
		return s.SetWritingTopicState(ctx, input.TopicID, WritingStateUpdateRequest{Section: input.Section, Content: input.Content})
	default:
		return nil, errors.New("unsupported writing action")
	}
}
