# PixelsMcp

Simple MCP server built with `github.com/mark3labs/mcp-go`.

## Local run

The default transport is `stdio`, which is suitable for local MCP clients that
start the server process themselves.

```bash
go run ./cmd/pixelsmcp
```

## VPS run

Use the HTTP transport for a long-running VPS process:

```bash
PIXELSMCP_TRANSPORT=http \
PIXELSMCP_ADDR=127.0.0.1:8080 \
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

- `echo`: returns the provided `message` unchanged.
