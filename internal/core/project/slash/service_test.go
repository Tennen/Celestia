package slash

import (
	"context"
	"path/filepath"
	"strings"
	"testing"
	"time"

	coreagent "github.com/chentianyu/celestia/internal/core/agent"
	"github.com/chentianyu/celestia/internal/core/audit"
	"github.com/chentianyu/celestia/internal/core/control"
	"github.com/chentianyu/celestia/internal/core/policy"
	"github.com/chentianyu/celestia/internal/core/registry"
	"github.com/chentianyu/celestia/internal/core/state"
	"github.com/chentianyu/celestia/internal/models"
	"github.com/chentianyu/celestia/internal/storage"
	sqlitestore "github.com/chentianyu/celestia/internal/storage/sqlite"
)

type fakeCommandExecutor struct {
	calls []struct {
		device models.Device
		req    models.CommandRequest
	}
}

type fakeAgentRuntime struct {
	snapshot        models.AgentSnapshot
	knowledgeReqs   []models.AgentKnowledgeRequest
	startedSessions []models.AgentKnowledgeRequest
	knowledgeResult models.AgentKnowledgeResult
	knowledgeErr    error
	answerListReqs  []models.AgentKnowledgeAnswersRequest
	answerGetReqs   []models.AgentKnowledgeAnswerRequest
	evolutionGoals  []coreagent.EvolutionGoalRequest
	evolutionOps    []coreagent.EvolutionOperationRequest
	screenshots     []models.AgentScreenshotRequest
	serviceOps      []coreagent.ServiceOperationRequest
}

func (f *fakeAgentRuntime) Snapshot(context.Context) (models.AgentSnapshot, error) {
	return f.snapshot, nil
}

func (f *fakeAgentRuntime) RunMarketAnalysis(context.Context, coreagent.MarketRunRequest) (models.AgentMarketRun, error) {
	return models.AgentMarketRun{}, nil
}

func (f *fakeAgentRuntime) ImportMarketPortfolioCodes(context.Context, models.AgentMarketImportCodesRequest) (models.AgentMarketImportCodesResponse, error) {
	return models.AgentMarketImportCodesResponse{}, nil
}

func (f *fakeAgentRuntime) CreateEvolutionGoal(_ context.Context, req coreagent.EvolutionGoalRequest) (models.AgentEvolutionGoal, error) {
	f.evolutionGoals = append(f.evolutionGoals, req)
	return models.AgentEvolutionGoal{ID: "goal-1", Goal: req.Goal, CommitMessage: req.CommitMessage, Status: "pending", Stage: "queued"}, nil
}

func (f *fakeAgentRuntime) RunEvolutionGoal(_ context.Context, id string) (models.AgentEvolutionGoal, error) {
	return models.AgentEvolutionGoal{ID: id, Goal: "ship feature", Status: "succeeded", Stage: "completed"}, nil
}

func (f *fakeAgentRuntime) RunEvolutionOperation(_ context.Context, req coreagent.EvolutionOperationRequest) (models.AgentEvolutionTestResult, error) {
	f.evolutionOps = append(f.evolutionOps, req)
	return models.AgentEvolutionTestResult{Name: req.Action, OK: true, Output: "ok"}, nil
}

func (f *fakeAgentRuntime) CreateApproval(_ context.Context, req models.AgentApprovalCreateRequest) (models.AgentApprovalRequest, error) {
	return models.AgentApprovalRequest{ID: "approval-1", Action: req.Action, Status: "pending", Title: req.Title}, nil
}

func (f *fakeAgentRuntime) ApproveApproval(_ context.Context, id string, _ models.AgentApprovalDecisionRequest) (models.AgentApprovalRequest, error) {
	return models.AgentApprovalRequest{ID: id, Action: "rebuild", Status: "executed"}, nil
}

func (f *fakeAgentRuntime) RejectApproval(_ context.Context, id string, _ models.AgentApprovalDecisionRequest) (models.AgentApprovalRequest, error) {
	return models.AgentApprovalRequest{ID: id, Action: "rebuild", Status: "rejected"}, nil
}

func (f *fakeAgentRuntime) RunScreenshot(_ context.Context, req models.AgentScreenshotRequest) (models.AgentScreenshotResult, error) {
	f.screenshots = append(f.screenshots, req)
	return models.AgentScreenshotResult{
		URL: req.URL,
		Image: models.AgentMarkdownImage{
			Path:        "/tmp/screenshot.png",
			ContentType: "image/png",
		},
	}, nil
}

func (f *fakeAgentRuntime) RunServiceOperation(_ context.Context, req coreagent.ServiceOperationRequest) (models.AgentTerminalResult, error) {
	f.serviceOps = append(f.serviceOps, req)
	return models.AgentTerminalResult{Command: req.Action, Output: "running pid=123 log=data/runtime/gateway.log"}, nil
}

func (f *fakeAgentRuntime) StartKnowledgeSession(_ context.Context, req models.AgentKnowledgeRequest) (models.AgentKnowledgeSession, error) {
	f.startedSessions = append(f.startedSessions, req)
	session := models.AgentKnowledgeSession{ID: "kb-session", KnowledgeBaseID: req.KnowledgeBaseID, UserID: req.UserID, Active: true, Status: "ready"}
	return session, nil
}

func (f *fakeAgentRuntime) RunKnowledge(_ context.Context, req models.AgentKnowledgeRequest) (models.AgentKnowledgeResult, error) {
	f.knowledgeReqs = append(f.knowledgeReqs, req)
	if f.knowledgeResult.Session.ID == "" {
		f.knowledgeResult = models.AgentKnowledgeResult{
			MarkdownPath: "/tmp/answer.md",
			Images: []models.AgentMarkdownImage{{
				Path:        "/tmp/answer.png",
				ContentType: "image/png",
			}},
			Session: models.AgentKnowledgeSession{
				ID:              "kb-session",
				KnowledgeBaseID: req.KnowledgeBaseID,
				UserID:          req.UserID,
				CodexSessionID:  "codex-session",
				Active:          true,
				Status:          "succeeded",
			},
			Codex: models.AgentCodexResult{OutputFile: "data/agent/knowledge/codex/out.txt"},
		}
	}
	return f.knowledgeResult, f.knowledgeErr
}

func (f *fakeAgentRuntime) ListKnowledgeAnswers(_ context.Context, req models.AgentKnowledgeAnswersRequest) ([]models.AgentKnowledgeAnswer, error) {
	f.answerListReqs = append(f.answerListReqs, req)
	return []models.AgentKnowledgeAnswer{{
		ID:              "20260502-090000-answer",
		KnowledgeBaseID: req.KnowledgeBaseID,
		Filename:        "20260502-090000-answer.md",
		Title:           "Release checklist",
		CreatedAt:       time.Date(2026, 5, 2, 9, 0, 0, 0, time.UTC),
	}}, nil
}

func (f *fakeAgentRuntime) RenderKnowledgeAnswer(_ context.Context, req models.AgentKnowledgeAnswerRequest) (models.AgentKnowledgeAnswerRenderResult, error) {
	f.answerGetReqs = append(f.answerGetReqs, req)
	return models.AgentKnowledgeAnswerRenderResult{
		Answer: models.AgentKnowledgeAnswer{
			ID:              req.ID,
			KnowledgeBaseID: req.KnowledgeBaseID,
			Path:            "/tmp/answer.md",
		},
		Images: []models.AgentMarkdownImage{{
			Path:        "/tmp/answer.png",
			ContentType: "image/png",
		}},
	}, nil
}

func (f *fakeCommandExecutor) ExecuteCommand(_ context.Context, device models.Device, req models.CommandRequest) (models.CommandResponse, error) {
	f.calls = append(f.calls, struct {
		device models.Device
		req    models.CommandRequest
	}{device: device, req: req})
	return models.CommandResponse{Accepted: true, Message: "accepted"}, nil
}

func newSlashHomeTestService(t *testing.T) (*Service, *fakeCommandExecutor, storage.Store) {
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
	registrySvc := registry.New(store)
	stateSvc := state.New(store)
	device := models.Device{
		ID:             "xiaomi:light:kitchen",
		PluginID:       "xiaomi",
		VendorDeviceID: "light-kitchen",
		Kind:           models.DeviceKindLight,
		Name:           "Kitchen Light",
		Online:         true,
		Metadata: map[string]any{
			"controls": []models.DeviceControlSpec{
				{
					ID:       "power",
					Kind:     models.DeviceControlKindToggle,
					Label:    "Power",
					StateKey: "power",
					OnCommand: &models.DeviceControlCommand{
						Action: "set_power",
						Params: map[string]any{"on": true},
					},
					OffCommand: &models.DeviceControlCommand{
						Action: "set_power",
						Params: map[string]any{"on": false},
					},
				},
				{
					ID:       "brightness",
					Kind:     models.DeviceControlKindNumber,
					Label:    "Brightness",
					StateKey: "brightness",
					Command: &models.DeviceControlCommand{
						Action:     "set_brightness",
						ValueParam: "value",
					},
				},
			},
		},
	}
	if err := registrySvc.Upsert(ctx, []models.Device{device}); err != nil {
		t.Fatalf("registry.Upsert() error = %v", err)
	}
	if err := stateSvc.Upsert(ctx, []models.DeviceStateSnapshot{{
		DeviceID: device.ID,
		PluginID: device.PluginID,
		TS:       time.Now().UTC(),
		State: map[string]any{
			"power":      true,
			"brightness": 30,
		},
	}}); err != nil {
		t.Fatalf("state.Upsert() error = %v", err)
	}
	executor := &fakeCommandExecutor{}
	home := control.NewHomeService(store, registrySvc, stateSvc, control.New(), policy.New(), audit.New(store), executor, nil)
	svc := New(home, nil)
	return svc, executor, store
}

func TestRunHomeListShowsDeviceControls(t *testing.T) {
	ctx := context.Background()
	svc, _, _ := newSlashHomeTestService(t)

	result, handled, err := svc.Run(ctx, models.ProjectInputRequest{Input: "/home list kitchen", Actor: "test"})
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if !handled {
		t.Fatal("Run() handled = false, want true")
	}
	for _, want := range []string{"Kitchen Light", "Power", "Brightness"} {
		if !strings.Contains(result.Output, want) {
			t.Fatalf("Run() output = %q, want %q", result.Output, want)
		}
	}
	if result.Metadata["count"] != 1 {
		t.Fatalf("metadata count = %#v, want 1", result.Metadata["count"])
	}
}

func TestRunHomeToggleDispatchesResolvedCommand(t *testing.T) {
	ctx := context.Background()
	svc, executor, _ := newSlashHomeTestService(t)

	result, handled, err := svc.Run(ctx, models.ProjectInputRequest{Input: `/home "Kitchen Light" Power off`, Actor: "admin"})
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if !handled {
		t.Fatal("Run() handled = false, want true")
	}
	if !strings.Contains(result.Output, "Home command accepted") {
		t.Fatalf("Run() output = %q, want accepted text", result.Output)
	}
	if len(executor.calls) != 1 {
		t.Fatalf("executor calls = %d, want 1", len(executor.calls))
	}
	call := executor.calls[0]
	if call.device.ID != "xiaomi:light:kitchen" || call.req.Action != "set_power" {
		t.Fatalf("executor call = %#v", call)
	}
	if got, ok := call.req.Params["on"].(bool); !ok || got {
		t.Fatalf("call.req.Params[on] = %#v, want false", call.req.Params["on"])
	}
}

func TestRunHomeNumberControlDispatchesValueParam(t *testing.T) {
	ctx := context.Background()
	svc, executor, _ := newSlashHomeTestService(t)

	_, handled, err := svc.Run(ctx, models.ProjectInputRequest{Input: `/home "Kitchen Light" Brightness 60`, Actor: "admin"})
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if !handled {
		t.Fatal("Run() handled = false, want true")
	}
	if len(executor.calls) != 1 {
		t.Fatalf("executor calls = %d, want 1", len(executor.calls))
	}
	call := executor.calls[0]
	if call.req.Action != "set_brightness" {
		t.Fatalf("call.req.Action = %q, want set_brightness", call.req.Action)
	}
	if got, ok := call.req.Params["value"].(float64); !ok || got != 60 {
		t.Fatalf("call.req.Params[value] = %#v, want 60", call.req.Params["value"])
	}
}

func TestRunHomeResolvesDeviceAndControlAliases(t *testing.T) {
	ctx := context.Background()
	svc, executor, store := newSlashHomeTestService(t)

	if err := store.UpsertDevicePreference(ctx, models.DevicePreference{
		DeviceID:  "xiaomi:light:kitchen",
		Alias:     "Dinner Lamp",
		UpdatedAt: time.Now().UTC(),
	}); err != nil {
		t.Fatalf("UpsertDevicePreference() error = %v", err)
	}
	if err := store.UpsertDeviceControlPreference(ctx, models.DeviceControlPreference{
		DeviceID:  "xiaomi:light:kitchen",
		ControlID: "brightness",
		Alias:     "Glow",
		Visible:   true,
		UpdatedAt: time.Now().UTC(),
	}); err != nil {
		t.Fatalf("UpsertDeviceControlPreference() error = %v", err)
	}

	_, handled, err := svc.Run(ctx, models.ProjectInputRequest{Input: `/home "Dinner Lamp" Glow 72`, Actor: "admin"})
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if !handled {
		t.Fatal("Run() handled = false, want true")
	}
	if len(executor.calls) != 1 {
		t.Fatalf("executor calls = %d, want 1", len(executor.calls))
	}
	if got, ok := executor.calls[0].req.Params["value"].(float64); !ok || got != 72 {
		t.Fatalf("call.req.Params[value] = %#v, want 72", executor.calls[0].req.Params["value"])
	}
}

func TestRunHomeResolvesGlobalControlAlias(t *testing.T) {
	ctx := context.Background()
	svc, executor, store := newSlashHomeTestService(t)

	if err := store.UpsertDeviceControlPreference(ctx, models.DeviceControlPreference{
		DeviceID:  "xiaomi:light:kitchen",
		ControlID: "brightness",
		Alias:     "Glow",
		Visible:   true,
		UpdatedAt: time.Now().UTC(),
	}); err != nil {
		t.Fatalf("UpsertDeviceControlPreference() error = %v", err)
	}

	_, handled, err := svc.Run(ctx, models.ProjectInputRequest{Input: `/home Glow 55`, Actor: "admin"})
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if !handled {
		t.Fatal("Run() handled = false, want true")
	}
	if len(executor.calls) != 1 {
		t.Fatalf("executor calls = %d, want 1", len(executor.calls))
	}
	if got, ok := executor.calls[0].req.Params["value"].(float64); !ok || got != 55 {
		t.Fatalf("call.req.Params[value] = %#v, want 55", executor.calls[0].req.Params["value"])
	}
}

func TestRunKnowledgeAskDispatchesCodexKnowledgeRuntime(t *testing.T) {
	ctx := context.Background()
	agent := &fakeAgentRuntime{}
	svc := New(nil, agent)

	result, handled, err := svc.Run(ctx, models.ProjectInputRequest{
		Input:     `/kb ask "设备接入流程是什么"`,
		SessionID: "wecom-alice",
		Source:    "wecom",
	})
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if !handled {
		t.Fatal("Run() handled = false, want true")
	}
	if result.Output != "Knowledge answer rendered to image." {
		t.Fatalf("Run() output = %q, want image render status", result.Output)
	}
	if len(result.Images) != 1 || result.Images[0].Path != "/tmp/answer.png" {
		t.Fatalf("Run() images = %+v, want rendered answer image", result.Images)
	}
	if len(agent.knowledgeReqs) != 1 {
		t.Fatalf("knowledge reqs = %d, want 1", len(agent.knowledgeReqs))
	}
	req := agent.knowledgeReqs[0]
	if req.Question != "设备接入流程是什么" || req.UserID != "wecom-alice" || req.Source != "wecom" || req.NewSession {
		t.Fatalf("knowledge req = %+v", req)
	}
	if result.Metadata["codex_session_id"] != "codex-session" {
		t.Fatalf("codex session metadata = %#v", result.Metadata["codex_session_id"])
	}
}

func TestRunKnowledgeAskDispatchesSelectedKnowledgeBase(t *testing.T) {
	ctx := context.Background()
	agent := &fakeAgentRuntime{}
	svc := New(nil, agent)

	_, handled, err := svc.Run(ctx, models.ProjectInputRequest{
		Input:  `/kb @ops ask release checklist`,
		UserID: "alice",
		Source: "wecom",
	})
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if !handled {
		t.Fatal("Run() handled = false, want true")
	}
	if len(agent.knowledgeReqs) != 1 {
		t.Fatalf("knowledge reqs = %d, want 1", len(agent.knowledgeReqs))
	}
	req := agent.knowledgeReqs[0]
	if req.KnowledgeBaseID != "ops" || req.Question != "release checklist" {
		t.Fatalf("knowledge req = %+v", req)
	}
}

func TestRunKnowledgeAnswersListUsesSelectedBase(t *testing.T) {
	ctx := context.Background()
	agent := &fakeAgentRuntime{}
	svc := New(nil, agent)

	result, handled, err := svc.Run(ctx, models.ProjectInputRequest{Input: `/kb @ops answers list`, UserID: "alice"})
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if !handled {
		t.Fatal("Run() handled = false, want true")
	}
	if len(agent.answerListReqs) != 1 || agent.answerListReqs[0].KnowledgeBaseID != "ops" || agent.answerListReqs[0].Limit != 20 {
		t.Fatalf("answer list reqs = %+v", agent.answerListReqs)
	}
	if !strings.Contains(result.Output, "20260502-090000-answer") {
		t.Fatalf("Run() output = %q, want answer id", result.Output)
	}
}

func TestRunKnowledgeAnswersGetRendersImage(t *testing.T) {
	ctx := context.Background()
	agent := &fakeAgentRuntime{}
	svc := New(nil, agent)

	result, handled, err := svc.Run(ctx, models.ProjectInputRequest{Input: `/kb @ops answers get 20260502-090000-answer`, UserID: "alice"})
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if !handled {
		t.Fatal("Run() handled = false, want true")
	}
	if len(agent.answerGetReqs) != 1 || agent.answerGetReqs[0].KnowledgeBaseID != "ops" || agent.answerGetReqs[0].ID != "20260502-090000-answer" {
		t.Fatalf("answer get reqs = %+v", agent.answerGetReqs)
	}
	if len(result.Images) != 1 || result.Images[0].Path != "/tmp/answer.png" {
		t.Fatalf("Run() images = %+v, want rendered answer image", result.Images)
	}
}

func TestRunKnowledgeNewStartsFreshSessionWithoutQuestion(t *testing.T) {
	ctx := context.Background()
	agent := &fakeAgentRuntime{}
	svc := New(nil, agent)

	result, handled, err := svc.Run(ctx, models.ProjectInputRequest{Input: `/kb new`, UserID: "alice", Source: "wecom"})
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if !handled {
		t.Fatal("Run() handled = false, want true")
	}
	if !strings.Contains(result.Output, "kb-session") {
		t.Fatalf("Run() output = %q, want session id", result.Output)
	}
	if len(agent.startedSessions) != 1 {
		t.Fatalf("started sessions = %d, want 1", len(agent.startedSessions))
	}
	if agent.startedSessions[0].UserID != "alice" {
		t.Fatalf("started session user = %q, want alice", agent.startedSessions[0].UserID)
	}
}
