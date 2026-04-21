package agent

import (
	"context"
	"time"

	"github.com/chentianyu/celestia/internal/models"
)

func (s *Service) runScheduler() {
	defer close(s.done)
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-s.stop:
			return
		case <-ticker.C:
			s.runDuePushTasks()
		}
	}
}

func (s *Service) runDuePushTasks() {
	ctx, cancel := context.WithTimeout(context.Background(), 25*time.Second)
	defer cancel()
	snapshot, err := s.Snapshot(ctx)
	if err != nil {
		return
	}
	now := time.Now().UTC()
	changed := false
	userByID := make(map[string]models.AgentPushUser, len(snapshot.Push.Users))
	for _, user := range snapshot.Push.Users {
		userByID[user.ID] = user
	}
	for idx := range snapshot.Push.Tasks {
		task := &snapshot.Push.Tasks[idx]
		if !task.Enabled || task.NextRunAt == nil || task.NextRunAt.After(now) {
			continue
		}
		user := userByID[task.UserID]
		if user.Enabled && user.WeComUser != "" {
			message := task.Text
			conversation, convErr := s.Converse(ctx, models.AgentConversationRequest{
				SessionID: "push:" + task.ID,
				Input:     task.Text,
				Actor:     "scheduler",
			})
			if convErr == nil && conversation.Response != "" {
				message = conversation.Response
			}
			if err := s.SendWeComMessage(ctx, WeComSendRequest{ToUser: user.WeComUser, Text: message}); err != nil {
				_ = s.emit(ctx, models.EventAgentTaskFailed, map[string]any{"task_id": task.ID, "error": err.Error()})
			}
		}
		last := now
		next := now.Add(time.Duration(maxInt(task.IntervalM, 1)) * time.Minute)
		task.LastRunAt = &last
		task.NextRunAt = &next
		changed = true
	}
	if !changed {
		return
	}
	_, _ = s.update(ctx, func(next *models.AgentSnapshot) error {
		next.Push.Tasks = snapshot.Push.Tasks
		next.Push.UpdatedAt = now
		next.UpdatedAt = now
		return nil
	})
}
