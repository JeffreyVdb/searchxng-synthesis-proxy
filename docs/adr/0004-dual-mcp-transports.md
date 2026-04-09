# ADR 0004: Support Dual MCP Transports

**Status:** Accepted

**Date:** 2026-04-09

## Context

The repository originally shipped an MCP wrapper that exposed the `search` tool over SSE only.

That was sufficient for early remote MCP clients, but the transport landscape changed:

- modern native MCP clients such as Hermes prefer StreamableHTTP
- existing users and examples already rely on `/mcp/sse` and `/mcp/messages`
- replacing SSE outright would create an unnecessary breaking change
- the MCP implementation already keeps tool registration separate from transport wiring

## Decision

Expose one shared MCP server through two transports:

- StreamableHTTP on `/mcp`
- SSE compatibility endpoints on `/mcp/sse` and `/mcp/messages`

The `cmd/mcp` binary remains the transport adapter. `internal/mcpserver` continues to own tool registration and now mounts both transport adapters against the same MCP server instance.

## Why not replace SSE outright

1. Existing OpenCode, Claude Code, and Gemini CLI setups already use the SSE endpoints.
2. StreamableHTTP support is the direction we want for Hermes and other native MCP clients, but client support is not uniform yet.
3. The code can support both transports without duplicating business logic or widening the trust boundary.
4. Keeping SSE avoids a migration-only release with no user-facing search improvement.

## Consequences

- `/mcp` becomes the preferred endpoint to document for modern clients.
- `/mcp/sse` and `/mcp/messages` remain stable compatibility endpoints.
- Documentation must clearly separate preferred StreamableHTTP guidance from legacy or compatibility SSE examples.
- Reverse proxies and access controls should treat both transports the same because they expose the same tool surface.
