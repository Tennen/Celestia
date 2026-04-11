package vision

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"slices"
	"strings"
	"time"

	"github.com/chentianyu/celestia/internal/models"
)

const (
	visionEntityCatalogSchemaVersion = "celestia.vision.catalog.v1"
)

func (s *Service) GetCatalog(ctx context.Context) (models.VisionEntityCatalog, bool, error) {
	return s.store.GetVisionCatalog(ctx)
}

func (s *Service) RefreshCatalog(ctx context.Context, req models.VisionEntityCatalogRefreshRequest) (models.VisionEntityCatalog, error) {
	serviceWSURL, err := s.resolveCatalogServiceWSURL(ctx, req.ServiceWSURL)
	if err != nil {
		return models.VisionEntityCatalog{}, err
	}
	modelName, err := s.resolveCatalogModelName(ctx, serviceWSURL, req)
	if err != nil {
		return models.VisionEntityCatalog{}, err
	}
	catalog, err := s.fetchCatalog(ctx, serviceWSURL, modelName)
	if err != nil {
		return models.VisionEntityCatalog{}, err
	}
	if err := s.store.UpsertVisionCatalog(ctx, catalog); err != nil {
		return models.VisionEntityCatalog{}, err
	}
	return catalog, nil
}

func (s *Service) resolveCatalogServiceWSURL(ctx context.Context, rawURL string) (string, error) {
	serviceWSURL := normalizeServiceWSURL(rawURL)
	if serviceWSURL != "" {
		if err := validateServiceWSURL(serviceWSURL); err != nil {
			return "", err
		}
		return serviceWSURL, nil
	}
	config, err := s.GetConfig(ctx)
	if err != nil {
		return "", err
	}
	if config.ServiceWSURL == "" {
		return "", errors.New("vision service_ws_url is required to fetch supported entities")
	}
	if err := validateServiceWSURL(config.ServiceWSURL); err != nil {
		return "", err
	}
	return config.ServiceWSURL, nil
}

func (s *Service) resolveCatalogModelName(
	ctx context.Context,
	serviceWSURL string,
	req models.VisionEntityCatalogRefreshRequest,
) (string, error) {
	if normalized := normalizeModelName(req.ModelName); normalized != "" {
		return normalized, nil
	}
	config, err := s.GetConfig(ctx)
	if err != nil {
		return "", err
	}
	if req.ServiceWSURL != "" && normalizeServiceWSURL(req.ServiceWSURL) != normalizeServiceWSURL(config.ServiceWSURL) {
		return "", nil
	}
	return normalizeModelName(config.ModelName), nil
}

func (s *Service) fetchCatalog(ctx context.Context, serviceWSURL, modelName string) (models.VisionEntityCatalog, error) {
	catalog, err := s.session.FetchCatalog(ctx, serviceWSURL, modelName)
	if err != nil {
		return models.VisionEntityCatalog{}, fmt.Errorf("vision entity catalog fetch failed: %w", err)
	}
	return catalog, nil
}

func normalizeCatalog(serviceWSURL string, payload models.VisionServiceEntityCatalog) (models.VisionEntityCatalog, error) {
	schemaVersion := strings.TrimSpace(payload.SchemaVersion)
	if schemaVersion == "" {
		schemaVersion = visionEntityCatalogSchemaVersion
	}
	if schemaVersion != visionEntityCatalogSchemaVersion {
		return models.VisionEntityCatalog{}, fmt.Errorf("unsupported vision entity catalog schema_version %q", payload.SchemaVersion)
	}

	catalog := models.VisionEntityCatalog{
		ServiceWSURL:   normalizeServiceWSURL(serviceWSURL),
		SchemaVersion:  schemaVersion,
		ServiceVersion: strings.TrimSpace(payload.ServiceVersion),
		ModelName:      normalizeModelName(payload.ModelName),
		Entities:       make([]models.VisionEntityDescriptor, 0, len(payload.Entities)),
	}
	catalog.FetchedAt = payload.FetchedAt.UTC()
	if catalog.FetchedAt.IsZero() {
		catalog.FetchedAt = time.Now().UTC()
	}

	seen := map[string]struct{}{}
	for _, item := range payload.Entities {
		normalized := normalizeCatalogEntity(item)
		if normalized.Value == "" {
			continue
		}
		key := normalized.Kind + "\x00" + normalized.Value
		if _, exists := seen[key]; exists {
			continue
		}
		seen[key] = struct{}{}
		catalog.Entities = append(catalog.Entities, normalized)
	}
	slices.SortFunc(catalog.Entities, func(left, right models.VisionEntityDescriptor) int {
		if diff := strings.Compare(left.Kind, right.Kind); diff != 0 {
			return diff
		}
		return strings.Compare(left.Value, right.Value)
	})
	return catalog, nil
}

func normalizeCatalogEntity(item models.VisionEntityDescriptor) models.VisionEntityDescriptor {
	item.Kind = strings.TrimSpace(item.Kind)
	if item.Kind == "" {
		item.Kind = "label"
	}
	item.Value = strings.TrimSpace(item.Value)
	item.DisplayName = strings.TrimSpace(item.DisplayName)
	if item.DisplayName == "" {
		item.DisplayName = item.Value
	}
	return item
}

func normalizeServiceWSURL(value string) string {
	return strings.TrimRight(strings.TrimSpace(value), "/")
}

func validateServiceWSURL(value string) error {
	parsed, err := url.Parse(normalizeServiceWSURL(value))
	if err != nil {
		return fmt.Errorf("vision service_ws_url is invalid: %w", err)
	}
	if parsed.Scheme != "ws" && parsed.Scheme != "wss" {
		return errors.New("vision service_ws_url must use ws or wss")
	}
	if strings.TrimSpace(parsed.Host) == "" {
		return errors.New("vision service_ws_url host is required")
	}
	if strings.TrimSpace(parsed.Path) == "" || parsed.Path == "/" {
		return errors.New("vision service_ws_url must include the websocket endpoint path")
	}
	return nil
}

func normalizeModelName(value string) string {
	return strings.TrimSpace(value)
}

func (s *Service) validateConfigAgainstCatalog(ctx context.Context, config models.VisionCapabilityConfig) error {
	if config.ServiceWSURL == "" {
		return nil
	}
	catalog, ok, err := s.store.GetVisionCatalog(ctx)
	if err != nil || !ok {
		return err
	}
	if normalizeServiceWSURL(catalog.ServiceWSURL) != normalizeServiceWSURL(config.ServiceWSURL) {
		return nil
	}
	if expectedModel := normalizeModelName(config.ModelName); expectedModel != "" && normalizeModelName(catalog.ModelName) != expectedModel {
		return nil
	}
	if len(catalog.Entities) == 0 {
		return nil
	}

	allowed := make(map[string]struct{}, len(catalog.Entities))
	for _, entity := range catalog.Entities {
		key := entity.Kind + "\x00" + entity.Value
		allowed[key] = struct{}{}
	}
	for _, rule := range config.Rules {
		key := rule.EntitySelector.Kind + "\x00" + rule.EntitySelector.Value
		if _, ok := allowed[key]; ok {
			continue
		}
		return fmt.Errorf("vision rule %q entity_selector %q/%q is not advertised by the current vision model", rule.ID, rule.EntitySelector.Kind, rule.EntitySelector.Value)
	}
	return nil
}
