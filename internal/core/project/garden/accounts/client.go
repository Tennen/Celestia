package accounts

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/cookiejar"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/google/uuid"
)

type Account struct {
	ID                string `json:"id"`
	Name              string `json:"name"`
	AutomationEnabled bool   `json:"automation_enabled"`
}

type API interface {
	Accounts(context.Context, string) ([]Account, error)
	SetExecution(context.Context, string, string, bool) (Account, error)
}

// LocalClient only uses garden's existing authenticated, narrow HTTP API.
// It never writes garden's database or replaces an account configuration.
type LocalClient struct{}

type localSession struct {
	base string
	http *http.Client
}

type wireAccount struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	RoleName string `json:"roleName"`
	Archived bool   `json:"archived"`
	Schedule struct {
		Enabled *bool `json:"enabled"`
	} `json:"schedule"`
}

func (a wireAccount) account() (Account, error) {
	if _, err := uuid.Parse(a.ID); err != nil || a.Schedule.Enabled == nil {
		return Account{}, errors.New("花园账号响应缺少有效 ID 或自动化开关")
	}
	name := strings.TrimSpace(a.RoleName)
	if name == "" {
		name = strings.TrimSpace(a.Name)
	}
	if name == "" {
		name = "未命名账号 " + a.ID[:8]
	}
	return Account{ID: a.ID, Name: name, AutomationEnabled: *a.Schedule.Enabled}, nil
}

func openLocal(ctx context.Context, root string) (*localSession, error) {
	var info struct {
		Port     int    `json:"port"`
		Host     string `json:"host"`
		Instance string `json:"instance"`
	}
	data, err := os.ReadFile(filepath.Join(root, ".local/admin/service.json"))
	if err != nil {
		return nil, fmt.Errorf("读取花园服务信息: %w", err)
	}
	if err = json.Unmarshal(data, &info); err != nil {
		return nil, err
	}
	if info.Host != "127.0.0.1" && info.Host != "localhost" && info.Host != "0.0.0.0" && info.Host != "::" {
		return nil, errors.New("花园服务必须是本机地址")
	}
	if info.Port < 1 || info.Port > 65535 || info.Instance == "" {
		return nil, errors.New("花园服务信息无效")
	}
	jar, _ := cookiejar.New(nil)
	s := &localSession{base: fmt.Sprintf("http://127.0.0.1:%d", info.Port), http: &http.Client{Timeout: 15 * time.Second, Jar: jar, CheckRedirect: func(*http.Request, []*http.Request) error { return errors.New("花园接口不允许重定向") }}}
	var session struct {
		Authenticated bool   `json:"authenticated"`
		NoAuth        bool   `json:"noAuth"`
		Instance      string `json:"serviceInstance"`
	}
	if err = s.request(ctx, http.MethodGet, "/api/session", nil, &session); err != nil {
		return nil, err
	}
	if session.Instance != info.Instance {
		return nil, errors.New("花园服务实例与本机记录不匹配")
	}
	if !session.Authenticated && !session.NoAuth {
		key, err := os.ReadFile(filepath.Join(root, ".local/admin/access-key"))
		if err != nil {
			return nil, fmt.Errorf("读取花园管理口令: %w", err)
		}
		if err = s.request(ctx, http.MethodPost, "/api/login", map[string]string{"password": strings.TrimSpace(string(key))}, nil); err != nil {
			return nil, err
		}
	}
	return s, nil
}

func (s *localSession) request(ctx context.Context, method, path string, body, out any) error {
	var input io.Reader
	if body != nil {
		data, err := json.Marshal(body)
		if err != nil {
			return err
		}
		input = bytes.NewReader(data)
	}
	req, err := http.NewRequestWithContext(ctx, method, s.base+path, input)
	if err != nil {
		return err
	}
	req.Header.Set("Origin", s.base)
	req.Header.Set("Content-Type", "application/json")
	resp, err := s.http.Do(req)
	if err != nil {
		return fmt.Errorf("花园管理接口连接失败: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		var failure struct {
			Error string `json:"error"`
		}
		_ = json.NewDecoder(io.LimitReader(resp.Body, 4096)).Decode(&failure)
		return fmt.Errorf("花园管理接口 HTTP %d: %s", resp.StatusCode, failure.Error)
	}
	if out == nil {
		return nil
	}
	return json.NewDecoder(io.LimitReader(resp.Body, 16<<20)).Decode(out)
}

func (LocalClient) Accounts(ctx context.Context, root string) ([]Account, error) {
	s, err := openLocal(ctx, root)
	if err != nil {
		return nil, err
	}
	var state struct {
		Accounts []wireAccount `json:"accounts"`
	}
	if err = s.request(ctx, http.MethodGet, "/api/state", nil, &state); err != nil {
		return nil, err
	}
	accounts := make([]Account, 0, len(state.Accounts))
	seen := map[string]bool{}
	for _, wire := range state.Accounts {
		if wire.Archived {
			continue
		}
		a, err := wire.account()
		if err != nil {
			return nil, err
		}
		if seen[a.ID] {
			return nil, errors.New("花园返回重复账号 ID")
		}
		seen[a.ID] = true
		accounts = append(accounts, a)
	}
	return accounts, nil
}

func (LocalClient) SetExecution(ctx context.Context, root, id string, enabled bool) (Account, error) {
	if _, err := uuid.Parse(id); err != nil {
		return Account{}, errors.New("花园账号 ID 无效")
	}
	s, err := openLocal(ctx, root)
	if err != nil {
		return Account{}, err
	}
	var wire wireAccount
	if err = s.request(ctx, http.MethodPut, "/api/accounts/"+id+"/execution", map[string]bool{"enabled": enabled}, &wire); err != nil {
		return Account{}, err
	}
	a, err := wire.account()
	if err != nil {
		return Account{}, err
	}
	if a.ID != id || a.AutomationEnabled != enabled {
		return Account{}, errors.New("花园未确认自动化开关结果，请在管理页核对")
	}
	return a, nil
}
