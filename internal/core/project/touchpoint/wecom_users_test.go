package touchpoint

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/chentianyu/celestia/internal/core/agent"
	"github.com/chentianyu/celestia/internal/core/eventbus"
	"github.com/chentianyu/celestia/internal/models"
	sqlitestore "github.com/chentianyu/celestia/internal/storage/sqlite"
)

func newTouchpointPersistenceTestService(t *testing.T) (*Service, *agent.Service) {
	t.Helper()
	ctx := context.Background()
	store, err := sqlitestore.New(filepath.Join(t.TempDir(), "celestia.db"))
	if err != nil {
		t.Fatalf("sqlite.New() error = %v", err)
	}
	t.Cleanup(func() {
		_ = store.Close()
	})
	if err := store.EnsureSchema(ctx); err != nil {
		t.Fatalf("EnsureSchema() error = %v", err)
	}
	agentSvc := agent.New(store, eventbus.New())
	return New(agentSvc, agentSvc), agentSvc
}

func TestResolveWeComRecipientRequiresConfiguredEnabledUser(t *testing.T) {
	ctx := context.Background()
	svc, _ := newTouchpointPersistenceTestService(t)
	if _, err := svc.SaveWeComUsers(ctx, models.AgentPushSnapshot{Users: []models.AgentPushUser{
		{ID: "user-alice", Name: "Alice", WeComUser: "alice", Enabled: true},
		{ID: "user-disabled", Name: "Disabled", WeComUser: "disabled", Enabled: false},
	}}); err != nil {
		t.Fatalf("SaveWeComUsers() error = %v", err)
	}

	byID, err := svc.ResolveWeComRecipient(ctx, "user-alice")
	if err != nil {
		t.Fatalf("ResolveWeComRecipient(id) error = %v", err)
	}
	if byID.WeComUser != "alice" {
		t.Fatalf("ResolveWeComRecipient(id) = %+v, want alice", byID)
	}
	byWeComUser, err := svc.ResolveWeComRecipient(ctx, "alice")
	if err != nil {
		t.Fatalf("ResolveWeComRecipient(wecom_user) error = %v", err)
	}
	if byWeComUser.ID != "user-alice" {
		t.Fatalf("ResolveWeComRecipient(wecom_user) = %+v, want user-alice", byWeComUser)
	}
	if _, err := svc.ResolveWeComRecipient(ctx, "disabled"); err == nil || !strings.Contains(err.Error(), "disabled") {
		t.Fatalf("ResolveWeComRecipient(disabled) error = %v, want disabled rejection", err)
	}
	if _, err := svc.ResolveWeComRecipient(ctx, "missing"); err == nil || !strings.Contains(err.Error(), "not a configured recipient") {
		t.Fatalf("ResolveWeComRecipient(missing) error = %v, want configured-recipient rejection", err)
	}
}

func TestResolveWeComRecipientSupportsGroupChat(t *testing.T) {
	ctx := context.Background()
	svc, _ := newTouchpointPersistenceTestService(t)
	if _, err := svc.SaveWeComUsers(ctx, models.AgentPushSnapshot{Users: []models.AgentPushUser{{
		ID: "chat-ops", Name: "Ops", WeComChatID: "wrNqCFEAAAE7gC2VfY", Enabled: true,
	}}}); err != nil {
		t.Fatalf("SaveWeComUsers() error = %v", err)
	}

	byID, err := svc.ResolveWeComRecipient(ctx, "chat-ops")
	if err != nil {
		t.Fatalf("ResolveWeComRecipient(id) error = %v", err)
	}
	if byID.WeComChatID != "wrNqCFEAAAE7gC2VfY" {
		t.Fatalf("ResolveWeComRecipient(id) = %+v, want group chat", byID)
	}
	byChatID, err := svc.ResolveWeComRecipient(ctx, "wrNqCFEAAAE7gC2VfY")
	if err != nil || byChatID.ID != "chat-ops" {
		t.Fatalf("ResolveWeComRecipient(chatid) = %+v, %v", byChatID, err)
	}
}

func TestSavePushValidatesWeComRecipients(t *testing.T) {
	ctx := context.Background()
	svc, _ := newTouchpointPersistenceTestService(t)

	if _, err := svc.SaveWeComUsers(ctx, models.AgentPushSnapshot{Users: []models.AgentPushUser{{Name: "Missing WeCom"}}}); err == nil || !strings.Contains(err.Error(), "user or chat id is required") {
		t.Fatalf("SaveWeComUsers(missing recipient) error = %v, want required rejection", err)
	}
	if _, err := svc.SaveWeComUsers(ctx, models.AgentPushSnapshot{Users: []models.AgentPushUser{
		{Name: "One", WeComUser: "alice", Enabled: true},
		{Name: "Two", WeComUser: "alice", Enabled: true},
	}}); err == nil || !strings.Contains(err.Error(), "must be unique") {
		t.Fatalf("SaveWeComUsers(duplicate wecom_user) error = %v, want duplicate rejection", err)
	}
	if _, err := svc.SaveWeComUsers(ctx, models.AgentPushSnapshot{Users: []models.AgentPushUser{
		{Name: "Invalid", WeComUser: "alice", WeComChatID: "chat-ops", Enabled: true},
	}}); err == nil || !strings.Contains(err.Error(), "not both") {
		t.Fatalf("SaveWeComUsers(user and chat) error = %v, want exclusive-target rejection", err)
	}
}

func TestSendWeComMessageRejectsUnconfiguredTargetBeforeDelivery(t *testing.T) {
	ctx := context.Background()
	svc, _ := newTouchpointPersistenceTestService(t)

	err := svc.SendWeComMessage(ctx, WeComSendRequest{ToUser: "missing", Text: "hello"})
	if err == nil || !strings.Contains(err.Error(), "not a configured recipient") {
		t.Fatalf("SendWeComMessage() error = %v, want configured-recipient rejection", err)
	}
}

func TestSendWeComGroupMessageUsesBridgeAppChatEndpoint(t *testing.T) {
	ctx := context.Background()
	var delivered map[string]any
	bridge := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/proxy/gettoken":
			_, _ = w.Write([]byte(`{"errcode":0,"errmsg":"ok","access_token":"bridge-access-token"}`))
		case "/proxy/appchat/send":
			var envelope struct {
				AccessToken string         `json:"access_token"`
				Message     map[string]any `json:"message"`
			}
			if err := json.NewDecoder(r.Body).Decode(&envelope); err != nil {
				t.Errorf("decode appchat payload: %v", err)
			}
			if envelope.AccessToken != "bridge-access-token" {
				t.Errorf("access_token = %q", envelope.AccessToken)
			}
			delivered = envelope.Message
			_, _ = w.Write([]byte(`{"errcode":0,"errmsg":"ok"}`))
		default:
			t.Errorf("unexpected bridge path %s", r.URL.Path)
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	t.Cleanup(bridge.Close)

	svc, agentSvc := newTouchpointPersistenceTestService(t)
	configureWeComGroupTest(t, ctx, svc, agentSvc, bridge.URL)
	if err := svc.SendWeComMessage(ctx, WeComSendRequest{ToUser: "chat-ops", Text: "alarm"}); err != nil {
		t.Fatalf("SendWeComMessage() error = %v", err)
	}
	if delivered["chatid"] != "wrNqCFEAAAE7gC2VfY" || delivered["msgtype"] != "text" {
		t.Fatalf("appchat payload = %#v", delivered)
	}
	if _, exists := delivered["touser"]; exists {
		t.Fatalf("appchat payload unexpectedly contains touser: %#v", delivered)
	}
	if _, exists := delivered["agentid"]; exists {
		t.Fatalf("appchat payload unexpectedly contains agentid: %#v", delivered)
	}
}

func TestSendWeComGroupMessageUsesDirectAppChatEndpoint(t *testing.T) {
	ctx := context.Background()
	var delivered map[string]any
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/cgi-bin/gettoken":
			_, _ = w.Write([]byte(`{"errcode":0,"errmsg":"ok","access_token":"direct-access-token"}`))
		case "/cgi-bin/appchat/send":
			if r.URL.Query().Get("access_token") != "direct-access-token" {
				t.Errorf("access_token = %q", r.URL.Query().Get("access_token"))
			}
			if err := json.NewDecoder(r.Body).Decode(&delivered); err != nil {
				t.Errorf("decode appchat payload: %v", err)
			}
			_, _ = w.Write([]byte(`{"errcode":0,"errmsg":"ok"}`))
		default:
			t.Errorf("unexpected direct path %s", r.URL.Path)
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	t.Cleanup(upstream.Close)

	svc, agentSvc := newTouchpointPersistenceTestService(t)
	configureWeComGroupTest(t, ctx, svc, agentSvc, "")
	if _, err := agentSvc.UpdateSnapshot(ctx, func(snapshot *models.AgentSnapshot) error {
		snapshot.Settings.WeCom.BaseURL = upstream.URL
		return nil
	}); err != nil {
		t.Fatalf("UpdateSnapshot() error = %v", err)
	}
	if err := svc.SendWeComMessage(ctx, WeComSendRequest{ToUser: "chat-ops", Text: "alarm"}); err != nil {
		t.Fatalf("SendWeComMessage() error = %v", err)
	}
	if delivered["chatid"] != "wrNqCFEAAAE7gC2VfY" || delivered["msgtype"] != "text" {
		t.Fatalf("appchat payload = %#v", delivered)
	}
}

func configureWeComGroupTest(t *testing.T, ctx context.Context, svc *Service, agentSvc *agent.Service, bridgeURL string) {
	t.Helper()
	if _, err := agentSvc.UpdateSnapshot(ctx, func(snapshot *models.AgentSnapshot) error {
		snapshot.Settings.WeCom = models.AgentWeComConfig{
			Enabled: true, CorpID: "corp-id", CorpSecret: "corp-secret", AgentID: "1000001", BridgeURL: bridgeURL,
		}
		return nil
	}); err != nil {
		t.Fatalf("UpdateSnapshot() error = %v", err)
	}
	if _, err := svc.SaveWeComUsers(ctx, models.AgentPushSnapshot{Users: []models.AgentPushUser{{
		ID: "chat-ops", Name: "Ops", WeComChatID: "wrNqCFEAAAE7gC2VfY", Enabled: true,
	}}}); err != nil {
		t.Fatalf("SaveWeComUsers() error = %v", err)
	}
}

func TestSaveWeComMenuUsesBridgeWhenConfigured(t *testing.T) {
	ctx := context.Background()
	var mu sync.Mutex
	var tokenPayload map[string]string
	var menuPayload struct {
		AccessToken string         `json:"access_token"`
		AgentID     string         `json:"agentid"`
		Menu        map[string]any `json:"menu"`
	}
	bridge := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer bridge-secret" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		switch r.URL.Path {
		case "/proxy/gettoken":
			var payload map[string]string
			if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
				t.Errorf("decode token payload: %v", err)
				w.WriteHeader(http.StatusBadRequest)
				return
			}
			mu.Lock()
			tokenPayload = payload
			mu.Unlock()
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"errcode":0,"errmsg":"ok","access_token":"bridge-access-token"}`))
		case "/proxy/menu/create":
			var payload struct {
				AccessToken string         `json:"access_token"`
				AgentID     string         `json:"agentid"`
				Menu        map[string]any `json:"menu"`
			}
			if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
				t.Errorf("decode menu payload: %v", err)
				w.WriteHeader(http.StatusBadRequest)
				return
			}
			mu.Lock()
			menuPayload = payload
			mu.Unlock()
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"errcode":0,"errmsg":"ok"}`))
		default:
			t.Errorf("unexpected bridge path %s", r.URL.Path)
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	t.Cleanup(bridge.Close)

	svc, agentSvc := newTouchpointPersistenceTestService(t)
	if _, err := agentSvc.UpdateSnapshot(ctx, func(snapshot *models.AgentSnapshot) error {
		snapshot.Settings.WeCom = models.AgentWeComConfig{
			Enabled:     true,
			CorpID:      "corp-id",
			CorpSecret:  "corp-secret",
			AgentID:     "1000001",
			BaseURL:     "http://127.0.0.1:1",
			BridgeURL:   bridge.URL,
			BridgeToken: "bridge-secret",
		}
		return nil
	}); err != nil {
		t.Fatalf("UpdateSnapshot() error = %v", err)
	}
	menu, err := svc.SaveWeComMenu(ctx, models.AgentWeComMenuConfig{Buttons: []models.AgentWeComButton{{
		ID:           "menu-status",
		Name:         "Status",
		Key:          "status",
		Enabled:      true,
		DispatchText: "/status",
	}}})
	if err != nil {
		t.Fatalf("SaveWeComMenu() error = %v", err)
	}
	if menu.Config.LastPublishedAt == nil {
		t.Fatalf("SaveWeComMenu() did not record last_published_at")
	}

	mu.Lock()
	defer mu.Unlock()
	if tokenPayload["corpid"] != "corp-id" || tokenPayload["corpsecret"] != "corp-secret" {
		t.Fatalf("bridge token payload = %+v", tokenPayload)
	}
	if menuPayload.AccessToken != "bridge-access-token" || menuPayload.AgentID != "1000001" {
		t.Fatalf("bridge menu payload token/agent = %+v", menuPayload)
	}
	buttons, ok := menuPayload.Menu["button"].([]any)
	if !ok || len(buttons) != 1 {
		t.Fatalf("bridge menu buttons = %#v", menuPayload.Menu["button"])
	}
	button, ok := buttons[0].(map[string]any)
	if !ok || button["type"] != "click" || button["name"] != "Status" || button["key"] != "status" {
		t.Fatalf("bridge menu button = %#v", buttons[0])
	}
}

func TestSaveWeComMenuIncludesBridgeErrorBody(t *testing.T) {
	ctx := context.Background()
	bridge := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/proxy/gettoken":
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"errcode":0,"errmsg":"ok","access_token":"bridge-access-token"}`))
		case "/proxy/menu/create":
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusBadGateway)
			_, _ = w.Write([]byte(`{"errcode":40058,"errmsg":"invalid button name"}`))
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	t.Cleanup(bridge.Close)

	svc, agentSvc := newTouchpointPersistenceTestService(t)
	if _, err := agentSvc.UpdateSnapshot(ctx, func(snapshot *models.AgentSnapshot) error {
		snapshot.Settings.WeCom = models.AgentWeComConfig{
			Enabled:    true,
			CorpID:     "corp-id",
			CorpSecret: "corp-secret",
			AgentID:    "1000001",
			BridgeURL:  bridge.URL,
		}
		return nil
	}); err != nil {
		t.Fatalf("UpdateSnapshot() error = %v", err)
	}
	_, err := svc.SaveWeComMenu(ctx, models.AgentWeComMenuConfig{Buttons: []models.AgentWeComButton{{
		ID:           "menu-status",
		Name:         "Status",
		Key:          "status",
		Enabled:      true,
		DispatchText: "/status",
	}}})
	if err == nil || !strings.Contains(err.Error(), "invalid button name") {
		t.Fatalf("SaveWeComMenu() error = %v, want bridge body detail", err)
	}
}

func TestGetAndDeleteWeComMenuUseBridgeWhenConfigured(t *testing.T) {
	ctx := context.Background()
	var mu sync.Mutex
	paths := []string{}
	bridge := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		paths = append(paths, r.URL.Path)
		mu.Unlock()
		switch r.URL.Path {
		case "/proxy/gettoken":
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"errcode":0,"errmsg":"ok","access_token":"bridge-access-token"}`))
		case "/proxy/menu/get":
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"errcode":0,"errmsg":"ok","button":[{"type":"click","name":"Status","key":"status"}]}`))
		case "/proxy/menu/delete":
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"errcode":0,"errmsg":"ok"}`))
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	t.Cleanup(bridge.Close)

	svc, agentSvc := newTouchpointPersistenceTestService(t)
	if _, err := agentSvc.UpdateSnapshot(ctx, func(snapshot *models.AgentSnapshot) error {
		snapshot.Settings.WeCom = models.AgentWeComConfig{
			Enabled:    true,
			CorpID:     "corp-id",
			CorpSecret: "corp-secret",
			AgentID:    "1000001",
			BridgeURL:  bridge.URL,
		}
		return nil
	}); err != nil {
		t.Fatalf("UpdateSnapshot() error = %v", err)
	}

	menu, err := svc.GetWeComMenu(ctx)
	if err != nil {
		t.Fatalf("GetWeComMenu() error = %v", err)
	}
	if len(menu.Config.Buttons) != 1 || menu.Config.Buttons[0].Key != "status" {
		t.Fatalf("GetWeComMenu() = %+v", menu.Config.Buttons)
	}
	deleted, err := svc.DeleteWeComMenu(ctx)
	if err != nil {
		t.Fatalf("DeleteWeComMenu() error = %v", err)
	}
	if len(deleted.Config.Buttons) != 0 {
		t.Fatalf("DeleteWeComMenu() buttons = %+v, want empty", deleted.Config.Buttons)
	}

	mu.Lock()
	defer mu.Unlock()
	if !containsText(paths, "/proxy/menu/get") || !containsText(paths, "/proxy/menu/delete") {
		t.Fatalf("bridge paths = %+v", paths)
	}
}

func TestSaveWeComMenuRejectsUnpublishablePayload(t *testing.T) {
	ctx := context.Background()
	svc, _ := newTouchpointPersistenceTestService(t)

	menu, err := svc.SaveWeComMenu(ctx, models.AgentWeComMenuConfig{})
	if err == nil {
		t.Fatalf("SaveWeComMenu(empty) error = nil, want validation error")
	}
	if len(menu.ValidationErrors) != 1 || !strings.Contains(menu.ValidationErrors[0], "at least one enabled button") {
		t.Fatalf("empty menu validation = %+v", menu.ValidationErrors)
	}

	menu, err = svc.SaveWeComMenu(ctx, models.AgentWeComMenuConfig{Buttons: []models.AgentWeComButton{{
		ID:      "group-empty",
		Name:    "Group",
		Enabled: true,
		SubButtons: []models.AgentWeComButton{{
			ID:      "child-disabled",
			Name:    "Disabled",
			Key:     "disabled",
			Enabled: false,
		}},
	}}})
	if err == nil {
		t.Fatalf("SaveWeComMenu(empty group) error = nil, want validation error")
	}
	if !containsText(menu.ValidationErrors, "at least one enabled sub-button") {
		t.Fatalf("empty group validation = %+v", menu.ValidationErrors)
	}
}

func containsText(values []string, want string) bool {
	for _, value := range values {
		if strings.Contains(value, want) {
			return true
		}
	}
	return false
}
