//go:build !linux && hikvision_sdk

package app

import (
	"context"
	"errors"
	"time"
)

type unsupportedClient struct{}

func newHCNetClient() cameraClient {
	return &unsupportedClient{}
}

func (c *unsupportedClient) Connect(context.Context, CameraConfig) (cameraStatus, error) {
	return cameraStatus{}, errors.New("hikvision sdk runtime requires linux/arm64")
}

func (c *unsupportedClient) Disconnect(context.Context) error {
	return nil
}

func (c *unsupportedClient) Status(context.Context) (cameraStatus, error) {
	return cameraStatus{Connected: false, Playback: map[string]any{}}, nil
}

func (c *unsupportedClient) PTZMove(context.Context, string, int, int) error {
	return errors.New("hikvision sdk runtime requires linux/arm64")
}

func (c *unsupportedClient) PTZStop(context.Context, string, int) error {
	return errors.New("hikvision sdk runtime requires linux/arm64")
}

func (c *unsupportedClient) PlaybackOpen(context.Context, time.Time, time.Time) (map[string]any, error) {
	return nil, errors.New("hikvision sdk runtime requires linux/arm64")
}

func (c *unsupportedClient) PlaybackControl(context.Context, string, string, *float64) (map[string]any, error) {
	return nil, errors.New("hikvision sdk runtime requires linux/arm64")
}

func (c *unsupportedClient) PlaybackClose(context.Context, string) (map[string]any, error) {
	return nil, errors.New("hikvision sdk runtime requires linux/arm64")
}

func (c *unsupportedClient) ListRecordings(context.Context, time.Time, int) ([]map[string]any, error) {
	return nil, errors.New("hikvision sdk runtime requires linux/arm64")
}
