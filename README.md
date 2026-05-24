# PixelsMcp

Simple MCP server built with `github.com/mark3labs/mcp-go` for local image generation.

## Local run

The default transport is `stdio`, which is suitable for local MCP clients that
start the server process themselves.

```bash
PIXELSMCP_PROVIDER=openai-compatible \
PIXELSMCP_API_KEY=your-key \
PIXELSMCP_BASE_URL=https://api.example.com \
PIXELSMCP_MODEL=your-model \
go run ./cmd/pixelsmcp
```

To create or update a local config file first, run:

```bash
go run ./cmd/pixelsmcp init
```

## VPS run

Use the HTTP transport for a long-running VPS process:

```bash
PIXELSMCP_TRANSPORT=http \
PIXELSMCP_ADDR=127.0.0.1:8080 \
PIXELSMCP_PROVIDER=openai-compatible \
PIXELSMCP_API_KEY=your-key \
PIXELSMCP_BASE_URL=https://api.example.com \
PIXELSMCP_MODEL=your-model \
go run ./cmd/pixelsmcp
```

The MCP endpoint is available at:

```text
http://127.0.0.1:8080/mcp
```

The health check endpoint is:

```text
http://127.0.0.1:8080/healthz
```

### Configuration

| Variable | Default | Description |
| --- | --- | --- |
| `PIXELSMCP_TRANSPORT` | `stdio` | `stdio` for local clients, `http` for VPS/server mode. |
| `PIXELSMCP_ADDR` | `:8080` | HTTP listen address used when transport is `http`. |
| `PIXELSMCP_ENDPOINT` | `/mcp` | MCP HTTP endpoint path. |
| `PIXELSMCP_HEALTH_ENDPOINT` | `/healthz` | Health check endpoint path. |
| `PIXELSMCP_CORS_ORIGINS` | empty | Optional comma-separated list of allowed browser origins. |
| `PIXELSMCP_PROVIDER` | `openai-compatible` | Image provider adapter to use. |
| `PIXELSMCP_API_KEY` | required | API key used for image generation. |
| `PIXELSMCP_BASE_URL` | required | Image generation API base URL. |
| `PIXELSMCP_MODEL` | `Kwai-Kolors/Kolors` | Default image model used by the server. |
| `PIXELSMCP_EXTRA_HEADERS` | empty | Optional JSON object or key/value list of extra HTTP headers. |
| `PIXELSMCP_TIMEOUT` | `2m0s` | HTTP timeout for provider requests. |
| `PIXELSMCP_IMAGE_SAVE_DIR` | `./generated-images` | Local directory where generated images are saved. |

`PIXELSMCP_IMAGE_MODEL` is still accepted as a legacy alias for
`PIXELSMCP_MODEL`.

### Build and install on a VPS

```bash
go build -o pixelsmcp ./cmd/pixelsmcp
sudo useradd --system --home /opt/pixelsmcp --shell /usr/sbin/nologin pixelsmcp
sudo mkdir -p /opt/pixelsmcp
sudo cp pixelsmcp /opt/pixelsmcp/pixelsmcp
sudo cp deploy/pixelsmcp.service /etc/systemd/system/pixelsmcp.service
sudo systemctl daemon-reload
sudo systemctl enable --now pixelsmcp
```

Check the service:

```bash
systemctl status pixelsmcp
curl http://127.0.0.1:8080/healthz
```

## Tool

- `generate_image`: generates an image from `prompt`, saves it locally, and returns the file information.
- `generate_sprite_sheet`: generates a sprite sheet from `prompt`, `action`, `frame_count`, and `layout`, saves it locally, and returns the file information.

Example sprite sheet arguments:

```json
{
  "prompt": "pixel art knight with a blue cape",
  "action": "walk",
  "frame_count": 8,
  "layout": "horizontal"
}
```

Supported layout prompts include `horizontal`, `vertical`, and `3x3`. Other
layout text is passed through to the image model instead of being blocked by
the MCP server.
