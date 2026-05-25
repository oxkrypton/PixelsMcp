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
PIXELSMCP_REFERENCE_MODEL=your-model \
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
| `PIXELSMCP_REFERENCE_MODEL` | `Qwen/Qwen-Image-Edit-2509` | Image model used when a request includes `reference_image`. |
| `PIXELSMCP_EXTRA_HEADERS` | empty | Optional JSON object or key/value list of extra provider headers. |
| `PIXELSMCP_TIMEOUT` | `2m0s` | HTTP timeout for provider requests. |
| `PIXELSMCP_IMAGE_SAVE_DIR` | `./generated-images` | Server-side directory where generated images are saved when `output_path` is omitted. |

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
PIXELSMCP_REFERENCE_MODEL=Qwen/Qwen-Image-Edit-2509
```

Then start and check the service:

```bash
sudo systemctl daemon-reload
sudo systemctl enable --now pixelsmcp
systemctl status pixelsmcp
curl http://127.0.0.1:8080/healthz
```

## Tools

- `generate_image`: generates an image from `prompt`, optional `reference_image`, optional `background_color`, optional tuning args, optional `seed`, optional `negative_prompt`, and optional `output_path`, saves it on the server, and returns the file information.
- `generate_sprite_sheet`: generates a sprite sheet from `prompt`, `action`, `frame_count`, `layout`, optional `reference_image`, optional `frame_width`, `frame_height`, `spacing`, optional `background_color`, optional tuning args, optional `seed`, optional `negative_prompt`, and optional `output_path`, saves it on the server, and returns the file information.

Use `background_color` for a solid key color like `#00FF00` or `#FF00FF`.
Sprite sheets default to 64x64 frames with 2px spacing and a light-gray background if you omit those geometry fields and `background_color`.
Use `output_path` when the caller wants the image written to a specific local
workspace path. It must be an absolute path. If it points to an existing
directory, PixelsMcp writes an automatically named image into that directory.
If it points to a file, PixelsMcp writes that file and adds the image extension
when the file path has none. The response `saved_path` is the actual absolute
path written by the server; `local_path` is kept as a compatibility alias.
Use `reference_image` only when you want the server to route the request through
`PIXELSMCP_REFERENCE_MODEL`; it may be an http(s) URL, a `data:image/...`
base64 URL, or raw base64 image data without the `data:` prefix. If your
client auto-renders `data:image/...` strings as images, send the raw base64
payload instead; the server will wrap it back into a data URL before calling
the provider. Do not set the default `PIXELSMCP_MODEL` to a reference-only
edit model if you still want plain text-to-image requests to work. For
SiliconFlow's Qwen edit models, `image_size` and `guidance_scale` are omitted
automatically because those fields are not supported by those models.

Example image arguments:

```json
{
  "prompt": "portrait of a cyberpunk engineer",
  "background_color": "#FF00FF",
  "reference_image": "https://example.com/reference.png",
  "image_size": "1024x1024",
  "guidance_scale": 7.5,
  "num_inference_steps": 28,
  "seed": 12345,
  "negative_prompt": "blurry, low quality",
  "output_path": "/Users/krypton/Documents/go-tiny-claw/generated-images/cyberpunk-engineer.png"
}
```

Example sprite sheet arguments:

```json
{
  "prompt": "pixel art knight with a blue cape",
  "action": "walk",
  "frame_count": 8,
  "layout": "horizontal",
  "frame_width": 64,
  "frame_height": 64,
  "spacing": 2,
  "background_color": "#00FF00",
  "reference_image": "data:image/png;base64,...",
  "image_size": "1024x1024",
  "guidance_scale": 7.5,
  "num_inference_steps": 28,
  "seed": 12345,
  "negative_prompt": "extra limbs, realistic 3D, gradient background",
  "output_path": "/Users/krypton/Documents/go-tiny-claw/generated-images"
}
```

Supported layout prompts include `horizontal`, `vertical`, and `3x3`. Other
layout text is passed through to the image model instead of being blocked by
the MCP server. Optional tuning fields, `seed`, and `negative_prompt` are
forwarded to compatible image providers when supplied.
