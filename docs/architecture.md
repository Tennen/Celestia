# Celestia Software Architecture

Celestia is a Go Core gateway plus a React admin console for real home-device orchestration, project-level touchpoints, workflow triggers, and a local Agent runtime.

## Top-Level Runtime

```text
Admin UI / HTTP / WeCom / workflow triggers / external callers
        |
        v
Gateway HTTP API
        |
        v
Core Runtime
  - SQLite Store
  - Event Bus
  - Registry / State
  - Control Catalog
  - Policy / Audit
  - Plugin Manager
  - Touchpoint Input
  - Slash Commands
  - Agent Runtime
  - Vision / OAuth / Capabilities
        |
        v
Vendor plugin processes over gRPC
```

Core owns the unified runtime model. Vendor plugins own vendor authentication, discovery, state translation, command execution, and event ingestion. Admin and external callers talk only to the gateway API.

## Device And Plugin Pipeline

```text
Plugin enable/discover
        |
        v
Plugin process starts
        |
        v
DiscoverDevices / GetDeviceState
        |
        v
Core registry + state store
        |
        v
Controls generated from device metadata
        |
        v
HTTP/API/admin command
        |
        v
Policy + audit
        |
        v
PluginMgr.ExecuteCommand -> owning vendor plugin
```

Supported production plugins are Xiaomi, Petkit, Haier hOn, and Hikvision/EZVIZ. Home Assistant is not a Celestia runtime dependency.

## Project Input Layer

Touchpoints are project-level input/output adapters. They are not Agent tools and they are not owned by the Agent runtime.

```text
HTTP conversation / WeCom text / WeCom click / WeCom voice / workflow agent_function
        |
        v
ProjectInput envelope
        |
        v
Slash command dispatcher
        |
        +-- matched: run project workflow directly
        |      - /home -> Celestia native device controls
        |      - /market -> market workflow
        |
        +-- not matched: Agent Eino ReAct conversation
```

Voice is currently a provider inside the WeCom voice-message chain. WeCom media is downloaded first, then the configured STT provider transcribes the audio, then the resulting text enters the same project input flow.

## Slash Commands

Slash commands are deterministic project workflows that run before Agent inference:

- `/home list [query]`
- `/home <device> <command> [value|key=value ...]`
- `/home <device-or-room.command> [value|key=value ...]`
- `/home <command> [value|key=value ...]`
- `/home action <device> <raw_action> [key=value ...]`
- `/market portfolio`
- `/market run [open|midday|close] [notes]`
- `/market import <fund codes>`
- `/workflow list`
- `/workflow run [workflow-id]`
- `/workflow runs`

Home commands use the native registry, state, control catalog, policy, audit, and plugin command executor. They do not require LLM intent detection.

## Agent Runtime

The Agent runtime is a single Eino ReAct loop. It handles non-slash input after project input processing.

Agent-owned domains:

- LLM providers
- Conversation and memory
- Search settings and recent search logs
- Topic summary
- Writing organizer
- Market portfolio/report orchestration
- Evolution operator
- Terminal, Codex runner, md2img, Apple Notes, Apple Reminders tools

Not Agent-owned:

- WeCom transport
- HTTP input transport
- Slash command dispatch
- Voice message ingress and STT provider chain
- Search provider HTTP execution
- Eastmoney market data lookup
- md2img renderer implementation
- Generic workflow execution, scheduling, triggers, RSS source state, and workflow run history
- Device command execution
- Home Assistant, ChatGPT bridge, OpenAI quota, and system maintenance paths

## Workflow Triggers

Core workflows own orchestration graph state. Trigger nodes such as `timer`, `device_state_changed`, and `device_state_is` can start a workflow run without a manual API call.

`time_window` is modeled as an accessory gate that can be connected beside a trigger or directly into a triggered execution path. It constrains when the trigger path can continue, while remaining separate from the trigger node itself.

Execution nodes use Core-owned runtime boundaries:

- `device_command` passes through policy, audit, and the plugin command executor.
- `agent_function` sends a normalized project input envelope through slash dispatch and the Agent runtime.
- `wecom_output` sends through the Touchpoints WeCom runtime.

Workflow output touchpoints can deliver Agent or slash results to:

- WeCom users
- voice-capable devices through native device commands

WeCom users are validated before save or send; arbitrary undeclared WeCom targets are rejected.

## HTTP Surfaces

- `/api/v1`: admin and project runtime APIs
- `/api/v1/touchpoints`: project-level input/touchpoint configuration and WeCom ingress
- `/api/v1/agent`: Agent configuration and the compatibility workflow API surface
- `/api/v1/devices`: admin device inventory and controls
- `/api/external/v1`: stable external device query/control
- `/api/ai/v1`: semantic device catalog and command execution

## Admin Console

The admin console uses a single app shell and shared workspace region.

Top-level workspaces:

- Overview
- Plugins
- Touchpoints
- Agent
- Capabilities
- Devices
- Activity

Agent sidebar subpages are limited to Agent-owned domains plus the workflow API surface. WeCom, slash commands, voice provider settings, and input mappings live under Touchpoints.

## Persistence

Core persistence is SQLite-backed. Agent and touchpoint configuration currently share the Agent snapshot document store for migrated data, while the active runtime ownership is split through Core services:

- `internal/core/project/input`: project input envelope and pre-Agent routing
- `internal/core/project/slash`: deterministic slash workflows
- `internal/core/project/touchpoint`: project touchpoint facade
- `internal/core/project/voice`: STT provider execution
- `internal/core/llm`: Core LLM provider execution
- `internal/core/search`: search provider execution
- `internal/core/workflow`: workflow definitions, execution, scheduling, triggers, RSS source state, sent logs, timer state, and run history
- `internal/core/workflow/market`: Eastmoney estimate/security lookup and Market prompt helpers
- `internal/core/workflow/renderer`: md2img renderer implementation and assets
- `internal/core/agent`: public Agent facade only
- `internal/core/agent/runtime`: Eino Agent loop, tool registry, Agent-owned orchestration, and persistence
- `internal/core/agent/runtime/memory`: Agent memory windowing, retrieval, and compaction

New runtime behavior should follow those ownership boundaries even when existing persisted document keys still contain historical `agent/*` names.
