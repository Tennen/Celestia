package client

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/google/uuid"
	"nhooyr.io/websocket"
	"nhooyr.io/websocket/wsjson"
)

const (
	wssHeartbeatInterval = 60 * time.Second
	wssReconnectDelay    = 30 * time.Second
	wssConnectTimeout    = 15 * time.Second
)

// WssListener manages the Haier UWS WebSocket connection for one account.
type WssListener struct {
	mu         sync.Mutex
	client     *UWSClient
	deviceIDs  []string
	onUpdate   func(deviceID string, attributes map[string]string)
	agClientID string
	started    bool
	connected  bool
	conn       *websocket.Conn
	stopCh     chan struct{}
}

func NewWssListener(
	client *UWSClient,
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
	if !l.started {
		l.mu.Unlock()
		return
	}
	l.started = false
	l.connected = false
	l.agClientID = ""
	conn := l.conn
	l.conn = nil
	close(l.stopCh)
	l.mu.Unlock()
	if conn != nil {
		_ = conn.Close(websocket.StatusNormalClosure, "stopping")
	}
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

// SendCommand sends a device control command over the WSS connection.
// Returns an error if the connection is not available.
func (l *WssListener) SendCommand(ctx context.Context, deviceID string, params map[string]any) error {
	l.mu.Lock()
	connected := l.connected
	conn := l.conn
	agClientID := l.agClientID
	l.mu.Unlock()

	if !connected || conn == nil || agClientID == "" {
		return errors.New("uws wss: connection not available, cannot send command")
	}

	cmdList := make([]map[string]any, 0, len(params))
	for name, value := range params {
		cmdList = append(cmdList, map[string]any{
			"name":  name,
			"value": value,
		})
	}

	msg := wssMessage{
		AgClientID: agClientID,
		Topic:      "BatchCmdReq",
		Content: map[string]any{
			"deviceId": deviceID,
			"sn":       uuid.NewString(),
			"cmdList":  cmdList,
		},
	}
	if err := wsjson.Write(ctx, conn, msg); err != nil {
		return fmt.Errorf("uws wss send command: %w", err)
	}
	return nil
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
			l.conn = nil
			l.mu.Unlock()
			log.Printf("haier: WSS connect failed account=%s err=%v", l.client.cfg.NormalizedName(), err)
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
	accountName := l.client.cfg.NormalizedName()

	// Refresh token before connecting.
	if err := l.client.Authenticate(ctx); err != nil {
		return fmt.Errorf("uws wss auth: %w", err)
	}

	gatewayURL, err := l.client.getWSSGatewayURL(ctx)
	if err != nil {
		return err
	}

	accessToken := l.client.auth.AccessToken
	wsURL, agClientID, err := buildWSSConnectURL(gatewayURL, accessToken)
	if err != nil {
		return err
	}

	connCtx, cancel := context.WithTimeout(ctx, wssConnectTimeout)
	conn, _, err := websocket.Dial(connCtx, wsURL, nil)
	cancel()
	if err != nil {
		return fmt.Errorf("uws wss dial: %w", err)
	}
	defer conn.CloseNow()

	l.mu.Lock()
	l.conn = conn
	l.agClientID = agClientID
	deviceIDs := make([]string, len(l.deviceIDs))
	copy(deviceIDs, l.deviceIDs)
	l.mu.Unlock()
	log.Printf("haier: WSS connected account=%s vendor_devices=%d", accountName, len(deviceIDs))

	// Subscribe to bound device updates.
	subMsg := wssMessage{
		AgClientID: agClientID,
		Topic:      "BoundDevs",
		Content: map[string]any{
			"devs": deviceIDs,
		},
	}
	if err := wsjson.Write(ctx, conn, subMsg); err != nil {
		l.mu.Lock()
		l.conn = nil
		l.agClientID = ""
		l.connected = false
		l.mu.Unlock()
		return fmt.Errorf("uws wss subscribe: %w", err)
	}
	l.mu.Lock()
	l.connected = true
	l.mu.Unlock()
	log.Printf("haier: WSS subscribed account=%s vendor_devices=%d", accountName, len(deviceIDs))

	heartbeatCtx, cancelHeartbeat := context.WithCancel(ctx)
	defer cancelHeartbeat()
	go l.sendHeartbeats(heartbeatCtx, conn, agClientID)

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
			l.conn = nil
			l.agClientID = ""
			l.mu.Unlock()
			log.Printf("haier: WSS disconnected account=%s err=%v", accountName, err)
			return fmt.Errorf("uws wss read: %w", err)
		}
		l.handleRawMessage(raw)
	}
}

func (l *WssListener) sendHeartbeats(ctx context.Context, conn *websocket.Conn, clientID string) {
	ticker := time.NewTicker(wssHeartbeatInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			hb := wssMessage{
				AgClientID: clientID,
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
		if err != nil {
			log.Printf("haier: WSS message ignored account=%s err=%v", l.client.cfg.NormalizedName(), err)
		}
		return
	}
	log.Printf("haier: WSS push received account=%s vendor_device_id=%s attrs=%v", l.client.cfg.NormalizedName(), deviceID, attributes)
	if l.onUpdate != nil {
		l.onUpdate(deviceID, attributes)
	}
}
