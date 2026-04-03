# MCP Server

The search synthesis proxy ships an MCP (Model Context Protocol) server that exposes the search capability as a remote tool over Server-Sent Events (SSE).

## Architecture

The MCP server is a **separate binary** (`cmd/mcp/`) that acts as a transport adapter. It does not handle search or LLM synthesis itself — instead, it calls the main proxy's `/v1/search` API and formats the results for MCP clients.

```
Agent Client  ──SSE──►  MCP Server (cmd/mcp)  ──HTTP──►  Main Proxy (cmd/proxy)
                                                       ├──► SearXNG
                                                       └──► LLM
```

## Exposed Tool

### `search`

Search the web through the synthesis proxy and return a synthesized answer with cited sources.

**Input:**

| Parameter | Type   | Required | Description         |
|-----------|--------|----------|---------------------|
| `query`   | string | yes      | The search query    |

**Output:**

Human-readable text containing the synthesized answer followed by numbered sources with titles, URLs, and snippets. Structured content is also attached for clients that support it.

## Endpoints

| Method | Path             | Purpose              |
|--------|------------------|----------------------|
| GET    | `/mcp/sse`       | SSE connection       |
| POST   | `/mcp/messages`  | JSON-RPC messages    |
| GET    | `/healthz`       | Health check         |

## Environment Variables

### Required

| Variable             | Purpose                                  |
|----------------------|------------------------------------------|
| `MCP_PROXY_BASE_URL` | Base URL of the main proxy (e.g. `http://127.0.0.1:8080`) |

### Optional (with defaults)

| Variable                  | Default | Purpose                          |
|---------------------------|---------|----------------------------------|
| `MCP_PORT`                | `8090`  | HTTP listen port                 |
| `MCP_REQUEST_TIMEOUT`     | `30s`   | Timeout for calls to the proxy   |
| `MCP_SERVER_READ_TIMEOUT` | `10s`   | HTTP server read timeout         |
| `MCP_SERVER_WRITE_TIMEOUT`| `0s`    | HTTP server write timeout (0 = no timeout for SSE) |
| `MCP_SERVER_IDLE_TIMEOUT` | `60s`   | HTTP server idle timeout         |
| `MCP_SHUTDOWN_TIMEOUT`    | `10s`   | Graceful shutdown deadline       |

> **Note:** `MCP_SERVER_WRITE_TIMEOUT` defaults to 0 (no timeout) to keep SSE connections alive. Do not set a short write timeout — it will break long-lived event streams.

## Running Locally

### Prerequisites

- A running instance of the main search synthesis proxy
- Go 1.26+ (if building from source)

### Build

```bash
go build -o mcp-server ./cmd/mcp
```

### Run

```bash
export MCP_PROXY_BASE_URL=http://127.0.0.1:8080
./mcp-server
```

Or with `go run`:

```bash
MCP_PROXY_BASE_URL=http://127.0.0.1:8080 go run ./cmd/mcp
```

### Smoke Test

```bash
# Health check
curl http://127.0.0.1:8090/healthz

# SSE endpoint should respond with text/event-stream
curl -N http://127.0.0.1:8090/mcp/sse
```

## Agent Client Configuration

### OpenCode

Add a remote MCP server in your OpenCode configuration file (typically `~/.config/opencode/opencode.json` or the project-level `opencode.json`):

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

For a remotely deployed server, replace the URL accordingly:

```json
{
  "$schema": "https://opencode.ai/config.json",
  "mcp": {
    "search": {
      "type": "remote",
      "url": "https://synth.example.com/mcp/sse",
      "enabled": true
    }
  }
}
```

### Claude Code

Use the CLI to register the MCP server:

```bash
claude mcp add search --transport sse http://127.0.0.1:8090/mcp/sse
```

Or add it to your Claude Code settings file (`~/.claude/settings.json`):

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

Add the MCP server to your Gemini CLI settings (`~/.gemini/settings.json`):

```json
{
  "mcpServers": {
    "search": {
      "url": "http://127.0.0.1:8090/mcp/sse"
    }
  }
}
```

## Deployment

See [DEPLOY.md](DEPLOY.md) for full deployment instructions including systemd units, reverse proxy configuration, and SSE-specific considerations.
