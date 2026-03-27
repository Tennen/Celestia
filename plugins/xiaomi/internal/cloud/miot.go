package cloud

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
)

func (c *Client) GetProps(ctx context.Context, params []map[string]any) ([]map[string]any, error) {
	if c.usesLegacyAuth() {
		var body struct {
			Result []map[string]any `json:"result"`
		}
		if err := c.legacyAPIJSON(ctx, http.MethodPost, "miotspec/prop/get", map[string]any{
			"params": params,
		}, &body); err != nil {
			return nil, err
		}
		return body.Result, nil
	}
	var body struct {
		Result []map[string]any `json:"result"`
	}
	if err := c.oauthAPIJSON(ctx, "/app/v2/miotspec/prop/get", map[string]any{
		"datasource": 1,
		"params":     params,
	}, &body); err != nil {
		return nil, err
	}
	return body.Result, nil
}

func (c *Client) SetProps(ctx context.Context, params []map[string]any) ([]map[string]any, error) {
	if c.usesLegacyAuth() {
		var body struct {
			Result []map[string]any `json:"result"`
		}
		if err := c.legacyAPIJSON(ctx, http.MethodPost, "miotspec/prop/set", map[string]any{
			"params": params,
		}, &body); err != nil {
			return nil, err
		}
		return body.Result, nil
	}
	var body struct {
		Result []map[string]any `json:"result"`
	}
	if err := c.oauthAPIJSON(ctx, "/app/v2/miotspec/prop/set", map[string]any{
		"params": params,
	}, &body); err != nil {
		return nil, err
	}
	return body.Result, nil
}

func (c *Client) Action(ctx context.Context, did string, siid, aiid int, inputs []any) (map[string]any, error) {
	if c.usesLegacyAuth() {
		var body struct {
			Result map[string]any `json:"result"`
		}
		if err := c.legacyAPIJSON(ctx, http.MethodPost, "miotspec/action", map[string]any{
			"params": map[string]any{
				"did":  did,
				"siid": siid,
				"aiid": aiid,
				"in":   inputs,
			},
		}, &body); err != nil {
			return nil, err
		}
		return body.Result, nil
	}
	var body struct {
		Result map[string]any `json:"result"`
	}
	if err := c.oauthAPIJSON(ctx, "/app/v2/miotspec/action", map[string]any{
		"params": map[string]any{
			"did":  did,
			"siid": siid,
			"aiid": aiid,
			"in":   inputs,
		},
	}, &body); err != nil {
		return nil, err
	}
	return body.Result, nil
}

func (c *Client) oauthAPIJSON(ctx context.Context, path string, payload map[string]any, out any) error {
	if err := c.ensureOAuthToken(ctx); err != nil {
		return err
	}
	raw, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.oauthBaseURL+path, bytes.NewReader(raw))
	if err != nil {
		return err
	}
	req.Header.Set("Host", strings.TrimPrefix(strings.TrimPrefix(c.oauthBaseURL, "https://"), "http://"))
	req.Header.Set("X-Client-BizId", "haapi")
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+c.AccessToken())
	req.Header.Set("X-Client-AppId", c.clientID)

	var envelope struct {
		Code    int             `json:"code"`
		Message string          `json:"message"`
		Result  json.RawMessage `json:"result"`
	}
	if err := c.do(req, &envelope); err != nil {
		return err
	}
	if envelope.Code != 0 {
		return fmt.Errorf("xiaomi api error %d: %s", envelope.Code, envelope.Message)
	}
	if out == nil {
		return nil
	}
	body := map[string]json.RawMessage{
		"result": envelope.Result,
	}
	rawOut, err := json.Marshal(body)
	if err != nil {
		return err
	}
	return json.Unmarshal(rawOut, out)
}

func (c *Client) do(req *http.Request, out any) error {
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("xiaomi request failed: %s", resp.Status)
	}
	if out == nil {
		return nil
	}
	return json.NewDecoder(resp.Body).Decode(out)
}
