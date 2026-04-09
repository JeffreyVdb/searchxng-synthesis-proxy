# StreamableHTTP MCP Endpoint Implementation Plan

> **For Hermes:** Use subagent-driven-development skill to implement this plan task-by-task.

**Goal:** Add a native StreamableHTTP MCP endpoint alongside the existing SSE transport so Hermes can connect to this repository’s MCP server without breaking existing SSE clients.

**Architecture:** Keep `internal/mcpserver.New(...)` as the single source of MCP tool registration and expose two transport adapters from the same `*server.MCPServer`: the existing SSE transport on `/mcp/sse` + `/mcp/messages`, and a new StreamableHTTP transport on `/mcp`. The HTTP mux in `internal/mcpserver` should own transport composition so `cmd/mcp` only wires config, upstream client, and the shared handler.

**Tech Stack:** Go 1.26, `github.com/mark3labs/mcp-go v0.46.0`, `net/http`, `httptest`, existing repo docs under `docs/`.

---

## Current repo findings

1. The repo already cleanly separates MCP tool logic from transport wiring:
   - `internal/mcpserver/server.go` builds the tool surface.
   - `internal/mcpserver/http.go` currently exposes only SSE.
   - `cmd/mcp/main.go` only boots the server and should stay thin.
2. The current MCP server is explicitly SSE-only in code and docs:
   - `README.md`
   - `docs/MCP.md`
   - `docs/ARCHITECTURE.md`
   - `docs/adr/0001-mcp-sse-wrapper.md`
3. The dependency already supports StreamableHTTP in the pinned version:
   - `github.com/mark3labs/mcp-go@v0.46.0/server/streamable_http.go`
   - constructor: `server.NewStreamableHTTPServer(...)`
4. The code comments in `internal/mcpserver/server.go` already anticipated this exact extension.
5. Because deployment is out of scope for this pass, the deliverable is code + tests + docs + a stable endpoint shape, not infra changes.

## Target endpoint shape

Keep existing SSE endpoints unchanged:
- `GET /mcp/sse`
- `POST /mcp/messages`

Add StreamableHTTP endpoint:
- `GET /mcp`
- `POST /mcp`
- `DELETE /mcp`

Recommended endpoint to hand back to Jeffrey for Hermes native MCP:
- `https://search.vandenborne.co/mcp`

That keeps old OpenCode/Claude SSE setups working while giving Hermes a modern HTTP transport endpoint.

## Design constraints

- Do not rewrite the `search` tool logic.
- Do not break existing SSE clients.
- Do not move search or synthesis logic into the MCP binary.
- Prefer stateless StreamableHTTP unless a concrete need for stateful sessions appears during implementation.
- Keep docs explicit that `/mcp` is the StreamableHTTP endpoint and `/mcp/sse` remains legacy-compatible.
- Update `docs/ARCHITECTURE.md` and add a new ADR because transport support is materially changing.
- Update `docs/SECURITY.md` only if implementation introduces new security-relevant behavior worth documenting.

## Implementation overview

The implementation should introduce a combined MCP HTTP handler that mounts:
- health check at `/healthz`
- SSE server at `/mcp/sse` and `/mcp/messages`
- StreamableHTTP server at `/mcp`

Suggested internal API shape:

```go
// internal/mcpserver/http.go
func NewHTTPHandler(logger *slog.Logger, client *synthproxy.Client, opts HTTPOptions) http.Handler
```

Where `HTTPOptions` contains transport-specific path options, for example:

```go
type HTTPOptions struct {
    BaseURL               string
    SSEEndpoint           string
    MessageEndpoint       string
    StreamableHTTPEndpoint string
}
```

The handler should create one shared MCP server instance:

```go
mcpSrv := New(logger, client)
sseServer := mcpserver.NewSSEServer(mcpSrv, ...)
streamableServer := mcpserver.NewStreamableHTTPServer(mcpSrv, ...)
```

Then route requests explicitly:

```go
mux := http.NewServeMux()
mux.HandleFunc("GET /healthz", ...)
mux.Handle("/mcp/sse", sseServer)
mux.Handle("/mcp/messages", sseServer)
mux.Handle("/mcp", streamableServer)
```

Note: verify the exact mux registrations needed for the SSE handler with `mcp-go`; if the SSE server expects to inspect multiple paths through one root mount, register the concrete paths it requires rather than a broad catch-all.

## Testing strategy summary

Add tests for:
- shared handler still serves `/healthz`
- SSE still works at `/mcp/sse`
- StreamableHTTP responds on `/mcp`
- tool registration and tool execution still use the existing `search` tool
- docs/config examples point clients to the right transport-specific endpoint

When validating StreamableHTTP, cover at minimum:
- initialize request over `POST /mcp`
- tools/list over `POST /mcp`
- tools/call for `search` over `POST /mcp`
- optional GET stream behavior when client sends `Accept: text/event-stream`

---

### Task 1: Record the transport expansion in the plan baseline

**Objective:** Freeze the intended endpoint contract and impacted files before code changes begin.

**Files:**
- Modify: `docs/plans/2026-04-09-streamable-http-endpoint.md`

**Step 1: Confirm impacted source files**

Files expected to change:
- `internal/mcpserver/http.go`
- `internal/mcpserver/server_test.go`
- `cmd/mcp/main.go`
- `README.md`
- `docs/MCP.md`
- `docs/ARCHITECTURE.md`
- `docs/adr/0004-dual-mcp-transports.md` (new)

Possible change if config evolves:
- `internal/config/mcp.go`
- `internal/config/mcp_test.go`

**Step 2: Confirm no deployment-file scope**

Do not touch reverse proxy or deployment manifests in this pass unless a code-level doc example absolutely requires it.

**Step 3: Commit**

```bash
git add docs/plans/2026-04-09-streamable-http-endpoint.md
git commit -m "docs: add streamable http implementation plan"
```

### Task 2: Add a failing test for the new StreamableHTTP endpoint reachability

**Objective:** Prove the current code does not yet expose `/mcp` as a StreamableHTTP endpoint.

**Files:**
- Modify: `internal/mcpserver/server_test.go`

**Step 1: Write failing test**

Add a test like:

```go
func TestHTTPHandler_StreamableHTTPEndpointReachable(t *testing.T) {
    upstream := httptest.NewServer(fakeUpstream())
    defer upstream.Close()

    client := synthproxy.NewClient(upstream.URL, synthproxy.DefaultHTTPClient(5*time.Second))
    testSrv := httptest.NewServer(NewHTTPHandler(nil, client, HTTPOptions{}))
    defer testSrv.Close()

    req, err := http.NewRequest(http.MethodPost, testSrv.URL+"/mcp", strings.NewReader(`{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2025-03-26","capabilities":{},"clientInfo":{"name":"test","version":"1.0.0"}}}`))
    require.NoError(t, err)
    req.Header.Set("Content-Type", "application/json")
    req.Header.Set("Accept", "application/json, text/event-stream")

    resp, err := http.DefaultClient.Do(req)
    require.NoError(t, err)
    defer resp.Body.Close()

    require.Equal(t, http.StatusOK, resp.StatusCode)
}
```

**Step 2: Run test to verify failure**

Run:
```bash
go test ./internal/mcpserver -run TestHTTPHandler_StreamableHTTPEndpointReachable -v
```

Expected: FAIL because `NewHTTPHandler` does not exist yet or `/mcp` is not served.

**Step 3: Commit**

```bash
git add internal/mcpserver/server_test.go
git commit -m "test: add failing streamable http endpoint test"
```

### Task 3: Add a failing StreamableHTTP tool-call test

**Objective:** Lock in the requirement that the new transport must expose the existing `search` tool, not a second divergent implementation.

**Files:**
- Modify: `internal/mcpserver/server_test.go`

**Step 1: Write failing test**

Add a test that:
- initializes a StreamableHTTP session on `/mcp`
- sends `tools/list`
- asserts a tool named `search` exists
- sends `tools/call` with `{ "query": "test query" }`
- asserts the returned content includes the synthesized answer and source URL

Use the existing fake upstream payload from `fakeUpstream()` or an inline `httptest.Server` returning deterministic JSON.

**Step 2: Run test to verify failure**

Run:
```bash
go test ./internal/mcpserver -run TestHTTPHandler_StreamableHTTPToolCall -v
```

Expected: FAIL until `/mcp` is backed by `NewStreamableHTTPServer(...)`.

**Step 3: Commit**

```bash
git add internal/mcpserver/server_test.go
git commit -m "test: add failing streamable http tool call coverage"
```

### Task 4: Refactor the HTTP transport wrapper into a combined handler

**Objective:** Replace the SSE-only HTTP wrapper with a transport-composing handler that can serve both SSE and StreamableHTTP.

**Files:**
- Modify: `internal/mcpserver/http.go`

**Step 1: Introduce a combined options struct**

Replace `SSEOptions` with a broader options type, or keep `SSEOptions` and add a new `HTTPOptions` wrapper.

Suggested implementation:

```go
type HTTPOptions struct {
    BaseURL                string
    SSEEndpoint            string
    MessageEndpoint        string
    StreamableHTTPEndpoint string
}
```

Default values:
- `SSEEndpoint = "/mcp/sse"`
- `MessageEndpoint = "/mcp/messages"`
- `StreamableHTTPEndpoint = "/mcp"`

**Step 2: Add the new combined constructor**

```go
func NewHTTPHandler(logger *slog.Logger, client *synthproxy.Client, opts HTTPOptions) http.Handler
```

Implementation requirements:
- create one shared `mcpSrv := New(logger, client)`
- create `sseServer := mcpserver.NewSSEServer(mcpSrv, ...)`
- create `streamableServer := mcpserver.NewStreamableHTTPServer(mcpSrv)`
- register `/healthz`
- register SSE endpoints explicitly
- register `/mcp` to the StreamableHTTP server

**Step 3: Preserve a compatibility constructor if useful**

If minimizing churn helps, keep:

```go
func NewSSEHandler(...) http.Handler {
    return NewHTTPHandler(...)
}
```

That lets existing callers continue compiling while tests and docs migrate.

**Step 4: Run package tests**

Run:
```bash
go test ./internal/mcpserver -v
```

Expected: previous SSE tests still pass; new `/mcp` reachability test now passes or moves to the next failing assertion.

**Step 5: Commit**

```bash
git add internal/mcpserver/http.go internal/mcpserver/server_test.go
git commit -m "feat: expose streamable http alongside sse"
```

### Task 5: Update the MCP server entrypoint to use the combined handler

**Objective:** Ensure the runnable binary serves both transports.

**Files:**
- Modify: `cmd/mcp/main.go`
- Test: `cmd/mcp/main_test.go`

**Step 1: Replace the SSE-only constructor call**

Change:

```go
handler := mcpserver.NewSSEHandler(...)
```

To:

```go
handler := mcpserver.NewHTTPHandler(logger, client, mcpserver.HTTPOptions{
    SSEEndpoint: "/mcp/sse",
    MessageEndpoint: "/mcp/messages",
    StreamableHTTPEndpoint: "/mcp",
})
```

**Step 2: Add or extend a server-construction test**

Add a `newServer(...)` test that boots the handler and verifies:
- `GET /healthz` returns 200
- `/mcp` is mounted

If a direct network test is too heavy, assert `newServer(...)` returns non-nil with a valid handler and then use `httptest.NewServer(srv.Handler)`.

**Step 3: Run tests**

Run:
```bash
go test ./cmd/mcp -v
```

Expected: PASS.

**Step 4: Commit**

```bash
git add cmd/mcp/main.go cmd/mcp/main_test.go
git commit -m "feat: wire dual mcp transports in entrypoint"
```

### Task 6: Add full StreamableHTTP protocol tests

**Objective:** Verify the new `/mcp` transport is not just reachable but functionally correct.

**Files:**
- Modify: `internal/mcpserver/server_test.go`

**Step 1: Add initialize coverage**

Send a JSON-RPC initialize request to `/mcp` and assert:
- `200 OK`
- JSON-RPC response body
- returned server info name/version are present

**Step 2: Add tools/list coverage**

Send:

```json
{"jsonrpc":"2.0","id":2,"method":"tools/list","params":{}}
```

Assert one tool named `search` exists.

**Step 3: Add tools/call coverage**

Send:

```json
{"jsonrpc":"2.0","id":3,"method":"tools/call","params":{"name":"search","arguments":{"query":"test query"}}}
```

Assert:
- response contains human-readable answer text
- response contains source URL
- structured payload remains attached if the library surfaces it in JSON

**Step 4: Add GET behavior coverage if practical**

Issue:

```bash
curl -i -H 'Accept: text/event-stream' http://127.0.0.1:8090/mcp
```

If library behavior is stable in tests, assert it returns either:
- SSE-compatible stream response, or
- the library’s documented expected behavior for listening GET connections

If this is flaky or awkward in unit tests, document it as a manual smoke test instead of forcing brittle assertions.

**Step 5: Run package tests**

Run:
```bash
go test ./internal/mcpserver -v
```

Expected: PASS.

**Step 6: Commit**

```bash
git add internal/mcpserver/server_test.go
git commit -m "test: cover streamable http protocol flow"
```

### Task 7: Evaluate whether MCP config needs transport-specific fields

**Objective:** Decide whether the current environment config is sufficient or whether explicit transport-path fields are worth adding.

**Files:**
- Maybe modify: `internal/config/mcp.go`
- Maybe modify: `internal/config/mcp_test.go`

**Step 1: Make the default decision**

Default recommendation: do not add new env vars in this pass unless there is a real requirement.

Reasoning:
- endpoint paths are straightforward and stable
- Jeffrey is handling deployment separately
- fewer knobs means less documentation drift

**Step 2: If no config changes are needed**

Leave config untouched and note in docs that the binary exposes:
- `/mcp`
- `/mcp/sse`
- `/mcp/messages`

**Step 3: If config changes are needed**

Only then add fields like:
- `MCP_STREAMABLE_HTTP_ENDPOINT`
- `MCP_SSE_ENDPOINT`
- `MCP_MESSAGE_ENDPOINT`

But avoid this unless tests or deployment assumptions force it.

**Step 4: Run config tests if touched**

Run:
```bash
go test ./internal/config -v
```

Expected: PASS.

**Step 5: Commit if touched**

```bash
git add internal/config/mcp.go internal/config/mcp_test.go
git commit -m "refactor: extend mcp config for dual transports"
```

### Task 8: Update the README to advertise both transport options

**Objective:** Make the repository landing page accurately describe the MCP surface.

**Files:**
- Modify: `README.md`

**Step 1: Update the feature list**

Change the MCP bullet from SSE-only wording to dual-transport wording, for example:

```md
- **MCP server** — separate remote MCP server exposing both StreamableHTTP (`/mcp`) and legacy SSE (`/mcp/sse`) transports
```

**Step 2: Update project structure text**

Change `cmd/mcp` and `internal/mcpserver` descriptions so they no longer say SSE-only.

**Step 3: Update docs links section if needed**

Make sure the MCP doc link copy says both StreamableHTTP and SSE.

**Step 4: Verify README references are accurate**

Run:
```bash
grep -n "SSE\|streamable\|cmd/mcp\|mcpserver" README.md
```

Expected: wording reflects both transports.

**Step 5: Commit**

```bash
git add README.md
git commit -m "docs: document dual mcp transport support in readme"
```

### Task 9: Rewrite `docs/MCP.md` around transport-aware client setup

**Objective:** Make the MCP guide the authoritative transport document for operators and clients.

**Files:**
- Modify: `docs/MCP.md`

**Step 1: Update the opening summary**

Replace SSE-only language with:
- StreamableHTTP as the modern endpoint for Hermes/native HTTP MCP clients
- SSE retained for compatibility with existing agent clients/configs

**Step 2: Update the architecture diagram**

Change the transport line to something like:

```text
Agent Client ──StreamableHTTP or SSE──► MCP Server (cmd/mcp) ──HTTP──► Main Proxy
```

**Step 3: Update the endpoints table**

Document:
- `GET /mcp`
- `POST /mcp`
- `DELETE /mcp`
- `GET /mcp/sse`
- `POST /mcp/messages`
- `GET /healthz`

Include a short note on intended use:
- `/mcp` for Hermes/native StreamableHTTP clients
- `/mcp/sse` + `/mcp/messages` for legacy SSE clients

**Step 4: Add Hermes configuration example**

Document a native Hermes config block similar to:

```yaml
mcp_servers:
  search:
    url: "https://search.vandenborne.co/mcp"
```

Also include a local example:

```yaml
mcp_servers:
  search:
    url: "http://127.0.0.1:8090/mcp"
```

**Step 5: Keep existing SSE client examples**

Retain OpenCode / Claude Code / Gemini examples, but label them as SSE compatibility examples.

**Step 6: Add smoke tests**

Document:

```bash
curl http://127.0.0.1:8090/healthz
curl -i -H 'Accept: application/json, text/event-stream' \
  -H 'Content-Type: application/json' \
  -d '{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2025-03-26","capabilities":{},"clientInfo":{"name":"smoke-test","version":"1.0.0"}}}' \
  http://127.0.0.1:8090/mcp
curl -N http://127.0.0.1:8090/mcp/sse
```

**Step 7: Commit**

```bash
git add docs/MCP.md
git commit -m "docs: add streamable http mcp usage guide"
```

### Task 10: Update architecture documentation

**Objective:** Reflect the new transport model in the architecture doc.

**Files:**
- Modify: `docs/ARCHITECTURE.md`

**Step 1: Expand package responsibilities**

Add `cmd/mcp`, `internal/mcpserver`, and `internal/synthproxy` explicitly to the package table.

**Step 2: Add MCP transport flow**

Include a second diagram or subsection showing:
- shared `search` tool registration
- transport fan-out to SSE and StreamableHTTP
- upstream dependency on `/v1/search`

Suggested text:

```md
The MCP wrapper is a separate binary that reuses the same tool handlers across two remote transports: StreamableHTTP on `/mcp` and compatibility SSE on `/mcp/sse` + `/mcp/messages`.
```

**Step 3: Document the transport choice**

Add a short tradeoff section:
- StreamableHTTP is the preferred modern endpoint for Hermes
- SSE remains for compatibility with tools that still expect it
- both transports share one MCP server instance and one search tool surface

**Step 4: Commit**

```bash
git add docs/ARCHITECTURE.md
git commit -m "docs: update architecture for dual mcp transports"
```

### Task 11: Add an ADR for dual transport support

**Objective:** Record why the project now supports both SSE and StreamableHTTP instead of replacing one with the other.

**Files:**
- Create: `docs/adr/0004-dual-mcp-transports.md`

**Step 1: Write the ADR**

Structure:
- Status: Accepted
- Date: 2026-04-09
- Context:
  - current repo ships SSE-only
  - Hermes native MCP expects HTTP / StreamableHTTP
  - existing clients already use `/mcp/sse`
- Decision:
  - expose `/mcp` via StreamableHTTP
  - retain `/mcp/sse` + `/mcp/messages`
  - keep one MCP tool server and two transport adapters
- Consequences:
  - better client compatibility
  - slightly more transport testing/docs burden
  - no breaking change for existing SSE users

**Step 2: Link from surrounding docs if the repo convention does that**

Check whether README or architecture docs mention ADRs directly; if not, no extra linking needed.

**Step 3: Commit**

```bash
git add docs/adr/0004-dual-mcp-transports.md
git commit -m "docs: add adr for dual mcp transport support"
```

### Task 12: Decide whether `docs/SECURITY.md` needs an update

**Objective:** Only touch the security doc if the change genuinely affects the security model.

**Files:**
- Maybe modify: `docs/SECURITY.md`

**Step 1: Evaluate impact**

Update security docs only if implementation adds material new behavior such as:
- session semantics worth documenting
- long-lived GET stream handling on `/mcp`
- new reverse-proxy header assumptions
- security-sensitive transport differences between SSE and StreamableHTTP

**Step 2: If updated, keep it narrow**

Suggested additions if needed:
- StreamableHTTP and SSE are both unauthenticated remote transports
- reverse proxy should rate-limit and gate access identically for both `/mcp` and `/mcp/sse`
- session-oriented transports do not add server-side trust of client data

**Step 3: Run a quick docs consistency pass**

Run:
```bash
grep -Rni "SSE\|streamable\|/mcp" docs README.md
```

Expected: no stale SSE-only claims remain except where intentionally marked compatibility-only.

**Step 4: Commit if touched**

```bash
git add docs/SECURITY.md
git commit -m "docs: update security guidance for dual mcp transports"
```

### Task 13: Run the full verification suite

**Objective:** Verify the code and docs are internally consistent before handoff.

**Files:**
- No source changes required unless issues are found

**Step 1: Run focused tests first**

```bash
go test ./internal/mcpserver -v
go test ./cmd/mcp -v
go test ./internal/config -v
go test ./internal/synthproxy -v
```

Expected: PASS.

**Step 2: Run full repository tests**

```bash
go test ./...
```

Expected: PASS.

**Step 3: Manual smoke test**

Run the MCP binary locally against a fake or real proxy:

```bash
MCP_PROXY_BASE_URL=http://127.0.0.1:8080 go run ./cmd/mcp
```

Then from another shell:

```bash
curl http://127.0.0.1:8090/healthz
curl -N http://127.0.0.1:8090/mcp/sse
curl -i -H 'Accept: application/json, text/event-stream' \
  -H 'Content-Type: application/json' \
  -d '{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2025-03-26","capabilities":{},"clientInfo":{"name":"manual-test","version":"1.0.0"}}}' \
  http://127.0.0.1:8090/mcp
```

Expected:
- `/healthz` returns 200 with `{"status":"ok"}`
- `/mcp/sse` returns an event stream
- `/mcp` returns a valid initialize response

**Step 4: Final commit**

```bash
git add .
git commit -m "feat: add streamable http mcp endpoint"
```

---

## Expected final deliverables

1. Existing SSE clients continue working unchanged.
2. Hermes can be configured against a StreamableHTTP endpoint at `/mcp`.
3. The MCP binary serves both transports from the same process.
4. Docs clearly distinguish preferred `/mcp` usage from compatibility `/mcp/sse` usage.
5. Architecture and ADR docs explain why both transports exist.
6. Security docs remain untouched unless implementation reveals a genuine transport-specific security concern.

## Handoff note

Once implemented, the user-facing answer should explicitly include:
- the new endpoint: `https://search.vandenborne.co/mcp`
- confirmation that SSE remains available at `https://search.vandenborne.co/mcp/sse`
- any doc files updated
- any follow-up deployment caveats Jeffrey should handle at the reverse proxy layer
