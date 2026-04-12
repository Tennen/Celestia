package vision

import (
	"strings"

	"github.com/chentianyu/celestia/internal/models"
)

func normalizeReportedEventStatus(value models.VisionServiceEventStatus) (models.VisionServiceEventStatus, bool) {
	switch models.VisionServiceEventStatus(strings.TrimSpace(string(value))) {
	case models.VisionServiceEventStatusThresholdMet:
		return models.VisionServiceEventStatusThresholdMet, true
	default:
		return "", false
	}
}
