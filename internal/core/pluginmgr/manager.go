package pluginmgr

import (
	"context"
	"net"
	"os/exec"
	"sync"

	"github.com/chentianyu/celestia/internal/core/eventbus"
	"github.com/chentianyu/celestia/internal/core/registry"
	"github.com/chentianyu/celestia/internal/core/state"
	"github.com/chentianyu/celestia/internal/coreapi"
	"github.com/chentianyu/celestia/internal/models"
	"github.com/chentianyu/celestia/internal/pluginapi"
	"github.com/chentianyu/celestia/internal/storage"
	"google.golang.org/grpc"
)

type Manager struct {
	store    storage.Store
	registry *registry.Service
	state    *state.Service
	bus      *eventbus.Bus

	mu       sync.RWMutex
	runtimes map[string]*managedPlugin
	catalog  map[string]models.CatalogPlugin

	coreAddr   string
	coreServer *grpc.Server
}

type managedPlugin struct {
	record           models.PluginInstallRecord
	manifest         *models.PluginManifest
	health           models.PluginHealth
	running          bool
	lastError        string
	addr             string
	pid              int
	logs             *logBuffer
	stoppedByManager bool

	cmd          *exec.Cmd
	conn         *grpc.ClientConn
	client       pluginapi.PluginClient
	cancel       context.CancelFunc
	streamCancel context.CancelFunc
}

type configServiceServer struct {
	manager *Manager
}

type InstallRequest struct {
	PluginID   string         `json:"plugin_id"`
	BinaryPath string         `json:"binary_path,omitempty"`
	Config     map[string]any `json:"config,omitempty"`
	Metadata   map[string]any `json:"metadata,omitempty"`
}

func New(store storage.Store, registry *registry.Service, state *state.Service, bus *eventbus.Bus) *Manager {
	catalog := map[string]models.CatalogPlugin{}
	for _, item := range BuiltinCatalog() {
		catalog[item.ID] = item
	}
	return &Manager{
		store:    store,
		registry: registry,
		state:    state,
		bus:      bus,
		runtimes: make(map[string]*managedPlugin),
		catalog:  catalog,
	}
}

func (m *Manager) ensureCoreAPI() error {
	m.mu.Lock()
	if m.coreServer != nil && m.coreAddr != "" {
		m.mu.Unlock()
		return nil
	}
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		m.mu.Unlock()
		return err
	}
	server := grpc.NewServer()
	coreapi.RegisterConfigServiceServer(server, &configServiceServer{manager: m})
	m.coreAddr = listener.Addr().String()
	m.coreServer = server
	m.mu.Unlock()

	go func() {
		_ = server.Serve(listener)
	}()
	return nil
}

func (m *Manager) stopCoreAPI() {
	m.mu.Lock()
	server := m.coreServer
	m.coreServer = nil
	m.coreAddr = ""
	m.mu.Unlock()
	if server != nil {
		server.GracefulStop()
	}
}
