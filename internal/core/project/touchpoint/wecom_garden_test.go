package touchpoint

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/chentianyu/celestia/internal/core/project/garden"
	"github.com/chentianyu/celestia/internal/models"
)

type gardenSnapshot struct{ value models.AgentSnapshot }

func (s *gardenSnapshot) Snapshot(context.Context) (models.AgentSnapshot, error) { return s.value, nil }
func (s *gardenSnapshot) UpdateSnapshot(_ context.Context, f func(*models.AgentSnapshot) error) (models.AgentSnapshot, error) {
	err := f(&s.value)
	return s.value, err
}

type gardenRunner struct{ calls int }

func (g *gardenRunner) Submit(_ context.Context, sender, key, msg string) (garden.Request, error) {
	g.calls++
	return garden.Request{Action: key, ThreadID: "thread", TurnID: "turn"}, nil
}

func gardenTestService() (*Service, *gardenSnapshot, *gardenRunner, weComBridgePayload) {
	state := &gardenSnapshot{}
	state.value.Settings.WeCom = models.AgentWeComConfig{Enabled: true, BridgeStreamEnabled: true, BridgeURL: "https://bridge.invalid", CorpID: "corp", AgentID: "100", CorpSecret: "secret"}
	state.value.Push.Users = []models.AgentPushUser{{ID: "frypan", WeComUser: "Frypan", Enabled: true}}
	s := New(state, nil)
	runner := &gardenRunner{}
	s.SetGardenMenuRunner(runner)
	p := weComBridgePayload{MessageID: "message", FromUser: "Frypan", ToUser: "corp", AgentID: "100", MsgType: "event", Event: "click", EventKey: garden.CheckAlerts}
	return s, state, runner, p
}

func TestGardenBridgeRejectsUntrustedIdentityAndWrongEvent(t *testing.T) {
	cases := map[string]func(*gardenSnapshot, *weComBridgePayload){
		"other user":               func(_ *gardenSnapshot, p *weComBridgePayload) { p.FromUser = "Jellyfish" },
		"display alias":            func(_ *gardenSnapshot, p *weComBridgePayload) { p.FromUser = "frypan" },
		"wrong corporation":        func(_ *gardenSnapshot, p *weComBridgePayload) { p.ToUser = "other" },
		"wrong application":        func(_ *gardenSnapshot, p *weComBridgePayload) { p.AgentID = "999" },
		"message instead of click": func(_ *gardenSnapshot, p *weComBridgePayload) { p.MsgType = "text" },
		"non-click event":          func(_ *gardenSnapshot, p *weComBridgePayload) { p.Event = "view" },
		"missing message ID":       func(_ *gardenSnapshot, p *weComBridgePayload) { p.MessageID = "" },
		"disabled owner":           func(s *gardenSnapshot, _ *weComBridgePayload) { s.value.Push.Users[0].Enabled = false },
		"bridge disabled":          func(s *gardenSnapshot, _ *weComBridgePayload) { s.value.Settings.WeCom.BridgeStreamEnabled = false },
	}
	for name, mutate := range cases {
		t.Run(name, func(t *testing.T) {
			s, state, runner, p := gardenTestService()
			mutate(state, &p)
			if err := s.handleWeComBridgePayload(context.Background(), p); err != nil {
				t.Fatal(err)
			}
			if runner.calls != 0 {
				t.Fatal("Codex reached for denied event")
			}
			if state.value.WeComMenu.RecentEvents[0].Status != "denied" {
				t.Fatal(state.value.WeComMenu.RecentEvents)
			}
		})
	}
}

func TestGardenRawXMLCannotSpoofFrypan(t *testing.T) {
	s, state, runner, _ := gardenTestService()
	raw := []byte(`<xml><MsgType>event</MsgType><Event>click</Event><EventKey>garden:check-alerts</EventKey><FromUserName>Frypan</FromUserName><ToUserName>corp</ToUserName><AgentID>100</AgentID></xml>`)
	result, err := s.HandleWeComXML(context.Background(), raw)
	if err != nil {
		t.Fatal(err)
	}
	if runner.calls != 0 || result.Record.DispatchText != "" || state.value.WeComMenu.RecentEvents[0].Status != "blocked" {
		t.Fatal("raw XML reached business dispatch", result)
	}
}

func TestGardenOwnerBridgeClickUsesFixedRecipient(t *testing.T) {
	s, state, runner, p := gardenTestService()
	var recipient, reply string
	bridge := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/proxy/gettoken":
			_, _ = w.Write([]byte(`{"errcode":0,"access_token":"token"}`))
		case "/proxy/send":
			var body struct {
				Message struct {
					ToUser string `json:"touser"`
					Text   struct {
						Content string `json:"content"`
					} `json:"text"`
				} `json:"message"`
			}
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
				t.Error(err)
			}
			recipient = body.Message.ToUser
			reply = body.Message.Text.Content
			_, _ = w.Write([]byte(`{"errcode":0,"errmsg":"ok"}`))
		default:
			t.Error("unexpected path", r.URL.Path)
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer bridge.Close()
	state.value.Settings.WeCom.BridgeURL = bridge.URL
	if err := s.handleWeComBridgePayload(context.Background(), p); err != nil {
		t.Fatal(err)
	}
	if runner.calls != 1 || recipient != "Frypan" || !strings.Contains(reply, "花园脚本") {
		t.Fatalf("calls=%d recipient=%q reply=%q", runner.calls, recipient, reply)
	}
	if state.value.WeComMenu.RecentEvents[0].Status != "submitted" || state.value.WeComMenu.RecentEvents[0].Error != "" {
		t.Fatal(state.value.WeComMenu.RecentEvents)
	}
}
