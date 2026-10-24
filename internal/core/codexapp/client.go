package codexapp

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"nhooyr.io/websocket"
)

// Client connects to the existing app-server daemon. It never starts a new
// daemon, forks a thread, changes permissions, or approves a tool request.
type Client struct{ SocketPath string }

type Thread struct {
	ID     string `json:"id"`
	Name   string `json:"name"`
	CWD    string `json:"cwd"`
	Status struct {
		Type string `json:"type"`
	} `json:"status"`
}

type Result struct {
	ThreadID string `json:"thread_id"`
	TurnID   string `json:"turn_id"`
}

type rpcMessage struct {
	ID     json.RawMessage `json:"id"`
	Method string          `json:"method"`
	Result json.RawMessage `json:"result"`
	Error  *struct {
		Message string `json:"message"`
	} `json:"error"`
}

type connection struct {
	ws     *websocket.Conn
	ctx    context.Context
	cancel context.CancelFunc
	id     int
}

func (c Client) connect(ctx context.Context) (*connection, error) {
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	path := c.SocketPath
	if path == "" {
		home := os.Getenv("CODEX_HOME")
		if home == "" {
			userHome, err := os.UserHomeDir()
			if err != nil {
				cancel()
				return nil, err
			}
			home = filepath.Join(userHome, ".codex")
		}
		path = filepath.Join(home, "app-server-control", "app-server-control.sock")
	}
	transport := &http.Transport{DialContext: func(ctx context.Context, _, _ string) (net.Conn, error) {
		return (&net.Dialer{}).DialContext(ctx, "unix", path)
	}}
	ws, _, err := websocket.Dial(ctx, "ws://localhost/", &websocket.DialOptions{HTTPClient: &http.Client{Transport: transport}})
	if err != nil {
		cancel()
		return nil, fmt.Errorf("connect Codex app-server: %w", err)
	}
	ws.SetReadLimit(16 << 20)
	conn := &connection{ws: ws, ctx: ctx, cancel: cancel}
	if err := conn.call("initialize", map[string]any{
		"clientInfo":   map[string]any{"name": "celestia", "title": "Celestia", "version": "0.1.0"},
		"capabilities": map[string]any{"experimentalApi": true},
	}, nil); err != nil {
		conn.close()
		return nil, err
	}
	if err := conn.write(map[string]any{"method": "initialized"}); err != nil {
		conn.close()
		return nil, err
	}
	return conn, nil
}

func (c *connection) write(value any) error {
	data, err := json.Marshal(value)
	if err != nil {
		return err
	}
	return c.ws.Write(c.ctx, websocket.MessageText, data)
}

func (c *connection) call(method string, params any, out any) error {
	c.id++
	if err := c.write(map[string]any{"id": c.id, "method": method, "params": params}); err != nil {
		return err
	}
	for {
		var message rpcMessage
		_, data, err := c.ws.Read(c.ctx)
		if err != nil {
			return fmt.Errorf("Codex %s connection closed: %w", method, err)
		}
		if err := json.Unmarshal(data, &message); err != nil {
			return err
		}
		if message.Method != "" {
			// Approval and input requests remain with the existing Codex UI.
			if len(message.ID) > 0 {
				return fmt.Errorf("Codex requires interaction in the existing session: %s", message.Method)
			}
			continue
		}
		var id int
		if json.Unmarshal(message.ID, &id) != nil || id != c.id {
			continue
		}
		if message.Error != nil {
			return fmt.Errorf("Codex %s: %s", method, message.Error.Message)
		}
		if len(message.Result) == 0 {
			return errors.New("Codex response is missing result")
		}
		if out == nil {
			return nil
		}
		return json.Unmarshal(message.Result, out)
	}
}

func (c *connection) close() {
	_ = c.ws.CloseNow()
	c.cancel()
}

func (c Client) ReadThread(ctx context.Context, id string) (Thread, error) {
	conn, err := c.connect(ctx)
	if err != nil {
		return Thread{}, err
	}
	defer conn.close()
	var out struct {
		Thread Thread `json:"thread"`
	}
	err = conn.call("thread/read", map[string]any{"threadId": id, "includeTurns": false}, &out)
	return out.Thread, err
}

func validateThread(thread Thread, id, cwd string) error {
	if thread.ID != id || filepath.Clean(thread.CWD) != filepath.Clean(cwd) {
		return errors.New("花园 Codex 会话或工作目录不匹配，未提交任务")
	}
	if thread.Status.Type != "idle" && thread.Status.Type != "notLoaded" {
		return errors.New("花园脚本会话正在处理其他任务，请完成后再点击")
	}
	return nil
}

func (c Client) Submit(ctx context.Context, id, cwd, prompt, messageID string) (Result, error) {
	conn, err := c.connect(ctx)
	if err != nil {
		return Result{}, err
	}
	defer conn.close()
	var read struct {
		Thread Thread `json:"thread"`
	}
	if err := conn.call("thread/read", map[string]any{"threadId": id, "includeTurns": false}, &read); err != nil {
		return Result{}, err
	}
	if err := validateThread(read.Thread, id, cwd); err != nil {
		return Result{}, err
	}
	// Omit model, sandbox, approval policy and instructions to preserve the
	// existing session's settings and context.
	if err := conn.call("thread/resume", map[string]any{"threadId": id, "excludeTurns": true}, &read); err != nil {
		return Result{}, err
	}
	if err := validateThread(read.Thread, id, cwd); err != nil {
		return Result{}, err
	}
	var out struct {
		Turn struct {
			ID string `json:"id"`
		} `json:"turn"`
	}
	err = conn.call("turn/start", map[string]any{
		"threadId": id, "clientUserMessageId": messageID,
		"input": []map[string]any{{"type": "text", "text": prompt}},
	}, &out)
	if err != nil {
		return Result{}, err
	}
	if out.Turn.ID == "" {
		return Result{}, errors.New("Codex did not return a turn ID; check the existing session before retrying")
	}
	return Result{ThreadID: id, TurnID: out.Turn.ID}, nil
}
