package vision

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"slices"
	"strings"
	"time"

	"github.com/chentianyu/celestia/internal/models"
)

const (
	visionEntityCatalogPath          = "/api/v1/capabilities/" + models.VisionCapabilityID + "/entities"
	visionEntityCatalogSchemaVersion = "celestia.vision.catalog.v1"
)

func (s *Service) GetCatalog(ctx context.Context) (models.VisionEntityCatalog, bool, error) {
	return s.store.GetVisionCatalog(ctx)
}

func (s *Service) RefreshCatalog(ctx context.Context, req models.VisionEntityCatalogRefreshRequest) (models.VisionEntityCatalog, error) {
	serviceURL, err := s.resolveCatalogServiceURL(ctx, req.ServiceURL)
	if err != nil {
		return models.VisionEntityCatalog{}, err
	}
	catalog, err := s.fetchCatalog(ctx, serviceURL)
	if err != nil {
		return models.VisionEntityCatalog{}, err
	}
	if err := s.store.UpsertVisionCatalog(ctx, catalog); err != nil {
		return models.VisionEntityCatalog{}, err
	}
	return catalog, nil
}

func (s *Service) resolveCatalogServiceURL(ctx context.Context, rawURL string) (string, error) {
	serviceURL := normalizeServiceURL(rawURL)
	if serviceURL != "" {
		return serviceURL, nil
	}
	config, err := s.GetConfig(ctx)
	if err != nil {
		return "", err
	}
	if config.ServiceURL == "" {
		return "", errors.New("vision service_url is required to fetch supported entities")
	}
	return config.ServiceURL, nil
}

func (s *Service) fetchCatalog(ctx context.Context, serviceURL string) (models.VisionEntityCatalog, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, serviceURL+visionEntityCatalogPath, nil)
	if err != nil {
		return models.VisionEntityCatalog{}, err
	}
	resp, err := s.client.Do(req)
	if err != nil {
		return models.VisionEntityCatalog{}, fmt.Errorf("vision entity catalog fetch failed: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		return models.VisionEntityCatalog{}, fmt.Errorf("vision entity catalog fetch failed with status %d", resp.StatusCode)
	}

	var payload models.VisionServiceEntityCatalog
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return models.VisionEntityCatalog{}, fmt.Errorf("decode vision entity catalog: %w", err)
	}
	return normalizeCatalog(serviceURL, payload)
}

func normalizeCatalog(serviceURL string, payload models.VisionServiceEntityCatalog) (models.VisionEntityCatalog, error) {
	schemaVersion := strings.TrimSpace(payload.SchemaVersion)
	if schemaVersion == "" {
		schemaVersion = visionEntityCatalogSchemaVersion
	}
	if schemaVersion != visionEntityCatalogSchemaVersion {
		return models.VisionEntityCatalog{}, fmt.Errorf("unsupported vision entity catalog schema_version %q", payload.SchemaVersion)
	}

	catalog := models.VisionEntityCatalog{
		ServiceURL:     normalizeServiceURL(serviceURL),
		SchemaVersion:  schemaVersion,
		ServiceVersion: strings.TrimSpace(payload.ServiceVersion),
		ModelName:      strings.TrimSpace(payload.ModelName),
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

func normalizeServiceURL(value string) string {
	return strings.TrimRight(strings.TrimSpace(value), "/")
}

func (s *Service) validateConfigAgainstCatalog(ctx context.Context, config models.VisionCapabilityConfig) error {
	if config.ServiceURL == "" {
		return nil
	}
	catalog, ok, err := s.store.GetVisionCatalog(ctx)
	if err != nil || !ok {
		return err
	}
	if normalizeServiceURL(catalog.ServiceURL) != normalizeServiceURL(config.ServiceURL) {
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
