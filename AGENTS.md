# akernel

Go CLI backend that proxies an LLM (Ollama) with conversation persistence in SQLite.

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
- HTTP server listens on **localhost:50500**.
- In production, the DB lives at `$XDG_DATA_HOME/akernel/akernel.db`. Migrations run on every startup via goose.

## Architecture

Three-layer structure under `internal/`:

```
cmd/main.go          → single entrypoint, flag parsing, mux setup
internal/handlers/   → HTTP handlers (chat, conversation, model)
internal/service/    → business logic + Ollama client + tool definitions
internal/repository/ → sqlx DB access
migrations/          → goose SQL migrations (run automatically on startup)
```

Tests use table-driven and mock-based patterns from the standard `testing` package with `github.com/stretchr/testify`. A sample test invocation:

```
go test ./internal/service/model/...
```

No CI, linter, or formatter config configured. Standard gofmt applies.

## External dependency: Ollama

The service calls **Ollama** at `http://localhost:11434/api`. It must be running before the app starts, otherwise `main()` will `log.Fatal`.

System prompt is loaded from `$XDG_CONFIG_HOME/akernel/system.md` at startup. If the file doesn't exist (and `-dev` isn't set to skip), the app will fail to start.

## Tool calls

Ollama function-calling tools live in `internal/service/ollama/tools/`. Available:
- `get_current_time` — returns current time string
- `write_to_file` — writes content to `$XDG_DATA_HOME/akernel/<filename>`

Adding a new tool requires: schema function + handler function + registration in `GetAllToolSchemas()` and `ToolCallRouter()`.

## Database

SQLite with foreign keys enabled. Migrations auto-run on startup via goose. Schema lives in `migrations/*.sql`. To add a migration, write a new timestamped `.sql` file in `migrations/` — no manual command needed since it's invoked at startup.

The `/database/` directory is git-ignored and holds the dev DB file.

## Architecture notes

This is intended as a single-user local daemon (e.g., systemd service). Do not add concurrency safeguards, multi-tenant logic, or auth beyond what's already present.

## Quirks

- Go module name is `akernel` (not `locassist`).
- Module requires Go 1.26+ (`go.mod` declares `go 1.26.3`).
