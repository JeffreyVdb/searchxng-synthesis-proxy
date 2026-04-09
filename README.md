# Search Synthesis Proxy

A small, stateless Go service that takes a search query, fetches results from SearXNG, synthesizes an answer using an LLM (via OpenRouter or any OpenAI-compatible API), and returns a JSON response with the answer and cited sources.

## Features

- **SearXNG integration** — forwards search queries to a self-hosted SearXNG instance
- **LLM synthesis** — uses an OpenAI-compatible chat completions API (default: OpenRouter with `xiaomi/mimo-v2-flash`)
- **Cited sources** — the model cites specific sources, which are validated and returned in the response
- **JSON-only API** — clean, machine-readable responses with stable error codes
- **MCP server** — separate remote MCP server exposing the same tool surface over StreamableHTTP (`/mcp`) and SSE compatibility endpoints (`/mcp/sse`, `/mcp/messages`)
- **Standard library HTTP** — no web frameworks, just `net/http` with method-based routing
- **Configurable** — all settings via environment variables with sensible defaults
- **Tested** — unit tests for all internal packages using table-driven tests and `httptest`

## How It Works

1. You send `GET /v1/search?q=your+query`
2. The service searches SearXNG for results
3. The top results are formatted into a numbered source list
4. An LLM synthesizes an answer using only those sources
5. You get back a JSON response with the answer and cited sources

## API

### Search

```
GET /v1/search?q=<query>
```

**Response (200):**

```json
{
  "query": "golang context",
  "answer": "In Go, context is used to propagate cancellation, deadlines, and request-scoped values across API boundaries.",
  "sources": [
    {
      "index": 1,
      "title": "Go Concurrency Patterns: Context",
      "url": "https://go.dev/blog/context",
      "snippet": "The context package makes it easy to pass request-scoped values...",
      "engine": "google"
    }
  ],
  "meta": {
    "model": "xiaomi/mimo-v2-flash",
    "results_considered": 5,
    "took_ms": 812
  }
}
```

**Error Response:**

```json
{
  "error": {
    "code": "invalid_query",
    "message": "query parameter q is required"
  }
}
```

### Health Check

```
GET /healthz
```

```json
{"status":"ok"}
```

## Configuration

| Variable | Required | Default | Purpose |
|---|---|---|---|
| `LLM_API_KEY` | **yes** | — | API key for the LLM provider |
| `PROXY_PORT` | no | `8080` | HTTP listen port |
| `SEARXNG_BASE_URL` | no | `http://127.0.0.1:8888` | SearXNG base URL |
| `LLM_BASE_URL` | no | `https://openrouter.ai/api/v1` | OpenAI-compatible API base URL |
| `LLM_MODEL` | no | `xiaomi/mimo-v2-flash` | Model name |
| `MAX_SEARCH_RESULTS` | no | `5` | Max results passed to the LLM |
| `SERVER_READ_TIMEOUT` | no | `10s` | HTTP server read timeout |
| `SERVER_WRITE_TIMEOUT` | no | `60s` | HTTP server write timeout |
| `SERVER_IDLE_TIMEOUT` | no | `60s` | HTTP server idle timeout |
| `SEARCH_TIMEOUT` | no | `10s` | SearXNG request timeout |
| `LLM_TIMEOUT` | no | `45s` | LLM request timeout |
| `SHUTDOWN_TIMEOUT` | no | `10s` | Graceful shutdown deadline |
| `OPENROUTER_REFERER` | no | *(empty)* | OpenRouter attribution header |
| `OPENROUTER_TITLE` | no | *(empty)* | OpenRouter attribution header |

## Running Locally

```bash
# Prerequisites: Go 1.26+, running SearXNG instance, LLM API key

export LLM_API_KEY=***
go run ./cmd/proxy

# Test it
curl 'http://localhost:8080/v1/search?q=golang+context'
```

## Testing

```bash
# Run all tests
go test ./...

# Run with race detector
go test -race ./...
```

## Project Structure

```
.
├── cmd/
│   ├── proxy/           # Main proxy entrypoint and server lifecycle
│   └── mcp/             # MCP entrypoint exposing StreamableHTTP and SSE transports
├── internal/
│   ├── api/             # HTTP handlers and routing
│   ├── config/          # Environment variable loading and validation
│   ├── llm/             # OpenAI-compatible LLM client adapter
│   ├── mcpserver/       # Shared MCP tool registration and HTTP transport wiring
│   ├── proxy/           # Orchestration, prompts, and business logic
│   ├── searx/           # SearXNG client adapter
│   └── synthproxy/      # HTTP client for the MCP server's upstream /v1/search contract
├── docs/
│   ├── ARCHITECTURE.md  # Architecture overview with diagrams
│   ├── DEPLOY.md        # Deployment guide
│   ├── MCP.md           # MCP transports, endpoints, and client setup
│   ├── SECURITY.md      # Security considerations
│   └── adr/             # Architecture Decision Records
├── go.mod
└── README.md
```

## Documentation

- [Architecture](docs/ARCHITECTURE.md) — package responsibilities, request lifecycle, MCP transport layout, tradeoffs
- [Deployment](docs/DEPLOY.md) — building, running, systemd, smoke tests, binary releases and verification
- [MCP Server](docs/MCP.md) — authoritative MCP transport guide for StreamableHTTP and SSE compatibility clients
- [Security](docs/SECURITY.md) — trust boundaries, prompt injection, hardening

## Release Verification

GitHub releases include SHA256 checksums (`checksums.txt`) and a minisign signature (`checksums.txt.minisig`). The public verification key is stored in [`minisign.pub`](minisign.pub).

For full verification instructions including single-file verification, see [**Deployment Guide → Verifying Releases**](docs/DEPLOY.md#verifying-releases).

## License

MIT License

Copyright (c) 2026 Jeffrey Vandenborne

Permission is hereby granted, free of charge, to any person obtaining a copy
of this software and associated documentation files (the "Software"), to deal
in the Software without restriction, including without limitation the rights
to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
copies of the Software, and to permit persons to whom the Software is
furnished to do so, subject to the following conditions:

The above copyright notice and this permission notice shall be included in all
copies or substantial portions of the Software.

THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE
SOFTWARE.
