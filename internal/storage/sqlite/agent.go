package sqlite

import (
	"context"
	"database/sql"
	"encoding/json"
	"strings"
	"time"

	"github.com/chentianyu/celestia/internal/models"
)

func (s *Store) UpsertAgentDocument(ctx context.Context, doc models.AgentDocument) error {
	key := strings.TrimSpace(doc.Key)
	if key == "" {
		return sql.ErrNoRows
	}
	domain := strings.TrimSpace(doc.Domain)
	if domain == "" {
		domain = "agent"
	}
	payload := doc.Payload
	if len(payload) == 0 {
		payload = json.RawMessage(`{}`)
	}
	updatedAt := doc.UpdatedAt
	if updatedAt.IsZero() {
		updatedAt = time.Now().UTC()
	}
	_, err := s.db.ExecContext(ctx, `
		insert into agent_documents(key, domain, payload_json, updated_at)
		values (?, ?, ?, ?)
		on conflict(key) do update set
			domain=excluded.domain,
			payload_json=excluded.payload_json,
			updated_at=excluded.updated_at
	`, key, domain, string(payload), updatedAt.UTC().Format(time.RFC3339Nano))
	return err
}

func (s *Store) GetAgentDocument(ctx context.Context, key string) (models.AgentDocument, bool, error) {
	row := s.db.QueryRowContext(ctx, `
		select key, domain, payload_json, updated_at
		from agent_documents
		where key = ?
	`, strings.TrimSpace(key))
	return scanAgentDocument(row)
}

func (s *Store) DeleteAgentDocument(ctx context.Context, key string) error {
	_, err := s.db.ExecContext(ctx, `delete from agent_documents where key = ?`, strings.TrimSpace(key))
	return err
}

func scanAgentDocument(scanner interface{ Scan(...any) error }) (models.AgentDocument, bool, error) {
	var (
		doc       models.AgentDocument
		payload   string
		updatedAt string
	)
	if err := scanner.Scan(&doc.Key, &doc.Domain, &payload, &updatedAt); err != nil {
		if err == sql.ErrNoRows {
			return models.AgentDocument{}, false, nil
		}
		return models.AgentDocument{}, false, err
	}
	doc.Payload = json.RawMessage(payload)
	parsed, err := time.Parse(time.RFC3339Nano, updatedAt)
	if err != nil {
		return models.AgentDocument{}, false, err
	}
	doc.UpdatedAt = parsed.UTC()
	return doc, true, nil
}
