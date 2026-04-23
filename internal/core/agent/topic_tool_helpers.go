package agent

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/chentianyu/celestia/internal/models"
	"github.com/google/uuid"
)

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

func (s *Service) useTopicProfile(ctx context.Context, profileID string) (models.AgentTopicSnapshot, error) {
	snapshot, err := s.updateTopic(ctx, func(topic *models.AgentTopicSnapshot) error {
		if findTopicProfileIndex(*topic, profileID) < 0 {
			return errors.New("topic profile not found")
		}
		topic.ActiveProfileID = strings.TrimSpace(profileID)
		return nil
	})
	return snapshot.TopicSummary, err
}

func (s *Service) addTopicProfileTool(ctx context.Context, input topicToolInput) (models.AgentTopicSnapshot, error) {
	name := strings.TrimSpace(input.ProfileName)
	if name == "" {
		return models.AgentTopicSnapshot{}, errors.New("profile_name is required")
	}
	id := firstNonEmpty(input.ProfileID, slugID(name))
	snapshot, err := s.updateTopic(ctx, func(topic *models.AgentTopicSnapshot) error {
		if findTopicProfileIndex(*topic, id) >= 0 {
			return errors.New("topic profile already exists")
		}
		profile := models.AgentTopicProfile{ID: id, Name: name, Sources: []models.AgentTopicSource{}, UpdatedAt: time.Now().UTC()}
		if input.CloneFrom != "" {
			idx := findTopicProfileIndex(*topic, input.CloneFrom)
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
	return snapshot.TopicSummary, err
}

func (s *Service) deleteTopicProfileTool(ctx context.Context, profileID string) (models.AgentTopicSnapshot, error) {
	snapshot, err := s.updateTopic(ctx, func(topic *models.AgentTopicSnapshot) error {
		idx := findTopicProfileIndex(*topic, profileID)
		if idx < 0 {
			return errors.New("topic profile not found")
		}
		topic.Profiles = append(topic.Profiles[:idx], topic.Profiles[idx+1:]...)
		if topic.ActiveProfileID == profileID {
			topic.ActiveProfileID = ""
		}
		return nil
	})
	return snapshot.TopicSummary, err
}

func (s *Service) addTopicSourceTool(ctx context.Context, input topicToolInput) (models.AgentTopicSnapshot, error) {
	if strings.TrimSpace(input.SourceName) == "" || strings.TrimSpace(input.Category) == "" || strings.TrimSpace(input.FeedURL) == "" {
		return models.AgentTopicSnapshot{}, errors.New("source_name, category, and feed_url are required")
	}
	source := models.AgentTopicSource{
		ID:       firstNonEmpty(input.SourceID, slugID(input.SourceName)),
		Name:     strings.TrimSpace(input.SourceName),
		Category: strings.TrimSpace(input.Category),
		FeedURL:  strings.TrimSpace(input.FeedURL),
		Weight:   input.Weight,
		Enabled:  true,
	}
	if source.Weight <= 0 {
		source.Weight = 1
	}
	snapshot, err := s.updateTopic(ctx, func(topic *models.AgentTopicSnapshot) error {
		idx := findTopicProfileIndex(*topic, input.ProfileID)
		if idx < 0 {
			return errors.New("topic profile not found")
		}
		topic.Profiles[idx].Sources = append(topic.Profiles[idx].Sources, source)
		return nil
	})
	return snapshot.TopicSummary, err
}

func (s *Service) updateTopicSourceTool(ctx context.Context, input topicToolInput) (models.AgentTopicSnapshot, error) {
	snapshot, err := s.updateTopic(ctx, func(topic *models.AgentTopicSnapshot) error {
		source, err := mutableTopicSource(topic, input.ProfileID, input.SourceID)
		if err != nil {
			return err
		}
		if input.SourceName != "" {
			source.Name = strings.TrimSpace(input.SourceName)
		}
		if input.Category != "" {
			source.Category = strings.TrimSpace(input.Category)
		}
		if input.FeedURL != "" {
			source.FeedURL = strings.TrimSpace(input.FeedURL)
		}
		if input.Weight > 0 {
			source.Weight = input.Weight
		}
		return nil
	})
	return snapshot.TopicSummary, err
}

func (s *Service) setTopicSourceEnabled(ctx context.Context, profileID string, sourceID string, enabled bool) (models.AgentTopicSnapshot, error) {
	snapshot, err := s.updateTopic(ctx, func(topic *models.AgentTopicSnapshot) error {
		source, err := mutableTopicSource(topic, profileID, sourceID)
		if err != nil {
			return err
		}
		source.Enabled = enabled
		return nil
	})
	return snapshot.TopicSummary, err
}

func (s *Service) deleteTopicSourceTool(ctx context.Context, profileID string, sourceID string) (models.AgentTopicSnapshot, error) {
	snapshot, err := s.updateTopic(ctx, func(topic *models.AgentTopicSnapshot) error {
		profileIdx := findTopicProfileIndex(*topic, profileID)
		if profileIdx < 0 {
			return errors.New("topic profile not found")
		}
		sourceIdx := findTopicSourceIndex(topic.Profiles[profileIdx].Sources, sourceID)
		if sourceIdx < 0 {
			return errors.New("topic source not found")
		}
		sources := topic.Profiles[profileIdx].Sources
		topic.Profiles[profileIdx].Sources = append(sources[:sourceIdx], sources[sourceIdx+1:]...)
		return nil
	})
	return snapshot.TopicSummary, err
}

func (s *Service) clearTopicSentLog(ctx context.Context) (models.AgentTopicSnapshot, error) {
	snapshot, err := s.updateTopic(ctx, func(topic *models.AgentTopicSnapshot) error {
		topic.SentLog = []models.AgentTopicSentLogItem{}
		return nil
	})
	return snapshot.TopicSummary, err
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
