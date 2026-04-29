# maclawbot — AGENTS.md

## Project Overview

**MAClawBot** (v2.1.0) — multi-agent WeChat proxy. Polls iLink for messages, routes to per-agent HTTP proxy servers. Agents (Hermes, OpenClaw, Claude Code, etc.) connect to their proxy ports and think they're talking directly to iLink.

## Build & Test

```bash
# Build
go build -o maclawbot ./cmd/maclawbot

# Test (all packages)
go test ./...

# Test with race detector
go test -race ./...

# Vet
go vet ./...

# Build + test + vet (full check)
go build ./... && go vet ./... && go test ./...
```

## Code Conventions

| Rule | Detail |
|------|--------|
| **Thread safety** | Use `sync.RWMutex` for shared state. Lock before writing maps/slices; RLock before reading. |
| **Atomic saves** | State: write to `*.tmp`, then `os.Rename`. Never write directly to the state file. |
| **Exponential backoff** | Poll loop backs off after consecutive failures (3 fails → 30s backoff). Don't silently retry on error. |
| **Proxy endpoints allowlist** | `proxy.go:allowedEndpoints` — only these endpoints are forwarded from agents to iLink. |
| **No blocking in ServeHTTP** | Proxy handlers must return quickly. Long operations (queue wait) use non-blocking patterns. |
| **Graceful shutdown** | Always use `context.WithTimeout` + `srv.Shutdown()` on HTTP servers. No `os.Exit` in library code. |

## Architecture

```
iLink API
    │
MAClawBot (polls, routes)
    │
    ├── router/       — message parsing, command handling, state management
    ├── proxy/        — per-agent HTTP proxy + message queue
    ├── ilink/        — iLink HTTP client (send, poll)
    └── config/       — env var loader (singleton)

Agents connect to 127.0.0.1:<port> — NOT to iLink directly.
```

### Key data flows

- **Incoming:** iLink → `pollLoop` → `procMsg` → `/clawbot` command or → `queue.Enqueue(msg)` → Agent
- **Outgoing:** Agent → proxy handler → `forwardToILink` → iLink

### Message types
`router.MessageType`: 1=text, 2=image, 3=voice, 4=video, 5=file

## Testing Guidelines

| Scenario | Coverage needed |
|----------|----------------|
| Mutating state | Test read-your-own-write (file-based) |
| Concurrent access | `go test -race` must pass |
| Queue overflow | Oldest message dropped at 200 capacity |
| Graceful stop | Server port released after `StopAgent` |
| Cleanup on delete | `handleAgentChange` must call `OnAgentRemoved` |

## Commit Convention

```
type: short description

- bullet of what changed
- bullet of what changed
```

Types: `fix:`, `feat:`, `chore:`, `docs:`. Keep the first line under 72 chars.

## Common Tasks

### Add a new iLink endpoint
1. Add to `allowedEndpoints` in `proxy.go`
2. If proxy passthrough needed: update `handleSendMessage` / `proxyPassthrough` switch
3. If response parsing needed: add types to `router/message.go`

### Add agent command
1. Add to `processClawbotCommand` switch in `router/message.go`
2. Add test in `router/message_test.go`

### Change poll behavior
1. `cmd/maclawbot/main.go:pollLoop` — main loop
2. `internal/ilink/client.go:GetUpdates` — HTTP call
3. Backoff logic: `main.go:handleFailure`

## Known Quirks

- Welcome message author name uses `@ian4hu` in `procMsg` but `@AaronYonW` was used in the deleted `formatWhoami` — clarify if editing