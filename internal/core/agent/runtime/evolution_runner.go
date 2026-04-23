package runtime

import (
	"bytes"
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/chentianyu/celestia/internal/models"
)

func (s *Service) evolutionPlan(ctx context.Context, goal models.AgentEvolutionGoal, settings models.AgentEvolutionConfig) (models.AgentEvolutionGoal, error) {
	goal.Stage = "plan"
	goal.Events = append(goal.Events, models.AgentEvolutionEvent{At: time.Now().UTC(), Stage: "plan", Message: "Generating Codex plan."})
	goal, _ = s.saveEvolutionGoal(ctx, goal)
	result, err := s.RunCodex(ctx, models.AgentCodexRequest{
		TaskID:          goal.ID + "-plan",
		Prompt:          buildEvolutionPlanPrompt(goal.Goal),
		Model:           settings.CodexModel,
		ReasoningEffort: settings.CodexReasoning,
		TimeoutMS:       settings.TimeoutMS,
		CWD:             settings.CWD,
	})
	if err != nil {
		return goal, errors.New(firstNonEmpty(result.Error, err.Error()))
	}
	steps := parseEvolutionSteps(result.Output)
	if len(steps) == 0 {
		return goal, errors.New("Codex plan output did not include executable steps")
	}
	goal.Plan.Steps = steps
	goal.Plan.CurrentStep = 0
	goal.LastCodexOutput = result.Output
	goal.Stage = "plan_ready"
	goal.UpdatedAt = time.Now().UTC()
	goal.Events = append(goal.Events, models.AgentEvolutionEvent{At: goal.UpdatedAt, Stage: "plan_ready", Message: "Plan generated with " + intString(len(steps)) + " steps."})
	goal.RawTail = appendEvolutionRaw(goal.RawTail, result.Output)
	return s.saveEvolutionGoal(ctx, goal)
}

func (s *Service) evolutionStep(ctx context.Context, goal models.AgentEvolutionGoal, settings models.AgentEvolutionConfig, stepIndex int) (models.AgentEvolutionGoal, error) {
	stepText := goal.Plan.Steps[stepIndex]
	stage := "step_" + intString(stepIndex+1)
	goal.Stage = stage
	goal.Events = append(goal.Events, models.AgentEvolutionEvent{At: time.Now().UTC(), Stage: stage, Message: "Executing step " + intString(stepIndex+1) + "."})
	goal, _ = s.saveEvolutionGoal(ctx, goal)
	result, err := s.RunCodex(ctx, models.AgentCodexRequest{
		TaskID:          goal.ID + "-" + stage,
		Prompt:          buildEvolutionStepPrompt(goal, stepIndex, stepText),
		Model:           settings.CodexModel,
		ReasoningEffort: settings.CodexReasoning,
		TimeoutMS:       settings.TimeoutMS,
		CWD:             settings.CWD,
	})
	if err != nil {
		return goal, errors.New(firstNonEmpty(result.Error, err.Error()))
	}
	goal.Plan.CurrentStep = stepIndex + 1
	goal.LastCodexOutput = result.Output
	goal.Stage = stage + "_done"
	goal.UpdatedAt = time.Now().UTC()
	goal.Events = append(goal.Events, models.AgentEvolutionEvent{At: goal.UpdatedAt, Stage: goal.Stage, Message: "Step " + intString(stepIndex+1) + " completed."})
	goal.RawTail = appendEvolutionRaw(goal.RawTail, result.Output)
	return s.saveEvolutionGoal(ctx, goal)
}

func (s *Service) evolutionFix(ctx context.Context, goal models.AgentEvolutionGoal, settings models.AgentEvolutionConfig, attempt int) (models.AgentEvolutionGoal, error) {
	summary := summarizeEvolutionTests(goal.TestResults)
	result, err := s.RunCodex(ctx, models.AgentCodexRequest{
		TaskID:          goal.ID + "-fix-" + intString(attempt),
		Prompt:          buildEvolutionFixPrompt(goal.Goal, summary),
		Model:           settings.CodexModel,
		ReasoningEffort: settings.CodexReasoning,
		TimeoutMS:       settings.TimeoutMS,
		CWD:             settings.CWD,
	})
	if err != nil {
		return goal, errors.New(firstNonEmpty(result.Error, err.Error()))
	}
	goal.FixAttempts = attempt
	goal.Stage = "fix_" + intString(attempt) + "_done"
	goal.LastCodexOutput = result.Output
	goal.UpdatedAt = time.Now().UTC()
	goal.Events = append(goal.Events, models.AgentEvolutionEvent{At: goal.UpdatedAt, Stage: goal.Stage, Message: "Fix attempt " + intString(attempt) + " completed."})
	goal.RawTail = appendEvolutionRaw(goal.RawTail, result.Output)
	return s.saveEvolutionGoal(ctx, goal)
}

func (s *Service) evolutionStructureReview(ctx context.Context, goal models.AgentEvolutionGoal, settings models.AgentEvolutionConfig) (models.AgentEvolutionGoal, error) {
	result, err := s.RunCodex(ctx, models.AgentCodexRequest{
		TaskID:          goal.ID + "-structure",
		Prompt:          "Review the current repository structure for duplicated or misplaced code. Return JSON only: {\"issues\":[\"...\"]}.",
		Model:           settings.CodexModel,
		ReasoningEffort: settings.CodexReasoning,
		TimeoutMS:       settings.TimeoutMS,
		CWD:             settings.CWD,
	})
	if err != nil {
		return goal, errors.New(firstNonEmpty(result.Error, err.Error()))
	}
	goal.Stage = "structure_done"
	goal.LastCodexOutput = result.Output
	goal.UpdatedAt = time.Now().UTC()
	goal.Events = append(goal.Events, models.AgentEvolutionEvent{At: goal.UpdatedAt, Stage: goal.Stage, Message: "Structure review completed."})
	goal.RawTail = appendEvolutionRaw(goal.RawTail, result.Output)
	return s.saveEvolutionGoal(ctx, goal)
}

func (s *Service) evolutionChecks(ctx context.Context, goal models.AgentEvolutionGoal, settings models.AgentEvolutionConfig) (models.AgentEvolutionGoal, bool, error) {
	commands := resolveEvolutionTestCommands(settings)
	results := make([]models.AgentEvolutionTestResult, 0, len(commands))
	allOK := true
	for _, command := range commands {
		result := runEvolutionShell(ctx, settings, command.Command, maxInt(command.TimeoutMS, settings.TimeoutMS))
		result.Name = firstNonEmpty(command.Name, command.Command)
		results = append(results, result)
		if !result.OK {
			allOK = false
		}
	}
	goal.TestResults = results
	goal.Stage = "checks"
	goal.UpdatedAt = time.Now().UTC()
	message := "Checks passed."
	if !allOK {
		message = "Checks failed."
	}
	goal.Events = append(goal.Events, models.AgentEvolutionEvent{At: goal.UpdatedAt, Stage: "checks", Message: message})
	goal.RawTail = appendEvolutionRaw(goal.RawTail, summarizeEvolutionTests(results))
	goal, err := s.saveEvolutionGoal(ctx, goal)
	return goal, allOK, err
}

func (s *Service) evolutionCommit(ctx context.Context, goal models.AgentEvolutionGoal, settings models.AgentEvolutionConfig) (models.AgentEvolutionGoal, error) {
	message := firstNonEmpty(goal.CommitMessage, deterministicEvolutionCommitMessage(goal.Goal))
	if result := runEvolutionShell(ctx, settings, "git add -A", 120000); !result.OK {
		return goal, errors.New(result.Output)
	}
	if result := runEvolutionShell(ctx, settings, "git commit --allow-empty -m "+evolutionShellQuote(message), 120000); !result.OK {
		return goal, errors.New(result.Output)
	}
	ref, _ := evolutionGitOutput(ctx, settings, "git rev-parse HEAD")
	goal.CompletedCommit = strings.TrimSpace(ref)
	goal.CommitMessage = message
	goal.Stage = "commit_done"
	goal.UpdatedAt = time.Now().UTC()
	goal.Events = append(goal.Events, models.AgentEvolutionEvent{At: goal.UpdatedAt, Stage: "commit", Message: "Committed changes: " + message})
	return s.saveEvolutionGoal(ctx, goal)
}

func (s *Service) evolutionPush(ctx context.Context, goal models.AgentEvolutionGoal, settings models.AgentEvolutionConfig) (models.AgentEvolutionGoal, error) {
	remote := firstNonEmpty(settings.PushRemote, "origin")
	branch := strings.TrimSpace(settings.PushBranch)
	if branch == "" {
		out, err := evolutionGitOutput(ctx, settings, "git rev-parse --abbrev-ref HEAD")
		if err != nil {
			return goal, err
		}
		branch = strings.TrimSpace(out)
	}
	if result := runEvolutionShell(ctx, settings, "git push "+evolutionShellQuote(remote)+" HEAD:"+evolutionShellQuote(branch), 120000); !result.OK {
		return goal, errors.New(result.Output)
	}
	goal.Stage = "push_done"
	goal.UpdatedAt = time.Now().UTC()
	goal.Events = append(goal.Events, models.AgentEvolutionEvent{At: goal.UpdatedAt, Stage: "push", Message: "Pushed HEAD to " + remote + "/" + branch})
	return s.saveEvolutionGoal(ctx, goal)
}

func (s *Service) runLegacyEvolutionCommand(ctx context.Context, goal models.AgentEvolutionGoal, settings models.AgentEvolutionConfig) (models.AgentEvolutionGoal, error) {
	result := runEvolutionShellWithInput(ctx, settings, settings.Command, goal.Goal, settings.TimeoutMS)
	goal.TestResults = []models.AgentEvolutionTestResult{result}
	goal.RawTail = appendEvolutionRaw(goal.RawTail, result.Output)
	if !result.OK {
		return s.failEvolutionGoal(ctx, goal.ID, "legacy_failed", result.Output)
	}
	now := time.Now().UTC()
	goal.Status = "succeeded"
	goal.Stage = "completed"
	goal.CompletedAt = &now
	goal.UpdatedAt = now
	goal.Events = append(goal.Events, models.AgentEvolutionEvent{At: now, Stage: "completed", Message: "Legacy evolution command completed."})
	return s.saveEvolutionGoal(ctx, goal)
}

func resolveEvolutionTestCommands(settings models.AgentEvolutionConfig) []models.AgentEvolutionTestCommand {
	if len(settings.TestCommands) > 0 {
		return settings.TestCommands
	}
	commands := []models.AgentEvolutionTestCommand{}
	if fileExists(filepath.Join(firstNonEmpty(settings.CWD, "."), "go.mod")) {
		commands = append(commands, models.AgentEvolutionTestCommand{Name: "go test", Command: "GOCACHE=$(pwd)/.cache/go-build GOMODCACHE=$(pwd)/.cache/gomod go test ./..."})
	}
	if fileExists(filepath.Join(firstNonEmpty(settings.CWD, "."), "package.json")) {
		commands = append(commands, models.AgentEvolutionTestCommand{Name: "web build", Command: "npm run build --workspace web/admin"})
	}
	if len(commands) == 0 && strings.TrimSpace(settings.Command) != "" {
		commands = append(commands, models.AgentEvolutionTestCommand{Name: "configured command", Command: settings.Command})
	}
	return commands
}

func runEvolutionShell(ctx context.Context, settings models.AgentEvolutionConfig, command string, timeoutMS int) models.AgentEvolutionTestResult {
	return runEvolutionShellWithInput(ctx, settings, command, "", timeoutMS)
}

func runEvolutionShellWithInput(ctx context.Context, settings models.AgentEvolutionConfig, command string, input string, timeoutMS int) models.AgentEvolutionTestResult {
	started := time.Now().UTC()
	timeout := time.Duration(maxInt(timeoutMS, 600000)) * time.Millisecond
	runCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	cmd := exec.CommandContext(runCtx, "/bin/sh", "-lc", command)
	if strings.TrimSpace(settings.CWD) != "" {
		cmd.Dir = settings.CWD
	}
	if strings.TrimSpace(input) != "" {
		cmd.Stdin = strings.NewReader(input)
	}
	var output bytes.Buffer
	cmd.Stdout = &output
	cmd.Stderr = &output
	err := cmd.Run()
	finished := time.Now().UTC()
	exitCode := 0
	if err != nil {
		exitCode = 1
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			exitCode = exitErr.ExitCode()
		}
	}
	text := strings.TrimSpace(output.String())
	if runCtx.Err() == context.DeadlineExceeded {
		text = strings.TrimSpace(text + "\ncommand timed out")
		err = runCtx.Err()
	}
	return models.AgentEvolutionTestResult{
		Command:    command,
		OK:         err == nil,
		ExitCode:   exitCode,
		Output:     trimEvolutionText(text, 5000),
		StartedAt:  started,
		FinishedAt: finished,
	}
}

func evolutionGitOutput(ctx context.Context, settings models.AgentEvolutionConfig, command string) (string, error) {
	result := runEvolutionShell(ctx, settings, command, 120000)
	if !result.OK {
		return "", errors.New(result.Output)
	}
	return result.Output, nil
}

func appendEvolutionRaw(lines []models.AgentEvolutionRawLine, text string) []models.AgentEvolutionRawLine {
	for _, line := range strings.Split(strings.TrimSpace(text), "\n") {
		trimmed := strings.TrimSpace(line)
		if trimmed != "" {
			lines = append(lines, models.AgentEvolutionRawLine{At: time.Now().UTC(), Line: trimmed})
		}
	}
	if len(lines) > 120 {
		return lines[len(lines)-120:]
	}
	return lines
}

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}
