package agent

import (
	"context"
	"errors"
	"strings"

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
		RequestToJSON: func(req models.AgentCapabilityRunRequest) (string, error) {
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

type topicToolInput struct {
	Action      string  `json:"action,omitempty" jsonschema_description:"run, state, list_profiles, get_profile, use_profile, add_profile, delete_profile, list_sources, add_source, update_source, enable_source, disable_source, delete_source, clear_sent_log."`
	ProfileID   string  `json:"profile_id,omitempty" jsonschema_description:"Topic profile id."`
	ProfileName string  `json:"profile_name,omitempty" jsonschema_description:"Topic profile display name."`
	CloneFrom   string  `json:"clone_from,omitempty" jsonschema_description:"Profile id to clone sources from when creating a profile."`
	SourceID    string  `json:"source_id,omitempty" jsonschema_description:"RSS source id."`
	SourceName  string  `json:"source_name,omitempty" jsonschema_description:"RSS source name."`
	Category    string  `json:"category,omitempty" jsonschema_description:"RSS source category."`
	FeedURL     string  `json:"feed_url,omitempty" jsonschema_description:"RSS feed URL."`
	Weight      float64 `json:"weight,omitempty" jsonschema_description:"Source weight. Defaults to 1."`
}

func (s *Service) topicToolSpec() agentToolSpec {
	desc := "Run and manage Celestia topic summaries backed by RSS profiles and sources."
	return agentToolSpec{
		Name:         "topic_summary",
		Description:  desc,
		Keywords:     []string{"topic", "rss", "digest", "日报", "新闻摘要"},
		Params:       []string{"action", "profile_id", "profile_name", "source_id", "source_name", "category", "feed_url", "weight"},
		PreferResult: true,
		NewTool: func(s *Service) (einotool.InvokableTool, error) {
			return utils.InferTool("topic_summary", desc, s.runTopicTool)
		},
		RequestToJSON: func(req models.AgentCapabilityRunRequest) (string, error) {
			text := strings.TrimSpace(req.Input)
			if text != "" && isJSONObject(text) {
				return text, nil
			}
			return marshalCompactJSON(map[string]any{"action": "run", "profile_id": text})
		},
	}
}

func (s *Service) runTopicTool(ctx context.Context, input topicToolInput) (any, error) {
	action := strings.ToLower(firstNonEmpty(input.Action, "run"))
	switch action {
	case "run", "digest", "summary":
		return s.RunTopicSummary(ctx, input.ProfileID)
	case "state", "status":
		snapshot, err := s.Snapshot(ctx)
		if err != nil {
			return nil, err
		}
		return map[string]any{
			"active_profile_id": snapshot.TopicSummary.ActiveProfileID,
			"profiles":          len(snapshot.TopicSummary.Profiles),
			"runs":              len(snapshot.TopicSummary.Runs),
			"sent_log":          len(snapshot.TopicSummary.SentLog),
		}, nil
	case "list_profiles":
		snapshot, err := s.Snapshot(ctx)
		return snapshot.TopicSummary.Profiles, err
	case "get_profile":
		return s.topicProfile(ctx, input.ProfileID)
	case "use_profile":
		return s.useTopicProfile(ctx, input.ProfileID)
	case "add_profile":
		return s.addTopicProfileTool(ctx, input)
	case "delete_profile":
		return s.deleteTopicProfileTool(ctx, input.ProfileID)
	case "list_sources":
		profile, err := s.topicProfile(ctx, input.ProfileID)
		if err != nil {
			return nil, err
		}
		return profile.Sources, nil
	case "add_source":
		return s.addTopicSourceTool(ctx, input)
	case "update_source":
		return s.updateTopicSourceTool(ctx, input)
	case "enable_source", "disable_source":
		return s.setTopicSourceEnabled(ctx, input.ProfileID, input.SourceID, action == "enable_source")
	case "delete_source":
		return s.deleteTopicSourceTool(ctx, input.ProfileID, input.SourceID)
	case "clear_sent_log":
		return s.clearTopicSentLog(ctx)
	default:
		return nil, errors.New("unsupported topic action")
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
		RequestToJSON: func(req models.AgentCapabilityRunRequest) (string, error) {
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
