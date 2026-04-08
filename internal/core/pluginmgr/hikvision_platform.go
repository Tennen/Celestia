package pluginmgr

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

const (
	hikvisionNativePlatform   = "linux/arm64"
	hikvisionContainerSDKPath = "/opt/celestia/sdk/lib/arm64"
	hikvisionConfigModeCloud  = "cloud"
	hikvisionLauncherMode     = "launcher"
	hikvisionServerMode       = "server"
)

func hikvisionCatalogMetadata() map[string]any {
	return map[string]any{
		"runtime_mode":        hikvisionRuntimeMode(),
		"lan_runtime_mode":    hikvisionRuntimeMode(),
		"cloud_runtime_mode":  "native",
		"runtime_platform":    runtime.GOOS + "/" + runtime.GOARCH,
		"native_platform":     hikvisionNativePlatform,
		"native_supported":    hikvisionNativeSupported(),
		"sdk_lib_dir_default": hikvisionSDKLibDirDefault(),
	}
}

func hikvisionRuntimeMode() string {
	return hikvisionRuntimeModeFor(runtime.GOOS, runtime.GOARCH)
}

func hikvisionRuntimeModeFor(goos, goarch string) string {
	if hikvisionNativeSupportedFor(goos, goarch) {
		return "native"
	}
	return "docker"
}

func hikvisionNativeSupported() bool {
	return hikvisionNativeSupportedFor(runtime.GOOS, runtime.GOARCH)
}

func hikvisionNativeSupportedFor(goos, goarch string) bool {
	return goos == "linux" && goarch == "arm64"
}

func hikvisionSDKLibDirDefault() string {
	return hikvisionSDKLibDirDefaultFor(
		runtime.GOOS,
		runtime.GOARCH,
		strings.TrimSpace(os.Getenv("CELESTIA_HIKVISION_SDK_LIB_DIR")),
		func(path string) bool {
			info, err := os.Stat(path)
			return err == nil && info.IsDir()
		},
	)
}

func hikvisionSDKLibDirDefaultFor(goos, goarch, override string, dirExists func(string) bool) string {
	if override != "" {
		return override
	}

	candidates := []string{hikvisionContainerSDKPath}
	if hikvisionNativeSupportedFor(goos, goarch) {
		candidates = append(candidates,
			filepath.Join("plugins", "hikvision", "sdk", "lib", "arm64"),
			filepath.Join("sdk", "lib", "arm64"),
		)
	}

	for _, candidate := range candidates {
		if dirExists(candidate) {
			return candidate
		}
	}

	if hikvisionNativeSupportedFor(goos, goarch) {
		return filepath.Join("plugins", "hikvision", "sdk", "lib", "arm64")
	}
	return hikvisionContainerSDKPath
}

func hikvisionExtraEnv(pluginID string, config map[string]any) []string {
	if pluginID != "hikvision" || !hikvisionUsesCloudMode(config) {
		return nil
	}
	return []string{"CELESTIA_HIKVISION_PLUGIN_MODE=" + hikvisionServerMode}
}

func hikvisionUsesCloudMode(config map[string]any) bool {
	if config == nil {
		return false
	}
	mode, _ := config["mode"].(string)
	return strings.EqualFold(strings.TrimSpace(mode), hikvisionConfigModeCloud)
}

func hikvisionProcessModeForConfig(pluginID string, config map[string]any) string {
	if pluginID != "hikvision" {
		return ""
	}
	if hikvisionUsesCloudMode(config) {
		return hikvisionServerMode
	}
	if hikvisionNativeSupported() {
		return hikvisionServerMode
	}
	return hikvisionLauncherMode
}

func hikvisionRestartRequiredForConfig(pluginID string, current, next map[string]any) bool {
	currentMode := hikvisionProcessModeForConfig(pluginID, current)
	nextMode := hikvisionProcessModeForConfig(pluginID, next)
	return currentMode != "" && nextMode != "" && currentMode != nextMode
}
