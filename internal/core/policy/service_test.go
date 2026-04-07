package policy

import (
	"testing"

	"github.com/chentianyu/celestia/internal/models"
)

func TestEvaluateTreatsAutomationActorAsAdmin(t *testing.T) {
	svc := New()

	decision := svc.Evaluate("automation:washer-done", "push_voice_message")
	if !decision.Allowed {
		t.Fatalf("Evaluate() should allow automation actor, got %#v", decision)
	}
	if decision.RiskLevel != models.RiskLevelLow {
		t.Fatalf("RiskLevel = %q, want %q", decision.RiskLevel, models.RiskLevelLow)
	}
}

func TestEvaluateTreatsAutomationActorAsAdminForHighRiskActions(t *testing.T) {
	svc := New()

	decision := svc.Evaluate("automation:washer-done", "start")
	if !decision.Allowed {
		t.Fatalf("Evaluate() should allow automation actor for admin-level actions, got %#v", decision)
	}
	if decision.RiskLevel != models.RiskLevelHigh {
		t.Fatalf("RiskLevel = %q, want %q", decision.RiskLevel, models.RiskLevelHigh)
	}
}
