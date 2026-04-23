package agent

import (
	"encoding/json"
	"time"

	"github.com/chentianyu/celestia/internal/models"
)

func loadAgentConversationsDocument(doc models.AgentDocument, snapshot *models.AgentSnapshot) error {
	var payload agentConversationsDocument
	if err := decodeAgentDocument(doc, &payload); err != nil {
		return err
	}
	snapshot.Conversations = payload.Conversations
	return nil
}

func loadAgentSearchLogDocument(doc models.AgentDocument, snapshot *models.AgentSnapshot) error {
	var payload agentSearchLogDocument
	if err := decodeAgentDocument(doc, &payload); err != nil {
		return err
	}
	snapshot.Search.RecentQueries = payload.RecentQueries
	snapshot.Search.UpdatedAt = firstTime(payload.UpdatedAt, doc.UpdatedAt)
	return nil
}

func loadAgentMemoryRawDocument(doc models.AgentDocument, snapshot *models.AgentSnapshot) error {
	var payload agentMemoryRawDocument
	if err := decodeAgentDocument(doc, &payload); err != nil {
		return err
	}
	snapshot.Memory.RawRecords = payload.RawRecords
	snapshot.Memory.UpdatedAt = maxTime(snapshot.Memory.UpdatedAt, firstTime(payload.UpdatedAt, doc.UpdatedAt))
	return nil
}

func loadAgentMemorySummariesDocument(doc models.AgentDocument, snapshot *models.AgentSnapshot) error {
	var payload agentMemorySummariesDocument
	if err := decodeAgentDocument(doc, &payload); err != nil {
		return err
	}
	snapshot.Memory.Summaries = payload.Summaries
	snapshot.Memory.UpdatedAt = maxTime(snapshot.Memory.UpdatedAt, firstTime(payload.UpdatedAt, doc.UpdatedAt))
	return nil
}

func loadAgentMemoryWindowsDocument(doc models.AgentDocument, snapshot *models.AgentSnapshot) error {
	var payload agentMemoryWindowsDocument
	if err := decodeAgentDocument(doc, &payload); err != nil {
		return err
	}
	snapshot.Memory.Windows = payload.Windows
	snapshot.Memory.UpdatedAt = maxTime(snapshot.Memory.UpdatedAt, firstTime(payload.UpdatedAt, doc.UpdatedAt))
	return nil
}

func loadAgentTopicProfilesDocument(doc models.AgentDocument, snapshot *models.AgentSnapshot) error {
	var payload agentTopicProfilesDocument
	if err := decodeAgentDocument(doc, &payload); err != nil {
		return err
	}
	snapshot.TopicSummary.ActiveProfileID = payload.ActiveProfileID
	snapshot.TopicSummary.Profiles = payload.Profiles
	snapshot.TopicSummary.UpdatedAt = maxTime(snapshot.TopicSummary.UpdatedAt, firstTime(payload.UpdatedAt, doc.UpdatedAt))
	return nil
}

func loadAgentTopicRunsDocument(doc models.AgentDocument, snapshot *models.AgentSnapshot) error {
	var payload agentTopicRunsDocument
	if err := decodeAgentDocument(doc, &payload); err != nil {
		return err
	}
	snapshot.TopicSummary.Runs = payload.Runs
	snapshot.TopicSummary.SentLog = payload.SentLog
	snapshot.TopicSummary.UpdatedAt = maxTime(snapshot.TopicSummary.UpdatedAt, firstTime(payload.UpdatedAt, doc.UpdatedAt))
	return nil
}

func loadAgentWritingTopicsDocument(doc models.AgentDocument, snapshot *models.AgentSnapshot) error {
	var payload agentWritingTopicsDocument
	if err := decodeAgentDocument(doc, &payload); err != nil {
		return err
	}
	snapshot.Writing.Topics = payload.Topics
	snapshot.Writing.UpdatedAt = firstTime(payload.UpdatedAt, doc.UpdatedAt)
	return nil
}

func loadAgentMarketPortfolioDocument(doc models.AgentDocument, snapshot *models.AgentSnapshot) error {
	var payload agentMarketPortfolioDocument
	if err := decodeAgentDocument(doc, &payload); err != nil {
		return err
	}
	snapshot.Market.Portfolio = payload.Portfolio
	snapshot.Market.UpdatedAt = maxTime(snapshot.Market.UpdatedAt, firstTime(payload.UpdatedAt, doc.UpdatedAt))
	return nil
}

func loadAgentMarketConfigDocument(doc models.AgentDocument, snapshot *models.AgentSnapshot) error {
	var payload agentMarketConfigDocument
	if err := decodeAgentDocument(doc, &payload); err != nil {
		return err
	}
	snapshot.Market.Config = payload.Config
	snapshot.Market.UpdatedAt = maxTime(snapshot.Market.UpdatedAt, firstTime(payload.UpdatedAt, doc.UpdatedAt))
	return nil
}

func loadAgentMarketRunsDocument(doc models.AgentDocument, snapshot *models.AgentSnapshot) error {
	var payload agentMarketRunsDocument
	if err := decodeAgentDocument(doc, &payload); err != nil {
		return err
	}
	snapshot.Market.Runs = payload.Runs
	snapshot.Market.UpdatedAt = maxTime(snapshot.Market.UpdatedAt, firstTime(payload.UpdatedAt, doc.UpdatedAt))
	return nil
}

func loadAgentEvolutionGoalsDocument(doc models.AgentDocument, snapshot *models.AgentSnapshot) error {
	var payload agentEvolutionGoalsDocument
	if err := decodeAgentDocument(doc, &payload); err != nil {
		return err
	}
	snapshot.Evolution.Goals = payload.Goals
	snapshot.Evolution.UpdatedAt = firstTime(payload.UpdatedAt, doc.UpdatedAt)
	return nil
}

func loadPlainAgentDocument[T any](apply func(*models.AgentSnapshot, T, time.Time)) func(models.AgentDocument, *models.AgentSnapshot) error {
	return func(doc models.AgentDocument, snapshot *models.AgentSnapshot) error {
		var payload T
		if err := decodeAgentDocument(doc, &payload); err != nil {
			return err
		}
		apply(snapshot, payload, doc.UpdatedAt)
		return nil
	}
}

func loadWrappedAgentDocument[T any](apply func(*models.AgentSnapshot, T, time.Time)) func(models.AgentDocument, *models.AgentSnapshot) error {
	return func(doc models.AgentDocument, snapshot *models.AgentSnapshot) error {
		var wrapped struct {
			Data      T         `json:"data"`
			UpdatedAt time.Time `json:"updated_at"`
		}
		if err := decodeAgentDocument(doc, &wrapped); err != nil {
			return err
		}
		apply(snapshot, wrapped.Data, firstTime(wrapped.UpdatedAt, doc.UpdatedAt))
		return nil
	}
}

func decodeAgentDocument(doc models.AgentDocument, target any) error {
	if len(doc.Payload) == 0 {
		return nil
	}
	return json.Unmarshal(doc.Payload, target)
}

func withUpdatedAt[T any](data T, updatedAt time.Time) struct {
	Data      T         `json:"data"`
	UpdatedAt time.Time `json:"updated_at"`
} {
	return struct {
		Data      T         `json:"data"`
		UpdatedAt time.Time `json:"updated_at"`
	}{
		Data:      data,
		UpdatedAt: updatedAt,
	}
}

func firstTime(value time.Time, fallback time.Time) time.Time {
	if !value.IsZero() {
		return value.UTC()
	}
	return fallback.UTC()
}

func maxTime(left time.Time, right time.Time) time.Time {
	if left.IsZero() {
		return right.UTC()
	}
	if right.IsZero() {
		return left.UTC()
	}
	if right.After(left) {
		return right.UTC()
	}
	return left.UTC()
}

func clearSnapshotPersistenceTimes(snapshot *models.AgentSnapshot) {
	snapshot.UpdatedAt = time.Time{}
	snapshot.Settings.UpdatedAt = time.Time{}
	snapshot.Search.UpdatedAt = time.Time{}
	snapshot.DirectInput.UpdatedAt = time.Time{}
	snapshot.WeComMenu.Config.UpdatedAt = time.Time{}
	snapshot.Push.UpdatedAt = time.Time{}
	snapshot.Memory.UpdatedAt = time.Time{}
	snapshot.TopicSummary.UpdatedAt = time.Time{}
	snapshot.Writing.UpdatedAt = time.Time{}
	snapshot.Market.UpdatedAt = time.Time{}
	snapshot.Evolution.UpdatedAt = time.Time{}
}
