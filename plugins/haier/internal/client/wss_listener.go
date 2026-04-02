package client

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"nhooyr.io/websocket"
	"nhooyr.io/websocket/wsjson"
)

const (
	wssHeartbeatInterval = 60 * time.Second
	wssReconnectDelay    = 30 * time.Second
	wssConnectTimeout    = 15 * time.Second
)

// WssListener manages the Haier WebSocket connection for one account.
type WssListener struct {
	mu        sync.Mutex
	client    *HaierClient
	deviceIDs []string
	onUpdate  func(deviceID string, attributes map[string]string)
	started   bool
	connected bool
	stopCh    chan struct{}
}

func NewWssListener(
	client *HaierClient,
	deviceIDs []string,
	onUpdate func(deviceID string, attributes map[string]string),
) *WssListener {
	return &WssListener{
		client:    client,
		deviceIDs: deviceIDs,
		onUpdate:  onUpdate,
		stopCh:    make(chan struct{}),
	}
}

// Start begins the WebSocket listener loop in a background goroutine.
func (l *WssListener) Start(ctx context.Context) {
	l.mu.Lock()
	if l.started {
		l.mu.Unlock()
		return
	}
	l.started = true
	l.mu.Unlock()
	go l.runLoop(ctx)
}

// Stop shuts down the WebSocket listener.
func (l *WssListener) Stop() {
	l.mu.Lock()
	defer l.mu.Unlock()
	if !l.started {
		return
	}
	l.started = false
	l.connected = false
	close(l.stopCh)
}

// IsConnected reports whether the WebSocket connection is currently active.
func (l *WssListener) IsConnected() bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.connected
}

// UpdateDevices replaces the list of device IDs to subscribe to.
func (l *WssListener) UpdateDevices(deviceIDs []string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.deviceIDs = deviceIDs
}

func (l *WssListener) runLoop(ctx context.Context) {
	for {
		select {
		case <-l.stopCh:
			return
		case <-ctx.Done():
			return
		default:
		}

		if err := l.connect(ctx); err != nil {
			l.mu.Lock()
			l.connected = false
			l.mu.Unlock()
			select {
			case <-l.stopCh:
				return
			case <-ctx.Done():
				return
			case <-time.After(wssReconnectDelay):
			}
		}
	}
}

func (l *WssListener) connect(ctx context.Context) error {
	// Re-authenticate if needed before fetching the gateway URL.
	if err := l.client.Authenticate(ctx); err != nil {
		return fmt.Errorf("haier wss auth: %w", err)
	}

	gatewayURL, err := l.client.getWSSGatewayURL(ctx)
	if err != nil {
		return err
	}

	agClientID := l.client.auth.CognitoToken
	wsURL := fmt.Sprintf("%s/userag?token=%s&agClientId=%s", gatewayURL, l.client.auth.CognitoToken, agClientID)

	connCtx, cancel := context.WithTimeout(ctx, wssConnectTimeout)
	conn, _, err := websocket.Dial(connCtx, wsURL, nil)
	cancel()
	if err != nil {
		return fmt.Errorf("haier wss dial: %w", err)
	}
	defer conn.CloseNow()

	l.mu.Lock()
	l.connected = true
	deviceIDs := make([]string, len(l.deviceIDs))
	copy(deviceIDs, l.deviceIDs)
	l.mu.Unlock()

	// Subscribe to bound device updates.
	subMsg := wssMessage{
		AgClientID: agClientID,
		Topic:      "BoundDevs",
		Content: map[string]any{
			"devs": deviceIDs,
		},
	}
	if err := wsjson.Write(ctx, conn, subMsg); err != nil {
		return fmt.Errorf("haier wss subscribe: %w", err)
	}

	// Start heartbeat goroutine.
	heartbeatCtx, cancelHeartbeat := context.WithCancel(ctx)
	defer cancelHeartbeat()
	go l.sendHeartbeats(heartbeatCtx, conn, agClientID)

	// Read loop.
	for {
		select {
		case <-l.stopCh:
			conn.Close(websocket.StatusNormalClosure, "stopping")
			return nil
		case <-ctx.Done():
			conn.Close(websocket.StatusNormalClosure, "context done")
			return nil
		default:
		}

		var raw map[string]any
		if err := wsjson.Read(ctx, conn, &raw); err != nil {
			l.mu.Lock()
			l.connected = false
			l.mu.Unlock()
			return fmt.Errorf("haier wss read: %w", err)
		}

		l.handleRawMessage(raw)
	}
}

func (l *WssListener) sendHeartbeats(ctx context.Context, conn *websocket.Conn, agClientID string) {
	ticker := time.NewTicker(wssHeartbeatInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			hb := wssMessage{
				AgClientID: agClientID,
				Topic:      "HeartBeat",
				Content: map[string]any{
					"sn":       randomHex(32),
					"duration": 0,
				},
			}
			if err := wsjson.Write(ctx, conn, hb); err != nil {
				return
			}
		}
	}
}

func (l *WssListener) handleRawMessage(raw map[string]any) {
	rawBytes, err := json.Marshal(raw)
	if err != nil {
		return
	}
	var msg wssMessage
	if err := json.Unmarshal(rawBytes, &msg); err != nil {
		return
	}
	if msg.Topic != "GenMsgDown" {
		return
	}
	deviceID, attributes, err := parseWSSDeviceUpdate(msg)
	if err != nil || deviceID == "" || len(attributes) == 0 {
		return
	}
	if l.onUpdate != nil {
		l.onUpdate(deviceID, attributes)
	}
}
