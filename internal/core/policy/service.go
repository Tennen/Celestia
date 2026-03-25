package policy

import (
	"strings"

	"github.com/chentianyu/celestia/internal/models"
)

type Service struct{}

func New() *Service {
	return &Service{}
}

func (s *Service) Evaluate(actor, action string) models.PolicyDecision {
	role := strings.ToLower(strings.TrimSpace(actor))
	if role == "" {
		role = "admin"
	}

	risk := models.RiskLevelLow
	lowerAction := strings.ToLower(action)
	switch {
	case strings.Contains(lowerAction, "stop"),
		strings.Contains(lowerAction, "clean"),
		strings.Contains(lowerAction, "feed"),
		strings.Contains(lowerAction, "start"):
		risk = models.RiskLevelHigh
	case strings.Contains(lowerAction, "resume"),
		strings.Contains(lowerAction, "pause"),
		strings.Contains(lowerAction, "set"),
		strings.Contains(lowerAction, "power"):
		risk = models.RiskLevelMedium
	}

	switch role {
	case "admin":
		return models.PolicyDecision{Allowed: true, RiskLevel: risk}
	case "agent":
		return models.PolicyDecision{
			Allowed:   risk != models.RiskLevelHigh,
			RiskLevel: risk,
			Reason:    "agent actor cannot execute high-risk actions",
		}
	case "viewer":
		return models.PolicyDecision{
			Allowed:   false,
			RiskLevel: risk,
			Reason:    "viewer actor is read-only",
		}
	default:
		return models.PolicyDecision{
			Allowed:   false,
			RiskLevel: risk,
			Reason:    "unknown actor role",
		}
	}
}

