# Deployment Guide

## Prerequisites

- Go 1.24+ (for building from source)
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

## Building from Source

```bash
git clone <repo-url> && cd search-synthesis-proxy
go build -o search-synthesis-proxy ./cmd/proxy
```

## Running Locally

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

## systemd Service

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

## Health Check

```bash
curl http://127.0.0.1:8080/healthz
# Expected: {"status":"ok"}
```

## Smoke Test

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

## Upgrade Notes

1. Build the new binary
2. Test with `./search-synthesis-proxy` in a separate port if needed
3. `sudo systemctl restart search-synthesis-proxy`
4. Verify with `curl http://127.0.0.1:8080/healthz`

No database migration is needed (stateless service).
