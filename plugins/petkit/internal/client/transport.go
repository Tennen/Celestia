package client

import (
	"bytes"
	"compress/gzip"
	"compress/zlib"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
)

func (c *Client) postSessionForm(ctx context.Context, endpoint string, form url.Values) (any, error) {
	return c.doRequest(ctx, http.MethodPost, endpoint, nil, form, requestAuthSession)
}

func (c *Client) postSessionJSON(ctx context.Context, endpoint string, params url.Values) (any, error) {
	return c.doRequest(ctx, http.MethodPost, endpoint, params, nil, requestAuthSession)
}

func (c *Client) getSessionJSON(ctx context.Context, endpoint string, params url.Values) (any, error) {
	return c.doRequest(ctx, http.MethodGet, endpoint, params, nil, requestAuthSession)
}

func (c *Client) getPublicJSON(ctx context.Context, endpoint string, params url.Values) (any, error) {
	return c.doRequest(ctx, http.MethodGet, endpoint, params, nil, requestAuthPublic)
}

func (c *Client) doRequest(
	ctx context.Context,
	method string,
	endpoint string,
	params url.Values,
	form url.Values,
	authMode requestAuthMode,
) (any, error) {
	useSession := authMode == requestAuthSession
	for attempt := 0; attempt < 2; attempt++ {
		baseURL, session, err := c.snapshotTransport()
		if err != nil {
			return nil, err
		}
		reqURL := endpoint
		if !strings.HasPrefix(endpoint, "http") {
			reqURL = strings.TrimRight(baseURL, "/") + "/" + strings.TrimLeft(endpoint, "/")
		}
		if params != nil && len(params) > 0 {
			join := "?"
			if strings.Contains(reqURL, "?") {
				join = "&"
			}
			reqURL += join + params.Encode()
		}

		var body io.Reader
		if form != nil {
			body = strings.NewReader(form.Encode())
		}
		req, err := http.NewRequestWithContext(ctx, method, reqURL, body)
		if err != nil {
			return nil, err
		}
		req.Header.Set("Accept", "*/*")
		req.Header.Set("Accept-Language", c.Compat.AcceptLanguage)
		req.Header.Set("Accept-Encoding", "gzip, deflate")
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		req.Header.Set("User-Agent", c.Compat.UserAgent)
		req.Header.Set("X-Img-Version", "1")
		req.Header.Set("X-Locale", c.Compat.Locale)
		req.Header.Set("X-Client", c.compatClientHeader())
		req.Header.Set("X-Hour", c.Compat.HourMode)
		req.Header.Set("X-TimezoneId", c.Cfg.Timezone)
		req.Header.Set("X-Api-Version", c.Compat.APIVersion)
		req.Header.Set("X-Timezone", timezoneOffset(c.Cfg.Timezone))
		if useSession && session != nil {
			req.Header.Set("F-Session", session.ID)
			req.Header.Set("X-Session", session.ID)
		}
		resp, err := c.httpClient.Do(req)
		if err != nil {
			return nil, err
		}
		bodyBytes, err := readPetkitBody(resp)
		resp.Body.Close()
		if err != nil {
			return nil, err
		}
		if resp.StatusCode == http.StatusUnauthorized && useSession && attempt == 0 {
			if err := c.login(ctx); err != nil {
				return nil, err
			}
			continue
		}
		if resp.StatusCode >= 400 {
			if code, _, ok := petkitErrorFromStatusBody(bodyBytes); ok && code == 97 && attempt == 0 {
				c.clearSessionTransport()
				if err := c.login(ctx); err != nil {
					return nil, err
				}
				continue
			}
			return nil, newPetkitRequestError(resp.StatusCode, method, reqURL, form, bodyBytes)
		}
		var payload any
		if len(bodyBytes) == 0 {
			return nil, nil
		}
		if err := json.Unmarshal(bodyBytes, &payload); err != nil {
			return nil, fmt.Errorf("invalid Petkit response: %w", err)
		}
		switch value := payload.(type) {
		case map[string]any:
			if code, message, ok := petkitAPIError(value); ok {
				if code == 5 && useSession && attempt == 0 {
					if err := c.login(ctx); err != nil {
						return nil, err
					}
					continue
				}
				return nil, fmt.Errorf("petkit api error map[code:%d msg:%s]", code, message)
			}
			if result, ok := value["result"]; ok {
				return result, nil
			}
			if sessionValue, ok := value["session"]; ok {
				return sessionValue, nil
			}
			return value, nil
		default:
			return payload, nil
		}
	}
	return nil, errors.New("petkit request failed after re-authentication")
}

func newPetkitRequestError(status int, method string, reqURL string, form url.Values, body []byte) error {
	code, message, ok := petkitErrorFromStatusBody(body)
	sanitizedURL := sanitizePetkitURL(reqURL)
	sanitizedForm := sanitizePetkitValues(form)
	if len(body) == 0 {
		return &PetkitRequestError{
			Status: status,
			Method: method,
			URL:    sanitizedURL,
			Code:   code,
			Form:   sanitizedForm,
			Body:   "",
		}
	}
	trimmed := strings.TrimSpace(string(body))
	if ok {
		return &PetkitRequestError{
			Status:  status,
			Method:  method,
			URL:     sanitizedURL,
			Code:    code,
			Message: message,
			Form:    sanitizedForm,
			Body:    trimmed,
		}
	}
	return &PetkitRequestError{
		Status: status,
		Method: method,
		URL:    sanitizedURL,
		Form:   sanitizedForm,
		Body:   trimmed,
	}
}

func sanitizePetkitURL(rawURL string) string {
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return rawURL
	}
	if parsed.RawQuery == "" {
		return rawURL
	}
	parsed.RawQuery = sanitizePetkitValues(parsed.Query())
	return parsed.String()
}

func sanitizePetkitValues(values url.Values) string {
	if len(values) == 0 {
		return ""
	}
	sanitized := url.Values{}
	for key, rawValues := range values {
		if isSensitivePetkitField(key) {
			sanitized[key] = []string{"[REDACTED]"}
			continue
		}
		copied := make([]string, 0, len(rawValues))
		for _, value := range rawValues {
			copied = append(copied, value)
		}
		sanitized[key] = copied
	}
	return sanitized.Encode()
}

func isSensitivePetkitField(key string) bool {
	key = strings.ToLower(strings.TrimSpace(key))
	switch key {
	case "password", "session", "f-session", "x-session", "token", "secret", "username", "validcode":
		return true
	default:
		return false
	}
}

func petkitAPIError(payload map[string]any) (int, string, bool) {
	if errObj, ok := payload["error"].(map[string]any); ok {
		code := intFromAny(errObj["code"], 0)
		message := firstNonEmpty(
			stringFromAny(errObj["msg"], ""),
			stringFromAny(errObj["message"], ""),
			stringFromAny(errObj["desc"], ""),
		)
		if code != 0 || message != "" {
			return code, message, true
		}
	}
	code := intFromAny(payload["code"], 0)
	message := firstNonEmpty(
		stringFromAny(payload["msg"], ""),
		stringFromAny(payload["message"], ""),
		stringFromAny(payload["desc"], ""),
	)
	if code != 0 || message != "" {
		return code, message, true
	}
	return 0, "", false
}

func petkitErrorFromStatusBody(body []byte) (int, string, bool) {
	if len(body) == 0 {
		return 0, "", false
	}
	var payload map[string]any
	if err := json.Unmarshal(body, &payload); err != nil {
		return 0, "", false
	}
	return petkitAPIError(payload)
}

func readPetkitBody(resp *http.Response) ([]byte, error) {
	encoding := strings.ToLower(strings.TrimSpace(resp.Header.Get("Content-Encoding")))
	switch {
	case strings.Contains(encoding, "gzip"):
		reader, err := gzip.NewReader(resp.Body)
		if err != nil {
			return nil, err
		}
		defer reader.Close()
		return io.ReadAll(reader)
	case strings.Contains(encoding, "deflate"):
		reader, err := zlib.NewReader(resp.Body)
		if err != nil {
			return nil, err
		}
		defer reader.Close()
		return io.ReadAll(reader)
	default:
		bodyBytes, err := io.ReadAll(resp.Body)
		if err != nil {
			return nil, err
		}
		if len(bodyBytes) >= 2 && bodyBytes[0] == 0x1f && bodyBytes[1] == 0x8b {
			reader, err := gzip.NewReader(bytes.NewReader(bodyBytes))
			if err != nil {
				return nil, err
			}
			defer reader.Close()
			return io.ReadAll(reader)
		}
		return bodyBytes, nil
	}
}
