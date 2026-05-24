# PixelsMcp

PixelsMcp is a remote-first MCP service for image generation. The intended
production setup is to run this service on a server and expose one HTTP MCP URL
for users' agents to connect to. Users should not need to download this binary,
run local setup, or provide image provider credentials on their machines.

## Cloud / HTTP MCP Service

Run the service with provider credentials configured on the server:

```bash
PIXELSMCP_API_KEY=your-key \
PIXELSMCP_BASE_URL=https://api.example.com \
PIXELSMCP_MODEL=your-model \
go run ./cmd/pixelsmcp
```

The default transport is HTTP. The MCP endpoint is:

```text
http://127.0.0.1:8080/mcp
```

The health check endpoint is:

```text
http://127.0.0.1:8080/healthz
```

After deployment, give users the public MCP URL, for example:

```text
https://your-domain.example/mcp
```

### Codex Client Configuration

Users can connect Codex to the deployed MCP service by adding the remote URL to
`~/.codex/config.toml`:

```toml
[mcp_servers.pixelsmcp]
url = "https://your-domain.example/mcp"
```

Provider, API key, model, and image storage configuration stay on the server.
Users only need the deployed MCP URL.

## Developer Debugging

`init` and `setup` are for developers who clone the open source project and want
to debug or modify the service locally. They create a local `.env.local` file in
the repository root and validate the provider configuration.

```bash
git clone https://github.com/oxkrypton/PixelsMcp.git
cd PixelsMcp
go run ./cmd/pixelsmcp setup   # or: go run ./cmd/pixelsmcp init
go run ./cmd/pixelsmcp
```

For local MCP client debugging only, developers can still run the stdio
transport explicitly:

```bash
PIXELSMCP_TRANSPORT=stdio go run ./cmd/pixelsmcp
```

## Configuration

These variables are server-side configuration. End users connecting to the
deployed MCP URL do not need to set them.

| Variable | Default | Description |
| --- | --- | --- |
| `PIXELSMCP_TRANSPORT` | `http` | `http` for the cloud/server service. `stdio` is only for local developer MCP debugging. |
| `PIXELSMCP_ADDR` | `:8080` | HTTP listen address used when transport is `http`. |
| `PIXELSMCP_ENDPOINT` | `/mcp` | MCP HTTP endpoint path. |
| `PIXELSMCP_HEALTH_ENDPOINT` | `/healthz` | Health check endpoint path. |
| `PIXELSMCP_CORS_ORIGINS` | empty | Optional comma-separated list of allowed browser origins. |
| `PIXELSMCP_PROVIDER` | `openai-compatible` | Image provider adapter used by the server. |
| `PIXELSMCP_API_KEY` | required | Server-side API key used for image generation. |
| `PIXELSMCP_BASE_URL` | required | Server-side image generation API base URL. |
| `PIXELSMCP_MODEL` | `Kwai-Kolors/Kolors` | Default image model used by the server. |
| `PIXELSMCP_EXTRA_HEADERS` | empty | Optional JSON object or key/value list of extra provider headers. |
| `PIXELSMCP_TIMEOUT` | `2m0s` | HTTP timeout for provider requests. |
| `PIXELSMCP_IMAGE_SAVE_DIR` | `./generated-images` | Server-side directory where generated images are saved. |

`PIXELSMCP_IMAGE_MODEL` is still accepted as a legacy alias for
`PIXELSMCP_MODEL`.

## Build and Install on a VPS

```bash
go build -o pixelsmcp ./cmd/pixelsmcp
sudo useradd --system --home /opt/pixelsmcp --shell /usr/sbin/nologin pixelsmcp
sudo mkdir -p /opt/pixelsmcp
sudo cp pixelsmcp /opt/pixelsmcp/pixelsmcp
sudo cp deploy/pixelsmcp.service /etc/systemd/system/pixelsmcp.service
```

Before starting the service, configure the server-side provider credentials
through your secret manager, systemd drop-in, or service environment:

```text
PIXELSMCP_API_KEY=your-key
PIXELSMCP_BASE_URL=https://api.example.com
PIXELSMCP_MODEL=your-model
```

Then start and check the service:

```bash
sudo systemctl daemon-reload
sudo systemctl enable --now pixelsmcp
systemctl status pixelsmcp
curl http://127.0.0.1:8080/healthz
```

## Tools

- `generate_image`: generates an image from `prompt` and optional `background_color`, saves it on the server, and returns the file information.
- `generate_sprite_sheet`: generates a sprite sheet from `prompt`, `action`, `frame_count`, `layout`, optional `background_color`, and optional tuning args, saves it on the server, and returns the file information.

Use `background_color` for a solid key color like `#00FF00` or `#FF00FF`.
If you omit it on sprite sheets, the existing light-gray background stays in place.

Example image arguments:

```json
{
  "prompt": "portrait of a cyberpunk engineer",
  "background_color": "#FF00FF"
}
```

Example sprite sheet arguments:

```json
{
  "prompt": "pixel art knight with a blue cape",
  "action": "walk",
  "frame_count": 8,
  "layout": "horizontal",
  "background_color": "#00FF00",
  "image_size": "1024x1024",
  "guidance_scale": 7.5,
  "num_inference_steps": 28
}
```

Supported layout prompts include `horizontal`, `vertical`, and `3x3`. Other
layout text is passed through to the image model instead of being blocked by
the MCP server. Optional tuning fields are forwarded to compatible image
providers when supplied.
