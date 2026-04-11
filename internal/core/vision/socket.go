package vision

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"sync"
	"sync/atomic"

	"nhooyr.io/websocket"
	"nhooyr.io/websocket/wsjson"
)

const visionWSReadLimitBytes = 64 << 20

type wsDialFunc func(context.Context, string, *websocket.DialOptions) (*websocket.Conn, *http.Response, error)

type visionSocket struct {
	conn       *websocket.Conn
	onAsync    func(wsEnvelope)
	onClosed   func(error)
	writeMu    sync.Mutex
	pendingMu  sync.Mutex
	pending    map[string]chan wsEnvelope
	requestSeq atomic.Uint64
	closed     atomic.Bool
}

func dialVisionSocket(
	ctx context.Context,
	wsURL string,
	dial wsDialFunc,
	onAsync func(wsEnvelope),
	onClosed func(error),
) (*visionSocket, error) {
	if dial == nil {
		dial = websocket.Dial
	}
	conn, _, err := dial(ctx, wsURL, nil)
	if err != nil {
		return nil, err
	}
	conn.SetReadLimit(visionWSReadLimitBytes)
	socket := &visionSocket{
		conn:     conn,
		onAsync:  onAsync,
		onClosed: onClosed,
		pending:  map[string]chan wsEnvelope{},
	}
	go socket.readLoop(context.Background())
	return socket, nil
}

func (s *visionSocket) IsClosed() bool {
	return s == nil || s.closed.Load()
}

func (s *visionSocket) Close() {
	s.shutdown(nil)
}

func (s *visionSocket) Request(ctx context.Context, requestType string, payload any, responseType string, out any) error {
	if s == nil || s.IsClosed() {
		return errors.New("vision websocket is not connected")
	}
	requestID := fmt.Sprintf("req-%d", s.requestSeq.Add(1))
	replyCh := make(chan wsEnvelope, 1)
	s.pendingMu.Lock()
	if s.closed.Load() {
		s.pendingMu.Unlock()
		return errors.New("vision websocket is not connected")
	}
	s.pending[requestID] = replyCh
	s.pendingMu.Unlock()
	defer func() {
		s.pendingMu.Lock()
		delete(s.pending, requestID)
		s.pendingMu.Unlock()
	}()

	envelope := wsEnvelope{
		Type:      requestType,
		RequestID: requestID,
	}
	if payload != nil {
		raw, err := json.Marshal(payload)
		if err != nil {
			return err
		}
		envelope.Payload = raw
	}

	s.writeMu.Lock()
	writeErr := wsjson.Write(ctx, s.conn, envelope)
	s.writeMu.Unlock()
	if writeErr != nil {
		return writeErr
	}

	select {
	case <-ctx.Done():
		return ctx.Err()
	case response, ok := <-replyCh:
		if !ok {
			return errors.New("vision websocket disconnected")
		}
		if response.Type == visionMessageTypeError {
			var payload wsErrorPayload
			if err := decodeWSPayload(response, &payload); err != nil {
				return err
			}
			if payload.Code != "" {
				return fmt.Errorf("%s: %s", payload.Code, payload.Message)
			}
			return errors.New(payload.Message)
		}
		if response.Type != responseType {
			return fmt.Errorf("unexpected vision websocket response %q", response.Type)
		}
		if out == nil || len(response.Payload) == 0 {
			return nil
		}
		return json.Unmarshal(response.Payload, out)
	}
}

func (s *visionSocket) readLoop(ctx context.Context) {
	for {
		var envelope wsEnvelope
		if err := wsjson.Read(ctx, s.conn, &envelope); err != nil {
			s.shutdown(err)
			return
		}
		if envelope.RequestID != "" {
			s.pendingMu.Lock()
			replyCh, ok := s.pending[envelope.RequestID]
			s.pendingMu.Unlock()
			if ok {
				replyCh <- envelope
				continue
			}
		}
		if s.onAsync != nil {
			s.onAsync(envelope)
		}
	}
}

func (s *visionSocket) shutdown(reason error) {
	if s == nil || !s.closed.CompareAndSwap(false, true) {
		return
	}
	_ = s.conn.Close(websocket.StatusNormalClosure, "closing")
	s.pendingMu.Lock()
	for requestID, replyCh := range s.pending {
		delete(s.pending, requestID)
		close(replyCh)
	}
	s.pendingMu.Unlock()
	if s.onClosed != nil {
		s.onClosed(reason)
	}
}

func decodeWSPayload[T any](envelope wsEnvelope, out *T) error {
	if len(envelope.Payload) == 0 {
		return nil
	}
	if err := json.Unmarshal(envelope.Payload, out); err != nil {
		return fmt.Errorf("decode vision websocket payload %q: %w", envelope.Type, err)
	}
	return nil
}
