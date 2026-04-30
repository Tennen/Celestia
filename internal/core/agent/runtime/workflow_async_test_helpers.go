package runtime

import (
	"testing"
	"time"
)

func waitForWorkflowTestCondition(t *testing.T, timeout time.Duration, message string, check func() bool) {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if check() {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	if !check() {
		t.Fatal(message)
	}
}
