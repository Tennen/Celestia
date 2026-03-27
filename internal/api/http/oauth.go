package httpapi

import (
	"encoding/json"
	"errors"
	"fmt"
	"html"
	"net/http"
	"strings"

	oauth "github.com/chentianyu/celestia/internal/core/oauth"
	"github.com/chentianyu/celestia/internal/models"
)

func (s *Server) handleXiaomiOAuthStart(w http.ResponseWriter, r *http.Request) {
	var payload struct {
		PluginID        string `json:"plugin_id"`
		AccountName     string `json:"account_name"`
		Region          string `json:"region"`
		ClientID        string `json:"client_id"`
		RedirectBaseURL string `json:"redirect_base_url"`
	}
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	session, err := s.runtime.OAuth.StartXiaomi(r.Context(), oauth.StartXiaomiRequest{
		PluginID:    payload.PluginID,
		AccountName: payload.AccountName,
		Region:      payload.Region,
		ClientID:    payload.ClientID,
		RedirectURL: buildCallbackURL(r, payload.RedirectBaseURL),
	})
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{
		"session": session,
	})
}

func (s *Server) handleXiaomiOAuthSession(w http.ResponseWriter, r *http.Request) {
	session, ok, err := s.runtime.OAuth.GetSession(r.Context(), r.PathValue("id"))
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	if !ok {
		writeError(w, http.StatusNotFound, errors.New("oauth session not found"))
		return
	}
	writeJSON(w, http.StatusOK, session)
}

func (s *Server) handleXiaomiOAuthCallback(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query()
	state := strings.TrimSpace(query.Get("state"))
	code := strings.TrimSpace(query.Get("code"))
	callbackError := strings.TrimSpace(query.Get("error"))
	callbackErrorDescription := strings.TrimSpace(query.Get("error_description"))

	var (
		session models.OAuthSession
		err     error
	)
	switch {
	case callbackError != "":
		session, _, _ = s.runtime.OAuth.FailXiaomi(r.Context(), state, joinNonEmpty(": ", callbackError, callbackErrorDescription))
		writeOAuthCallbackPage(w, session.ID, models.OAuthSessionFailed, joinNonEmpty(": ", callbackError, callbackErrorDescription))
		return
	case state == "" || code == "":
		writeOAuthCallbackPage(w, "", models.OAuthSessionFailed, "xiaomi callback missing code or state")
		return
	default:
		session, err = s.runtime.OAuth.CompleteXiaomi(r.Context(), state, code)
		if err != nil {
			if stored, ok, lookupErr := s.runtime.Store.GetOAuthSessionByState(r.Context(), models.OAuthProviderXiaomi, state); lookupErr == nil && ok {
				session = stored
			}
			writeOAuthCallbackPage(w, session.ID, models.OAuthSessionFailed, err.Error())
			return
		}
		writeOAuthCallbackPage(w, session.ID, session.Status, "")
	}
}

func buildCallbackURL(r *http.Request, redirectBaseURL string) string {
	baseURL := strings.TrimRight(strings.TrimSpace(redirectBaseURL), "/")
	if baseURL == "" {
		baseURL = requestOrigin(r)
	}
	return baseURL + "/api/v1/oauth/xiaomi/callback"
}

func requestOrigin(r *http.Request) string {
	scheme := "http"
	if forwardedProto := strings.TrimSpace(strings.Split(r.Header.Get("X-Forwarded-Proto"), ",")[0]); forwardedProto != "" {
		scheme = forwardedProto
	} else if r.TLS != nil {
		scheme = "https"
	}
	host := strings.TrimSpace(strings.Split(r.Header.Get("X-Forwarded-Host"), ",")[0])
	if host == "" {
		host = r.Host
	}
	return scheme + "://" + host
}

func writeOAuthCallbackPage(w http.ResponseWriter, sessionID string, status models.OAuthSessionStatus, message string) {
	payload, _ := json.Marshal(map[string]any{
		"type":       "celestia:xiaomi-oauth",
		"session_id": sessionID,
		"status":     status,
		"error":      message,
	})
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	_, _ = fmt.Fprintf(w, `<!doctype html>
<html lang="en">
<head>
  <meta charset="utf-8">
  <meta name="viewport" content="width=device-width, initial-scale=1">
  <title>Celestia Xiaomi OAuth</title>
  <style>
    body { font-family: ui-sans-serif, system-ui, sans-serif; background: #f6efe6; color: #211b16; margin: 0; }
    main { min-height: 100vh; display: grid; place-items: center; padding: 24px; }
    article { max-width: 420px; background: #fffaf4; border: 1px solid #e6d7c3; border-radius: 18px; padding: 24px; box-shadow: 0 18px 60px rgba(50, 30, 10, 0.08); }
    h1 { margin: 0 0 8px; font-size: 1.2rem; }
    p { margin: 0; line-height: 1.5; color: #5e5146; }
  </style>
</head>
<body>
  <main>
    <article>
      <h1>%s</h1>
      <p>%s</p>
    </article>
  </main>
  <script>
    const payload = %s;
    if (window.opener) {
      window.opener.postMessage(payload, window.location.origin);
    }
    window.setTimeout(() => window.close(), 250);
  </script>
</body>
</html>`, html.EscapeString(callbackTitle(status)), html.EscapeString(callbackMessage(status, message)), payload)
}

func callbackTitle(status models.OAuthSessionStatus) string {
	if status == models.OAuthSessionCompleted {
		return "Xiaomi authorization completed"
	}
	return "Xiaomi authorization failed"
}

func callbackMessage(status models.OAuthSessionStatus, message string) string {
	if status == models.OAuthSessionCompleted {
		return "You can return to the Celestia admin window."
	}
	if strings.TrimSpace(message) == "" {
		return "Close this window and try the flow again."
	}
	return message
}

func joinNonEmpty(sep string, parts ...string) string {
	items := make([]string, 0, len(parts))
	for _, part := range parts {
		if trimmed := strings.TrimSpace(part); trimmed != "" {
			items = append(items, trimmed)
		}
	}
	return strings.Join(items, sep)
}
