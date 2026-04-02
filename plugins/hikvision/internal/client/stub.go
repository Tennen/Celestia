//go:build !hikvision_sdk

package client

import (
	"context"
	"errors"
	"time"
)

type stubClient struct{}

func newHCNetClient() CameraClient {
	return &stubClient{}
}

func (c *stubClient) Connect(context.Context, CameraConfig) (CameraStatus, error) {
	return CameraStatus{}, errors.New("hikvision sdk runtime is disabled: rebuild plugin with -tags hikvision_sdk on linux/arm64")
}

func (c *stubClient) Disconnect(context.Context) error {
	return nil
}

func (c *stubClient) Status(context.Context) (CameraStatus, error) {
	return CameraStatus{Connected: false, Playback: map[string]any{}}, nil
}

func (c *stubClient) PTZMove(context.Context, string, int, int) error {
	return errors.New("hikvision sdk runtime is disabled")
}

func (c *stubClient) PTZStop(context.Context, string, int) error {
	return errors.New("hikvision sdk runtime is disabled")
}

func (c *stubClient) PlaybackOpen(context.Context, time.Time, time.Time) (map[string]any, error) {
	return nil, errors.New("hikvision sdk runtime is disabled")
}

func (c *stubClient) PlaybackControl(context.Context, string, string, *float64) (map[string]any, error) {
	return nil, errors.New("hikvision sdk runtime is disabled")
}

func (c *stubClient) PlaybackClose(context.Context, string) (map[string]any, error) {
	return nil, errors.New("hikvision sdk runtime is disabled")
}

func (c *stubClient) ListRecordings(context.Context, time.Time, int) ([]map[string]any, error) {
	return nil, errors.New("hikvision sdk runtime is disabled")
}
