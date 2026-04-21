package agent

import (
	"encoding/json"
	"strings"

	"github.com/chentianyu/celestia/internal/models"
)

func buildEvolutionPlanPrompt(goal string) string {
	return strings.Join([]string{
		"You are a senior implementation planner for this repository.",
		"Analyze the current repo and produce a concise executable plan.",
		"Return strict JSON only in this shape:",
		`{"steps":["step 1","step 2"]}`,
		"Each step must be independently verifiable and limited to necessary files.",
		"",
		"GOAL:",
		goal,
	}, "\n")
}

func buildEvolutionStepPrompt(goal models.AgentEvolutionGoal, stepIndex int, stepText string) string {
	lines := []string{
		"You are implementing a planned repository change.",
		"Modify files directly. Keep edits focused and leave unrelated changes alone.",
		"",
		"GOAL:",
		goal.Goal,
		"",
		"PLAN:",
	}
	for idx, step := range goal.Plan.Steps {
		lines = append(lines, intString(idx+1)+". "+step)
	}
	lines = append(lines,
		"",
		"CURRENT STEP "+intString(stepIndex+1)+":",
		stepText,
	)
	return strings.Join(lines, "\n")
}

func buildEvolutionFixPrompt(goal string, checks string) string {
	return strings.Join([]string{
		"You are fixing failed checks for the current repository.",
		"Only fix the failures shown below. Avoid unrelated refactors.",
		"",
		"GOAL:",
		goal,
		"",
		"CHECK FAILURES:",
		checks,
	}, "\n")
}

func parseEvolutionSteps(output string) []string {
	var parsed struct {
		Steps []string `json:"steps"`
	}
	if json.Unmarshal([]byte(extractJSONObject(output)), &parsed) != nil {
		return nil
	}
	steps := []string{}
	for _, step := range parsed.Steps {
		trimmed := strings.TrimSpace(step)
		if trimmed != "" {
			steps = append(steps, trimmed)
		}
	}
	return steps
}

func extractJSONObject(text string) string {
	trimmed := strings.TrimSpace(text)
	start := strings.Index(trimmed, "{")
	end := strings.LastIndex(trimmed, "}")
	if start >= 0 && end > start {
		return trimmed[start : end+1]
	}
	return trimmed
}

func summarizeEvolutionTests(results []models.AgentEvolutionTestResult) string {
	lines := []string{}
	for _, result := range results {
		status := "passed"
		if !result.OK {
			status = "failed"
		}
		lines = append(lines, "- "+firstNonEmpty(result.Name, result.Command)+" "+status+": "+trimEvolutionText(result.Output, 1200))
	}
	return strings.Join(lines, "\n")
}

func deterministicEvolutionCommitMessage(goal string) string {
	trimmed := strings.TrimSpace(goal)
	if trimmed == "" {
		return "chore: apply evolution goal"
	}
	words := strings.Fields(trimmed)
	if len(words) > 8 {
		words = words[:8]
	}
	message := "chore: " + strings.Join(words, " ")
	if len(message) > 72 {
		return message[:72]
	}
	return message
}

func trimEvolutionText(text string, max int) string {
	trimmed := strings.TrimSpace(text)
	if max <= 0 || len(trimmed) <= max {
		return trimmed
	}
	return trimmed[len(trimmed)-max:]
}

func evolutionShellQuote(value string) string {
	return "'" + strings.ReplaceAll(value, "'", "'\"'\"'") + "'"
}
