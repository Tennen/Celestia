package agent

import (
	"context"
	"strings"
	"time"

	coresearch "github.com/chentianyu/celestia/internal/core/search"
	"github.com/chentianyu/celestia/internal/models"
	"github.com/google/uuid"
)

func (s *Service) RunSearch(ctx context.Context, req models.AgentSearchRequest) (models.AgentSearchResult, error) {
	started := time.Now().UTC()
	snapshot, err := s.Snapshot(ctx)
	if err != nil {
		return models.AgentSearchResult{}, err
	}
	run, err := coresearch.Run(ctx, snapshot.Settings.SearchEngines, req)
	s.recordSearchQuery(ctx, req, run.Profile, started, run.Result, err)
	return run.Result, err
}

func (s *Service) recordSearchQuery(ctx context.Context, req models.AgentSearchRequest, profile models.AgentSearchProvider, started time.Time, result models.AgentSearchResult, runErr error) {
	finished := time.Now().UTC()
	entry := models.AgentSearchQueryLog{
		ID:             uuid.NewString(),
		Query:          joinedSearchQueries(req.Plans),
		Plans:          cloneSearchPlans(req.Plans),
		EngineSelector: strings.TrimSpace(req.EngineSelector),
		EngineID:       strings.TrimSpace(profile.ID),
		EngineName:     strings.TrimSpace(profile.Name),
		EngineType:     strings.TrimSpace(profile.Type),
		LogContext:     strings.TrimSpace(req.LogContext),
		MaxItems:       req.MaxItems,
		Status:         "succeeded",
		ResultCount:    len(result.Items),
		ErrorCount:     len(result.Errors),
		Errors:         append([]string{}, result.Errors...),
		SourceChain:    append([]string{}, result.SourceChain...),
		StartedAt:      started,
		FinishedAt:     finished,
		DurationMS:     finished.Sub(started).Milliseconds(),
	}
	if runErr != nil {
		entry.Status = "failed"
		entry.Errors = append(entry.Errors, runErr.Error())
		entry.ErrorCount = len(entry.Errors)
	} else if len(result.Errors) > 0 || len(result.Items) == 0 {
		entry.Status = "degraded"
	}
	_, _ = s.update(ctx, func(snapshot *models.AgentSnapshot) error {
		now := time.Now().UTC()
		snapshot.Search.RecentQueries = append([]models.AgentSearchQueryLog{entry}, snapshot.Search.RecentQueries...)
		snapshot.Search.RecentQueries = truncateList(snapshot.Search.RecentQueries, 50)
		snapshot.Search.UpdatedAt = now
		snapshot.UpdatedAt = now
		return nil
	})
}

func joinedSearchQueries(plans []models.AgentSearchPlan) string {
	queries := make([]string, 0, len(plans))
	for _, plan := range plans {
		if query := strings.TrimSpace(plan.Query); query != "" {
			queries = append(queries, query)
		}
	}
	return strings.Join(queries, " | ")
}

func cloneSearchPlans(plans []models.AgentSearchPlan) []models.AgentSearchPlan {
	out := make([]models.AgentSearchPlan, 0, len(plans))
	for _, plan := range plans {
		plan.Sites = append([]string{}, plan.Sites...)
		out = append(out, plan)
	}
	return out
}
