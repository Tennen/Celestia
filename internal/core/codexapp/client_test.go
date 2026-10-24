package codexapp

import (
	"context"
	"encoding/json"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"nhooyr.io/websocket"
)

type testRPC struct {
	ID     int            `json:"id"`
	Method string         `json:"method"`
	Params map[string]any `json:"params"`
}

func testServer(t *testing.T, status, cwd string) (Client, chan testRPC) {
	t.Helper()
	// Unix socket names on macOS must fit within sockaddr_un's short path.
	dir, err := os.MkdirTemp("/tmp", "celestia-codex-")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(dir) })
	path := filepath.Join(dir, "rpc.sock")
	listener, err := net.Listen("unix", path)
	if err != nil {
		t.Fatal(err)
	}
	calls := make(chan testRPC, 20)
	server := &http.Server{Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ws, err := websocket.Accept(w, r, nil)
		if err != nil {
			return
		}
		defer ws.CloseNow()
		for {
			_, data, err := ws.Read(r.Context())
			if err != nil {
				return
			}
			var call testRPC
			if json.Unmarshal(data, &call) != nil {
				return
			}
			calls <- call
			if call.Method == "initialized" {
				continue
			}
			var result any = map[string]any{}
			switch call.Method {
			case "thread/read", "thread/resume":
				result = map[string]any{"thread": map[string]any{"id": "garden-thread", "name": "花园脚本", "cwd": cwd, "status": map[string]any{"type": status}}}
			case "turn/start":
				result = map[string]any{"turn": map[string]any{"id": "turn-1"}}
			}
			data, _ = json.Marshal(map[string]any{"id": call.ID, "result": result})
			if ws.Write(r.Context(), websocket.MessageText, data) != nil {
				return
			}
		}
	})}
	go server.Serve(listener)
	t.Cleanup(func() { _ = server.Close() })
	return Client{SocketPath: path}, calls
}

func TestSubmitReusesThreadWithoutOverridingPermissions(t *testing.T) {
	client, calls := testServer(t, "idle", "/garden")
	result, err := client.Submit(context.Background(), "garden-thread", "/garden", "检查最近告警，需要修复的话修复。", "message-1")
	if err != nil {
		t.Fatal(err)
	}
	if result.ThreadID != "garden-thread" || result.TurnID != "turn-1" {
		t.Fatalf("result=%+v", result)
	}
	var methods []string
	for len(calls) > 0 {
		call := <-calls
		methods = append(methods, call.Method)
		if call.Method == "thread/resume" && !reflect.DeepEqual(call.Params, map[string]any{"threadId": "garden-thread", "excludeTurns": true}) {
			t.Fatalf("resume overrides: %+v", call.Params)
		}
		if call.Method == "turn/start" {
			if len(call.Params) != 3 || call.Params["threadId"] != "garden-thread" || call.Params["clientUserMessageId"] != "message-1" {
				t.Fatalf("turn params=%+v", call.Params)
			}
			input := call.Params["input"].([]any)[0].(map[string]any)
			if input["text"] != "检查最近告警，需要修复的话修复。" {
				t.Fatal(input)
			}
		}
	}
	if !reflect.DeepEqual(methods, []string{"initialize", "initialized", "thread/read", "thread/resume", "turn/start"}) {
		t.Fatal(methods)
	}
}

func TestSubmitRejectsBusyOrWrongWorkspaceBeforeResume(t *testing.T) {
	for _, tc := range []struct{ name, status, cwd string }{{"busy", "active", "/garden"}, {"wrong workspace", "idle", "/another-project"}} {
		t.Run(tc.name, func(t *testing.T) {
			client, calls := testServer(t, tc.status, tc.cwd)
			_, err := client.Submit(context.Background(), "garden-thread", "/garden", "prompt", "message-1")
			if err == nil {
				t.Fatal("expected rejection")
			}
			for len(calls) > 0 {
				call := <-calls
				if strings.HasPrefix(call.Method, "turn/") || call.Method == "thread/resume" {
					t.Fatal("side effect after rejection", call.Method)
				}
			}
		})
	}
}
