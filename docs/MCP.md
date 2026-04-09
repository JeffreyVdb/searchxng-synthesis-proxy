# MCP Server

This is the authoritative transport guide for the MCP server shipped with the search synthesis proxy.

The MCP server is a separate binary (`cmd/mcp/`) that exposes one shared `search` tool surface through two HTTP transports:

- StreamableHTTP at `/mcp` — modern and preferred, especially for Hermes native MCP
- SSE compatibility endpoints at `/mcp/sse` and `/mcp/messages` — retained for clients that still expect SSE

The MCP server does not perform search or synthesis itself. It calls the main proxy's `/v1/search` API and reformats the result for MCP clients.

## Transport layout

```
Agent Client  ──StreamableHTTP or SSE──►  MCP Server (cmd/mcp)  ──HTTP──►  Main Proxy (cmd/proxy)
                                                                          ├──► SearXNG
                                                                          └──► LLM
```

## Exposed tool

### `search`

Search the web through the synthesis proxy and return a synthesized answer with cited sources.

| Parameter | Type | Required | Description |
|---|---|---|---|
| `query` | string | yes | Search query text |

The tool returns human-readable text with the synthesized answer followed by numbered sources. Structured content is also attached for clients that support it.

## Endpoints

| Method | Path | Purpose |
|---|---|---|
| GET | `/mcp` | StreamableHTTP listener / stream negotiation |
| POST | `/mcp` | StreamableHTTP JSON-RPC requests |
| DELETE | `/mcp` | StreamableHTTP session termination |
| GET | `/mcp/sse` | SSE connection endpoint |
| POST | `/mcp/messages` | SSE JSON-RPC message endpoint |
| GET | `/healthz` | Health check |

## Environment variables

### Required

| Variable | Purpose |
|---|---|
| `MCP_PROXY_BASE_URL` | Base URL of the main proxy, for example `http://127.0.0.1:8080` |

### Optional

| Variable | Default | Purpose |
|---|---|---|
| `MCP_PORT` | `8090` | HTTP listen port |
| `MCP_REQUEST_TIMEOUT` | `30s` | Timeout for calls to the main proxy |
| `MCP_SERVER_READ_TIMEOUT` | `10s` | HTTP server read timeout |
| `MCP_SERVER_WRITE_TIMEOUT` | `0s` | HTTP server write timeout. `0s` avoids breaking long-lived streams, including SSE compatibility connections. |
| `MCP_SERVER_IDLE_TIMEOUT` | `60s` | HTTP server idle timeout |
| `MCP_SHUTDOWN_TIMEOUT` | `10s` | Graceful shutdown deadline |

## Running locally

```bash
go build -o mcp-server ./cmd/mcp

export MCP_PROXY_BASE_URL=http://127.0.0.1:8080
./mcp-server
```

Or:

```bash
MCP_PROXY_BASE_URL=http://127.0.0.1:8080 go run ./cmd/mcp
```

## Smoke tests

```bash
# Health check
curl http://127.0.0.1:8090/healthz

# Preferred StreamableHTTP endpoint should answer on /mcp
curl -i -H 'Accept: text/event-stream' http://127.0.0.1:8090/mcp

# Compatibility SSE endpoint should stay reachable
curl -N http://127.0.0.1:8090/mcp/sse
```

A slightly deeper StreamableHTTP initialize smoke test:

```bash
curl -i \
  -X POST http://127.0.0.1:8090/mcp \
  -H 'Content-Type: application/json' \
  -H 'Accept: application/json, text/event-stream' \
  --data '{
    "jsonrpc": "2.0",
    "id": 1,
    "method": "initialize",
    "params": {
      "protocolVersion": "2025-03-26",
      "capabilities": {},
      "clientInfo": {"name": "smoke-test", "version": "1.0.0"}
    }
  }'
```

## Preferred client configuration: Hermes native MCP

Use the StreamableHTTP endpoint at `/mcp`.

Local example:

```yaml
mcp_servers:
  search:
    url: "http://127.0.0.1:8090/mcp"
```

Remote example:

```yaml
mcp_servers:
  search:
    url: "https://search.vandenborne.co/mcp"
```

## SSE compatibility examples

These examples are for clients that still expect SSE endpoints. Prefer `/mcp` when your client supports StreamableHTTP.

### OpenCode

```json
{
  "$schema": "https://opencode.ai/config.json",
  "mcp": {
    "search": {
      "type": "remote",
      "url": "http://127.0.0.1:8090/mcp/sse",
      "enabled": true
    }
  }
}
```

Remote SSE example:

```json
{
  "$schema": "https://opencode.ai/config.json",
  "mcp": {
    "search": {
      "type": "remote",
      "url": "https://search.vandenborne.co/mcp/sse",
      "enabled": true
    }
  }
}
```

### Claude Code

```bash
claude mcp add search --transport sse http://127.0.0.1:8090/mcp/sse
```

```json
{
  "mcpServers": {
    "search": {
      "type": "sse",
      "url": "http://127.0.0.1:8090/mcp/sse"
    }
  }
}
```

### Gemini CLI

```json
{
  "mcpServers": {
    "search": {
      "url": "http://127.0.0.1:8090/mcp/sse"
    }
  }
}
```

## Notes

- `/mcp` and `/mcp/sse` expose the same underlying tool surface.
- `/healthz` remains the simple server health endpoint.
- See [DEPLOY.md](DEPLOY.md) for deployment details such as systemd units and reverse proxy configuration.
