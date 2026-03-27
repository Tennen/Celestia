package cloud

import (
	"context"
	"fmt"
	"net/http"

	"github.com/chentianyu/celestia/plugins/xiaomi/internal/spec"
)

func (c *Client) SpecInstance(ctx context.Context, urn string) (spec.Instance, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, "https://miot-spec.org/miot-spec-v2/instance", nil)
	if err != nil {
		return spec.Instance{}, err
	}
	query := req.URL.Query()
	query.Set("type", urn)
	req.URL.RawQuery = query.Encode()
	var instance spec.Instance
	if err := c.do(req, &instance); err != nil {
		return spec.Instance{}, err
	}
	if instance.Type == "" || len(instance.Services) == 0 {
		return spec.Instance{}, fmt.Errorf("xiaomi spec unavailable for %s", urn)
	}
	return instance, nil
}
