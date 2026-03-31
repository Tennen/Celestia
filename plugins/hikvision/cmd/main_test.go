package main

import (
	"os"
	"testing"
)

func TestResolvePluginMode(t *testing.T) {
	tests := []struct {
		name      string
		requested string
		goos      string
		goarch    string
		sdk       bool
		want      string
	}{
		{
			name:   "native linux arm64 uses server mode",
			goos:   "linux",
			goarch: "arm64",
			sdk:    true,
			want:   modeServer,
		},
		{
			name:   "non native falls back to launcher",
			goos:   "darwin",
			goarch: "arm64",
			sdk:    false,
			want:   modeLauncher,
		},
		{
			name:      "explicit launcher override wins",
			requested: modeLauncher,
			goos:      "linux",
			goarch:    "arm64",
			sdk:       true,
			want:      modeLauncher,
		},
		{
			name:      "explicit server override wins",
			requested: modeServer,
			goos:      "darwin",
			goarch:    "amd64",
			sdk:       true,
			want:      modeServer,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := resolvePluginMode(tt.requested, tt.goos, tt.goarch, tt.sdk)
			if got != tt.want {
				t.Fatalf("resolvePluginMode() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestRemapCoreAddr(t *testing.T) {
	got := remapCoreAddr("127.0.0.1:18080")
	want := "host.docker.internal:18080"
	if got != want {
		t.Fatalf("remapCoreAddr() = %q, want %q", got, want)
	}
	if unchanged := remapCoreAddr("10.0.0.5:18080"); unchanged != "10.0.0.5:18080" {
		t.Fatalf("unexpected remap for non-loopback: %q", unchanged)
	}
}

func TestAddHostEnabledFlag(t *testing.T) {
	original := os.Getenv("CELESTIA_HIKVISION_DOCKER_ADD_HOST_GATEWAY")
	t.Cleanup(func() {
		_ = os.Setenv("CELESTIA_HIKVISION_DOCKER_ADD_HOST_GATEWAY", original)
	})

	_ = os.Setenv("CELESTIA_HIKVISION_DOCKER_ADD_HOST_GATEWAY", "false")
	if addHostEnabled() {
		t.Fatal("addHostEnabled() should be false")
	}
	_ = os.Setenv("CELESTIA_HIKVISION_DOCKER_ADD_HOST_GATEWAY", "")
	if !addHostEnabled() {
		t.Fatal("addHostEnabled() should default to true")
	}
}
