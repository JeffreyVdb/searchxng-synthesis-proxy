# ADR 0001: MCP SSE Wrapper Architecture

**Status:** Accepted

**Date:** 2026-04-03

## Context

The search synthesis proxy needs to expose its search capability to agent clients (OpenCode, Claude Code, Gemini CLI, etc.) via the Model Context Protocol (MCP). The primary transport requirement is Server-Sent Events (SSE).

Two approaches were considered:

1. **In-process MCP mode** — add MCP SSE endpoints directly to the main proxy binary
2. **Separate MCP wrapper** — a second binary that calls the existing proxy API over HTTP

## Decision

We will build a **separate MCP wrapper binary** under `cmd/mcp/` that treats the existing `/v1/search` API as its upstream contract.

### Consequences

- The search API stays stable and independently testable.
- The MCP layer stays thin and easy to reason about.
- MCP client bugs cannot directly tangle with the SearXNG/LLM pipeline.
- The MCP server can be deployed next to, or separately from, the main service.
- MCP-specific config does not require main-service-only secrets like `LLM_API_KEY`.

## Transport

SSE is the initial transport because it is explicitly required and widely supported by agent tooling.

The implementation uses the `mark3labs/mcp-go` SDK to handle JSON-RPC and SSE protocol details. Tool registration and search client logic are kept independent from transport wiring so the project can later add streamable-HTTP endpoints without rewriting tool logic.

### SSE Endpoints

- `GET  /mcp/sse` — SSE connection endpoint
- `POST /mcp/messages` — JSON-RPC message endpoint
- `GET  /healthz` — health check

## Tool Surface

A single `search` tool is exposed:

- **Input:** `query` (required string)
- **Output:** Human-readable text (answer + numbered sources) plus structured content mirroring the upstream JSON payload

## Config Split

The existing `internal/config` package now provides:

- `LoadProxy()` — for the main service (requires `LLM_API_KEY`, etc.)
- `LoadMCP()` — for the MCP wrapper (requires `MCP_PROXY_BASE_URL` only)

The MCP config is intentionally minimal: it only needs to know where the upstream proxy lives and how to serve its own SSE endpoints.

## Upstream Client

A dedicated `internal/synthproxy` package handles HTTP communication with the main proxy, decoupled from internal service types. This ensures the MCP wrapper depends on the **remote API contract**, not the in-process implementation.
