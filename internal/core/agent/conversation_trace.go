package agent

import (
	"context"
	"strings"
	"sync"
	"time"

	einotool "github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/schema"
	"github.com/google/uuid"
)

const tracePreviewLimit = 900

type conversationTraceContextKey struct{}

type conversationTraceEvent struct {
	ID         string         `json:"id"`
	Kind       string         `json:"kind"`
	Name       string         `json:"name,omitempty"`
	Status     string         `json:"status"`
	StartedAt  time.Time      `json:"started_at"`
	FinishedAt time.Time      `json:"finished_at"`
	DurationMS int64          `json:"duration_ms"`
	Input      string         `json:"input,omitempty"`
	Output     string         `json:"output,omitempty"`
	Error      string         `json:"error,omitempty"`
	Metadata   map[string]any `json:"metadata,omitempty"`
}

type conversationTraceLogger struct {
	mu     sync.Mutex
	events []conversationTraceEvent
}

func newConversationTraceLogger() *conversationTraceLogger {
	return &conversationTraceLogger{events: []conversationTraceEvent{}}
}

func withConversationTrace(ctx context.Context, logger *conversationTraceLogger) context.Context {
	return context.WithValue(ctx, conversationTraceContextKey{}, logger)
}

func conversationTraceFromContext(ctx context.Context) *conversationTraceLogger {
	logger, _ := ctx.Value(conversationTraceContextKey{}).(*conversationTraceLogger)
	return logger
}

func (l *conversationTraceLogger) record(event conversationTraceEvent) {
	if l == nil {
		return
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	if event.ID == "" {
		event.ID = uuid.NewString()
	}
	l.events = append(l.events, event)
	if len(l.events) > 80 {
		l.events = l.events[len(l.events)-80:]
	}
}

func (l *conversationTraceLogger) eventsSnapshot() []conversationTraceEvent {
	if l == nil {
		return []conversationTraceEvent{}
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	out := make([]conversationTraceEvent, len(l.events))
	copy(out, l.events)
	return out
}

func statusFromError(err error) string {
	if err != nil {
		return "failed"
	}
	return "succeeded"
}

func clipTrace(value string) string {
	text := strings.TrimSpace(value)
	runes := []rune(text)
	if len(runes) <= tracePreviewLimit {
		return text
	}
	return string(runes[:tracePreviewLimit]) + "..."
}

type tracedAgentTool struct {
	name  string
	inner einotool.InvokableTool
}

func (t tracedAgentTool) Info(ctx context.Context) (*schema.ToolInfo, error) {
	return t.inner.Info(ctx)
}

func (t tracedAgentTool) InvokableRun(ctx context.Context, argumentsInJSON string, opts ...einotool.Option) (string, error) {
	started := time.Now().UTC()
	output, err := t.inner.InvokableRun(ctx, argumentsInJSON, opts...)
	finished := time.Now().UTC()
	if logger := conversationTraceFromContext(ctx); logger != nil {
		logger.record(conversationTraceEvent{
			Kind:       "tool_call",
			Name:       t.name,
			Status:     statusFromError(err),
			StartedAt:  started,
			FinishedAt: finished,
			DurationMS: finished.Sub(started).Milliseconds(),
			Input:      clipTrace(argumentsInJSON),
			Output:     clipTrace(output),
			Error:      errorString(err),
			Metadata: map[string]any{
				"output_chars": len([]rune(output)),
			},
		})
	}
	return output, err
}

func traceToolRuntimes(runtimes []agentToolRuntime) []agentToolRuntime {
	out := make([]agentToolRuntime, 0, len(runtimes))
	for _, runtime := range runtimes {
		next := runtime
		next.tool = tracedAgentTool{name: runtime.spec.Name, inner: runtime.tool}
		out = append(out, next)
	}
	return out
}

func errorString(err error) string {
	if err == nil {
		return ""
	}
	return err.Error()
}
