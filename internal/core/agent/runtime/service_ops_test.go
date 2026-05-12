package runtime

import (
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"
)

func TestCelestiaServiceRestartStartsNewGatewayProcess(t *testing.T) {
	script := findServiceScript()
	if script == "" {
		t.Fatal("tool/celestia-service.sh not found")
	}
	tmp := t.TempDir()
	bin := filepath.Join(tmp, "fake-gateway.sh")
	if err := os.WriteFile(bin, []byte(`#!/usr/bin/env bash
trap 'exit 0' TERM INT
printf 'fake gateway started pid=%s addr=%s\n' "$$" "$CELESTIA_ADDR"
while true; do sleep 1; done
`), 0o755); err != nil {
		t.Fatalf("WriteFile(fake gateway) error = %v", err)
	}
	env := serviceScriptTestEnv(tmp, bin)
	if out, err := runServiceScriptForTest(script, env, "start"); err != nil {
		t.Fatalf("start error = %v\n%s", err, out)
	}
	t.Cleanup(func() {
		_, _ = runServiceScriptForTest(script, env, "stop")
	})
	firstPID := readServiceTestPID(t, tmp)
	if out, err := runServiceScriptForTest(script, env, "restart"); err != nil {
		t.Fatalf("restart error = %v\n%s", err, out)
	} else if !strings.Contains(out, "restart_scheduled") {
		t.Fatalf("restart output = %q, want restart_scheduled", out)
	}
	secondPID := waitForServiceTestNewPID(t, tmp, firstPID)
	if secondPID == firstPID {
		t.Fatalf("pid did not change after restart: %d", secondPID)
	}
	if out, err := runServiceScriptForTest(script, env, "status"); err != nil {
		t.Fatalf("status error = %v\n%s", err, out)
	} else if !strings.Contains(out, "running pid="+strconv.Itoa(secondPID)) {
		t.Fatalf("status output = %q, want running pid=%d", out, secondPID)
	}
}

func serviceScriptTestEnv(tmp string, bin string) []string {
	return append(os.Environ(),
		"CELESTIA_RUNTIME_DIR="+tmp,
		"CELESTIA_GATEWAY_PID_FILE="+filepath.Join(tmp, "gateway.pid"),
		"CELESTIA_GATEWAY_LOG_FILE="+filepath.Join(tmp, "gateway.log"),
		"CELESTIA_RESTART_PID_FILE="+filepath.Join(tmp, "gateway-restart.pid"),
		"CELESTIA_GATEWAY_BIN="+bin,
		"CELESTIA_ADDR=127.0.0.1:0",
		"CELESTIA_RESTART_DELAY_SECONDS=0",
		"CELESTIA_STOP_TIMEOUT_SECONDS=3",
	)
}

func runServiceScriptForTest(script string, env []string, args ...string) (string, error) {
	cmd := exec.Command(script, args...)
	cmd.Env = env
	out, err := cmd.CombinedOutput()
	return strings.TrimSpace(string(out)), err
}

func readServiceTestPID(t *testing.T, tmp string) int {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join(tmp, "gateway.pid"))
	if err != nil {
		t.Fatalf("ReadFile(pid) error = %v", err)
	}
	pid, err := strconv.Atoi(strings.TrimSpace(string(raw)))
	if err != nil {
		t.Fatalf("pid parse error = %v", err)
	}
	return pid
}

func waitForServiceTestNewPID(t *testing.T, tmp string, oldPID int) int {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		pid := readServiceTestPID(t, tmp)
		if pid != oldPID {
			return pid
		}
		time.Sleep(100 * time.Millisecond)
	}
	return readServiceTestPID(t, tmp)
}
