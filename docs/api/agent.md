# Agent Runtime API

Back to the [API index](../api.md).

Celestia embeds one Agent runtime under `/api/v1/agent`: an Eino ReAct loop with Agent-owned tools and memory. Project touchpoints, WeCom transport, voice message ingress, slash command dispatch, and device command ownership are outside this package and documented in [Project Touchpoints API](touchpoints.md).

Home Assistant, ChatGPT bridge, OpenAI quota management, and system maintenance behavior are intentionally not included.

## Snapshot

```http
GET /api/v1/agent
```

Returns the full Agent snapshot:

- `settings`: LLM providers, Agent providers, terminal, search, memory, md2img, evolution, knowledge, WeCom, and STT configuration. WeCom/STT settings are retained in the snapshot for migrated storage compatibility but are owned by Touchpoints at runtime.
- `settings.knowledge`: Codex-backed knowledge base settings. `bases[]` lists host directories that Codex can use as knowledge roots for `/kb` slash commands; `default_base_id` is used when a command does not specify `@base-id`; `agent_provider_id` must reference an Agent provider whose `type` is `codex`.
- `tools`: Agent-owned Eino tool contracts.
- `direct_input`: input mapping rules owned by Touchpoints before Agent execution.
- `wecom_menu` and `push`: migrated Touchpoint state retained for compatibility. Runtime WeCom menu read/write/delete uses the Touchpoints WeCom menu API and the remote WeCom app menu as source of truth.
- `conversations`: retained Agent conversation turns, including slash command result records.
- `memory`: raw turns, compacted summary memory, and active short conversation windows.
- `search`: recent search query logs, capped at the latest 50 runs.
- `workflow`: Core-owned generic workflow state surfaced in the Agent snapshot for API compatibility. It stores the workflow canvas (`active_workflow_id`, `workflows[]`) plus runtime history/state (`runs[]`, `sent_log`, `source_states[]`, `timer_states[]`) used for modular orchestration.
- `writing`, `market`, `evolution`, and `knowledge`: Agent-owned state. `knowledge` stores retained Codex knowledge sessions keyed by caller and knowledge base.

## Runtime Settings

```http
PUT /api/v1/agent/settings
```

Accepts `settings` from the snapshot and returns the updated snapshot.

LLM providers support `openai`, `openai-like`, `llama-server`, `gpt-plugin`, `ollama`, `gemini`, and `gemini-like` through HTTP-compatible transports.

Agent providers are separate provider profiles for Agent-owned executors. The current `codex` Agent provider invokes the local `codex exec --json --sandbox workspace-write` runner and supplies model, reasoning effort, and timeout defaults to modules such as Evolution and Knowledge.

Terminal execution is disabled unless `settings.terminal.enabled` is true. Memory defaults to enabled when no memory config exists; set `settings.memory.enabled=false` to disable prompt memory injection and compaction. md2img defaults to enabled when no md2img config exists and uses the bundled renderer at `internal/core/workflow/renderer/md2img/render.mjs`, writing to `data/agent/renderer/md2img` unless overridden. Knowledge-base Q&A is disabled unless `settings.knowledge.enabled=true` and the selected enabled knowledge base points at an accessible host directory.

## Conversation

```http
POST /api/v1/agent/conversation
```

Body:

```json
{
  "session_id": "default",
  "input": "summarize today's topic feed"
}
```

The HTTP conversation endpoint enters the project input layer first:

1. Slash commands are dispatched by `internal/core/project/slash`.
2. A matched slash command records a conversation row with `runtime_mode: "slash"` and does not run the Agent loop.
3. Non-slash input falls through to the Agent.
4. Agent direct-input mappings are resolved before the Eino ReAct loop.
5. Eino may call standard Agent tools and then records the final response plus process trace.

When `settings.memory.enabled=true`, non-command turns inject session memory before the model call:

- active `conversation_window`: recent real user/assistant messages within `settings.memory.window_timeout_seconds`
- `hybrid_memory`: summary hits ranked by hashed vector similarity plus lexical coverage
- `raw_replay`: raw records referenced by summary hits, limited by `raw_ref_limit` and `raw_record_limit`

After each turn Celestia appends a raw memory record, refreshes the active short window, and compacts unsummarized raw turns once `compact_every_rounds` is reached.

## Agent Tools

The Agent tool registry is built through Eino-compatible tool specs. Agent-owned tools include:

- `search_web`
- `workflow`
- `writing_organizer`
- `market_analysis`
- `evolution_operator`
- `terminal_run`
- `codex_runner`
- `markdown_render`
- `apple_notes`
- `apple_reminders`

WeCom send, HTTP ingress, slash command dispatch, voice message input, and native device execution are not Agent tools.

## Search Engine

```http
POST /api/v1/agent/search/run
```

Body:

```json
{
  "engine_selector": "default",
  "timeout_ms": 12000,
  "max_items": 8,
  "plans": [
    {
      "label": "fund-news",
      "query": "基金 公告 净值 风险",
      "recency": "month"
    }
  ]
}
```

Search engines are read from `settings.search_engines`. Supported providers:

- `serpapi`: calls `GET /search.json` with `engine`, `q`, `hl`, `gl`, `num`, and `api_key`
- `qianfan`: calls Baidu Qianfan `POST /v2/ai_search/web_search`

Provider execution lives in `internal/core/search`; the Agent wrapper records the latest 50 query logs into `snapshot.search.recent_queries`.

If no profile is configured, Celestia bootstraps from `SERPAPI_KEY` and `QIANFAN_SEARCH_*` environment variables.

## Tool Metadata API

```http
GET /api/v1/agent/tools
GET /api/v1/agent/tools/{name}
POST /api/v1/agent/tools/{name}/run
```

These endpoints expose Celestia-owned Agent tool metadata. A tool record contains `name`, `description`, optional terminal dependency metadata, direct commands, and the internal action contract.

`POST /run` accepts:

```json
{
  "input": "notes -s project",
  "command": "memo",
  "args": ["notes", "-s", "project"]
}
```

Terminal-backed tools such as Apple Notes and Apple Reminders execute through the same guarded terminal runner used by `/agent/terminal`; `settings.terminal.enabled` must be true.

## Workflow Canvas

```http
PUT /api/v1/agent/workflow
POST /api/v1/agent/workflow/run
```

`PUT /api/v1/agent/workflow` saves the workflow workspace snapshot:

- `active_workflow_id`
- `workflows[]`
  - `nodes[]`
  - `edges[]`

Workflow runtime fields such as `runs[]`, `sent_log[]`, `source_states[]`, and `timer_states[]` are returned in the Agent snapshot and are maintained by the execution layer. `PUT /api/v1/agent/workflow` preserves those runtime fields while updating the editable workflow definitions.

`POST /api/v1/agent/workflow/run` accepts:

```json
{
  "workflow_id": "daily-digest"
}
```

Legacy `profile_id` is still accepted as an alias during the migration window for older clients.

Workflow canvas supports these node types:

- `group`
- `timer`
- `device_state_changed`
- `device_state_is`
- `time_window`
- `rss_sources`
- `text`
- `llm`
- `search_provider`
- `wecom_output`
- `device_command`
- `agent_function`

Current executable ports:

- `timer.trigger -> rss_sources.trigger`
- `timer.trigger -> device_command.trigger`
- `timer.trigger -> agent_function.trigger`
- `device_state_changed.trigger -> device_command.trigger`
- `device_state_changed.trigger -> agent_function.trigger`
- `device_state_is.trigger -> device_command.trigger`
- `device_state_is.trigger -> agent_function.trigger`
- `time_window.gate -> <triggered node>.trigger`
- `time_window.gate -> <execution node>.trigger`
- `device_state_is.gate -> <execution node>.trigger`
- `rss_sources -> llm.context`
- `text -> text`
- `text -> llm.prompt`
- `search_provider -> llm.search`
- `llm.text -> wecom_output.text`

`llm.tool` and `llm.skill` are reserved canvas ports in this release. If they are connected, the run fails explicitly instead of fabricating behavior.

Runtime behavior:

1. `timer`, `device_state_changed`, and `device_state_is` are autonomous trigger nodes. Saving the workflow through `PUT /api/v1/agent/workflow` makes enabled trigger nodes eligible for backend scheduling or event dispatch immediately; no separate publish step is required.
2. `timer` supports `schedule: "daily"` with `at/timezone`, and `schedule: "interval"` with `interval_seconds/timezone`. Time windows are modeled with connected `time_window` nodes instead of fields on the timer node.
3. `device_state_changed` fires from the persisted `device.state.changed` event stream and can match a device id, state key, operator, value, and optional `from`/`to` values.
4. `device_state_is` can act as an autonomous trigger when connected from its `trigger` output, or as a gate when connected from its `gate` output. It evaluates the current Core-owned device state snapshot.
5. `time_window` is an accessory gate. When connected directly to a trigger or in parallel to the same downstream execution path, it constrains the trigger to its configured `start/end/timezone` window.
6. The scheduler creates one workflow run per due trigger node. If multiple triggers in the same workflow fire on the same tick or state event, they execute as separate queued runs instead of being merged into one fan-in run.
7. Manual `POST /api/v1/agent/workflow/run` does not activate autonomous trigger nodes. Only non-trigger paths, or paths already receiving manual input, execute in that manual run.
8. `rss_sources` may accept trigger input. When a trigger edge exists and no trigger fired for the current run, the RSS node emits no items.
9. `rss_sources` stores per-source request state in `workflow.source_states[]`, sends `If-Modified-Since` using the last stored `Last-Modified` value or previous request time, records the latest raw response body, and emits only items newly appearing relative to the previous body by RSS `guid` or Atom `id` (falling back to URL/text when a feed omits stable ids).
10. A `304 Not Modified` response advances the RSS source request timestamp and emits no items.
11. Without trigger-driven async fan-in, multiple upstream `rss_sources` connected to one `llm` are aggregated in a single run. When separate triggers feed that shared `llm`, each trigger run uses the same `llm` configuration but executes independently.
12. `text` concatenates upstream `text` inputs in edge order, then appends its own inline-authored text block.
13. `search_provider` uses the configured Core search provider profile.
14. `llm` uses the configured Agent LLM provider or the workflow-selected provider id.
15. `device_command` executes a real gateway device command through Core policy, audit, and the owning plugin command executor. It does not bypass command authorization.
16. `agent_function` sends a project input envelope through the existing project input layer, so slash commands run before the Agent ReAct loop and configured touchpoint output is reused.
17. `wecom_output` sends through the existing Touchpoints WeCom runtime.
18. `sent_log` is still appended only after a successful WeCom delivery and remains an execution history record.

## Writing Organizer

```http
POST /api/v1/agent/writing/topics
POST /api/v1/agent/writing/topics/{id}/materials
POST /api/v1/agent/writing/topics/{id}/summarize
```

Writing topics store raw materials and maintain `summary`, `outline`, and `draft` state with a backup of the previous state. Summarization uses the configured LLM when available and otherwise generates a deterministic material-based draft.

Celestia writes organizer artifacts under `data/agent/writing/topics/{topic_id}`:

- `raw/*.md`: appended source material with rollover
- `state/{summary,outline,draft}.md`: latest topic state
- `backup/*.prev.md`: previous state
- `knowledge/materials/YYYY/MM/*.json`: normalized material records
- `knowledge/insights/YYYY/MM/*.json`: extracted insight records
- `knowledge/documents/YYYY/MM/*.md` and `.meta.json`: composed documents

## Market Analysis

```http
PUT /api/v1/agent/market/portfolio
POST /api/v1/agent/market/portfolio/import-codes
POST /api/v1/agent/market/run
```

The Agent owns the Market portfolio state and report orchestration. Reusable Eastmoney estimate/security lookup code lives in `internal/core/workflow/market`.

A run calls Eastmoney fund estimate data for each holding and runs the configured search engine for recent fund news. The run is marked `eastmoney_search` and records per-asset source chain, search results, and errors.

`/market/portfolio/import-codes` accepts `{ "codes": "510300, 159915" }`, resolves names through Eastmoney suggest endpoints, preserves existing quantity/cost fields, and returns per-code `added`, `updated`, `exists`, `not_found`, or `error` status.

If `settings.md2img.enabled=true`, the generated markdown report is rendered through the Core renderer and returned in `images[]`. A renderer failure returns `MARKET_IMAGE_PIPELINE_FAILED` instead of silently falling back to text-only output.

## md2img

```http
POST /api/v1/agent/md2img/render
```

Body:

```json
{
  "markdown": "# Report\n\n- item",
  "mode": "long-image"
}
```

`mode` can be `long-image` or `multi-page`. The renderer reads `settings.md2img.command` when a custom command is configured and writes PNG files under `settings.md2img.output_dir` unless `output_dir` is supplied in the request. When `command` is empty, Celestia locates and runs the bundled `internal/core/workflow/renderer/md2img/render.mjs` script from the repository root; it requires the root npm dependencies `playwright`, `unified`, `remark-parse`, `remark-gfm`, `remark-rehype`, and `rehype-stringify`, plus an installed Playwright Chromium browser.

## Evolution And Terminal

```http
POST /api/v1/agent/evolution/goals
POST /api/v1/agent/evolution/goals/{id}/run
POST /api/v1/agent/evolution/ops/run
POST /api/v1/agent/approvals
POST /api/v1/agent/approvals/{id}/approve
POST /api/v1/agent/approvals/{id}/reject
POST /api/v1/agent/service/ops
POST /api/v1/agent/terminal
POST /api/v1/agent/codex/run
POST /api/v1/agent/screenshot
```

Evolution goals are queued in Agent state. Running a goal follows the Agent operator flow: generate a Codex JSON plan, execute each plan step through `codex exec`, run checks, optionally ask Codex for fixes, optionally run a structure review, and optionally commit/push/rebuild/restart when `settings.evolution.auto_commit`, `auto_push`, `auto_rebuild`, or `auto_restart` are enabled. `settings.evolution.codex_approval_policy` is passed to Codex as `--ask-for-approval`; supported values are `never`, `on-request`, and `untrusted`.

`/agent/evolution/ops/run` runs explicit local operations:

```json
{
  "action": "rebuild",
  "goal_id": "optional-goal-id",
  "commit_message": "optional commit message"
}
```

Supported actions are `commit`, `push`, `rebuild`, and `restart`. Rebuild defaults to `./deploy.sh`. Restart defaults to `./tool/celestia-service.sh restart`, which restarts the background gateway process without requiring an interactive terminal.

Approval requests are stored in Agent state:

```json
{
  "kind": "evolution_operation",
  "action": "restart",
  "goal_id": "optional-goal-id",
  "title": "Approve evolution restart"
}
```

Approving an `evolution_operation` executes the requested operation and stores the result on the approval record. Rejecting marks the request rejected and does not execute it.

`/agent/service/ops` wraps `tool/celestia-service.sh` for local gateway process management:

```json
{
  "action": "logs",
  "lines": 120
}
```

Supported actions are `status`, `start`, `stop`, `restart`, and `logs`. The script runs `bin/gateway` in the background on `0.0.0.0:8080` by default, with stdout/stderr appended to `data/runtime/gateway.log` and a PID file at `data/runtime/gateway.pid`. Set `CELESTIA_ADDR` when invoking the script to override the listen address.

Terminal commands require `settings.terminal.enabled=true` and execute through `/bin/sh -lc` with the configured timeout.

`/agent/codex/run` invokes local `codex exec` directly with workspace-write sandboxing by default and writes command output under `data/agent/codex` in the selected working directory. Request bodies may override `cwd`, `output_dir`, `sandbox`, `model`, `reasoning_effort`, `timeout_ms`, set `skip_git_repo_check=true`, or set `resume_session_id` to continue a prior Codex CLI session. Higher-level modules such as evolution and knowledge do not store raw Codex model names; they select an Agent provider with `type: "codex"`, and that provider supplies the Codex model and reasoning effort.

`/agent/screenshot` captures loopback web pages through Playwright:

```json
{
  "url": "http://localhost:3000",
  "width": 1440,
  "height": 1000,
  "full_page": true
}
```

Only `http` and `https` URLs targeting `localhost` or loopback IPs are accepted. PNG files are written under `data/agent/screenshots` by default.

## Codex Knowledge Base

Knowledge-base Q&A is configured through `PUT /api/v1/agent/settings`:

```json
{
  "knowledge": {
    "enabled": true,
    "default_base_id": "ops",
    "bases": [
      {
        "id": "ops",
        "name": "Operations",
        "base_dir": "/Users/me/Documents/Knowledge/Ops",
        "enabled": true
      },
      {
        "id": "design",
        "name": "Design",
        "base_dir": "/Users/me/Documents/Knowledge/Design",
        "enabled": true
      }
    ],
    "agent_provider_id": "codex-main",
    "timeout_ms": 600000
  }
}
```

Runtime entry is via ProjectInput slash commands, not a separate WeCom path:

```text
/kb ask <question>
/kb <question>
/kb @<base-id> ask <question>
/kb @<base-id> <question>
/kb new [@base-id] [question]
/kb list
/kb status [@base-id]
/kb answers list
/kb @<base-id> answers list
/kb answers get <id>
/kb @<base-id> answers get <id>
```

The runner starts `codex exec --sandbox workspace-write --skip-git-repo-check --cd <base_dir>` for the selected knowledge base and resumes the caller's active Codex session for the same `user + knowledge_base_id` when the CLI returns a session id. Codex is instructed to inspect files under `base_dir`, cite file paths and line numbers where possible, and write the final Markdown answer to `<base_dir>/.answers/*.md` without modifying other knowledge-base files.

After Codex creates the Markdown answer, Celestia renders it through the md2img renderer and sends the rendered image(s) from WeCom. `answers list` returns the latest 20 Markdown answer ids from the selected `<base_dir>/.answers`, and `answers get <id>` re-renders that Markdown answer to image(s). Render failures fail the command; Celestia does not fall back to returning long Markdown text. Configure knowledge base directories only to paths approved for model-assisted analysis, because file contents may be sent to the configured Agent provider.
