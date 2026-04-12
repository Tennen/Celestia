package vision

import (
	"strings"

	"github.com/chentianyu/celestia/internal/models"
)

func normalizeReportedEntities(item models.VisionServiceEvent) []models.VisionEntityDescriptor {
	out := make([]models.VisionEntityDescriptor, 0, len(item.Entities)+1)
	seen := map[string]struct{}{}
	for _, entity := range item.Entities {
		appendReportedEntity(&out, seen, entity)
	}
	if fallback := strings.TrimSpace(item.EntityValue); fallback != "" {
		appendReportedEntity(&out, seen, models.VisionEntityDescriptor{
			Kind:  "label",
			Value: fallback,
		})
	}
	return out
}

func appendReportedEntity(
	out *[]models.VisionEntityDescriptor,
	seen map[string]struct{},
	item models.VisionEntityDescriptor,
) {
	normalized := normalizeCatalogEntity(item)
	if normalized.Value == "" {
		return
	}
	key := normalized.Kind + "\x00" + normalized.Value
	if _, exists := seen[key]; exists {
		return
	}
	seen[key] = struct{}{}
	*out = append(*out, normalized)
}

func primaryReportedEntityValue(entities []models.VisionEntityDescriptor, fallback string) string {
	if len(entities) > 0 {
		return entities[0].Value
	}
	return strings.TrimSpace(fallback)
}

func summarizeReportedEntities(entities []models.VisionEntityDescriptor, fallback string) string {
	if len(entities) == 0 {
		return strings.TrimSpace(fallback)
	}
	labels := make([]string, 0, len(entities))
	for _, entity := range entities {
		label := strings.TrimSpace(entity.DisplayName)
		if label == "" {
			label = entity.Value
		}
		if label == "" {
			continue
		}
		labels = append(labels, label)
	}
	return strings.Join(labels, ", ")
}
