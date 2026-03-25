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

## Repository Architecture Pattern

All work in this repository must preserve the current architecture:

1. Core gateway in Go under the root module.
2. Vendor integrations implemented as separate plugin processes.
3. Core-to-plugin communication over the existing gRPC plugin protocol.
4. Core owns the unified device model, policy, audit, registry, state store, and HTTP API.
5. Plugins own vendor auth, vendor discovery, vendor state translation, vendor command execution, and vendor event ingestion.
6. Persistence remains SQLite-backed in the core unless a requirement explicitly calls for another production-grade backing service.
7. Admin UI remains a Vite + React + shadcn/ui surface over the gateway API, not a side-channel integration path.

## Backend Implementation Rules

- Keep vendor-specific code inside its plugin tree.
- Prefer package boundaries that match the real integration flow: `auth`, `api/cloud`, `discovery`, `mapper`, `state`, `events`, `capability`.
- Map vendor models into `internal/models` without leaking vendor payloads into core behavior.
- Use polling only when it is a real vendor API strategy or a deliberate fallback for a real endpoint. Polling is not a substitute for missing logic.
- Enforce capability checks from real model data before command execution.
- Store and refresh tokens using the existing plugin config path until a dedicated secure secret mechanism is added. Do not replace this with local fixture files.

## Frontend Implementation Rules

- Admin must expose the real plugin configuration fields required to authenticate and operate against vendor APIs.
- Do not preload fake accounts, fake devices, or fake command presets that imply the backend is already connected.
- UI examples may illustrate JSON structure, but they must not masquerade as runnable demo sessions.

## Delivery Standard

When asked to implement a feature:

1. Treat the request as a production feature request by default.
2. Reuse the existing monorepo/plugin architecture unless the user explicitly asks to change it.
3. Complete the path end-to-end: config, runtime behavior, API surface, UI surface, and docs when needed.
4. Validate with builds/tests and, when credentials are available, real integration smoke checks.
5. If a requirement cannot be completed truthfully, say exactly what is missing instead of shipping a mock.
