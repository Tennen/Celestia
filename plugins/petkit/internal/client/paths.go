package client

import (
	"context"
	"fmt"
	"net/url"
	"strings"
)

func typedPetkitEndpoint(deviceType string, endpoint string) string {
	deviceType = strings.ToLower(strings.TrimSpace(deviceType))
	endpoint = strings.TrimLeft(strings.TrimSpace(endpoint), "/")
	if deviceType == "" {
		return endpoint
	}
	if endpoint == "" {
		return deviceType
	}
	return fmt.Sprintf("%s/%s", deviceType, endpoint)
}

func (c *Client) postTypedSessionForm(ctx context.Context, deviceType string, endpoint string, form url.Values) (any, error) {
	return c.postSessionForm(ctx, typedPetkitEndpoint(deviceType, endpoint), form)
}

func (c *Client) postTypedSessionJSON(ctx context.Context, deviceType string, endpoint string, params url.Values) (any, error) {
	return c.postSessionJSON(ctx, typedPetkitEndpoint(deviceType, endpoint), params)
}
