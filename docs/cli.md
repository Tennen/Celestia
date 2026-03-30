# Celestia CLI Architecture (Agent-Oriented)

## Tooling Research

For a production CLI that must be reliable for agent-driven automation, we evaluated mainstream Go CLI libraries:

- `spf13/cobra`
  - very high ecosystem adoption and long-running stability
  - mature subcommand and flag model
  - built-in shell completion and docs generation
- `urfave/cli/v3`
  - modern API and active v3 line
  - good developer ergonomics with strong flag support
- `alecthomas/kong`
  - declarative struct-based parsing, powerful validation/config mapping
  - strong fit for config-heavy CLIs
- `peterbourgon/ff/ffcli`
  - intentionally small API surface and easy maintainability
  - lower ecosystem penetration, and v4 remains in beta series

Decision: **Cobra** for Celestia `celctl`.

Rationale:

1. Best balance of stability + ecosystem maturity for long-term maintenance.
2. Strong subcommand model for extending the control plane surface.
3. Good fit for agent usage with predictable command tree and machine-readable output.

## Shared API/CLI Invocation Abstraction

To avoid duplicating behavior across HTTP handlers and CLI commands, Celestia now uses a shared service layer under:

- `internal/api/gateway`

Core pieces:

- `Service` interface: canonical methods for plugin/device/event/audit/dashboard/health flows.
- `RuntimeService`: implementation used by HTTP API (`internal/api/http`) and backed directly by Core runtime.
- `HTTPService`: implementation used by CLI (`cmd/celctl`) and backed by gateway HTTP endpoints.

This gives one method contract with two transport entry points:

1. HTTP API entrypoint (`internal/api/http`) for remote/system integrations.
2. CLI entrypoint (`cmd/celctl`) for local/operator/agent workflows.

## CLI Design Principles for Agents

- Default output is structured JSON (`--output json`).
- Human-friendly pretty output remains available (`--output pretty`).
- Explicit non-zero exit on errors.
- Stable global options:
  - `--base-url` (or `CELESTIA_URL`)
  - `--timeout`
  - `--actor`
- Structured argument flags to reduce shell escaping complexity:
  - `--config-json`
  - `--metadata-json`
  - `--params-json`

## Command Surface

- `celctl dashboard`
- `celctl health`
- `celctl plugins [list|catalog|install|config|enable|disable|discover|delete|logs]`
- `celctl devices [list|get|alias|control|command|toggle|action]`
- `celctl events`
- `celctl audits`
