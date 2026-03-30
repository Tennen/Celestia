package app

import (
	"context"
	"time"
)

type cameraStatus struct {
	Connected bool
	RTSPURL   string
	Playback  map[string]any
}

type cameraClient interface {
	Connect(context.Context, CameraConfig) (cameraStatus, error)
	Disconnect(context.Context) error
	Status(context.Context) (cameraStatus, error)
	PTZMove(context.Context, string, int, int) error
	PTZStop(context.Context, string, int) error
	PlaybackOpen(context.Context, time.Time, time.Time) (map[string]any, error)
	PlaybackControl(context.Context, string, string, *float64) (map[string]any, error)
	PlaybackClose(context.Context, string) (map[string]any, error)
	ListRecordings(context.Context, time.Time, int) ([]map[string]any, error)
}

func newCameraClient() cameraClient {
	return newHCNetClient()
}
