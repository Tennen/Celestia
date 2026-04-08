package cloud

import (
	"context"
	"crypto/md5"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
)

type apiResponse struct {
	Meta         map[string]any `json:"meta"`
	LoginArea    map[string]any `json:"loginArea"`
	LoginSession map[string]any `json:"loginSession"`
	LoginUser    map[string]any `json:"loginUser"`
	SessionInfo  map[string]any `json:"sessionInfo"`
	Page         map[string]any `json:"page"`
}

func (s *Session) ensureAuthenticated(ctx context.Context) error {
	if s.cfg.HasSession() {
		if err := s.refreshSession(ctx); err == nil {
			return nil
		}
		if !s.cfg.HasCredentials() {
			return errors.New("ezviz session expired and username/password are not configured")
		}
		s.cfg.SessionID = ""
		s.cfg.RefreshSessionID = ""
	}
	if !s.cfg.HasCredentials() {
		return errors.New("ezviz username/password are required")
	}
	return s.login(ctx)
}

func (s *Session) refreshSession(ctx context.Context) error {
	body := url.Values{
		"refreshSessionId": []string{s.cfg.RefreshSessionID},
		"featureCode":      []string{s.featureCode},
	}
	resp, err := s.doFormRequest(ctx, http.MethodPut, refreshSessionPath, nil, body, false)
	if err != nil {
		return err
	}
	if responseCode(resp) == 403 {
		return errors.New("ezviz session refresh was rejected")
	}
	if !responseOK(resp) {
		return responseError("ezviz session refresh failed", resp)
	}
	sessionInfo := resp.SessionInfo
	if len(sessionInfo) == 0 {
		return errors.New("ezviz session refresh response did not include session info")
	}
	s.cfg.SessionID = stringValue(sessionInfo["sessionId"])
	s.cfg.RefreshSessionID = firstNonEmpty(
		stringValue(sessionInfo["refreshSessionId"]),
		stringValue(sessionInfo["refreshSessionID"]),
	)
	s.cfg = s.cfg.Sanitized()
	return nil
}

func (s *Session) login(ctx context.Context) error {
	for attempt := 0; attempt < 2; attempt++ {
		body := url.Values{
			"account":     []string{s.cfg.Username},
			"password":    []string{md5Hex(s.cfg.Password)},
			"featureCode": []string{s.featureCode},
			"msgType":     []string{"0"},
			"bizType":     []string{""},
			"cuName":      []string{"SGFzc2lv"},
			"smsCode":     []string{""},
		}
		resp, err := s.doFormRequest(ctx, http.MethodPost, loginPath, nil, body, false)
		if err != nil {
			return err
		}
		switch responseCode(resp) {
		case 200:
			s.cfg.SessionID = stringValue(resp.LoginSession["sessionId"])
			s.cfg.RefreshSessionID = firstNonEmpty(
				stringValue(resp.LoginSession["rfSessionId"]),
				stringValue(resp.LoginSession["refreshSessionId"]),
			)
			s.cfg.UserName = firstNonEmpty(stringValue(resp.LoginUser["username"]), s.cfg.UserName)
			s.cfg.APIURL = firstNonEmpty(stringValue(resp.LoginArea["apiDomain"]), s.cfg.APIURL)
			s.cfg = s.cfg.Sanitized()
			return nil
		case 1100:
			if apiDomain := stringValue(resp.LoginArea["apiDomain"]); apiDomain != "" {
				s.cfg.APIURL = apiDomain
				continue
			}
			return errors.New("ezviz login requires a different region, but apiDomain was not returned")
		case 1012:
			return errors.New("ezviz MFA code is invalid")
		case 1013:
			return errors.New("ezviz username is invalid")
		case 1014:
			return errors.New("ezviz password is invalid")
		case 1015:
			return errors.New("ezviz account is locked")
		case 6002:
			return errors.New("ezviz account requires MFA; this plugin does not yet automate SMS verification")
		default:
			return responseError("ezviz login failed", resp)
		}
	}
	return errors.New("ezviz login failed after region retry")
}

func md5Hex(value string) string {
	sum := md5.Sum([]byte(value))
	return hex.EncodeToString(sum[:])
}

func (s *Session) doFormRequest(
	ctx context.Context,
	method string,
	path string,
	query url.Values,
	body url.Values,
	retryAuth bool,
) (apiResponse, error) {
	req, err := s.newRequest(ctx, method, path, query, strings.NewReader(body.Encode()))
	if err != nil {
		return apiResponse{}, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	resp, err := s.doJSON(req)
	if err != nil {
		return apiResponse{}, err
	}
	if retryAuth && responseCode(resp) == 401 && s.cfg.HasCredentials() {
		s.cfg.SessionID = ""
		s.cfg.RefreshSessionID = ""
		if err := s.ensureAuthenticated(ctx); err != nil {
			return apiResponse{}, err
		}
		req, err = s.newRequest(ctx, method, path, query, strings.NewReader(body.Encode()))
		if err != nil {
			return apiResponse{}, err
		}
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		return s.doJSON(req)
	}
	return resp, nil
}

func (s *Session) doJSONRequest(
	ctx context.Context,
	method string,
	path string,
	query url.Values,
	body io.Reader,
	contentType string,
	retryAuth bool,
) (apiResponse, error) {
	req, err := s.newRequest(ctx, method, path, query, body)
	if err != nil {
		return apiResponse{}, err
	}
	if contentType != "" {
		req.Header.Set("Content-Type", contentType)
	}
	resp, err := s.doJSON(req)
	if err != nil {
		return apiResponse{}, err
	}
	if retryAuth && responseCode(resp) == 401 && s.cfg.HasCredentials() {
		s.cfg.SessionID = ""
		s.cfg.RefreshSessionID = ""
		if err := s.ensureAuthenticated(ctx); err != nil {
			return apiResponse{}, err
		}
		req, err = s.newRequest(ctx, method, path, query, body)
		if err != nil {
			return apiResponse{}, err
		}
		if contentType != "" {
			req.Header.Set("Content-Type", contentType)
		}
		return s.doJSON(req)
	}
	return resp, nil
}

func (s *Session) newRequest(ctx context.Context, method, path string, query url.Values, body io.Reader) (*http.Request, error) {
	apiURL := s.cfg.APIURL
	if apiURL == "" {
		apiURL = DefaultAPIURL
	}
	fullURL := "https://" + strings.TrimSpace(apiURL) + path
	if len(query) > 0 {
		fullURL += "?" + query.Encode()
	}
	req, err := http.NewRequestWithContext(ctx, method, fullURL, body)
	if err != nil {
		return nil, err
	}
	for key, value := range s.defaultHeaders() {
		req.Header.Set(key, value)
	}
	return req, nil
}

func (s *Session) defaultHeaders() map[string]string {
	headers := map[string]string{
		"featureCode":   s.featureCode,
		"clientType":    "3",
		"osVersion":     "",
		"clientVersion": "",
		"netType":       "WIFI",
		"customno":      "1000001",
		"ssid":          "",
		"clientNo":      "web_site",
		"appId":         "ys7",
		"language":      "en_GB",
		"lang":          "en",
		"User-Agent":    "okhttp/3.12.1",
	}
	if s.cfg.SessionID != "" {
		headers["sessionId"] = s.cfg.SessionID
	}
	return headers
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

func responseCode(resp apiResponse) int {
	if code := intValue(resp.Meta["code"]); code != 0 {
		return code
	}
	return intValue(resp.Meta["status"])
}

func responseOK(resp apiResponse) bool {
	return responseCode(resp) == 200
}

func responseError(prefix string, resp apiResponse) error {
	message := firstNonEmpty(stringValue(resp.Meta["message"]), stringValue(resp.Meta["moreInfo"]))
	if message == "" {
		message = fmt.Sprintf("meta.code=%d", responseCode(resp))
	}
	return fmt.Errorf("%s: %s", prefix, message)
}
