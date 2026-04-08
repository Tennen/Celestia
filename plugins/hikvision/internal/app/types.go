package app

import (
	"context"
	"sync"
	"time"

	"github.com/chentianyu/celestia/internal/models"
	lanclient "github.com/chentianyu/celestia/plugins/hikvision/internal/client"
	ezvizcloud "github.com/chentianyu/celestia/plugins/hikvision/internal/cloud"
)

type RuntimeMode string

const (
	RuntimeModeLAN   RuntimeMode = "lan"
	RuntimeModeCloud RuntimeMode = "cloud"
)

type CloudConfig = ezvizcloud.Config

type Config struct {
	Mode                RuntimeMode   `json:"mode"`
	PollIntervalSeconds int           `json:"poll_interval_seconds"`
	Cloud               CloudConfig   `json:"cloud,omitempty"`
	Entries             []EntryConfig `json:"entries"`
}

type EntryConfig struct {
	Name                     string `json:"name,omitempty"`
	EntryID                  string `json:"entry_id,omitempty"`
	DeviceID                 string `json:"device_id,omitempty"`
	DeviceSerial             string `json:"device_serial,omitempty"`
	Host                     string `json:"host,omitempty"`
	Port                     int    `json:"port,omitempty"`
	Username                 string `json:"username,omitempty"`
	Password                 string `json:"password,omitempty"`
	Channel                  int    `json:"channel,omitempty"`
	RTSPURL                  string `json:"rtsp_url,omitempty"`
	RTSPHost                 string `json:"rtsp_host,omitempty"`
	RTSPPort                 int    `json:"rtsp_port,omitempty"`
	RTSPPath                 string `json:"rtsp_path,omitempty"`
	RTSPUsername             string `json:"rtsp_username,omitempty"`
	RTSPPassword             string `json:"rtsp_password,omitempty"`
	PTZDefaultSpeed          int    `json:"ptz_default_speed,omitempty"`
	PTZStepMS                int    `json:"ptz_step_ms,omitempty"`
	SDKLibDir                string `json:"sdk_lib_dir,omitempty"`
	MaxStreamSessions        int    `json:"max_stream_sessions,omitempty"`
	StreamIdleTimeoutSeconds int    `json:"stream_idle_timeout_seconds,omitempty"`
	WebRTCNATIP              string `json:"webrtc_nat_ip,omitempty"`
	WebRTCInterface          string `json:"webrtc_interface,omitempty"`
}

func (e EntryConfig) LocalCameraConfig() lanclient.CameraConfig {
	return lanclient.CameraConfig{
		Name:                     e.Name,
		EntryID:                  e.EntryID,
		DeviceID:                 e.DeviceID,
		Host:                     e.Host,
		Port:                     e.Port,
		Username:                 e.Username,
		Password:                 e.Password,
		Channel:                  e.Channel,
		RTSPPort:                 e.RTSPPort,
		RTSPPath:                 e.RTSPPath,
		PTZDefaultSpeed:          e.PTZDefaultSpeed,
		PTZStepMS:                e.PTZStepMS,
		SDKLibDir:                e.SDKLibDir,
		MaxStreamSessions:        e.MaxStreamSessions,
		StreamIdleTimeoutSeconds: e.StreamIdleTimeoutSeconds,
		WebRTCNATIP:              e.WebRTCNATIP,
		WebRTCInterface:          e.WebRTCInterface,
	}
}

type Plugin struct {
	mu          sync.RWMutex
	config      Config
	entries     map[string]*entryRuntime
	deviceIndex map[string]string
	events      chan models.Event
	cancel      context.CancelFunc
	cloud       *ezvizcloud.Session
	started     bool
	lastError   string
	lastSyncAt  time.Time
}

type entryRuntime struct {
	Config         EntryConfig
	Device         models.Device
	Client         lanclient.CameraClient
	CloudDevice    *ezvizcloud.DeviceInfo
	LastState      models.DeviceStateSnapshot
	Connected      bool
	LastError      string
	ControlBlocked string
}
