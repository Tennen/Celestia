# Project Touchpoints API

Back to the [API index](../api.md).

Touchpoints are project-level input/output adapters. They are not Agent tools. HTTP, WeCom, workflow agent-function nodes, and future external inputs normalize into a `ProjectInput` envelope before slash command dispatch and optional Agent execution.

## Input Flow

```text
HTTP / WeCom / workflow agent_function
        |
        v
ProjectInput
        |
        +-- slash matched: run Core workflow directly
        |
        +-- no slash: Agent Eino ReAct conversation
```

Slash commands are project workflows in `internal/core/project/slash`. WeCom transport lives in `internal/core/project/touchpoint`. Voice transcription lives in `internal/core/project/voice` and is currently used by the WeCom voice-message path.

Home slash commands support the same Core-owned home shortcut resolution used by `/api/ai/v1`: device aliases, quick-control aliases, room-qualified targets (`device-or-room.command`), and globally unique command aliases all resolve through the shared Home service before policy/audit and plugin dispatch.

Workflow slash commands run before the Agent ReAct loop and invoke the Core-owned workflow runtime:

- `/workflow list` reports configured workflows and the active workflow.
- `/workflow run [workflow-id]` starts the selected workflow, or the active workflow when no id is supplied.
- `/workflow runs` lists recent workflow runs.

Evolution and local service slash commands run before the Agent ReAct loop:

- `/evolution status [goal-id]` lists queued self-evolution goals or shows one goal.
- `/evolution queue <goal> [commit_message=...]` queues a Codex-backed evolution goal.
- `/evolution run [goal-id]` runs the selected goal, or the next non-succeeded goal when no id is supplied.
- `/evolution request <commit|push|rebuild|restart> [goal-id]` creates a remote approval request.
- `/evolution <commit|push|rebuild|restart> [goal_id=...] [commit_message=...]` runs an explicit operation.
- `/approve <approval-id>` approves and executes a pending operation request.
- `/reject <approval-id>` rejects a pending operation request.
- `/service status` reports the background gateway service status.
- `/service logs [lines=120]` returns recent lines from `data/runtime/gateway.log`.
- `/service start`, `/service stop`, and `/service restart` call `tool/celestia-service.sh`.
- `/screenshot <http://localhost:port/path> [width=1440 height=1000 full_page=true]` captures a loopback web page and returns the PNG image.

Knowledge slash commands run before the Agent ReAct loop and invoke the local Codex CLI against the selected configured knowledge base root:

- `/kb ask <question>` or `/kb <question>` asks against the default knowledge base for the current WeCom/HTTP user.
- `/kb @<base-id> ask <question>` or `/kb @<base-id> <question>` asks against a specific knowledge base.
- `/kb new [@base-id] [question]` starts a fresh Codex session for the selected base, useful when prior context is too large or should be discarded.
- `/kb list` reports configured bases.
- `/kb status [@base-id]` reports the configured bases and the active session for the caller on the selected base.
- `/kb answers list` or `/kb @<base-id> answers list` lists the latest 20 saved Markdown answers under `.answers`.
- `/kb answers get <id>` or `/kb @<base-id> answers get <id>` re-renders a saved Markdown answer to image messages.

Knowledge answers are saved as Markdown under the selected `<base_dir>/.answers`, rendered through the Agent md2img pipeline, and sent to WeCom as image messages. Image rendering or delivery errors fail the command instead of falling back to long text replies.

## Input Mappings

```http
PUT /api/v1/touchpoints/input-mappings
```

Stores direct input mapping rules and returns the updated Agent snapshot. Mappings are evaluated for non-slash input before the Agent ReAct loop.

Legacy `/api/v1/agent/direct-input` is not registered.

## WeCom Users

```http
PUT /api/v1/touchpoints/wecom/users
```

Body is the `push` object from the Agent snapshot:

```json
{
  "users": [
    {
      "id": "user-alice",
      "name": "Alice",
      "wecom_user": "alice",
      "enabled": true
    }
  ]
}
```

WeCom users are validated at save time:

- `wecom_user` is required.
- `wecom_user` must be unique.
- Send operations accept only configured and enabled users, by `id` or `wecom_user`.

Legacy `/api/v1/agent/push` is not registered.

## WeCom Menu

```http
GET /api/v1/touchpoints/wecom/menu
PUT /api/v1/touchpoints/wecom/menu
DELETE /api/v1/touchpoints/wecom/menu
```

`GET` reads the current app menu from WeCom. `PUT` validates and creates/overwrites the WeCom app menu. `DELETE` calls WeCom's menu delete endpoint and is the supported reset path; sending an empty `button` list to create is rejected before reaching WeCom. Celestia supports WeCom's 3 top-level buttons and 5 sub-buttons per group. A publishable menu must include at least one enabled button, and every enabled group must contain at least one enabled sub-button.

Menu get/create/delete use `settings.wecom`. If `settings.wecom.bridge_url` is set, these operations use bridge-compatible proxy routes. Without a bridge URL, they use the configured WeCom API base URL.

## WeCom Send

```http
POST /api/v1/touchpoints/wecom/send
POST /api/v1/touchpoints/wecom/image
```

Text body:

```json
{
  "to_user": "user-alice",
  "text": "hello"
}
```

Image body:

```json
{
  "to_user": "user-alice",
  "base64": "<base64-image>",
  "filename": "report.png",
  "content_type": "image/png"
}
```

`to_user` must resolve to a configured enabled WeCom user. Text is split by UTF-8 bytes using `settings.wecom.text_max_bytes` (default `1800`).

## WeCom Ingress

```http
POST /api/v1/touchpoints/wecom/callback
POST /api/v1/touchpoints/wecom/ingress
```

`/callback` records unencrypted XML callbacks and returns JSON.

`/ingress` is the synchronous WeCom entrypoint:

- text messages enter ProjectInput
- click events use WeCom `EventKey` as the ProjectInput text
- voice messages download media, run the configured voice provider when enabled, then enter ProjectInput

The HTTP response is WeCom XML text. Send `Accept: application/json` to inspect the structured result instead.

If `settings.wecom.bridge_stream_enabled=true` and `settings.wecom.bridge_url` is configured, `internal/core/project/touchpoint` starts a background SSE client against `{bridge_url}/stream`. Incoming bridge text, voice, image, and click events enter the same ProjectInput path and replies are sent with the bridge-compatible sender.

Downloaded voice media is stored under `settings.wecom.audio_dir` (default `data/touchpoints/wecom-audio`).

Encrypted callback verification is not implemented; deployments that require encrypted callbacks must terminate and decrypt before forwarding XML here.

Legacy `/api/v1/agent/wecom/*` routes are not registered.

## Voice Provider

```http
POST /api/v1/touchpoints/voice/transcribe
```

Body:

```json
{
  "audio_path": "/path/to/audio.wav"
}
```

This endpoint is primarily for diagnostics. Runtime voice input currently enters through WeCom voice messages.

STT requires `settings.stt.enabled=true`. The supported provider is `fast-whisper`; Celestia runs `settings.stt.command` when provided, otherwise it runs `python3 tools/fast-whisper-transcribe.py --audio <audio_path>`.

Legacy `/api/v1/agent/stt/transcribe` is not registered.
