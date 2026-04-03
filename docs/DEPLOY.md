# Deployment Guide

This guide covers deploying both the **main search synthesis proxy** and the **MCP SSE server**.

---

## Main Proxy

### Prerequisites

- Go 1.26+ (for building from source)
- A reachable SearXNG instance
- An API key for an OpenAI-compatible provider (e.g., OpenRouter)

## Environment Variables

### Required

| Variable | Purpose |
|---|---|
| `LLM_API_KEY` | API key for the LLM provider |

### Optional (with defaults)

| Variable | Default | Purpose |
|---|---|---|
| `PROXY_PORT` | `8080` | HTTP listen port |
| `SEARXNG_BASE_URL` | `http://127.0.0.1:8888` | SearXNG base URL |
| `LLM_BASE_URL` | `https://openrouter.ai/api/v1` | OpenAI-compatible API base URL |
| `LLM_MODEL` | `xiaomi/mimo-v2-flash` | Model name for the provider |
| `MAX_SEARCH_RESULTS` | `5` | Max search results passed to the LLM |
| `SERVER_READ_TIMEOUT` | `10s` | HTTP server read timeout |
| `SERVER_WRITE_TIMEOUT` | `60s` | HTTP server write timeout |
| `SERVER_IDLE_TIMEOUT` | `60s` | HTTP server idle timeout |
| `SEARCH_TIMEOUT` | `10s` | Timeout for SearXNG requests |
| `LLM_TIMEOUT` | `45s` | Timeout for LLM requests |
| `SHUTDOWN_TIMEOUT` | `10s` | Graceful shutdown deadline |
| `OPENROUTER_REFERER` | *(empty)* | Optional `HTTP-Referer` header for OpenRouter |
| `OPENROUTER_TITLE` | *(empty)* | Optional `X-Title` header for OpenRouter |

## Sample Environment File

Create `/etc/search-synthesis-proxy.env`:

```bash
LLM_API_KEY=sk-or-v1-xxxxxxxxxxxxxxxxx
PROXY_PORT=8080
SEARXNG_BASE_URL=http://127.0.0.1:8888
LLM_BASE_URL=https://openrouter.ai/api/v1
LLM_MODEL=xiaomi/mimo-v2-flash
MAX_SEARCH_RESULTS=5
SEARCH_TIMEOUT=10s
LLM_TIMEOUT=45s
```

### Building from Source

```bash
git clone <repo-url> && cd search-synthesis-proxy
go build -o search-synthesis-proxy ./cmd/proxy
```

### Running Locally

```bash
# Set required env
export LLM_API_KEY=your-key

# Run
go run ./cmd/proxy
```

Or with the compiled binary:

```bash
./search-synthesis-proxy
```

### systemd Service

Create `/etc/systemd/system/search-synthesis-proxy.service`:

```ini
[Unit]
Description=Search Synthesis Proxy
After=network-online.target
Wants=network-online.target

[Service]
Type=simple
User=www-data
WorkingDirectory=/opt/search-synthesis-proxy
EnvironmentFile=/etc/search-synthesis-proxy.env
ExecStart=/opt/search-synthesis-proxy/search-synthesis-proxy
Restart=on-failure
RestartSec=3

# Security hardening
NoNewPrivileges=true
ProtectSystem=strict
ProtectHome=true
PrivateTmp=true
ReadOnlyPaths=/opt/search-synthesis-proxy

[Install]
WantedBy=multi-user.target
```

Enable and start:

```bash
sudo systemctl daemon-reload
sudo systemctl enable search-synthesis-proxy
sudo systemctl start search-synthesis-proxy
```

### Health Check

```bash
curl http://127.0.0.1:8080/healthz
# Expected: {"status":"ok"}
```

### Smoke Test

```bash
curl -s 'http://127.0.0.1:8080/v1/search?q=golang+context' | jq .
```

Expected response:

```json
{
  "query": "golang context",
  "answer": "...",
  "sources": [
    {
      "index": 1,
      "title": "...",
      "url": "...",
      "snippet": "...",
      "engine": "..."
    }
  ],
  "meta": {
    "model": "xiaomi/mimo-v2-flash",
    "results_considered": 5,
    "took_ms": 1234
  }
}
```

### Upgrade Notes

1. Build the new binary
2. Test with `./search-synthesis-proxy` in a separate port if needed
3. `sudo systemctl restart search-synthesis-proxy`
4. Verify with `curl http://127.0.0.1:8080/healthz`

No database migration is needed (stateless service).

---

## Binary Releases

Tagged releases produce pre-built binaries alongside the container images.

### Cutting a release

1. Ensure all desired changes are merged to `main`.
2. Create and push a semver tag:

   ```bash
   git tag v1.2.3
   git push origin v1.2.3
   ```

3. GitHub Actions runs the GoReleaser binary release workflow automatically.
4. Release assets appear on the GitHub Releases page.

### Release artifacts

Each release includes:

- `search-synthesis-proxy_vX.Y.Z_linux_amd64.tar.gz` — proxy binary
- `search-synthesis-proxy_vX.Y.Z_linux_arm64.tar.gz` — proxy binary (ARM)
- `mcp-server_vX.Y.Z_linux_amd64.tar.gz` — MCP server binary
- `mcp-server_vX.Y.Z_linux_arm64.tar.gz` — MCP server binary (ARM)
- `checksums.txt` — SHA256 checksums for all archives
- `checksums.txt.minisig` — minisign signature of the checksum file

### Verifying a release

Download the assets you need, plus `checksums.txt`, `checksums.txt.minisig`, and `minisign.pub` from the repository.

**1. Verify the minisign signature:**

```bash
minisign -Vm checksums.txt -p minisign.pub -x checksums.txt.minisig
```

**2. Verify the SHA256 checksum of an archive:**

To check all assets in the current directory:

```bash
sha256sum -c checksums.txt
```

To check a single asset:

```bash
grep 'search-synthesis-proxy_.*_linux_amd64.tar.gz' checksums.txt | sha256sum -c -
```

### Public verification key

The minisign public key is stored in `minisign.pub` at the repository root:

```
untrusted comment: minisign public key F25B682482FBF72B
RWQr9/uCJGhb8kBVftfmdSa6oAxoNOTcdCn399XX8gMm+qlV/8AESfVz
```

You can also obtain it directly from the repository rather than the release page.

---

## MCP SSE Server

The MCP server is a separate binary that exposes the search capability to agent clients over SSE. It calls the main proxy's `/v1/search` API as its upstream.

### Prerequisites

- A running instance of the main search synthesis proxy
- Network access from the MCP server to the main proxy

### Environment Variables

#### Required

| Variable | Purpose |
|---|---|
| `MCP_PROXY_BASE_URL` | Base URL of the main proxy (e.g. `http://127.0.0.1:8080`) |

#### Optional (with defaults)

| Variable | Default | Purpose |
|---|---|---|
| `MCP_PORT` | `8090` | HTTP listen port |
| `MCP_REQUEST_TIMEOUT` | `30s` | Timeout for calls to the main proxy |
| `MCP_SERVER_READ_TIMEOUT` | `10s` | HTTP server read timeout |
| `MCP_SERVER_WRITE_TIMEOUT` | `0s` | HTTP server write timeout (0 = no timeout, required for SSE) |
| `MCP_SERVER_IDLE_TIMEOUT` | `60s` | HTTP server idle timeout |
| `MCP_SHUTDOWN_TIMEOUT` | `10s` | Graceful shutdown deadline |

### Building

```bash
go build -o mcp-server ./cmd/mcp
```

### Sample Environment File

Create `/etc/mcp-server.env`:

```bash
MCP_PROXY_BASE_URL=http://127.0.0.1:8080
MCP_PORT=8090
MCP_REQUEST_TIMEOUT=30s
```

### systemd Service

Create `/etc/systemd/system/mcp-server.service`:

```ini
[Unit]
Description=MCP SSE Server for Search Synthesis
After=network-online.target search-synthesis-proxy.service
Wants=network-online.target
Requires=search-synthesis-proxy.service

[Service]
Type=simple
User=www-data
WorkingDirectory=/opt/search-synthesis-proxy
EnvironmentFile=/etc/mcp-server.env
ExecStart=/opt/search-synthesis-proxy/mcp-server
Restart=on-failure
RestartSec=3

# Security hardening
NoNewPrivileges=true
ProtectSystem=strict
ProtectHome=true
PrivateTmp=true
ReadOnlyPaths=/opt/search-synthesis-proxy

[Install]
WantedBy=multi-user.target
```

Enable and start:

```bash
sudo systemctl daemon-reload
sudo systemctl enable mcp-server
sudo systemctl start mcp-server
```

### Health Check

```bash
curl http://127.0.0.1:8090/healthz
# Expected: {"status":"ok"}
```

### Endpoints

| Method | Path | Purpose |
|--------|------|--------|
| GET | `/mcp/sse` | SSE connection endpoint |
| POST | `/mcp/messages` | JSON-RPC message endpoint |
| GET | `/healthz` | Health check |

### Reverse Proxy Considerations

When placing the MCP server behind a reverse proxy (nginx, Caddy, etc.), SSE requires special attention:

1. **Disable proxy buffering** — SSE events must be forwarded immediately, not buffered.
2. **Allow long-lived connections** — SSE connections stay open; do not impose short timeouts.
3. **Set appropriate headers** — ensure `Connection: keep-alive` is passed through.

#### nginx Example

```nginx
location /mcp/ {
    proxy_pass http://127.0.0.1:8090;
    proxy_http_version 1.1;
    proxy_set_header Connection "";
    proxy_set_header Host $host;
    proxy_buffering off;
    proxy_cache off;
    proxy_read_timeout 86400s;
}

location /healthz {
    proxy_pass http://127.0.0.1:8090;
}
```

#### Caddy Example

```
synth.example.com {
    handle /mcp/* {
        reverse_proxy localhost:8090 {
            flush_interval -1
        }
    }
    handle /healthz {
        reverse_proxy localhost:8090
    }
}
```

### Co-located Deployment

When the MCP server and main proxy run on the same host, prefer direct local networking:

```bash
MCP_PROXY_BASE_URL=http://127.0.0.1:8080
```

This avoids unnecessary network hops and keeps latency minimal.
