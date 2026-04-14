package httpapi

import (
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"strings"
	"time"

	gatewayapi "github.com/chentianyu/celestia/internal/api/gateway"
)

func writeJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}

func writeError(w http.ResponseWriter, status int, err error) {
	writeJSON(w, status, map[string]any{
		"error": err.Error(),
	})
}

func writeServiceError(w http.ResponseWriter, err error) {
	var denied *gatewayapi.PolicyDeniedError
	if errors.As(err, &denied) {
		writeJSON(w, http.StatusForbidden, map[string]any{
			"allowed": false,
			"reason":  denied.Decision.Reason,
		})
		return
	}
	status := gatewayapi.StatusCode(err)
	if status == 0 {
		status = http.StatusInternalServerError
	}
	writeError(w, status, err)
}

func parseLimit(raw string, defaultValue int) int {
	if raw == "" {
		return defaultValue
	}
	var limit int
	if _, err := fmt.Sscanf(raw, "%d", &limit); err != nil || limit <= 0 {
		return defaultValue
	}
	return limit
}

func parseOptionalRFC3339Time(raw string) (*time.Time, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil, nil
	}
	value, err := time.Parse(time.RFC3339Nano, raw)
	if err != nil {
		return nil, fmt.Errorf("invalid RFC3339 timestamp %q", raw)
	}
	value = value.UTC()
	return &value, nil
}

func actorFromRequest(r *http.Request) string {
	return actorFromRequestWithDefault(r, "admin")
}

func actorFromRequestWithDefault(r *http.Request, fallback string) string {
	actor := strings.TrimSpace(r.Header.Get("X-Actor"))
	if actor == "" {
		actor = fallback
	}
	return actor
}

func writeAIServiceError(w http.ResponseWriter, err error) {
	var ambiguous *gatewayapi.AmbiguousReferenceError
	if errors.As(err, &ambiguous) {
		status := gatewayapi.StatusCode(err)
		if status == 0 {
			status = http.StatusConflict
		}
		writeJSON(w, status, map[string]any{
			"error":   ambiguous.Error(),
			"field":   ambiguous.Field,
			"value":   ambiguous.Value,
			"matches": ambiguous.Matches,
		})
		return
	}
	writeServiceError(w, err)
}

func withCORS(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET,POST,PUT,DELETE,OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type,X-Actor")
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func withLogging(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		started := time.Now()
		next.ServeHTTP(w, r)
		log.Printf("%s %s %s", r.Method, r.URL.Path, time.Since(started))
	})
}
