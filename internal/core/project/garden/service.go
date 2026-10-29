package garden

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/chentianyu/celestia/internal/core/codexapp"
	"github.com/chentianyu/celestia/internal/models"
	"github.com/google/uuid"
)

const (
	ConfigKey   = "project/garden/codex"
	RequestsKey = "project/garden/requests"
	CheckAlerts = "garden:check-alerts"
	GameUpgrade = "garden:game-upgrade"
	Owner       = "Frypan"
)

type Store interface {
	GetAgentDocument(context.Context, string) (models.AgentDocument, bool, error)
	UpsertAgentDocument(context.Context, models.AgentDocument) error
}

type Submitter interface {
	Submit(context.Context, string, string, string, string) (codexapp.Result, error)
}

type Config struct {
	ThreadID string `json:"thread_id"`
	CWD      string `json:"cwd"`
}

type Request struct {
	MessageID  string    `json:"message_id"`
	Action     string    `json:"action"`
	ThreadID   string    `json:"thread_id"`
	TurnID     string    `json:"turn_id,omitempty"`
	Status     string    `json:"status"`
	Error      string    `json:"error,omitempty"`
	Day        string    `json:"day,omitempty"`
	DailyCount int       `json:"daily_count,omitempty"`
	CreatedAt  time.Time `json:"created_at"`
}

type Service struct {
	store Store
	codex Submitter
	mu    sync.Mutex
	now   func() time.Time
}

func New(store Store, codex Submitter) *Service {
	return &Service{store: store, codex: codex, now: time.Now}
}

func IsMenuKey(key string) bool {
	return strings.HasPrefix(strings.ToLower(strings.TrimSpace(key)), "garden:")
}

func promptFor(key string) (string, error) {
	switch key {
	case CheckAlerts:
		return "检查花园脚本最近的告警，判断是游戏问题还是脚本问题，需要修复的话请修复。", nil
	case GameUpgrade:
		return "检查花园游戏是否更新，核对接口协议，必要时更新脚本适配。每次检查都要实测模拟器广告链路，即使游戏版本未变：沿用现有广告队列和账号运行配置，检查登录切换、角色绑定、原生广告加载播放及奖励领取，并在模拟器释放后通过独立查询确认奖励到账。只接受本次检查产生的证据，不能用协议检查、SDK 初始化或历史成功代替广告实测。发现问题需要修复的话请修复并复测；无可用广告、账号不在运行时段或尚未完成时，明确说明广告未验收及原因，不能报告全部正常。", nil
	default:
		return "", errors.New("未知花园菜单动作")
	}
}

// Submit is only called from the verified WeCom bridge event adapter. The
// immutable user-ID restriction is also enforced here before any Codex RPC.
func (s *Service) Submit(ctx context.Context, sender, key, messageID string) (Request, error) {
	if sender != Owner {
		return Request{}, errors.New("只有 Frypan 可以触发花园业务")
	}
	prompt, err := promptFor(key)
	if err != nil {
		return Request{}, err
	}
	if strings.TrimSpace(messageID) == "" {
		return Request{}, errors.New("花园菜单消息缺少唯一标识")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	now := s.now().UTC()
	var config Config
	if err := s.load(ctx, ConfigKey, &config); err != nil {
		return Request{}, err
	}
	if config.ThreadID == "" || !filepath.IsAbs(config.CWD) {
		return Request{}, errors.New("花园 Codex 会话尚未配置")
	}
	var requests []Request
	if err := s.load(ctx, RequestsKey, &requests); err != nil {
		return Request{}, err
	}
	day, count := dailyUsage(requests, key, now)
	if count >= 1 {
		return Request{Action: key, Day: day, DailyCount: count}, fmt.Errorf("%s今日次数已用完（%d/1，北京时间 %s），请明日 00:00 后再试", actionLabel(key), count, day)
	}
	for _, request := range requests {
		if request.MessageID == messageID {
			return request, fmt.Errorf("该点击已处理（%s），请在花园脚本会话查看，未重复提交", request.Status)
		}
	}
	request := Request{MessageID: messageID, Action: key, ThreadID: config.ThreadID, Status: "submitting", Day: day, DailyCount: count + 1, CreatedAt: now}
	requests = append([]Request{request}, requests...)
	if len(requests) > 100 {
		requests = requests[:100]
	}
	// Persist before sending: after a crash or ambiguous RPC failure, the same
	// event must never automatically start another business turn. All reserved
	// attempts, including failures, count against the natural-day limit.
	if err := s.save(ctx, requests); err != nil {
		return Request{}, err
	}
	result, runErr := s.codex.Submit(ctx, config.ThreadID, config.CWD, prompt, uuid.NewSHA1(uuid.NameSpaceOID, []byte(config.ThreadID+":"+messageID)).String())
	request.Status = "submitted"
	request.TurnID = result.TurnID
	if runErr != nil {
		request.Status = "failed"
		request.Error = runErr.Error()
	}
	requests[0] = request
	if err := s.save(context.WithoutCancel(ctx), requests); err != nil {
		return request, err
	}
	if runErr != nil {
		return request, fmt.Errorf("%s未确认提交，今日次数已使用（%d/1，北京时间 %s）；当天不再重复提交，请在原会话确认结果：%w", actionLabel(key), request.DailyCount, request.Day, runErr)
	}
	return request, runErr
}

var beijing = time.FixedZone("Asia/Shanghai", 8*60*60)

func dailyUsage(requests []Request, action string, now time.Time) (string, int) {
	day := now.In(beijing).Format("2006-01-02")
	count := 0
	for _, request := range requests {
		// Derive from CreatedAt so pre-upgrade requests also consume today's
		// allowance. Retaining 100 requests covers at least 50 quota days.
		if request.Action == action && request.CreatedAt.In(beijing).Format("2006-01-02") == day {
			count++
		}
	}
	return day, count
}

func actionLabel(action string) string {
	if action == CheckAlerts {
		return "检查告警"
	}
	return "处理游戏升级"
}

func (s *Service) load(ctx context.Context, key string, target any) error {
	doc, found, err := s.store.GetAgentDocument(ctx, key)
	if err != nil || !found {
		return err
	}
	return json.Unmarshal(doc.Payload, target)
}

func (s *Service) save(ctx context.Context, requests []Request) error {
	payload, err := json.Marshal(requests)
	if err != nil {
		return err
	}
	return s.store.UpsertAgentDocument(ctx, models.AgentDocument{Key: RequestsKey, Domain: "project.garden", Payload: payload, UpdatedAt: time.Now().UTC()})
}
