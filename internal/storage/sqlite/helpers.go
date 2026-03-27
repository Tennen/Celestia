package sqlite

import (
	"context"
	"database/sql"
	"encoding/json"
	"strings"
	"time"
)

func marshalJSON(v any) (string, error) {
	if v == nil {
		v = map[string]any{}
	}
	bytes, err := json.Marshal(v)
	if err != nil {
		return "", err
	}
	return string(bytes), nil
}

func parseJSON(data string, out any) error {
	if data == "" {
		data = "{}"
	}
	return json.Unmarshal([]byte(data), out)
}

func boolToInt(v bool) int {
	if v {
		return 1
	}
	return 0
}

func count(ctx context.Context, db *sql.DB, query string) (int, error) {
	var value int
	if err := db.QueryRowContext(ctx, query).Scan(&value); err != nil {
		return 0, err
	}
	return value, nil
}

func parseNullableTime(raw sql.NullString) (*time.Time, error) {
	if !raw.Valid || strings.TrimSpace(raw.String) == "" {
		return nil, nil
	}
	parsed, err := time.Parse(time.RFC3339Nano, raw.String)
	if err != nil {
		return nil, err
	}
	return &parsed, nil
}
