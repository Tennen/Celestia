# Agent Runtime API

Celestia embeds the Paimon-derived agent runtime directly in Core under `/api/v1/agent`.
Home Assistant and ChatGPT bridge behavior are intentionally not included.

## Snapshot

```http
GET /api/v1/agent
```

Returns the full agent snapshot:

- `settings`: LLM, STT, WeCom, terminal, search, and evolution runner configuration
- `direct_input`: fixed text mapping rules
- `wecom_menu`: WeCom menu config, publish payload, validation errors, and recent click events
- `push`: WeCom push users and interval tasks
- `conversations`: retained agent conversation turns
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

Terminal execution is disabled unless `settings.terminal.enabled` is true.

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

`/wecom/image` accepts `{ "to_user", "base64", "filename", "content_type" }`, uploads the image as WeCom media, then sends an image message. If `settings.wecom.bridge_url` is set, Celestia uses the Paimon bridge-compatible routes `/proxy/gettoken`, `/proxy/media/upload`, and `/proxy/send`; otherwise it calls the WeCom API directly.

`/wecom/callback` records unencrypted WeCom XML callbacks and returns JSON. `/wecom/ingress` is the Paimon-style synchronous WeCom entrypoint: text and click events enter the agent conversation, voice messages use WeCom `Recognition` text when present, and the HTTP response is a WeCom XML text reply. Send `Accept: application/json` to `/wecom/ingress` to inspect the structured result instead.

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

The runtime applies direct-input mapping first, then calls the configured LLM provider. Device command execution remains owned by `/api/ai/v1/commands`.

Paimon-style direct commands are handled before the LLM:

- `/search <query>`
- `/topic [profile_id]`
- `/writing create <title>`, `/writing append <topic_id> <content>`, `/writing summarize <topic_id>`
- `/market [phase]`
- `/evolution queue <goal>`, `/evolution run <goal_id>`
- `/terminal <command>`
- `/codex <prompt>`
- `/sync`, `/build`, `/deploy`
- `/celestia` returns the gateway-owned AI command path; device execution remains in `/api/ai/v1/commands`

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

Like Paimon, Celestia also writes organizer artifacts to disk under `data/agent/writing/topics/{topic_id}`:

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
POST /api/v1/agent/market/run
```

The market module stores fund holdings and cash. A run calls Eastmoney fund estimate data for each holding and runs the configured search engine for recent fund news. The run is marked `eastmoney_search` and records per-asset source chain, search results, and errors.

## Evolution And Terminal

```http
POST /api/v1/agent/evolution/goals
POST /api/v1/agent/evolution/goals/{id}/run
POST /api/v1/agent/terminal
POST /api/v1/agent/codex/run
```

Evolution goals are queued in Celestia state. Running a goal now follows the Paimon operator flow: generate a Codex JSON plan, execute each plan step through `codex exec`, run checks, optionally ask Codex for fixes, optionally run a structure review, and optionally commit/push when `settings.evolution.auto_commit` or `settings.evolution.auto_push` are enabled.

Relevant `settings.evolution` fields:

- `cwd`, `timeout_ms`, `codex_model`, `codex_reasoning`
- `test_commands`: ordered shell checks; if empty Celestia auto-detects Go and web build checks
- `max_fix_attempts`: default `2`
- `structure_review`: runs an extra Codex structure review
- `auto_commit`, `auto_push`, `push_remote`, `push_branch`
- `command`: legacy fallback command used only when Codex planning cannot run

Terminal commands require `settings.terminal.enabled=true` and execute through `/bin/sh -lc` with the configured timeout.

`/agent/codex/run` invokes local `codex exec` directly with workspace-write sandboxing and writes command output under `data/agent/codex` in the selected working directory.
