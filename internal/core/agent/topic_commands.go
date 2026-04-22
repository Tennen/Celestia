package agent

import (
	"context"
	"errors"
	"strings"

	"github.com/chentianyu/celestia/internal/models"
	"github.com/google/uuid"
)

func (s *Service) runTopicCommand(ctx context.Context, rest string) (string, bool, error) {
	tokens := shellFields(rest)
	if len(tokens) == 0 {
		run, err := s.RunTopicSummary(ctx, "")
		return marshalCommandResult(run), true, err
	}
	action := strings.ToLower(tokens[0])
	args := tokens[1:]
	switch action {
	case "run", "digest", "summary", "today":
		flags := parseFlags(args)
		profileID := firstNonEmpty(flagString(flags, "profile", "profile-id"), firstPositional(flags))
		run, err := s.RunTopicSummary(ctx, profileID)
		return marshalCommandResult(run), true, err
	case "profile", "profiles":
		return s.runTopicProfileCommand(ctx, args)
	case "source", "sources", "rss", "feeds":
		return s.runTopicSourceCommand(ctx, args)
	case "config", "settings":
		snapshot, err := s.Snapshot(ctx)
		return marshalCommandResult(snapshot.TopicSummary), true, err
	case "state", "status", "stats":
		if len(args) > 0 && isOneOf(strings.ToLower(args[0]), "clear", "reset", "clean") {
			return s.clearTopicSentState(ctx, args[1:])
		}
		snapshot, err := s.Snapshot(ctx)
		if err != nil {
			return "", true, err
		}
		return marshalCommandResult(map[string]any{
			"active_profile_id": snapshot.TopicSummary.ActiveProfileID,
			"profiles":          len(snapshot.TopicSummary.Profiles),
			"runs":              len(snapshot.TopicSummary.Runs),
			"sent_log":          len(snapshot.TopicSummary.SentLog),
		}), true, nil
	case "help", "h", "?":
		return topicHelpText(), true, nil
	default:
		run, err := s.RunTopicSummary(ctx, strings.TrimSpace(rest))
		return marshalCommandResult(run), true, err
	}
}

func (s *Service) runTopicProfileCommand(ctx context.Context, args []string) (string, bool, error) {
	op := "list"
	if len(args) > 0 {
		op = strings.ToLower(args[0])
		args = args[1:]
	}
	flags := parseFlags(args)
	switch op {
	case "list", "ls", "all":
		snapshot, err := s.Snapshot(ctx)
		if err != nil {
			return "", true, err
		}
		return marshalCommandResult(snapshot.TopicSummary.Profiles), true, nil
	case "get", "show":
		profile, err := s.topicProfile(ctx, firstNonEmpty(firstPositional(flags), flagString(flags, "id", "profile-id")))
		return marshalCommandResult(profile), true, err
	case "use", "switch", "activate":
		id := firstNonEmpty(firstPositional(flags), flagString(flags, "id", "profile-id"))
		snapshot, err := s.updateTopic(ctx, func(topic *models.AgentTopicSnapshot) error {
			if findTopicProfileIndex(*topic, id) < 0 {
				return errors.New("topic profile not found")
			}
			topic.ActiveProfileID = id
			return nil
		})
		return marshalCommandResult(snapshot.TopicSummary), true, err
	case "add", "create":
		return s.addTopicProfile(ctx, flags)
	case "update", "edit":
		return s.updateTopicProfile(ctx, flags)
	case "delete", "remove", "rm", "del":
		return s.deleteTopicProfile(ctx, flags)
	default:
		return topicHelpText(), true, nil
	}
}

func (s *Service) runTopicSourceCommand(ctx context.Context, args []string) (string, bool, error) {
	op := "list"
	if len(args) > 0 {
		op = strings.ToLower(args[0])
		args = args[1:]
	}
	flags := parseFlags(args)
	switch op {
	case "list", "ls", "all":
		profile, err := s.topicProfile(ctx, flagString(flags, "profile", "profile-id"))
		if err != nil {
			return "", true, err
		}
		return marshalCommandResult(profile.Sources), true, nil
	case "get", "show":
		source, err := s.topicSource(ctx, flags)
		return marshalCommandResult(source), true, err
	case "add", "create":
		return s.addTopicSource(ctx, flags)
	case "update", "edit":
		return s.updateTopicSource(ctx, flags)
	case "enable", "disable":
		return s.toggleTopicSource(ctx, flags, op == "enable")
	case "delete", "remove", "rm", "del":
		return s.deleteTopicSource(ctx, flags)
	default:
		return topicHelpText(), true, nil
	}
}

func (s *Service) addTopicProfile(ctx context.Context, flags parsedFlags) (string, bool, error) {
	name := flagString(flags, "name", "title")
	if name == "" {
		return "", true, errors.New("topic profile add requires --name")
	}
	id := firstNonEmpty(flagString(flags, "id", "profile-id"), slugID(name))
	cloneFrom := flagString(flags, "clone-from", "clone", "from")
	snapshot, err := s.updateTopic(ctx, func(topic *models.AgentTopicSnapshot) error {
		if findTopicProfileIndex(*topic, id) >= 0 {
			return errors.New("topic profile already exists")
		}
		profile := models.AgentTopicProfile{ID: id, Name: name, Sources: []models.AgentTopicSource{}}
		if cloneFrom != "" {
			idx := findTopicProfileIndex(*topic, cloneFrom)
			if idx < 0 {
				return errors.New("clone source profile not found")
			}
			profile.Sources = append([]models.AgentTopicSource{}, topic.Profiles[idx].Sources...)
		}
		topic.Profiles = append(topic.Profiles, profile)
		if topic.ActiveProfileID == "" {
			topic.ActiveProfileID = id
		}
		return nil
	})
	return marshalCommandResult(snapshot.TopicSummary), true, err
}

func (s *Service) addTopicSource(ctx context.Context, flags parsedFlags) (string, bool, error) {
	name := flagString(flags, "name", "title")
	category := flagString(flags, "category", "cat")
	feedURL := flagString(flags, "url", "feed", "feed-url")
	if name == "" || category == "" || feedURL == "" {
		return "", true, errors.New("topic source add requires --name, --category, and --url")
	}
	source := models.AgentTopicSource{
		ID:       firstNonEmpty(flagString(flags, "id", "source-id"), slugID(name)),
		Name:     name,
		Category: category,
		FeedURL:  feedURL,
		Weight:   1,
		Enabled:  true,
	}
	if weight, ok := flagFloat(flags, "weight"); ok {
		source.Weight = weight
	}
	if enabled, ok := flagBool(flags, "enabled"); ok {
		source.Enabled = enabled
	}
	snapshot, err := s.updateTopic(ctx, func(topic *models.AgentTopicSnapshot) error {
		idx := findTopicProfileIndex(*topic, flagString(flags, "profile", "profile-id"))
		if idx < 0 {
			return errors.New("topic profile not found")
		}
		topic.Profiles[idx].Sources = append(topic.Profiles[idx].Sources, source)
		return nil
	})
	return marshalCommandResult(snapshot.TopicSummary), true, err
}

func (s *Service) updateTopicProfile(ctx context.Context, flags parsedFlags) (string, bool, error) {
	id := firstNonEmpty(firstPositional(flags), flagString(flags, "id", "profile-id"))
	name := flagString(flags, "name", "title")
	if id == "" || name == "" {
		return "", true, errors.New("topic profile update requires id and --name")
	}
	snapshot, err := s.updateTopic(ctx, func(topic *models.AgentTopicSnapshot) error {
		idx := findTopicProfileIndex(*topic, id)
		if idx < 0 {
			return errors.New("topic profile not found")
		}
		topic.Profiles[idx].Name = name
		return nil
	})
	return marshalCommandResult(snapshot.TopicSummary), true, err
}

func (s *Service) deleteTopicProfile(ctx context.Context, flags parsedFlags) (string, bool, error) {
	id := firstNonEmpty(firstPositional(flags), flagString(flags, "id", "profile-id"))
	snapshot, err := s.updateTopic(ctx, func(topic *models.AgentTopicSnapshot) error {
		idx := findTopicProfileIndex(*topic, id)
		if idx < 0 {
			return errors.New("topic profile not found")
		}
		topic.Profiles = append(topic.Profiles[:idx], topic.Profiles[idx+1:]...)
		if topic.ActiveProfileID == id {
			topic.ActiveProfileID = ""
		}
		return nil
	})
	return marshalCommandResult(snapshot.TopicSummary), true, err
}

func (s *Service) updateTopicSource(ctx context.Context, flags parsedFlags) (string, bool, error) {
	id := firstNonEmpty(firstPositional(flags), flagString(flags, "id", "source-id"))
	snapshot, err := s.updateTopic(ctx, func(topic *models.AgentTopicSnapshot) error {
		source, err := mutableTopicSource(topic, flagString(flags, "profile", "profile-id"), id)
		if err != nil {
			return err
		}
		if value := flagString(flags, "name", "title"); value != "" {
			source.Name = value
		}
		if value := flagString(flags, "category", "cat"); value != "" {
			source.Category = value
		}
		if value := flagString(flags, "url", "feed", "feed-url"); value != "" {
			source.FeedURL = value
		}
		if value, ok := flagFloat(flags, "weight"); ok {
			source.Weight = value
		}
		if value, ok := flagBool(flags, "enabled"); ok {
			source.Enabled = value
		}
		return nil
	})
	return marshalCommandResult(snapshot.TopicSummary), true, err
}

func (s *Service) toggleTopicSource(ctx context.Context, flags parsedFlags, enabled bool) (string, bool, error) {
	id := firstNonEmpty(firstPositional(flags), flagString(flags, "id", "source-id"))
	snapshot, err := s.updateTopic(ctx, func(topic *models.AgentTopicSnapshot) error {
		source, err := mutableTopicSource(topic, flagString(flags, "profile", "profile-id"), id)
		if err != nil {
			return err
		}
		source.Enabled = enabled
		return nil
	})
	return marshalCommandResult(snapshot.TopicSummary), true, err
}

func (s *Service) deleteTopicSource(ctx context.Context, flags parsedFlags) (string, bool, error) {
	id := firstNonEmpty(firstPositional(flags), flagString(flags, "id", "source-id"))
	snapshot, err := s.updateTopic(ctx, func(topic *models.AgentTopicSnapshot) error {
		profileIdx := findTopicProfileIndex(*topic, flagString(flags, "profile", "profile-id"))
		if profileIdx < 0 {
			return errors.New("topic profile not found")
		}
		sourceIdx := findTopicSourceIndex(topic.Profiles[profileIdx].Sources, id)
		if sourceIdx < 0 {
			return errors.New("topic source not found")
		}
		sources := topic.Profiles[profileIdx].Sources
		topic.Profiles[profileIdx].Sources = append(sources[:sourceIdx], sources[sourceIdx+1:]...)
		return nil
	})
	return marshalCommandResult(snapshot.TopicSummary), true, err
}

func (s *Service) clearTopicSentState(ctx context.Context, args []string) (string, bool, error) {
	_ = parseFlags(args)
	snapshot, err := s.updateTopic(ctx, func(topic *models.AgentTopicSnapshot) error {
		topic.SentLog = []models.AgentTopicSentLogItem{}
		return nil
	})
	return marshalCommandResult(snapshot.TopicSummary), true, err
}

func (s *Service) topicProfile(ctx context.Context, profileID string) (models.AgentTopicProfile, error) {
	snapshot, err := s.Snapshot(ctx)
	if err != nil {
		return models.AgentTopicProfile{}, err
	}
	profile, ok := selectTopicProfile(snapshot.TopicSummary, profileID)
	if !ok {
		return models.AgentTopicProfile{}, errors.New("topic profile not found")
	}
	return profile, nil
}

func (s *Service) topicSource(ctx context.Context, flags parsedFlags) (models.AgentTopicSource, error) {
	profile, err := s.topicProfile(ctx, flagString(flags, "profile", "profile-id"))
	if err != nil {
		return models.AgentTopicSource{}, err
	}
	id := firstNonEmpty(firstPositional(flags), flagString(flags, "id", "source-id"))
	for _, source := range profile.Sources {
		if source.ID == id {
			return source, nil
		}
	}
	return models.AgentTopicSource{}, errors.New("topic source not found")
}

func (s *Service) updateTopic(ctx context.Context, mutate func(*models.AgentTopicSnapshot) error) (models.AgentSnapshot, error) {
	snapshot, err := s.Snapshot(ctx)
	if err != nil {
		return models.AgentSnapshot{}, err
	}
	topic := snapshot.TopicSummary
	if err := mutate(&topic); err != nil {
		return models.AgentSnapshot{}, err
	}
	return s.SaveTopic(ctx, topic)
}

func mutableTopicSource(topic *models.AgentTopicSnapshot, profileID string, sourceID string) (*models.AgentTopicSource, error) {
	profileIdx := findTopicProfileIndex(*topic, profileID)
	if profileIdx < 0 {
		return nil, errors.New("topic profile not found")
	}
	sourceIdx := findTopicSourceIndex(topic.Profiles[profileIdx].Sources, sourceID)
	if sourceIdx < 0 {
		return nil, errors.New("topic source not found")
	}
	return &topic.Profiles[profileIdx].Sources[sourceIdx], nil
}

func findTopicProfileIndex(topic models.AgentTopicSnapshot, profileID string) int {
	target := firstNonEmpty(profileID, topic.ActiveProfileID)
	for idx, profile := range topic.Profiles {
		if profile.ID == target {
			return idx
		}
	}
	if target == "" && len(topic.Profiles) > 0 {
		return 0
	}
	return -1
}

func findTopicSourceIndex(sources []models.AgentTopicSource, id string) int {
	for idx, source := range sources {
		if source.ID == id {
			return idx
		}
	}
	return -1
}

func firstPositional(flags parsedFlags) string {
	if len(flags.Positionals) == 0 {
		return ""
	}
	return strings.TrimSpace(flags.Positionals[0])
}

func slugID(input string) string {
	id := strings.ToLower(strings.TrimSpace(input))
	id = strings.ReplaceAll(id, " ", "-")
	id = strings.Map(func(r rune) rune {
		if r >= 'a' && r <= 'z' || r >= '0' && r <= '9' || r == '-' || r == '_' {
			return r
		}
		return -1
	}, id)
	if id == "" {
		return uuid.NewString()
	}
	return id
}

func isOneOf(value string, options ...string) bool {
	for _, option := range options {
		if value == option {
			return true
		}
	}
	return false
}

func topicHelpText() string {
	return "Topic commands: /topic run [--profile id], /topic profile list/add/use/delete, /topic source list/add/update/enable/disable/delete, /topic config, /topic state"
}
