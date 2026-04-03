Status: DRAFT

# Plan: MCP SSE server for the Search Synthesis Proxy

## Objective

Add a separate MCP server binary that exposes the existing search synthesis capability to agent clients over Server-Sent Events (SSE), with OpenCode as the primary target and additional configuration guidance for other agent CLI harnesses.

The MCP server must:

- live under `cmd/mcp/`
- connect to the main proxy service via `GET /v1/search`
- take the upstream proxy endpoint from environment configuration
- ship deployment documentation updates in `docs/DEPLOY.md`
- add an ADR under `docs/adr/`
- add MCP client setup guidance in `docs/MCP.md`
- link `docs/MCP.md` from `README.md`

## Current repo findings

### Existing architecture

The repo currently contains one HTTP JSON service:

- `cmd/proxy/` boots the existing search synthesis API
- `internal/api/` exposes `GET /healthz` and `GET /v1/search`
- `internal/proxy/` contains the orchestration logic and response types
- `internal/config/Load()` currently assumes the main synthesis service and requires `LLM_API_KEY`

### Architectural implication

The MCP server should be a **separate process** and **separate binary**, not a mode bolted into `cmd/proxy/`, because:

1. its job is different: MCP transport + tool exposure rather than JSON search serving
2. it should depend only on the existing `/v1/search` API, not on direct SearXNG + LLM configuration
3. it should not require `LLM_API_KEY`, `SEARXNG_BASE_URL`, or other main-service-only settings

That means the current config loading needs to be split or extended so the MCP binary can load only the settings it actually needs.

## Proposed architecture

### Decision summary

Build a small **remote MCP wrapper** around the existing proxy service.

- The main proxy remains the source of truth for search + synthesis.
- The new MCP server acts as a transport adapter for agent clients.
- The MCP server calls the existing proxy over HTTP at `/v1/search`.
- The MCP server exposes a single `search` tool over SSE.
- The MCP-specific transport and tool registration stay isolated so the codebase can add streamable HTTP later if needed.

## Why this shape

### Why a separate MCP wrapper instead of in-process reuse

The requirement explicitly says the MCP server must send requests to the main proxy server on `/v1/search`. Treating the proxy as an upstream dependency gives a cleaner deployment boundary:

- the search API stays stable and independently testable
- the MCP layer stays thin and easy to reason about
- MCP client bugs cannot directly tangle with the SearXNG/LLM pipeline
- the MCP server can be deployed next to, or separately from, the main service

### Why SSE despite the protocol moving toward streamable HTTP

SSE is explicitly required here and is still widely supported by agent tooling. However, SSE is no longer the long-term transport direction in MCP, so the implementation should keep the business logic transport-agnostic.

Concretely:

- use an MCP SDK instead of hand-rolling the protocol
- isolate tool handlers from transport setup
- keep the HTTP server layout ready for a future streamable-HTTP endpoint without rewriting the tool logic

## Package and file plan

### New entrypoint

- `cmd/mcp/main.go`

Responsibilities:

- load MCP-specific config
- initialize logging
- build the upstream search proxy client
- construct the MCP server and register tools
- serve the SSE transport endpoints
- handle graceful shutdown

### New internal package(s)

#### Option A: `internal/mcpserver/`

Recommended layout:

- `internal/mcpserver/server.go` — MCP server construction and tool registration
- `internal/mcpserver/format.go` — tool result formatting for text output
- `internal/mcpserver/http.go` — SSE transport wiring / HTTP handler exposure
- `internal/mcpserver/server_test.go` — tool-level and integration-style tests

#### New upstream client package

Add a small HTTP client package dedicated to the existing proxy API, for example:

- `internal/synthproxy/client.go`
- `internal/synthproxy/client_test.go`

Responsibilities:

- call `GET {baseURL}/v1/search?q=...`
- decode the existing JSON response shape
- map upstream HTTP failures into safe application errors for the MCP layer
- avoid coupling the MCP wrapper to internal in-process proxy service types

Using a dedicated upstream client is cleaner than importing `internal/proxy` response types directly, because the MCP server is consuming a **remote API contract**, not the internal service implementation.

### Config split

The current `internal/config/Config` and `Load()` are tailored to the main service and require `LLM_API_KEY`, which the MCP wrapper should not need.

Recommended refactor:

- keep shared env parsing helpers in `internal/config/`
- introduce separate load paths such as:
  - `LoadProxy()` for the main service
  - `LoadMCP()` for the MCP wrapper
- add a dedicated `MCPConfig` struct

This avoids accidental config coupling between two binaries with different responsibilities.

## SDK / transport approach

Use the official Go MCP SDK rather than implementing JSON-RPC + SSE details manually.

### Transport target

Expose an SSE-based remote MCP server with conventional endpoints under a stable prefix, for example:

- `GET /mcp/sse`
- `POST /mcp/messages`
- `GET /healthz`

This endpoint layout is easy to document and aligns with legacy SSE MCP client expectations.

### Important implementation note

Keep the tool registration and search client logic independent from the transport wiring so the project can later add a streamable-HTTP endpoint beside SSE if OpenCode or other clients evolve away from SSE.

## Tool surface

Expose one MCP tool first:

### `search`

Purpose:

- search the web through the main synthesis proxy and return a synthesized answer with cited sources

Input schema:

- `query` — required string

Keep the first version intentionally minimal. The upstream `/v1/search` API currently only requires the query string, so the MCP tool should mirror that rather than inventing controls the backend does not support.

### Tool result shape

Return both:

1. **human-readable text content** for CLI harnesses that display tool results as text
2. **structured content** that mirrors the upstream JSON payload for clients that can inspect structured tool output

Suggested text format:

```text
Answer:
<answer>

Sources:
1. <title>
   <url>
   <snippet>
```

This keeps OpenCode and similar harnesses pleasant to use while preserving the machine-readable payload.

### Error behavior

Map upstream failures predictably:

- empty query → tool error before hitting upstream
- upstream 4xx/5xx with JSON error payload → surface the upstream code/message cleanly
- network timeout / connection error → return a clear transport/upstream failure message
- malformed upstream success payload → return a safe internal/tool error

## Configuration plan

Add a dedicated MCP environment configuration set.

### Required

- `MCP_PROXY_BASE_URL` — base URL of the main search synthesis proxy, for example `http://127.0.0.1:8080`

### Optional

- `MCP_PORT` — listen port for the MCP server, default `8090`
- `MCP_REQUEST_TIMEOUT` — timeout for calls from MCP to the main proxy, default `30s`
- `MCP_SERVER_READ_TIMEOUT` — default `10s`
- `MCP_SERVER_WRITE_TIMEOUT` — default `0` or a long enough duration for SSE responses
- `MCP_SERVER_IDLE_TIMEOUT` — default `60s`
- `MCP_SHUTDOWN_TIMEOUT` — default `10s`

## Timeout note

Because SSE keeps connections open, write-timeout behavior must be chosen carefully. The implementation should not inherit a short write timeout that breaks long-lived event streams.

That should be called out explicitly in both code comments and deployment docs.

## Testing plan

### Unit tests

#### Config

Add tests for:

- missing `MCP_PROXY_BASE_URL`
- invalid upstream URL
- default MCP port/timeouts
- invalid duration values

#### Upstream client

Use `httptest.Server` to cover:

- successful `/v1/search` response decoding
- upstream JSON error payload propagation
- upstream non-JSON error response handling
- timeout / network failure path

#### MCP tool handler

Cover:

- valid `query` invokes upstream client
- blank `query` rejected locally
- tool output contains answer + numbered sources
- structured content matches upstream payload
- upstream failures become MCP tool errors

### Integration tests

Add at least one integration-style test that boots:

1. a fake upstream `/v1/search` server
2. the MCP HTTP handler

Then verify:

- the SSE endpoint is reachable
- the tool registration includes `search`
- calling the tool produces the expected answer and sources

## Documentation plan

### 1. `docs/adr/0001-mcp-sse-wrapper.md`

Create the repo’s first ADR and capture these decisions:

- use a separate MCP binary under `cmd/mcp/`
- treat the existing proxy API as the upstream contract
- support SSE first because that is the required client transport
- keep the internal design transport-agnostic for a future streamable-HTTP addition

### 2. `docs/MCP.md`

This should become the operator-facing guide for using the MCP server with agent clients.

Sections:

1. purpose and architecture in one screen
2. exposed tool(s)
3. environment variables
4. local run instructions
5. deployed endpoint layout
6. smoke test examples
7. agent CLI harness configuration examples

#### Harness examples to include

At minimum include tested examples for:

- **OpenCode** — primary target
- **Claude Code**
- **Gemini CLI**

If another harness is easy to support with the same remote SSE URL, add it as a short fourth example, but keep the doc tight.

### Suggested doc content per harness

#### OpenCode

Document a remote MCP config snippet pointing at the SSE endpoint, with a short explanation of where the config file lives and how to enable the server.

#### Claude Code

Document either:

- the `claude mcp add ... --transport sse ...` command, or
- the equivalent config file snippet

Prefer whichever is simpler and more stable in current docs.

#### Gemini CLI

Document the `settings.json` `mcpServers` block using the remote SSE URL and any optional headers/trust fields that are actually needed.

### 3. `docs/DEPLOY.md`

Update deployment docs to cover the MCP wrapper as a second service.

Add:

- build command for `./cmd/mcp`
- sample env file for the MCP service
- sample systemd unit for the MCP service
- health check endpoint
- SSE endpoint path(s)
- reverse proxy guidance for SSE

#### Important reverse-proxy notes

Because this is SSE, deployment docs should explicitly mention:

- disable proxy buffering where relevant
- allow long-lived HTTP responses
- keep read/write timeouts compatible with SSE
- prefer direct local networking between MCP wrapper and main proxy when co-located

### 4. `README.md`

Update the documentation section to add:

- `docs/MCP.md` — MCP server usage and client setup

Optionally also mention the repo now contains both the main JSON API and an MCP wrapper.

## Proposed implementation phases

### Phase 1: config and upstream client

- split config loading for proxy vs MCP
- add the dedicated upstream client for `/v1/search`
- add unit tests around config and upstream error handling

### Phase 2: MCP server

- add `cmd/mcp/`
- add MCP server construction and the `search` tool
- expose the SSE transport routes
- add tool/result tests

### Phase 3: docs

- add `docs/adr/0001-mcp-sse-wrapper.md`
- add `docs/MCP.md`
- update `docs/DEPLOY.md`
- update `README.md`

### Phase 4: verification

- `go test ./...`
- local smoke test against a running main proxy
- verify one real OpenCode configuration flow end-to-end
- spot-check at least one other CLI harness example from `docs/MCP.md`

## Acceptance criteria

The work is complete when all of the following are true:

- a new MCP entrypoint exists at `cmd/mcp/`
- the MCP server communicates with the main service via `/v1/search`
- the upstream base URL is configurable by environment variable
- the MCP server supports SSE transport
- a single `search` MCP tool is available and usable
- the MCP server returns a readable answer plus cited sources
- MCP-specific config does not require main-service-only secrets like `LLM_API_KEY`
- `docs/adr/0001-*.md` exists and captures the architecture decision
- `docs/MCP.md` documents OpenCode and multiple CLI harness configurations
- `docs/DEPLOY.md` documents how to run and expose the MCP service
- `README.md` links to `docs/MCP.md`
- tests cover config, upstream client behavior, and MCP tool behavior

## Risks and mitigations

### Risk: SSE client compatibility differences

Different MCP clients may vary in how they expect remote SSE URLs and reconnection behavior.

Mitigation:

- use an established Go MCP SDK
- keep the documented endpoint path stable
- verify the examples against at least OpenCode plus one more harness during implementation

### Risk: config coupling with the existing proxy service

The current config loader is tuned to the main service and can accidentally force unrelated settings into the MCP binary.

Mitigation:

- split proxy and MCP config loading early
- add tests that prove the MCP binary can start with only MCP-required settings

### Risk: broken SSE behind reverse proxies

SSE often fails when buffering or timeout defaults are wrong.

Mitigation:

- document reverse-proxy requirements clearly in `docs/DEPLOY.md`
- keep a simple direct localhost smoke test in the implementation notes

## Deliverables summary

Implementation should produce:

- `cmd/mcp/`
- MCP config support
- dedicated upstream client for `/v1/search`
- `docs/adr/0001-mcp-sse-wrapper.md`
- `docs/MCP.md`
- updated `docs/DEPLOY.md`
- updated `README.md`
- tests covering the new behavior
