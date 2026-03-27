package sqlite

import (
	"context"
	"time"

	"github.com/chentianyu/celestia/internal/models"
)

func (s *Store) UpsertOAuthSession(ctx context.Context, session models.OAuthSession) error {
	accountConfigJSON, err := marshalJSON(session.AccountConfig)
	if err != nil {
		return err
	}
	var completedAt any
	if session.CompletedAt != nil {
		completedAt = session.CompletedAt.UTC().Format(time.RFC3339Nano)
	}
	var stateExpiresAt any
	if session.StateExpiresAt != nil {
		stateExpiresAt = session.StateExpiresAt.UTC().Format(time.RFC3339Nano)
	}
	var tokenExpiresAt any
	if session.TokenExpiresAt != nil {
		tokenExpiresAt = session.TokenExpiresAt.UTC().Format(time.RFC3339Nano)
	}
	_, err = s.db.ExecContext(ctx, `
		insert into oauth_sessions(
			id, provider, plugin_id, account_name, region, client_id, redirect_url, device_id,
			state, auth_url, status, error_text, account_config_json,
			created_at, updated_at, completed_at, state_expires_at, token_expires_at
		) values (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		on conflict(id) do update set
			provider=excluded.provider,
			plugin_id=excluded.plugin_id,
			account_name=excluded.account_name,
			region=excluded.region,
			client_id=excluded.client_id,
			redirect_url=excluded.redirect_url,
			device_id=excluded.device_id,
			state=excluded.state,
			auth_url=excluded.auth_url,
			status=excluded.status,
			error_text=excluded.error_text,
			account_config_json=excluded.account_config_json,
			updated_at=excluded.updated_at,
			completed_at=excluded.completed_at,
			state_expires_at=excluded.state_expires_at,
			token_expires_at=excluded.token_expires_at
	`, session.ID, session.Provider, session.PluginID, session.AccountName, session.Region, session.ClientID,
		session.RedirectURL, session.DeviceID, session.State, session.AuthURL, session.Status, session.Error,
		accountConfigJSON, session.CreatedAt.UTC().Format(time.RFC3339Nano), session.UpdatedAt.UTC().Format(time.RFC3339Nano),
		completedAt, stateExpiresAt, tokenExpiresAt)
	return err
}

func (s *Store) GetOAuthSession(ctx context.Context, id string) (models.OAuthSession, bool, error) {
	rows, err := s.db.QueryContext(ctx, `
		select id, provider, plugin_id, account_name, region, client_id, redirect_url, device_id,
		       state, auth_url, status, error_text, account_config_json,
		       created_at, updated_at, completed_at, state_expires_at, token_expires_at
		from oauth_sessions where id = ?
	`, id)
	if err != nil {
		return models.OAuthSession{}, false, err
	}
	defer rows.Close()
	if !rows.Next() {
		return models.OAuthSession{}, false, nil
	}
	session, err := scanOAuthSession(rows)
	return session, err == nil, err
}

func (s *Store) GetOAuthSessionByState(ctx context.Context, provider models.OAuthProvider, state string) (models.OAuthSession, bool, error) {
	rows, err := s.db.QueryContext(ctx, `
		select id, provider, plugin_id, account_name, region, client_id, redirect_url, device_id,
		       state, auth_url, status, error_text, account_config_json,
		       created_at, updated_at, completed_at, state_expires_at, token_expires_at
		from oauth_sessions where provider = ? and state = ?
	`, provider, state)
	if err != nil {
		return models.OAuthSession{}, false, err
	}
	defer rows.Close()
	if !rows.Next() {
		return models.OAuthSession{}, false, nil
	}
	session, err := scanOAuthSession(rows)
	return session, err == nil, err
}
