# Agent Runtime API

Back to the [API index](../api.md).

Celestia embeds one Agent runtime under `/api/v1/agent`: an Eino ReAct loop with Agent-owned tools and memory. Project touchpoints, WeCom transport, voice message ingress, slash command dispatch, and device command ownership are outside this package and documented in [Project Touchpoints API](touchpoints.md).

Home Assistant, ChatGPT bridge, OpenAI quota management, and system maintenance behavior are intentionally not included.

## Snapshot

```http
GET /api/v1/agent
```

Returns the full Agent snapshot:

- `settings`: LLM, terminal, search, memory, md2img, evolution, WeCom, and STT configuration. WeCom/STT settings are retained in the snapshot for migrated storage compatibility but are owned by Touchpoints at runtime.
- `capabilities`: Agent-owned tool contracts.
- `direct_input`: input mapping rules owned by Touchpoints before Agent execution.
- `wecom_menu` and `push`: Touchpoint-owned WeCom menu/users stored in the migrated snapshot document store.
- `conversations`: retained Agent conversation turns, including slash command result records.
- `memory`: raw turns, compacted summary memory, and active short conversation windows.
- `search`: recent search query logs, capped at the latest 50 runs.
- `topic_summary`, `writing`, `market`, and `evolution`: Agent-owned workflow state.

## Runtime Settings

```http
PUT /api/v1/agent/settings
```

Accepts `settings` from the snapshot and returns the updated snapshot.

LLM providers support `openai`, `openai-like`, `llama-server`, `gpt-plugin`, `ollama`, `gemini`, and `gemini-like` through HTTP-compatible transports. `codex` invokes the local `codex exec --json --sandbox workspace-write` runner.

Terminal execution is disabled unless `settings.terminal.enabled` is true. Memory defaults to enabled when no memory config exists; set `settings.memory.enabled=false` to disable prompt memory injection and compaction. md2img defaults to enabled when no md2img config exists and uses `node internal/core/renderer/md2img/render.mjs`, writing to `data/renderer/md2img` unless overridden.

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

1. Slash commands are dispatched by `internal/core/slash`.
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
- `topic_summary`
- `writing_organizer`
- `market_analysis`
- `evolution_operator`
- `terminal_run`
- `codex_runner`
- `markdown_renderer`
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

## Agent Capabilities

```http
GET /api/v1/agent/capabilities
GET /api/v1/agent/capabilities/{name}
POST /api/v1/agent/capabilities/{name}/run
```

Capabilities expose Celestia-owned Agent tool metadata. A capability record contains `name`, `description`, optional terminal dependency metadata, direct commands, and the internal action contract.

`POST /run` accepts:

```json
{
  "input": "notes -s project",
  "command": "memo",
  "args": ["notes", "-s", "project"]
}
```

Terminal-backed tools such as Apple Notes and Apple Reminders execute through the same guarded terminal runner used by `/agent/terminal`; `settings.terminal.enabled` must be true.

## Topic Summary

```http
PUT /api/v1/agent/topic
POST /api/v1/agent/topic/run
```

Topic profiles store RSS or Atom sources. A run fetches enabled feeds, deduplicates against the sent log, and optionally summarizes selected items through the configured LLM provider. If no LLM provider is configured, the run still records fetched feed items and a deterministic summary.

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

The Agent owns the Market workflow state and report generation. Reusable Eastmoney estimate/security lookup code lives in `internal/core/market`.

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

`mode` can be `long-image` or `multi-page`. The renderer reads `settings.md2img.command` and writes PNG files under `settings.md2img.output_dir` unless `output_dir` is supplied in the request. The default command is `node internal/core/renderer/md2img/render.mjs`; it requires the root npm dependencies `playwright`, `unified`, `remark-parse`, `remark-gfm`, `remark-rehype`, and `rehype-stringify`, plus an installed Playwright Chromium browser.

## Evolution And Terminal

```http
POST /api/v1/agent/evolution/goals
POST /api/v1/agent/evolution/goals/{id}/run
POST /api/v1/agent/terminal
POST /api/v1/agent/codex/run
```

Evolution goals are queued in Agent state. Running a goal follows the Agent operator flow: generate a Codex JSON plan, execute each plan step through `codex exec`, run checks, optionally ask Codex for fixes, optionally run a structure review, and optionally commit/push when `settings.evolution.auto_commit` or `settings.evolution.auto_push` are enabled.

Terminal commands require `settings.terminal.enabled=true` and execute through `/bin/sh -lc` with the configured timeout.

`/agent/codex/run` invokes local `codex exec` directly with workspace-write sandboxing and writes command output under `data/agent/codex` in the selected working directory.
