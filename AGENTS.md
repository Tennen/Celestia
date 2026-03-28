# Celestia Agent Rules

## Goal

Celestia is a production-oriented monorepo for a process-isolated home device gateway.
Every implementation task must target the final operating model:

- real vendor cloud authentication
- real device discovery
- real state synchronization
- real command delivery
- real event propagation

Do not optimize for demos, placeholders, transitional adapters, or "temporary" persistence substitutes.

## Non-Negotiable Constraints

- Never introduce `mock`, `fake`, `stub`, `fixture`, `demo`, seeded accounts, or synthetic device data into production code paths.
- Never satisfy a product requirement with in-memory simulation when the requirement is for a real vendor integration.
- Never add dual-track code where a mock path silently substitutes for an unavailable real path.
- If credentials, vendor permissions, or upstream API limitations block completion, fail explicitly and surface the blocker. Do not fabricate behavior.
- Placeholder values are allowed only in documentation or config examples, and they must be clearly marked as user-supplied secrets or identifiers.

## Code Size Rule

- No code file may exceed 500 lines.
- When an existing file grows near this limit, split it by responsibility before adding more logic.
- New code must be organized into reasonably scoped modules instead of extending large catch-all files.

## Repository Architecture Pattern

All work in this repository must preserve the current architecture:

1. Core gateway in Go under the root module.
2. Vendor integrations implemented as separate plugin processes.
3. Core-to-plugin communication over the existing gRPC plugin protocol.
4. Core owns the unified device model, policy, audit, registry, state store, and HTTP API.
5. Plugins own vendor auth, vendor discovery, vendor state translation, vendor command execution, and vendor event ingestion.
6. Persistence remains SQLite-backed in the core unless a requirement explicitly calls for another production-grade backing service.
7. Admin UI remains a Vite + React + shadcn/ui surface over the gateway API, not a side-channel integration path.
8. Plugin configuration and runtime-derived credential persistence are Core-owned concerns. Plugins must request config changes through a Core-exposed abstraction and must not persist config through event side channels or direct storage access.
9. Admin configuration defaults and editable plugin config surfaces must be driven by Core-exposed catalog/schema data. Do not maintain a second frontend-owned source of truth for plugin defaults or vendor compatibility knobs.

## Runtime Flow

Treat the end-to-end runtime as a fixed pipeline unless the user explicitly asks to change it:

1. `cmd/gateway` boots Core runtime and SQLite persistence.
2. Core plugin manager starts each enabled vendor plugin as its own process.
3. Core and plugin communicate only through the existing gRPC plugin protocol in `internal/pluginapi`.
4. Core seeds device inventory by calling plugin discovery/state RPCs and persists unified devices/states into Core-owned storage.
5. Plugins keep vendor sessions, poll vendor APIs when required, translate vendor payloads into unified models, and emit runtime events back to Core.
6. HTTP command requests enter through `internal/api/http`, pass policy and audit, then are forwarded by Core to the owning plugin for real vendor execution.
7. Admin UI in `web/admin` reads and writes only through the gateway HTTP API.

## Directory Responsibilities

Use these boundaries when deciding where code belongs:

- `cmd/gateway`: production gateway bootstrap only. No vendor logic and no admin-only shortcuts.
- `cmd/celctl`: lightweight CLI for calling the gateway HTTP API.
- `docs`: repository documentation. Keep API and operational docs in sync with behavior changes.
- `proto`: plugin protocol definitions. Protocol changes must remain compatible with Core and plugin implementations.
- `web/admin`: admin UI only. It must not implement vendor-side business logic or maintain duplicate plugin defaults outside Core-owned schemas.
- `plugins/xiaomi/cmd`, `plugins/petkit/cmd`, `plugins/haier/cmd`: plugin process entrypoints only.
- `plugins/xiaomi/internal/app`: Xiaomi plugin runtime, RPC surface, refresh orchestration, command execution, and Core config persistence hooks.
- `plugins/xiaomi/internal/cloud`: Xiaomi cloud auth/session handling and MIoT HTTP transport.
- `plugins/xiaomi/internal/mapper`: Xiaomi vendor-to-unified model/capability mapping.
- `plugins/xiaomi/internal/spec`: MIoT spec lookup/parsing support.
- `plugins/petkit/internal/app`: Petkit auth, sync, device normalization, command dispatch, BLE relay handling, and runtime config persistence.
- `plugins/haier/internal/app`: Haier auth, appliance discovery, capability derivation, command mapping, refresh, and refresh-token persistence.
- `internal/api/http`: gateway HTTP handlers, SSE streaming, request validation, and transport-layer concerns.
- `internal/core/runtime`: top-level Core composition and lifecycle wiring.
- `internal/core/pluginmgr`: plugin install/update/enable/disable, process supervision, gRPC connection management, discovery sync, event consumption, and health checks.
- `internal/core/registry`: unified device inventory owned by Core.
- `internal/core/state`: unified device state store owned by Core.
- `internal/core/control`: quick-control generation, toggle/action resolution, and control preference application.
- `internal/core/policy`: command authorization and risk evaluation.
- `internal/core/audit`: command audit recording.
- `internal/core/eventbus`: in-process event fanout inside Core.
- `internal/core/oauth`: Core-owned Xiaomi OAuth session lifecycle and callback completion.
- `internal/coreapi`: the approved plugin-to-Core backchannel, including persisted config updates.
- `internal/models`: shared canonical models and payload shapes. Do not leak vendor-specific structs past this layer.
- `internal/pluginapi`: plugin RPC contract helpers and protobuf/grpc bindings.
- `internal/pluginruntime`: shared plugin server scaffolding used by plugin binaries.
- `internal/pluginutil`: shared helper utilities for plugin code only.
- `internal/storage/sqlite`: production persistence implementation. New persistent Core data should land here unless a different production-grade store is explicitly required.
- `internal/xiaomi/oauth`: Xiaomi OAuth helper code shared across Core/plugin boundaries.
- `data`: local runtime databases and smoke-test artifacts, not source-of-truth code.
- `bin`: compiled artifacts, not handwritten source.

## Module Placement Rules

- Put Core-owned cross-plugin concerns under `internal/core`, not inside a vendor plugin.
- Put vendor HTTP clients, auth flows, and payload translation inside that vendor's plugin tree.
- Put shared transport or protocol helpers in `internal/pluginapi`, `internal/pluginruntime`, or `internal/coreapi` only when they are truly vendor-agnostic.
- Keep admin presentation logic in `web/admin/src/components`, data fetching/hooks in `web/admin/src/lib` or `web/admin/src/hooks`, and styling split by responsibility.
- Do not add new top-level directories for feature work when an existing module boundary already fits.

## File Size And Modularization Rule

- Any code file over 500 lines must be split before the task is considered complete.
- New code must be added in module-focused files instead of growing existing files past 500 lines.
- Splits must follow real responsibility boundaries such as config, auth, discovery, mapping, state, commands, transport, handlers, or persistence helpers. Do not create arbitrary fragments that only move the line-count problem around.

## Backend Implementation Rules

- Keep vendor-specific code inside its plugin tree.
- Prefer package boundaries that match the real integration flow: `auth`, `api/cloud`, `discovery`, `mapper`, `state`, `events`, `capability`.
- Map vendor models into `internal/models` without leaking vendor payloads into core behavior.
- Use polling only when it is a real vendor API strategy or a deliberate fallback for a real endpoint. Polling is not a substitute for missing logic.
- Enforce capability checks from real model data before command execution.
- Store and refresh tokens using the existing plugin config path until a dedicated secure secret mechanism is added. Do not replace this with local fixture files.
- Do not implement plugin-driven config persistence through generic event streams. If a plugin obtains refreshed tokens, session cookies, or derived runtime credentials, it must hand them to a Core-owned config update capability for validation and persistence.
- Authentication requirements for vendor requests must be encoded by explicit transport methods or endpoint-specific helpers. Do not route protected vendor calls through generic request functions that rely on ad hoc boolean flags to decide whether auth/session headers are attached.

## Frontend Implementation Rules

- Admin must expose the real plugin configuration fields required to authenticate and operate against vendor APIs.
- If a vendor integration depends on app-signature values or compatibility knobs that may drift over time, model them in the Core-owned plugin config with documented defaults rather than hardcoding them only in frontend or plugin internals.
- Do not preload fake accounts, fake devices, or fake command presets that imply the backend is already connected.
- UI examples may illustrate JSON structure, but they must not masquerade as runnable demo sessions.

## Git Workflow Rules

- Never create or switch branches unless the user explicitly asks for it.
- When the user asks to `commit`, `push`, or `commit push`, stay on the current branch by default.
- Do not infer branch creation from generic publish workflows or helper skills.

## Delivery Standard

When asked to implement a feature:

1. Treat the request as a production feature request by default.
2. Reuse the existing monorepo/plugin architecture unless the user explicitly asks to change it.
3. Complete the path end-to-end: config, runtime behavior, API surface, UI surface, and docs when needed.
4. Validate with builds/tests and, when credentials are available, real integration smoke checks.
5. If a requirement cannot be completed truthfully, say exactly what is missing instead of shipping a mock.

## API Documentation Rule

- Any change that adds, removes, renames, or changes the behavior of an HTTP API must update the repository Markdown API documentation in the same task.
- Do not leave API docs stale after changing request paths, request bodies, response fields, auth expectations, or control semantics.
