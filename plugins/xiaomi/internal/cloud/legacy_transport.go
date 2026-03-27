package cloud

import (
	"context"
	"crypto/rc4"
	"crypto/sha1"
	"crypto/sha256"
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math/rand"
	"net/http"
	"net/url"
	"strings"
	"time"
)

func (c *Client) legacyAPIJSON(ctx context.Context, method, api string, payload map[string]any, out any) error {
	raw, err := c.doLegacyRequest(ctx, method, api, payload, true)
	if err != nil {
		return err
	}
	if out == nil {
		return nil
	}
	return json.Unmarshal(raw, out)
}

func (c *Client) doLegacyRequest(ctx context.Context, method, api string, payload map[string]any, allowRetry bool) ([]byte, error) {
	if err := c.ensureLegacySession(ctx); err != nil {
		return nil, err
	}

	endpoint := c.legacyAPIURL(api)
	params := make([]orderedParam, 0, 1)
	if payload != nil {
		data, err := json.Marshal(payload)
		if err != nil {
			return nil, err
		}
		params = append(params, orderedParam{Key: "data", Value: string(data)})
	}

	formParams, nonce, err := c.buildRC4Params(method, endpoint, params)
	if err != nil {
		return nil, err
	}

	body := url.Values{}
	for _, item := range formParams {
		body.Set(item.Key, item.Value)
	}
	req, err := http.NewRequestWithContext(ctx, method, endpoint, strings.NewReader(body.Encode()))
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", c.userAgent)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("X-XIAOMI-PROTOCAL-FLAG-CLI", legacyProtocolHeader)
	req.Header.Set("MIOT-ENCRYPT-ALGORITHM", "ENCRYPT-RC4")
	req.Header.Set("Accept-Encoding", "identity")
	c.addLegacyCookies(req)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	rawBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode == http.StatusUnauthorized {
		c.clearLegacySession()
		if allowRetry && strings.TrimSpace(c.cfg.Username) != "" && strings.TrimSpace(c.cfg.Password) != "" {
			if err := c.loginWithPassword(ctx); err != nil {
				return nil, err
			}
			return c.doLegacyRequest(ctx, method, api, payload, false)
		}
		return nil, fmt.Errorf("xiaomi legacy request failed: %s", resp.Status)
	}

	decoded, err := c.decodeLegacyBody(nonce, rawBody)
	if err != nil {
		return nil, err
	}
	if legacyAuthExpired(decoded) {
		c.clearLegacySession()
		if allowRetry && strings.TrimSpace(c.cfg.Username) != "" && strings.TrimSpace(c.cfg.Password) != "" {
			if err := c.loginWithPassword(ctx); err != nil {
				return nil, err
			}
			return c.doLegacyRequest(ctx, method, api, payload, false)
		}
	}
	return decoded, nil
}

func (c *Client) clearLegacySession() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.serviceToken = ""
	c.ssecurity = ""
	c.userID = ""
	c.cuserID = ""
}

func (c *Client) addLegacyCookies(req *http.Request) {
	c.mu.Lock()
	userID := c.userID
	serviceToken := c.serviceToken
	c.mu.Unlock()
	req.AddCookie(&http.Cookie{Name: "userId", Value: userID})
	req.AddCookie(&http.Cookie{Name: "yetAnotherServiceToken", Value: serviceToken})
	req.AddCookie(&http.Cookie{Name: "serviceToken", Value: serviceToken})
	req.AddCookie(&http.Cookie{Name: "locale", Value: c.locale})
	req.AddCookie(&http.Cookie{Name: "timezone", Value: c.timezone})
	req.AddCookie(&http.Cookie{Name: "is_daylight", Value: fmt.Sprintf("%d", boolToInt(time.Now().IsDST()))})
	req.AddCookie(&http.Cookie{Name: "dst_offset", Value: fmt.Sprintf("%d", boolToInt(time.Now().IsDST())*60*60*1000)})
	req.AddCookie(&http.Cookie{Name: "channel", Value: legacyChannel})
}

func boolToInt(v bool) int {
	if v {
		return 1
	}
	return 0
}

func (c *Client) legacyAPIURL(api string) string {
	api = strings.TrimPrefix(api, "/")
	return c.legacyBaseURL + "/" + api
}

func (c *Client) buildRC4Params(method, endpoint string, params []orderedParam) ([]orderedParam, string, error) {
	nonce, err := generateNonce()
	if err != nil {
		return nil, "", err
	}
	signedNonce, err := c.signedNonce(nonce)
	if err != nil {
		return nil, "", err
	}

	params = append(params, orderedParam{
		Key:   "rc4_hash__",
		Value: sha1Sign(method, endpoint, params, signedNonce),
	})

	encrypted := make([]orderedParam, 0, len(params)+3)
	for _, item := range params {
		value, err := encryptRC4(signedNonce, item.Value)
		if err != nil {
			return nil, "", err
		}
		encrypted = append(encrypted, orderedParam{Key: item.Key, Value: value})
	}
	encrypted = append(encrypted,
		orderedParam{Key: "signature", Value: sha1Sign(method, endpoint, encrypted, signedNonce)},
		orderedParam{Key: "ssecurity", Value: c.currentSSecurity()},
		orderedParam{Key: "_nonce", Value: nonce},
	)
	return encrypted, nonce, nil
}

func (c *Client) currentSSecurity() string {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.ssecurity
}

func (c *Client) signedNonce(nonce string) (string, error) {
	c.mu.Lock()
	ssecurity := c.ssecurity
	c.mu.Unlock()
	if ssecurity == "" {
		return "", errors.New("xiaomi legacy session missing ssecurity")
	}
	key, err := base64.StdEncoding.DecodeString(ssecurity)
	if err != nil {
		return "", err
	}
	nonceBytes, err := base64.StdEncoding.DecodeString(nonce)
	if err != nil {
		return "", err
	}
	hash := sha256.Sum256(append(key, nonceBytes...))
	return base64.StdEncoding.EncodeToString(hash[:]), nil
}

func sha1Sign(method, endpoint string, params []orderedParam, nonce string) string {
	path := mustURLPath(endpoint)
	if strings.HasPrefix(path, "/app/") {
		path = path[4:]
	}
	parts := []string{strings.ToUpper(method), path}
	for _, item := range params {
		parts = append(parts, item.Key+"="+item.Value)
	}
	parts = append(parts, nonce)
	sum := sha1.Sum([]byte(strings.Join(parts, "&")))
	return base64.StdEncoding.EncodeToString(sum[:])
}

func mustURLPath(endpoint string) string {
	parsed, err := url.Parse(endpoint)
	if err != nil {
		return endpoint
	}
	return parsed.Path
}

func encryptRC4(passwordB64, plain string) (string, error) {
	key, err := base64.StdEncoding.DecodeString(passwordB64)
	if err != nil {
		return "", err
	}
	cipher, err := rc4.NewCipher(key)
	if err != nil {
		return "", err
	}
	drop := make([]byte, 1024)
	cipher.XORKeyStream(drop, drop)
	out := make([]byte, len(plain))
	cipher.XORKeyStream(out, []byte(plain))
	return base64.StdEncoding.EncodeToString(out), nil
}

func decryptRC4(passwordB64 string, ciphertext []byte) ([]byte, error) {
	key, err := base64.StdEncoding.DecodeString(passwordB64)
	if err != nil {
		return nil, err
	}
	data, err := base64.StdEncoding.DecodeString(strings.TrimSpace(string(ciphertext)))
	if err != nil {
		return nil, err
	}
	cipher, err := rc4.NewCipher(key)
	if err != nil {
		return nil, err
	}
	drop := make([]byte, 1024)
	cipher.XORKeyStream(drop, drop)
	out := make([]byte, len(data))
	cipher.XORKeyStream(out, data)
	return out, nil
}

func generateNonce() (string, error) {
	random := make([]byte, 8)
	rand.New(rand.NewSource(time.Now().UnixNano())).Read(random)
	minutes := uint32(time.Now().UnixMilli() / 60000)
	suffix := make([]byte, 4)
	binary.BigEndian.PutUint32(suffix, minutes)
	return base64.StdEncoding.EncodeToString(append(random, suffix...)), nil
}

func (c *Client) decodeLegacyBody(nonce string, raw []byte) ([]byte, error) {
	text := strings.TrimSpace(string(raw))
	if text == "" {
		return nil, errors.New("xiaomi legacy request returned empty body")
	}
	if strings.HasPrefix(text, "{") {
		return []byte(text), nil
	}
	signedNonce, err := c.signedNonce(nonce)
	if err != nil {
		return nil, err
	}
	decoded, err := decryptRC4(signedNonce, raw)
	if err != nil {
		return nil, err
	}
	return decoded, nil
}

func legacyAuthExpired(raw []byte) bool {
	var body struct {
		Code    int    `json:"code"`
		Message string `json:"message"`
	}
	if err := json.Unmarshal(raw, &body); err != nil {
		return false
	}
	message := strings.ToLower(strings.TrimSpace(body.Message))
	return body.Code == 2 || body.Code == 3 || strings.Contains(message, "auth err") || strings.Contains(message, "servicetoken_expired") || strings.Contains(message, "invalid signature")
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}
