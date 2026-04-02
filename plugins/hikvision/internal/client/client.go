package client

import (
	"context"
	"time"
)

// CameraStatus holds the runtime state returned by a CameraClient.
type CameraStatus struct {
	Connected bool
	RTSPURL   string
	Playback  map[string]any
}

// CameraClient is the interface for interacting with a Hikvision camera via the HCNet SDK.
type CameraClient interface {
	Connect(context.Context, CameraConfig) (CameraStatus, error)
	Disconnect(context.Context) error
	Status(context.Context) (CameraStatus, error)
	PTZMove(context.Context, string, int, int) error
	PTZStop(context.Context, string, int) error
	PlaybackOpen(context.Context, time.Time, time.Time) (map[string]any, error)
	PlaybackControl(context.Context, string, string, *float64) (map[string]any, error)
	PlaybackClose(context.Context, string) (map[string]any, error)
	ListRecordings(context.Context, time.Time, int) ([]map[string]any, error)
}

// NewCameraClient returns a new CameraClient backed by the HCNet SDK (or a stub
// when the SDK build tag is not set).
func NewCameraClient() CameraClient {
	return newHCNetClient()
}
