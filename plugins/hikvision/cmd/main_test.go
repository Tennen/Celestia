package main

import (
	"os"
	"testing"
)

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
