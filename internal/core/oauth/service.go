package oauth

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/chentianyu/celestia/internal/models"
	"github.com/chentianyu/celestia/internal/storage"
	xiaomioauth "github.com/chentianyu/celestia/internal/xiaomi/oauth"
	"github.com/google/uuid"
)

const defaultStateTTL = 10 * time.Minute

type Service struct {
	store    storage.Store
	client   *xiaomioauth.Client
	stateTTL time.Duration
}

type StartXiaomiRequest struct {
	PluginID    string
	AccountName string
	Region      string
	ClientID    string
	RedirectURL string
}

func New(store storage.Store) *Service {
	return &Service{
		store:    store,
		client:   xiaomioauth.NewClient(nil),
		stateTTL: defaultStateTTL,
	}
}

func (s *Service) StartXiaomi(ctx context.Context, req StartXiaomiRequest) (models.OAuthSession, error) {
	region := xiaomioauth.NormalizeRegion(req.Region)
	if !isSupportedRegion(region) {
		return models.OAuthSession{}, fmt.Errorf("unsupported Xiaomi region %q", req.Region)
	}
	clientID := strings.TrimSpace(req.ClientID)
	if clientID == "" {
		return models.OAuthSession{}, errors.New("xiaomi client_id is required")
	}
	redirectURL := strings.TrimSpace(req.RedirectURL)
	if redirectURL == "" {
		return models.OAuthSession{}, errors.New("xiaomi redirect_url is required")
	}
	accountName := strings.TrimSpace(req.AccountName)
	if accountName == "" {
		accountName = "primary"
	}

	now := time.Now().UTC()
	sessionID := uuid.NewString()
	state := uuid.NewString()
	deviceID := "celestia." + strings.ReplaceAll(sessionID, "-", "")
	authURL, err := xiaomioauth.AuthorizeURL(clientID, redirectURL, deviceID, state, nil, false)
	if err != nil {
		return models.OAuthSession{}, err
	}
	stateExpiresAt := now.Add(s.stateTTL)
	session := models.OAuthSession{
		ID:             sessionID,
		Provider:       models.OAuthProviderXiaomi,
		PluginID:       strings.TrimSpace(req.PluginID),
		AccountName:    accountName,
		Region:         region,
		ClientID:       clientID,
		RedirectURL:    redirectURL,
		DeviceID:       deviceID,
		State:          state,
		AuthURL:        authURL,
		Status:         models.OAuthSessionPending,
		CreatedAt:      now,
		UpdatedAt:      now,
		StateExpiresAt: &stateExpiresAt,
	}
	if err := s.store.UpsertOAuthSession(ctx, session); err != nil {
		return models.OAuthSession{}, err
	}
	return session, nil
}

func (s *Service) GetSession(ctx context.Context, id string) (models.OAuthSession, bool, error) {
	session, ok, err := s.store.GetOAuthSession(ctx, id)
	if err != nil || !ok {
		return session, ok, err
	}
	return s.expireIfNeeded(ctx, session)
}

func (s *Service) CompleteXiaomi(ctx context.Context, state, code string) (models.OAuthSession, error) {
	session, ok, err := s.store.GetOAuthSessionByState(ctx, models.OAuthProviderXiaomi, strings.TrimSpace(state))
	if err != nil {
		return models.OAuthSession{}, err
	}
	if !ok {
		return models.OAuthSession{}, errors.New("oauth session not found")
	}
	session, _, err = s.expireIfNeeded(ctx, session)
	if err != nil {
		return models.OAuthSession{}, err
	}
	switch session.Status {
	case models.OAuthSessionCompleted:
		return session, nil
	case models.OAuthSessionExpired:
		return models.OAuthSession{}, errors.New("oauth session expired")
	case models.OAuthSessionFailed:
		return models.OAuthSession{}, errors.New("oauth session already failed")
	}

	tokenSet, err := s.client.ExchangeCode(ctx, session.Region, session.ClientID, session.RedirectURL, strings.TrimSpace(code), session.DeviceID)
	if err != nil {
		session.Status = models.OAuthSessionFailed
		session.Error = err.Error()
		session.UpdatedAt = time.Now().UTC()
		_ = s.store.UpsertOAuthSession(ctx, session)
		return models.OAuthSession{}, err
	}
	now := time.Now().UTC()
	tokenExpiresAt := tokenSet.ExpiresAt.UTC()
	session.Status = models.OAuthSessionCompleted
	session.Error = ""
	session.AccountConfig = map[string]any{
		"name":          session.AccountName,
		"region":        session.Region,
		"client_id":     session.ClientID,
		"redirect_url":  session.RedirectURL,
		"access_token":  tokenSet.AccessToken,
		"refresh_token": tokenSet.RefreshToken,
		"expires_at":    tokenExpiresAt.Format(time.RFC3339),
	}
	session.UpdatedAt = now
	session.CompletedAt = &now
	session.TokenExpiresAt = &tokenExpiresAt
	if err := s.store.UpsertOAuthSession(ctx, session); err != nil {
		return models.OAuthSession{}, err
	}
	return session, nil
}

func (s *Service) FailXiaomi(ctx context.Context, state, message string) (models.OAuthSession, bool, error) {
	session, ok, err := s.store.GetOAuthSessionByState(ctx, models.OAuthProviderXiaomi, strings.TrimSpace(state))
	if err != nil || !ok {
		return session, ok, err
	}
	if session.Status == models.OAuthSessionCompleted || session.Status == models.OAuthSessionExpired {
		return session, true, nil
	}
	session.Status = models.OAuthSessionFailed
	session.Error = strings.TrimSpace(message)
	session.UpdatedAt = time.Now().UTC()
	if err := s.store.UpsertOAuthSession(ctx, session); err != nil {
		return models.OAuthSession{}, false, err
	}
	return session, true, nil
}

func (s *Service) expireIfNeeded(ctx context.Context, session models.OAuthSession) (models.OAuthSession, bool, error) {
	if session.Status != models.OAuthSessionPending || session.StateExpiresAt == nil {
		return session, true, nil
	}
	if time.Now().UTC().Before(session.StateExpiresAt.UTC()) {
		return session, true, nil
	}
	session.Status = models.OAuthSessionExpired
	session.Error = "oauth session expired"
	session.UpdatedAt = time.Now().UTC()
	if err := s.store.UpsertOAuthSession(ctx, session); err != nil {
		return models.OAuthSession{}, false, err
	}
	return session, true, nil
}

func isSupportedRegion(region string) bool {
	switch region {
	case "cn", "de", "i2", "ru", "sg", "us":
		return true
	default:
		return false
	}
}
