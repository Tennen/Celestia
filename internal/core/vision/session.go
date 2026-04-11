package vision

import (
	"context"
	"errors"
	"log"
	"strings"
	"sync"
	"time"

	"github.com/chentianyu/celestia/internal/models"
	"nhooyr.io/websocket"
)

const (
	visionWSConnectTimeout = 10 * time.Second
	visionWSRequestTimeout = 10 * time.Second
	visionWSReconnectDelay = 5 * time.Second
)

type sessionManager struct {
	service        *Service
	dial           wsDialFunc
	reconnectDelay time.Duration
	requestTimeout time.Duration

	connectMu sync.Mutex
	mu        sync.Mutex
	socket    *visionSocket
	socketURL string
	desired   models.VisionCapabilityConfig
	started   bool
	lastError string

	loopCtx        context.Context
	loopCancel     context.CancelFunc
	loopWG         sync.WaitGroup
	triggerCh      chan struct{}
	disconnectedCh chan struct{}
}

func newSessionManager(service *Service) *sessionManager {
	return &sessionManager{
		service:        service,
		dial:           websocket.Dial,
		reconnectDelay: visionWSReconnectDelay,
		requestTimeout: visionWSRequestTimeout,
		triggerCh:      make(chan struct{}, 1),
		disconnectedCh: make(chan struct{}, 1),
	}
}

func (m *sessionManager) Init(ctx context.Context) error {
	config, err := m.service.GetConfig(ctx)
	if err != nil {
		return err
	}
	m.mu.Lock()
	m.desired = config
	if m.started {
		m.mu.Unlock()
		m.signal()
		return nil
	}
	m.loopCtx, m.loopCancel = context.WithCancel(context.Background())
	m.started = true
	m.mu.Unlock()

	m.loopWG.Add(1)
	go m.run()
	m.signal()
	return nil
}

func (m *sessionManager) Close() {
	m.mu.Lock()
	if !m.started {
		m.mu.Unlock()
		return
	}
	cancel := m.loopCancel
	m.started = false
	m.mu.Unlock()
	if cancel != nil {
		cancel()
	}
	m.closeActiveSocket()
	m.loopWG.Wait()
}

func (m *sessionManager) SyncConfig(ctx context.Context, config models.VisionCapabilityConfig) (models.VisionCapabilityStatus, error) {
	m.mu.Lock()
	m.desired = config
	started := m.started
	m.mu.Unlock()

	if started {
		m.signal()
	}
	if !config.RecognitionEnabled {
		m.closeActiveSocket()
		return m.syncStoppedStatus(ctx)
	}
	if config.ServiceWSURL == "" {
		m.closeActiveSocket()
		status, err := m.syncFailureStatus(ctx, "vision service websocket URL not configured", "service_ws_url is required to sync enabled vision rules")
		return status, err
	}

	var (
		socket *visionSocket
		err    error
	)
	if started {
		socket, err = m.ensureActiveSocket(ctx, config.ServiceWSURL)
	} else {
		socket, err = m.openTemporarySocket(ctx, config.ServiceWSURL)
	}
	if err != nil {
		status, statusErr := m.syncFailureStatus(ctx, "vision websocket connect failed", err.Error())
		return status, statusErr
	}
	if !started {
		defer socket.Close()
	}
	if err := m.applyDesiredState(ctx, socket, config); err != nil {
		if started {
			m.closeActiveSocket()
			m.signal()
		}
		status, statusErr := m.syncFailureStatus(ctx, "vision websocket sync failed", err.Error())
		return status, statusErr
	}
	m.clearLastError()
	return m.syncSuccessStatus(ctx)
}

func (m *sessionManager) FetchCatalog(ctx context.Context, serviceWSURL, modelName string) (models.VisionEntityCatalog, error) {
	socket, temporary, err := m.socketForCatalog(ctx, serviceWSURL)
	if err != nil {
		return models.VisionEntityCatalog{}, err
	}
	if temporary {
		defer socket.Close()
	}
	payload := wsGetEntitiesPayload{}
	if modelName != "" {
		payload.ModelName = modelName
	}
	var response models.VisionServiceEntityCatalog
	if err := m.requestWithTimeout(ctx, socket, visionMessageTypeGetEntities, payloadOrNil(payload), visionMessageTypeEntityCatalog, &response); err != nil {
		return models.VisionEntityCatalog{}, err
	}
	return normalizeCatalog(serviceWSURL, response)
}

func (m *sessionManager) run() {
	defer m.loopWG.Done()
	for {
		config := m.currentDesired()
		if !config.RecognitionEnabled || config.ServiceWSURL == "" {
			m.closeActiveSocket()
			select {
			case <-m.loopCtx.Done():
				return
			case <-m.triggerCh:
				continue
			}
		}

		socket, err := m.ensureActiveSocket(m.loopCtx, config.ServiceWSURL)
		if err != nil {
			m.recordLoopError("vision websocket connect failed", err)
			if !m.waitRetryOrTrigger() {
				return
			}
			continue
		}
		if err := m.applyDesiredState(m.loopCtx, socket, config); err != nil {
			m.recordLoopError("vision websocket sync failed", err)
			m.closeActiveSocket()
			if !m.waitRetryOrTrigger() {
				return
			}
			continue
		}
		m.clearLastError()

		select {
		case <-m.loopCtx.Done():
			return
		case <-m.triggerCh:
		case <-m.disconnectedCh:
		}
	}
}

func (m *sessionManager) waitRetryOrTrigger() bool {
	timer := time.NewTimer(m.reconnectDelay)
	defer timer.Stop()
	select {
	case <-m.loopCtx.Done():
		return false
	case <-m.triggerCh:
		return true
	case <-timer.C:
		return true
	}
}

func (m *sessionManager) socketForCatalog(ctx context.Context, serviceWSURL string) (*visionSocket, bool, error) {
	m.mu.Lock()
	socket := m.socket
	socketURL := m.socketURL
	m.mu.Unlock()
	if socket != nil && !socket.IsClosed() && socketURL == serviceWSURL {
		return socket, false, nil
	}
	socket, err := m.openTemporarySocket(ctx, serviceWSURL)
	if err != nil {
		return nil, false, err
	}
	return socket, true, nil
}

func (m *sessionManager) ensureActiveSocket(ctx context.Context, serviceWSURL string) (*visionSocket, error) {
	m.connectMu.Lock()
	defer m.connectMu.Unlock()

	m.mu.Lock()
	if m.socket != nil && !m.socket.IsClosed() && m.socketURL == serviceWSURL {
		socket := m.socket
		m.mu.Unlock()
		return socket, nil
	}
	stale := m.socket
	m.socket = nil
	m.socketURL = ""
	m.mu.Unlock()

	if stale != nil {
		stale.Close()
	}

	socket, err := m.openSocket(ctx, serviceWSURL)
	if err != nil {
		return nil, err
	}

	m.mu.Lock()
	m.socket = socket
	m.socketURL = serviceWSURL
	m.mu.Unlock()
	return socket, nil
}

func (m *sessionManager) openTemporarySocket(ctx context.Context, serviceWSURL string) (*visionSocket, error) {
	connectCtx, cancel := context.WithTimeout(ctx, visionWSConnectTimeout)
	defer cancel()
	return dialVisionSocket(connectCtx, serviceWSURL, m.dial, nil, nil)
}

func (m *sessionManager) openSocket(ctx context.Context, serviceWSURL string) (*visionSocket, error) {
	connectCtx, cancel := context.WithTimeout(ctx, visionWSConnectTimeout)
	defer cancel()
	socket, err := dialVisionSocket(
		connectCtx,
		serviceWSURL,
		m.dial,
		m.handleAsyncMessage,
		func(err error) {
			m.handleSocketClosed(serviceWSURL, err)
		},
	)
	if err != nil {
		return nil, err
	}
	return socket, nil
}

func (m *sessionManager) closeActiveSocket() {
	m.connectMu.Lock()
	defer m.connectMu.Unlock()

	m.mu.Lock()
	socket := m.socket
	m.socket = nil
	m.socketURL = ""
	m.mu.Unlock()
	if socket != nil {
		socket.Close()
	}
}

func (m *sessionManager) applyDesiredState(ctx context.Context, socket *visionSocket, config models.VisionCapabilityConfig) error {
	if err := m.selectModel(ctx, socket, config.ModelName); err != nil {
		return err
	}
	payload, err := m.service.buildSyncPayload(ctx, config)
	if err != nil {
		return err
	}
	var response wsSyncAppliedPayload
	if err := m.requestWithTimeout(ctx, socket, visionMessageTypeSyncConfig, payload, visionMessageTypeSyncApplied, &response); err != nil {
		return err
	}
	if !response.OK {
		return errors.New("vision service rejected sync_config")
	}
	return nil
}

func (m *sessionManager) selectModel(ctx context.Context, socket *visionSocket, modelName string) error {
	var payload wsSelectModelPayload
	if normalized := normalizeModelName(modelName); normalized != "" {
		payload.ModelName = &normalized
	}
	var response wsModelSelectedPayload
	if err := m.requestWithTimeout(ctx, socket, visionMessageTypeSelectModel, payload, visionMessageTypeModelSelected, &response); err != nil {
		return err
	}
	if !response.OK {
		return errors.New("vision service rejected select_model")
	}
	return nil
}

func (m *sessionManager) requestWithTimeout(ctx context.Context, socket *visionSocket, requestType string, payload any, responseType string, out any) error {
	requestCtx, cancel := context.WithTimeout(ctx, m.requestTimeout)
	defer cancel()
	return socket.Request(requestCtx, requestType, payload, responseType, out)
}

func (m *sessionManager) handleAsyncMessage(envelope wsEnvelope) {
	ctx := context.Background()
	switch envelope.Type {
	case visionMessageTypeHello:
		var hello wsHelloPayload
		if err := decodeWSPayload(envelope, &hello); err != nil {
			log.Printf("vision: decode hello failed: %v", err)
		}
	case visionMessageTypeRuntimeStatus:
		var payload models.VisionServiceStatusReport
		if err := decodeWSPayload(envelope, &payload); err != nil {
			log.Printf("vision: decode runtime_status failed: %v", err)
			return
		}
		if _, err := m.service.ReportStatus(ctx, payload); err != nil {
			log.Printf("vision: apply runtime_status failed: %v", err)
		}
	case visionMessageTypeRuleEvents:
		var payload wsRuleEventsPayload
		if err := decodeWSPayload(envelope, &payload); err != nil {
			log.Printf("vision: decode rule_events failed: %v", err)
			return
		}
		if err := m.service.ReportEvents(ctx, models.VisionServiceEventBatch{Events: payload.Events}); err != nil {
			log.Printf("vision: apply rule_events failed: %v", err)
		}
	case visionMessageTypeEvidence:
		var payload wsEvidencePayload
		if err := decodeWSPayload(envelope, &payload); err != nil {
			log.Printf("vision: decode evidence failed: %v", err)
			return
		}
		if err := m.service.ReportEvidence(ctx, models.VisionServiceEventCaptureBatch{Captures: payload.Captures}); err != nil {
			log.Printf("vision: apply evidence failed: %v", err)
		}
	case visionMessageTypeError:
		var payload wsErrorPayload
		if err := decodeWSPayload(envelope, &payload); err != nil {
			log.Printf("vision: decode websocket error failed: %v", err)
			return
		}
		log.Printf("vision: websocket error code=%s message=%s", payload.Code, payload.Message)
	default:
		log.Printf("vision: ignored websocket message type=%s", envelope.Type)
	}
}

func (m *sessionManager) handleSocketClosed(serviceWSURL string, reason error) {
	m.mu.Lock()
	if m.socketURL == serviceWSURL {
		m.socket = nil
		m.socketURL = ""
	}
	m.mu.Unlock()
	select {
	case m.disconnectedCh <- struct{}{}:
	default:
	}
	if reason != nil && !errors.Is(reason, context.Canceled) {
		log.Printf("vision: websocket disconnected url=%s err=%v", serviceWSURL, reason)
	}
}

func (m *sessionManager) recordLoopError(message string, err error) {
	errText := strings.TrimSpace(err.Error())
	if errText == "" {
		errText = message
	}
	m.mu.Lock()
	if m.lastError == errText {
		m.mu.Unlock()
		return
	}
	m.lastError = errText
	m.mu.Unlock()

	status, statusErr := m.syncFailureStatus(context.Background(), message, errText)
	if statusErr != nil {
		log.Printf("vision: persist failure status failed: %v", statusErr)
		return
	}
	if err := m.service.store.UpsertVisionStatus(context.Background(), status); err != nil {
		log.Printf("vision: upsert failure status failed: %v", err)
		return
	}
	m.service.publishStatusEvent(status)
}

func (m *sessionManager) clearLastError() {
	m.mu.Lock()
	m.lastError = ""
	m.mu.Unlock()
}

func (m *sessionManager) currentDesired() models.VisionCapabilityConfig {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.desired
}

func (m *sessionManager) signal() {
	select {
	case m.triggerCh <- struct{}{}:
	default:
	}
}

func (m *sessionManager) syncSuccessStatus(ctx context.Context) (models.VisionCapabilityStatus, error) {
	status, err := m.service.GetStatus(ctx)
	if err != nil {
		return models.VisionCapabilityStatus{}, err
	}
	now := time.Now().UTC()
	status.Status = models.HealthStateHealthy
	status.Message = "vision websocket config synced"
	status.SyncError = ""
	status.LastSyncedAt = &now
	status.UpdatedAt = now
	return status, nil
}

func (m *sessionManager) syncStoppedStatus(ctx context.Context) (models.VisionCapabilityStatus, error) {
	status, err := m.service.GetStatus(ctx)
	if err != nil {
		return models.VisionCapabilityStatus{}, err
	}
	now := time.Now().UTC()
	status.Status = models.HealthStateStopped
	status.Message = "vision recognition disabled"
	status.SyncError = ""
	status.UpdatedAt = now
	return status, nil
}

func (m *sessionManager) syncFailureStatus(ctx context.Context, message, syncError string) (models.VisionCapabilityStatus, error) {
	status, err := m.service.GetStatus(ctx)
	if err != nil {
		return models.VisionCapabilityStatus{}, err
	}
	now := time.Now().UTC()
	status.Status = models.HealthStateDegraded
	status.Message = message
	status.SyncError = syncError
	status.UpdatedAt = now
	return status, nil
}

func payloadOrNil(payload wsGetEntitiesPayload) any {
	if payload.ModelName == "" {
		return nil
	}
	return payload
}
