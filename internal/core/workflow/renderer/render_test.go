package renderer

import (
	"strings"
	"testing"
)

func TestRendererCommandRewritesBundledRelativePath(t *testing.T) {
	command := rendererCommand("node internal/core/workflow/renderer/md2img/render.mjs", "/repo/internal/core/workflow/renderer/md2img/render.mjs")
	if !strings.Contains(command, "/repo/internal/core/workflow/renderer/md2img/render.mjs") {
		t.Fatalf("rendererCommand() = %q, want absolute bundled script path", command)
	}
}

func TestRendererCommandRewritesOldBundledPath(t *testing.T) {
	command := rendererCommand("node internal/core/agent/md2img/render.mjs", "/repo/internal/core/workflow/renderer/md2img/render.mjs")
	if strings.Contains(command, "internal/core/agent/md2img/render.mjs") {
		t.Fatalf("rendererCommand() = %q, want old bundled path rewritten", command)
	}
}

func TestRendererCommandPreservesCustomCommand(t *testing.T) {
	command := rendererCommand("node /opt/renderers/custom.mjs", "/repo/internal/core/workflow/renderer/md2img/render.mjs")
	if command != "node /opt/renderers/custom.mjs" {
		t.Fatalf("rendererCommand() = %q, want custom command unchanged", command)
	}
}
