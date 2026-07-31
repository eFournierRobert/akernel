# akernel

Go CLI backend that proxies LLMs (Ollama and OpenRouter) with conversation persistence in SQLite.

## Agent role

Your primary responsibilities are:

1. **Write documentation** — add `go doc`-compatible comments for all public and private types, functions, and methods across the codebase as well as package level docs.
2. **Write unit tests** — when asked, use table-driven tests with the standard `testing` package. `github.com/stretchr/testify` is already a dependency. Be thorough and harsh: cover edge cases, error paths, nil inputs, boundary values, and failure scenarios — never test only the happy path. After writing tests, summarize any code changes that would benefit from refactoring and list which test failures could be resolved by those improvements.

**Never modify production code unless explicitly told to do so.** If production changes are suggested during review, note them in writing but leave the source untouched.

## Build and run

```
go run ./cmd/main.go              # production mode
go run ./cmd/main.go -dev         # dev mode (DB at ./database/database.db)
```

- Requires **CGO** (uses `github.com/mattn/go-sqlite3`).
- HTTP server binds to **0.0.0.0:50500** (reachable from the LAN; intentional, no auth).
- In production, the DB lives at `$XDG_DATA_HOME/akernel/akernel.db`. Migrations run on every startup via goose.

## Architecture

Three-layer structure under `internal/`:

```
cmd/main.go                  → single entrypoint, flag parsing, mux setup
internal/handlers/           → HTTP handlers (chat, conversation, model)
internal/service/chat/        → business logic for chat orchestration + tool-call dispatch
internal/service/conversation/→ conversation summary helpers
internal/service/inference/    → LLM request/response abstraction over Ollama and OpenRouter; hosts inference/tools
  └── tools/                  → function-calling tool schemas and handlers (get_current_time, fetch_webpage, web_search)
internal/repository/         → sqlx DB access
migrations/                  → goose SQL migrations (run automatically on startup)
```

The `chat` service now selects messages by row ID rather than matching prompt text — callers pass the int64 message ID returned from `InsertMessage`. The inference layer is what chat talks to instead of Ollama directly.

Tests use table-driven and mock-based patterns from the standard `testing` package with `github.com/stretchr/testify`. A sample test invocation:

```
go test ./internal/service/model/...
```

No CI, linter, or formatter config configured. Standard gofmt applies.

## External dependencies and configuration

Ollama-backed models call **Ollama** at `http://localhost:11434/api`; OpenRouter-backed models call the OpenRouter API. All provider calls are routed through the inference layer (`internal/service/inference/`) rather than directly from business logic. Neither provider is contacted at startup — failures only surface at request time.

Startup configuration is loaded by `internal/config/`:

- `$XDG_CONFIG_HOME/akernel/config.toml` — **required**; `main()` will `log.Fatal` without it. Declares `models` (name, id, description, provider), plus optional `brave_api_key` and `open_router_key`.
- `$XDG_CONFIG_HOME/akernel/system.md` — optional system prompt. If missing, the system prompt is silently empty (no error).

## Tool calls

Function-calling tools live in `internal/service/inference/tools/` and are provider-agnostic (the same schemas are advertised to both Ollama and OpenRouter models). The advertised set is gated by config: `web_search` is only included when `brave_api_key` is set.

Adding a new tool requires: schema function + handler function + registration in `GetAllToolSchemas()` and `ToolCallRouter()`.

All tools should include go-doc comments for handler functions, argument types (if complex), and schema functions following the pattern used in existing tools.

## Database

SQLite with foreign keys enabled. Migrations auto-run on startup via goose. Schema lives in `migrations/*.sql`. To add a migration, write a new timestamped `.sql` file in `migrations/` — no manual command needed since it's invoked at startup.

The `/database/` directory is git-ignored and holds the dev DB file.

## Architecture notes

This is intended as a single-user local daemon (e.g., systemd service). Do not add concurrency safeguards, multi-tenant logic, or auth beyond what's already present.

## Quirks

- Go module name is `akernel` (not `locassist`).
- Module requires Go 1.26+ (`go.mod` declares `go 1.26.3`).
