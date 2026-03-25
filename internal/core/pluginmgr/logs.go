package pluginmgr

import "sync"

type logBuffer struct {
	mu      sync.Mutex
	lines   []string
	maxSize int
}

func newLogBuffer(maxSize int) *logBuffer {
	if maxSize <= 0 {
		maxSize = 200
	}
	return &logBuffer{maxSize: maxSize}
}

func (b *logBuffer) Append(line string) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.lines = append(b.lines, line)
	if len(b.lines) > b.maxSize {
		b.lines = append([]string(nil), b.lines[len(b.lines)-b.maxSize:]...)
	}
}

func (b *logBuffer) Snapshot() []string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return append([]string(nil), b.lines...)
}

