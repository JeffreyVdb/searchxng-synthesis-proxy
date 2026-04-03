# Review: MCP SSE server

## Assessment

Needs changes before merge.

The overall shape is good: the MCP wrapper is kept separate from the main proxy, config is split cleanly, and the upstream HTTP client is isolated as planned. I found three non-trivial issues that should be fixed before this branch is merged.

## Findings

### 1. Startup failures are swallowed, so the process can hang instead of exiting
- **Severity:** high
- **File:** `cmd/mcp/main.go:52-58`

**Context**

`run()` starts `srv.ListenAndServe()` in a goroutine, logs any error, and then blocks on `<-ctx.Done()`.

```go
go func() {
    if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
        logger.Error("server error", slog.String("error", err.Error()))
    }
}()

<-ctx.Done()
```

**Why this is a problem**

If `ListenAndServe()` fails immediately (for example: port already in use, bad listener setup, permission issue), the goroutine only logs the error. `run()` does not return that failure, so the process keeps waiting for a signal forever.

In practice this means systemd / container startup can look "alive" even though the server never bound its port.

**Fix**

Propagate serve errors back to `run()` instead of only logging them. Typical pattern:

- create `serveErrCh := make(chan error, 1)`
- in the goroutine, send non-`http.ErrServerClosed` errors into the channel
- `select` between `ctx.Done()` and `serveErrCh`
- if a serve error arrives first, return it immediately

That gives operators a hard startup failure instead of a stuck process.

---

### 2. Raw upstream transport errors are returned to MCP clients
- **Severity:** medium
- **Files:**
  - `internal/mcpserver/server.go:69-76`
  - `internal/synthproxy/client.go:67-101`

**Context**

The upstream client wraps low-level failures with raw transport details:

```go
return nil, fmt.Errorf("request to upstream: %w", err)
```

The MCP tool handler then exposes that raw error text to the caller:

```go
return mcp.NewToolResultError(fmt.Sprintf("search failed: %s", err.Error())), nil
```

**Why this is a problem**

For connection failures / timeouts / malformed upstream responses, remote MCP callers will receive internal details such as:

- internal hostnames / ports
- full upstream URLs
- low-level dial / timeout errors

That is both a security/privacy leak and an unstable public error contract. The plan explicitly called for surfacing upstream failures cleanly.

**Fix**

Introduce a typed upstream error model with:

- a safe client-facing message/code
- the original wrapped cause for logs only

For example:

- transport failure → `search upstream unavailable`
- malformed upstream success payload → `search upstream returned an invalid response`
- proxied JSON error payloads can still preserve upstream `code`/`message`

Then in the MCP handler:

- log the full wrapped error
- return only the sanitized/public message to the tool caller

---

### 3. The OpenCode configuration example is incorrect for current remote MCP config
- **Severity:** medium
- **File:** `docs/MCP.md:98-128`

**Context**

The docs currently show:

```json
{
  "mcp": {
    "servers": {
      "search": {
        "url": "http://127.0.0.1:8090/mcp/sse",
        "transport": "sse"
      }
    }
  }
}
```

**Why this is a problem**

OpenCode's current config schema uses per-server entries directly under `mcp` with `type: "remote"` and `url`, not `mcp.servers` plus `transport`.

This is the primary target client called out in the plan, so operators following the docs will configure OpenCode incorrectly and fail to connect.

**Fix**

Replace the OpenCode example with the current schema, e.g.:

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

Also verify the example against a real OpenCode install (or at minimum against the current docs) before merging, since OpenCode is the primary target for this feature.
