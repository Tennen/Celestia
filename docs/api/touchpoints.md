# Project Touchpoints API

Back to the [API index](../api.md).

Touchpoints are project-level input/output adapters. They are not Agent tools. HTTP, WeCom, automation time triggers, and future external inputs normalize into a `ProjectInput` envelope before slash command dispatch and optional Agent execution.

## Input Flow

```text
HTTP / WeCom / Automation
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
PUT /api/v1/touchpoints/wecom/menu
POST /api/v1/touchpoints/wecom/menu/publish
```

`PUT` stores and validates a menu config. Nested WeCom menu groups are preserved through `sub_buttons`; Celestia supports WeCom's 3 top-level buttons and 5 sub-buttons per group.

`POST /publish` publishes the generated payload using `settings.wecom`. If `settings.wecom.bridge_url` is set, media/send operations use bridge-compatible routes; menu publishing uses the configured WeCom API base URL.

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
- click events resolve menu `dispatch_text` and enter ProjectInput
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
