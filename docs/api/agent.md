# Agent Runtime API

Celestia embeds the migrated agent runtime directly in Core under `/api/v1/agent`.
Home Assistant, ChatGPT bridge, OpenAI quota management, and system maintenance behavior are intentionally not included.

## Snapshot

```http
GET /api/v1/agent
```

Returns the full agent snapshot:

- `settings`: LLM, STT, WeCom, terminal, search, memory, md2img, and evolution runner configuration
- `capabilities`: built-in Celestia Agent capability contracts and direct command metadata
- `direct_input`: fixed text mapping rules
- `wecom_menu`: WeCom menu config, publish payload, validation errors, and recent click events
- `push`: WeCom push users and interval tasks
- `conversations`: retained agent conversation turns
- `memory`: raw turns, compacted summary memory, and active short conversation windows
- `topic_summary`: RSS topic profiles and run history
- `writing`: writing organizer topics, materials, summary, outline, and draft state
- `market`: fund portfolio and portfolio-only analysis run history
- `evolution`: queued evolution goals and runner events

## Runtime Config

```http
PUT /api/v1/agent/settings
PUT /api/v1/agent/direct-input
PUT /api/v1/agent/push
```

Each endpoint accepts the corresponding object from the snapshot and returns the updated snapshot.

LLM providers support `openai`, `openai-like`, `llama-server`, `gpt-plugin`, `ollama`, `gemini`, and `gemini-like` through HTTP-compatible transports. `codex` invokes the local `codex exec --json --sandbox workspace-write` runner.

Terminal execution is disabled unless `settings.terminal.enabled` is true. Memory defaults to enabled when no memory config exists; set `settings.memory.enabled=false` to disable prompt memory injection and compaction. md2img defaults to enabled when no md2img config exists and uses `node internal/core/agent/md2img/render.mjs`.

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

If no profile is configured, Celestia bootstraps from `SERPAPI_KEY` and `QIANFAN_SEARCH_*` environment variables.

## WeCom

```http
PUT /api/v1/agent/wecom/menu
POST /api/v1/agent/wecom/menu/publish
POST /api/v1/agent/wecom/send
POST /api/v1/agent/wecom/image
POST /api/v1/agent/wecom/callback
POST /api/v1/agent/wecom/ingress
```

`/wecom/menu` stores and validates a menu config. `/wecom/menu/publish` publishes the generated payload to the real WeCom menu API using `settings.wecom`. `/wecom/send` sends text to a real WeCom user and splits long content by UTF-8 bytes using `settings.wecom.text_max_bytes` (default `1800`).

`/wecom/image` accepts `{ "to_user", "base64", "filename", "content_type" }`, uploads the image as WeCom media, then sends an image message. If `settings.wecom.bridge_url` is set, Celestia uses bridge-compatible routes `/proxy/gettoken`, `/proxy/media/upload`, and `/proxy/send`; otherwise it calls the WeCom API directly.

`/wecom/callback` records unencrypted WeCom XML callbacks and returns JSON. `/wecom/ingress` is the synchronous WeCom entrypoint: text and click events enter the agent conversation, voice messages download media and run STT when `settings.stt.enabled=true`, then fall back to WeCom `Recognition` text when present. The HTTP response is a WeCom XML text reply. Send `Accept: application/json` to `/wecom/ingress` to inspect the structured result instead.

If `settings.wecom.bridge_stream_enabled=true` and `settings.wecom.bridge_url` is configured, the agent starts a background SSE client against `{bridge_url}/stream`. Incoming bridge text, voice, image, and click events enter the same conversation path and replies are sent with the bridge-compatible sender. Voice media is fetched through `/proxy/media/get` first and then falls back to direct WeCom media download. Downloaded audio is stored under `settings.wecom.audio_dir` (default `data/agent/wecom-audio`).

Encrypted callback verification is not implemented in these endpoints; deployments that require encrypted callbacks must terminate and decrypt before forwarding XML here.

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

The runtime applies direct-input mapping first, then handles slash commands. For non-command text, Celestia runs a local capability routing step over built-in tool contracts before falling back to a direct LLM response. The router can call local search, topic summary, writing organizer, market analysis, evolution, terminal, Codex, md2img, Apple Notes/Reminders, or the Celestia device handoff response. Device command execution remains owned by `/api/ai/v1/commands`.

When `settings.memory.enabled=true`, non-command conversation turns inject session memory before the LLM call:

- active `conversation_window`: recent real user/assistant messages within `settings.memory.window_timeout_seconds`
- `hybrid_memory`: summary hits ranked by hashed vector similarity plus lexical coverage
- `raw_replay`: raw records referenced by summary hits, limited by `raw_ref_limit` and `raw_record_limit`

After each turn Celestia appends a raw memory record, refreshes the active short window, and compacts unsummarized raw turns into summary memory once `compact_every_rounds` is reached. The deterministic fallback compactor preserves raw text references through `raw_refs`.

Direct commands are handled before the LLM:

- `/search <query>`
- `/agent-capability list`, `/agent-capability describe <name>`, `/agent-capability run <name> <input>`
- `/apple-notes <memo notes args>` and `/apple-reminders <remindctl args>`; both require terminal execution to be enabled and the corresponding macOS CLI to be installed
- `/topic`, `/topic run [--profile id]`, `/topic profile list/add/use/update/delete`, `/topic source list/add/update/enable/disable/delete`, `/topic config`, `/topic state`
- `/writing topics`, `/writing show <topic_id>`, `/writing create <title>`, `/writing append <topic_id> <content>`, `/writing summarize <topic_id>`, `/writing restore <topic_id>`, `/writing set <topic_id> <summary|outline|draft> <content>`
- `/market midday`, `/market close`, `/market status`, `/market portfolio`, `/market add <code> <quantity> <avg_cost> [name]`
- `/evolve <goal>`, `/coding <goal>`, `/evolve status [goal_id]`, `/evolve tick`, `/evolution queue <goal>`, `/evolution run <goal_id>`
- `/terminal <command>`
- `/codex <prompt>`, `/codex status`, `/codex model [model]`, `/codex effort [effort]`
- `/md2img <markdown>`
- `/sync`, `/build`, `/restart`, `/deploy`
- `/celestia` returns the gateway-owned AI command path; device execution remains in `/api/ai/v1/commands`

## Agent Capabilities

```http
GET /api/v1/agent/capabilities
GET /api/v1/agent/capabilities/{name}
POST /api/v1/agent/capabilities/{name}/run
```

Capabilities expose the migrated local Agent contracts as Celestia-owned capability metadata. A capability record contains `name`, `description`, optional terminal dependency metadata, direct commands, and the internal action contract.

`POST /run` accepts:

```json
{
  "input": "notes -s project",
  "command": "memo",
  "args": ["notes", "-s", "project"]
}
```

For terminal-backed capabilities such as `apple-notes` and `apple-reminders`, Celestia executes the configured CLI through the same terminal runner used by `/agent/terminal`; `settings.terminal.enabled` must be true. Missing CLI binaries, macOS permission errors, and non-zero command exits are returned as explicit terminal errors.

## STT

```http
POST /api/v1/agent/stt/transcribe
```

Body:

```json
{
  "audio_path": "/path/to/audio.wav"
}
```

STT requires `settings.stt.enabled=true`. The supported provider is `fast-whisper`; Celestia runs `settings.stt.command` when provided, otherwise it runs `python3 tools/fast-whisper-transcribe.py --audio <audio_path>`.

## Topic Summary

```http
PUT /api/v1/agent/topic
POST /api/v1/agent/topic/run
```

Topic profiles store RSS sources. A run fetches enabled feeds, parses RSS or Atom entries, and optionally summarizes the selected items through the configured LLM provider. If no LLM provider is configured, the run still records the fetched feed items and a deterministic summary.

## Writing Organizer

```http
POST /api/v1/agent/writing/topics
POST /api/v1/agent/writing/topics/{id}/materials
POST /api/v1/agent/writing/topics/{id}/summarize
```

Writing topics store raw materials and maintain `summary`, `outline`, and `draft` state with a backup of the previous state. Summarization uses the configured LLM when available and otherwise generates a deterministic material-based draft.

Celestia also writes organizer artifacts to disk under `data/agent/writing/topics/{topic_id}`:

- `raw/*.md`: appended source material with rollover
- `state/{summary,outline,draft}.md`: latest topic state
- `backup/*.prev.md`: previous state
- `knowledge/materials/YYYY/MM/*.json`: normalized material records
- `knowledge/insights/YYYY/MM/*.json`: extracted insight records
- `knowledge/documents/YYYY/MM/*.md` and `.meta.json`: composed documents

The topic snapshot includes `artifact_root`, `raw_files`, `artifacts`, and `last_summarized_at` so the admin UI and API callers can inspect the file-backed pipeline.

## Market Analysis

```http
PUT /api/v1/agent/market/portfolio
POST /api/v1/agent/market/portfolio/import-codes
POST /api/v1/agent/market/run
```

The market module stores fund holdings and cash. A run calls Eastmoney fund estimate data for each holding and runs the configured search engine for recent fund news. The run is marked `eastmoney_search` and records per-asset source chain, search results, and errors.

`/market/portfolio/import-codes` accepts `{ "codes": "510300, 159915" }`, resolves names through Eastmoney suggest endpoints, preserves existing quantity/cost fields, and returns per-code `added`, `updated`, `exists`, `not_found`, or `error` status.

If `settings.md2img.enabled=true`, the generated markdown report is rendered through the md2img pipeline and returned in `images[]`. A missing Playwright browser, missing npm dependency, empty markdown, or screenshot failure returns `MARKET_IMAGE_PIPELINE_FAILED` rather than silently falling back to text-only output.

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

`mode` can be `long-image` or `multi-page`. The renderer reads `settings.md2img.command` and writes PNG files under `settings.md2img.output_dir` unless `output_dir` is supplied in the request. The default command is `node internal/core/agent/md2img/render.mjs`; it requires the root npm dependencies `playwright`, `unified`, `remark-parse`, `remark-gfm`, `remark-rehype`, and `rehype-stringify`, plus an installed Playwright Chromium browser.

## Evolution And Terminal

```http
POST /api/v1/agent/evolution/goals
POST /api/v1/agent/evolution/goals/{id}/run
POST /api/v1/agent/terminal
POST /api/v1/agent/codex/run
```

Evolution goals are queued in Celestia state. Running a goal follows the Agent operator flow: generate a Codex JSON plan, execute each plan step through `codex exec`, run checks, optionally ask Codex for fixes, optionally run a structure review, and optionally commit/push when `settings.evolution.auto_commit` or `settings.evolution.auto_push` are enabled.

Relevant `settings.evolution` fields:

- `cwd`, `timeout_ms`, `codex_model`, `codex_reasoning`
- `test_commands`: ordered shell checks; if empty Celestia auto-detects Go and web build checks
- `max_fix_attempts`: default `2`
- `structure_review`: runs an extra Codex structure review
- `auto_commit`, `auto_push`, `push_remote`, `push_branch`
- `command`: legacy fallback command used only when Codex planning cannot run

Terminal commands require `settings.terminal.enabled=true` and execute through `/bin/sh -lc` with the configured timeout.

`/agent/codex/run` invokes local `codex exec` directly with workspace-write sandboxing and writes command output under `data/agent/codex` in the selected working directory.
