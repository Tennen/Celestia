package accounts

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/chentianyu/celestia/internal/core/project/garden"
	"github.com/chentianyu/celestia/internal/models"
	"github.com/google/uuid"
)

const TogglePrefix = "garden:account-toggle:"
const TogglesKey = "project/garden/account-toggles"
const MenuStateKey = "project/garden/account-menu"

type Store interface {
	GetAgentDocument(context.Context, string) (models.AgentDocument, bool, error)
	UpsertAgentDocument(context.Context, models.AgentDocument) error
}
type Menus interface {
	GetWeComMenu(context.Context) (models.AgentWeComMenuSnapshot, error)
	SaveWeComMenu(context.Context, models.AgentWeComMenuConfig) (models.AgentWeComMenuSnapshot, error)
}
type ToggleResult struct {
	MessageID string    `json:"message_id"`
	Sender    string    `json:"sender"`
	Account   Account   `json:"account"`
	Status    string    `json:"status"`
	Error     string    `json:"error,omitempty"`
	CreatedAt time.Time `json:"created_at"`
	Duplicate bool      `json:"-"`
}
type Service struct {
	store       Store
	api         API
	menus       Menus
	toggleMu    sync.Mutex
	syncMu      sync.Mutex
	lifecycleMu sync.Mutex
	cancel      context.CancelFunc
	done        chan struct{}
}

func New(store Store, api API, menus Menus) *Service {
	return &Service{store: store, api: api, menus: menus}
}
func IsMenuKey(key string) bool {
	return strings.HasPrefix(strings.ToLower(strings.TrimSpace(key)), TogglePrefix)
}
func CanToggle(sender string) bool {
	return strings.EqualFold(sender, garden.Owner) || strings.EqualFold(sender, "Jellyfish")
}

func (s *Service) root(ctx context.Context) (string, error) {
	var config garden.Config
	if err := s.load(ctx, garden.ConfigKey, &config); err != nil {
		return "", err
	}
	if !filepath.IsAbs(config.CWD) {
		return "", errors.New("花园项目目录尚未配置")
	}
	return config.CWD, nil
}
func (s *Service) load(ctx context.Context, key string, out any) error {
	d, ok, err := s.store.GetAgentDocument(ctx, key)
	if err != nil || !ok {
		return err
	}
	return json.Unmarshal(d.Payload, out)
}
func (s *Service) save(ctx context.Context, key string, value any) error {
	p, err := json.Marshal(value)
	if err != nil {
		return err
	}
	return s.store.UpsertAgentDocument(ctx, models.AgentDocument{Key: key, Domain: "project.garden.accounts", Payload: p, UpdatedAt: time.Now().UTC()})
}

func (s *Service) Toggle(ctx context.Context, sender, key, messageID string) (ToggleResult, error) {
	if !CanToggle(sender) {
		return ToggleResult{}, errors.New("仅 Frypan 和 Jellyfish 可以切换花园账号自动化")
	}
	id, ok := strings.CutPrefix(key, TogglePrefix)
	if _, err := uuid.Parse(id); !ok || err != nil || messageID == "" {
		return ToggleResult{}, errors.New("花园账号菜单消息无效")
	}
	s.toggleMu.Lock()
	defer s.toggleMu.Unlock()
	var history []ToggleResult
	if err := s.load(ctx, TogglesKey, &history); err != nil {
		return ToggleResult{}, err
	}
	for _, r := range history {
		if r.MessageID == messageID {
			if !strings.EqualFold(r.Sender, sender) || r.Account.ID != id {
				return ToggleResult{}, errors.New("账号菜单消息标识冲突，未切换")
			}
			r.Duplicate = true
			if r.Status != "succeeded" {
				return r, errors.New("该点击已经尝试处理，未重复切换；请在花园管理页核对结果")
			}
			return r, nil
		}
	}
	root, err := s.root(ctx)
	if err != nil {
		return ToggleResult{}, err
	}
	list, err := s.api.Accounts(ctx, root)
	if err != nil {
		return ToggleResult{}, err
	}
	var current *Account
	for i := range list {
		if list[i].ID == id {
			current = &list[i]
			break
		}
	}
	if current == nil {
		return ToggleResult{}, errors.New("该花园账号已不存在，菜单将在下一次账号同步时更新")
	}
	r := ToggleResult{MessageID: messageID, Sender: sender, Account: *current, Status: "applying", CreatedAt: time.Now().UTC()}
	r.Account.AutomationEnabled = !current.AutomationEnabled
	history = append([]ToggleResult{r}, history...)
	if len(history) > 200 {
		history = history[:200]
	}
	// Reserve before the narrow, idempotent PUT. Lost acknowledgements and SSE
	// replays must not toggle the account back again.
	if err = s.save(ctx, TogglesKey, history); err != nil {
		return ToggleResult{}, err
	}
	updated, applyErr := s.api.SetExecution(ctx, root, id, r.Account.AutomationEnabled)
	if applyErr == nil {
		r.Account = updated
		r.Status = "succeeded"
	} else {
		r.Status = "failed"
		r.Error = applyErr.Error()
	}
	history[0] = r
	if err = s.save(context.WithoutCancel(ctx), TogglesKey, history); err != nil {
		return r, err
	}
	if applyErr != nil {
		return r, fmt.Errorf("账号开关未确认，未自动重试，请在花园管理页核对：%w", applyErr)
	}
	return r, nil
}

func (s *Service) Start() {
	s.lifecycleMu.Lock()
	defer s.lifecycleMu.Unlock()
	if s.cancel != nil {
		return
	}
	ctx, cancel := context.WithCancel(context.Background())
	s.cancel = cancel
	s.done = make(chan struct{})
	go func() {
		defer close(s.done)
		timer := time.NewTicker(time.Hour)
		defer timer.Stop()
		for {
			checkCtx, stop := context.WithTimeout(ctx, time.Minute)
			if _, err := s.Sync(checkCtx); err != nil && ctx.Err() == nil {
				log.Printf("[garden-account-menu] sync failed: %v", err)
			}
			stop()
			select {
			case <-ctx.Done():
				return
			case <-timer.C:
			}
		}
	}()
}
func (s *Service) Close() {
	s.lifecycleMu.Lock()
	cancel, done := s.cancel, s.done
	s.lifecycleMu.Unlock()
	if cancel != nil {
		cancel()
		<-done
	}
}
