# Tools

This directory contains standalone scripts and bridge programs used by Celestia.

## Files

- `wecom-bridge.go`: production-ready WeCom callback bridge (recommended on VPS).

## Quick usage

## WeCom bridge (VPS)

Build Go bridge:

```bash
sudo apt-get update
sudo apt-get install -y golang
cd /path/to/Celestia
go build -o wecom-bridge ./tool
```

Create env file (e.g. `/etc/wecom-bridge.env`):

```env
WECOM_TOKEN=your_wecom_token
WECOM_AES_KEY=your_encoding_aes_key
WECOM_RECEIVE_ID=your_receive_id_optional
WECOM_BRIDGE_TOKEN=your_stream_token
BRIDGE_BUFFER_SIZE=200
PORT=8080
```

Run (foreground):

```bash
/path/to/Celestia/wecom-bridge
```

Systemd unit (silent):

```ini
[Unit]
Description=WeCom Bridge
After=network.target

[Service]
Type=simple
EnvironmentFile=/etc/wecom-bridge.env
WorkingDirectory=/path/to/Celestia
ExecStart=/path/to/Celestia/wecom-bridge
Restart=on-failure
RestartSec=2
StandardOutput=null
StandardError=null

[Install]
WantedBy=multi-user.target
```

Endpoints:

- `GET /health`
- `GET /wecom` (WeCom verification)
- `POST /wecom` (WeCom message callback)
- `GET /stream` (SSE stream for local agent)
- `POST /proxy/gettoken` (forward gettoken to WeCom)
- `POST /proxy/send` (forward send message to WeCom)
- `POST /proxy/menu/get` (forward app menu get to WeCom)
- `POST /proxy/menu/create` (forward app menu create to WeCom)
- `POST /proxy/menu/delete` (forward app menu delete to WeCom)
- `POST /proxy/media/upload` (forward media upload to WeCom, expects base64)
- `POST /proxy/media/get` (forward media get from WeCom, returns base64)

Security:

- WeCom signature is verified with `WECOM_TOKEN`.
- `/stream` requires `Authorization: Bearer <WECOM_BRIDGE_TOKEN>` if set.
