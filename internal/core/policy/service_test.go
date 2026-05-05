package policy

import (
	"testing"

	"github.com/chentianyu/celestia/internal/models"
)

func TestEvaluateTreatsWorkflowActorAsAdmin(t *testing.T) {
	svc := New()

	decision := svc.Evaluate("workflow:washer-done", "push_voice_message")
	if !decision.Allowed {
		t.Fatalf("Evaluate() should allow workflow actor, got %#v", decision)
	}
	if decision.RiskLevel != models.RiskLevelLow {
		t.Fatalf("RiskLevel = %q, want %q", decision.RiskLevel, models.RiskLevelLow)
	}
}

func TestEvaluateTreatsWorkflowActorAsAdminForHighRiskActions(t *testing.T) {
	svc := New()

	decision := svc.Evaluate("workflow:washer-done", "start")
	if !decision.Allowed {
		t.Fatalf("Evaluate() should allow workflow actor for admin-level actions, got %#v", decision)
	}
	if decision.RiskLevel != models.RiskLevelHigh {
		t.Fatalf("RiskLevel = %q, want %q", decision.RiskLevel, models.RiskLevelHigh)
	}
}
