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

LLM providers support `openai`, `openai-like`, `llama-server`, `gpt-plugin`, `ollama`, `gemini`, and `gemini-like` through HTTP-compatible transports. `codex` requires a configured external evolution runner and is not invoked through the chat transport.

Terminal execution is disabled unless `settings.terminal.enabled` is true.

## WeCom

```http
PUT /api/v1/agent/wecom/menu
POST /api/v1/agent/wecom/menu/publish
POST /api/v1/agent/wecom/send
POST /api/v1/agent/wecom/callback
```

`/wecom/menu` stores and validates a menu config. `/wecom/menu/publish` publishes the generated payload to the real WeCom menu API using `settings.wecom`. `/wecom/send` sends a text message to a real WeCom user.

`/wecom/callback` records unencrypted WeCom XML click callbacks and matches `EventKey` against the stored menu. Encrypted callback verification is not implemented in this endpoint; deployments that require encrypted callbacks must terminate and decrypt before forwarding XML here.

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

## Market Analysis

```http
PUT /api/v1/agent/market/portfolio
POST /api/v1/agent/market/run
```

The market module stores fund holdings and cash. Runs are explicitly marked `portfolio_only_no_market_feed` unless user notes or future configured data providers supply market data.

## Evolution And Terminal

```http
POST /api/v1/agent/evolution/goals
POST /api/v1/agent/evolution/goals/{id}/run
POST /api/v1/agent/terminal
```

Evolution goals are queued in Celestia state. Running a goal requires `settings.evolution.command`; the goal text is passed to the command through stdin.

Terminal commands require `settings.terminal.enabled=true` and execute through `/bin/sh -lc` with the configured timeout.
