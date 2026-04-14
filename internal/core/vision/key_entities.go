package vision

import (
	"fmt"
	"slices"
	"strings"

	"github.com/chentianyu/celestia/internal/models"
)

func normalizeRuleKeyEntities(ruleID string, items []models.VisionRuleKeyEntity) ([]models.VisionRuleKeyEntity, error) {
	if len(items) == 0 {
		return []models.VisionRuleKeyEntity{}, nil
	}

	out := make([]models.VisionRuleKeyEntity, 0, len(items))
	seen := make(map[int]struct{}, len(items))
	for idx, item := range items {
		normalized, err := normalizeRuleKeyEntity(ruleID, idx, item)
		if err != nil {
			return nil, err
		}
		if _, exists := seen[normalized.ID]; exists {
			return nil, fmt.Errorf("vision rule %q key_entities[%d].id %d is duplicated", ruleID, idx, normalized.ID)
		}
		seen[normalized.ID] = struct{}{}
		out = append(out, normalized)
	}

	slices.SortFunc(out, func(left, right models.VisionRuleKeyEntity) int {
		return left.ID - right.ID
	})
	return out, nil
}

func normalizeRuleKeyEntity(ruleID string, idx int, item models.VisionRuleKeyEntity) (models.VisionRuleKeyEntity, error) {
	if item.ID <= 0 {
		return models.VisionRuleKeyEntity{}, fmt.Errorf("vision rule %q key_entities[%d].id must be positive", ruleID, idx)
	}

	item.Description = strings.TrimSpace(item.Description)
	if item.Image != nil {
		image := *item.Image
		image.Base64 = strings.TrimSpace(image.Base64)
		image.ContentType = strings.TrimSpace(image.ContentType)
		if image.Base64 == "" {
			item.Image = nil
		} else {
			item.Image = &image
		}
	}

	if item.Image == nil && item.Description == "" {
		return models.VisionRuleKeyEntity{}, fmt.Errorf("vision rule %q key_entities[%d] requires image or description", ruleID, idx)
	}
	return item, nil
}

func normalizeReportedKeyEntityID(id *int) *int {
	if id == nil || *id <= 0 {
		return nil
	}
	value := *id
	return &value
}
