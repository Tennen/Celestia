package pluginmgr

import "testing"

func TestHikvisionRuntimeModeFor(t *testing.T) {
	if got := hikvisionRuntimeModeFor("linux", "arm64"); got != "native" {
		t.Fatalf("hikvisionRuntimeModeFor(native) = %q, want native", got)
	}
	if got := hikvisionRuntimeModeFor("darwin", "arm64"); got != "docker" {
		t.Fatalf("hikvisionRuntimeModeFor(non-native) = %q, want docker", got)
	}
}

func TestHikvisionSDKLibDirDefaultFor(t *testing.T) {
	tests := []struct {
		name     string
		goos     string
		goarch   string
		override string
		existing map[string]bool
		want     string
	}{
		{
			name:     "env override wins",
			goos:     "linux",
			goarch:   "arm64",
			override: "/custom/sdk",
			existing: map[string]bool{},
			want:     "/custom/sdk",
		},
		{
			name:   "native uses repo sdk dir when container path missing",
			goos:   "linux",
			goarch: "arm64",
			existing: map[string]bool{
				"plugins/hikvision/sdk/lib/arm64": true,
			},
			want: "plugins/hikvision/sdk/lib/arm64",
		},
		{
			name:   "non-native falls back to container path",
			goos:   "darwin",
			goarch: "arm64",
			existing: map[string]bool{
				"plugins/hikvision/sdk/lib/arm64": true,
			},
			want: hikvisionContainerSDKPath,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := hikvisionSDKLibDirDefaultFor(tt.goos, tt.goarch, tt.override, func(path string) bool {
				return tt.existing[path]
			})
			if got != tt.want {
				t.Fatalf("hikvisionSDKLibDirDefaultFor() = %q, want %q", got, tt.want)
			}
		})
	}
}
