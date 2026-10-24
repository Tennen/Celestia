package touchpoint

import (
	"context"
	"fmt"
	"strings"

	"github.com/chentianyu/celestia/internal/core/project/garden"
	"github.com/chentianyu/celestia/internal/models"
)

func isGardenMenuKey(key string) bool { return garden.IsMenuKey(key) }

func (s *Service) handleGardenBridgeEvent(ctx context.Context, payload weComBridgePayload) error {
	record, err := s.recordWeComEvent(ctx, payload.Event, payload.EventKey, payload.FromUser, payload.ToUser, payload.AgentID)
	if err != nil {
		return err
	}
	snapshot, err := s.Snapshot(ctx)
	if err != nil {
		return err
	}
	config := snapshot.Settings.WeCom
	ownerEnabled := false
	for _, user := range snapshot.Push.Users {
		if user.ID == "frypan" && user.WeComUser == garden.Owner && user.WeComChatID == "" && user.Enabled {
			ownerEnabled = true
		}
	}
	// Only our outgoing SSE connection to the configured signature-verifying
	// bridge enters this adapter; raw XML HTTP ingress never calls it.
	authorized := payload.FromUser == garden.Owner && ownerEnabled &&
		strings.EqualFold(payload.MsgType, "event") && strings.EqualFold(payload.Event, "click") &&
		config.Enabled && config.BridgeStreamEnabled && strings.TrimSpace(config.BridgeURL) != "" &&
		config.CorpID != "" && payload.ToUser == config.CorpID &&
		config.AgentID != "" && payload.AgentID == config.AgentID && strings.TrimSpace(payload.MessageID) != ""
	if !authorized {
		record.Status = "denied"
		record.Error = "仅允许 Frypan 通过当前企微应用的已验签菜单点击触发花园业务"
		return s.saveGardenEvent(ctx, record)
	}
	s.mu.Lock()
	runner := s.garden
	s.mu.Unlock()
	reply := "花园菜单处理器尚未配置"
	record.Status = "failed"
	record.Error = ""
	if runner != nil {
		request, submitErr := runner.Submit(ctx, payload.FromUser, payload.EventKey, payload.MessageID)
		if submitErr != nil {
			reply = submitErr.Error()
		} else {
			record.Status = "submitted"
			reply = fmt.Sprintf("已交给「花园脚本」会话处理：%s（今日次数 %d/1，北京时间 %s）。可在该会话查看进度与结果。", gardenActionLabel(request.Action), request.DailyCount, request.Day)
		}
	}
	if record.Status == "failed" {
		record.Error = reply
	}
	if err := s.saveGardenEvent(ctx, record); err != nil {
		return err
	}
	return s.SendWeComMessage(ctx, WeComSendRequest{ToUser: "frypan", Text: reply})
}

func (s *Service) saveGardenEvent(ctx context.Context, record models.AgentWeComEventRecord) error {
	_, err := s.update(ctx, func(snapshot *models.AgentSnapshot) error {
		for i := range snapshot.WeComMenu.RecentEvents {
			if snapshot.WeComMenu.RecentEvents[i].ID == record.ID {
				snapshot.WeComMenu.RecentEvents[i] = record
				break
			}
		}
		return nil
	})
	return err
}

func gardenActionLabel(key string) string {
	if key == garden.CheckAlerts {
		return "检查告警"
	}
	return "处理游戏升级"
}
