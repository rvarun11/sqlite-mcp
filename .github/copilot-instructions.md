# sqlite-mcp Copilot Instructions

## Project Overview

`sqlite-mcp` is a Go application that implements an [MCP (Model Context Protocol)](https://modelcontextprotocol.io) server for SQLite databases. It exposes four tools that allow AI agents to open databases, inspect schema, run read-only queries, and execute write operations. The server communicates exclusively over **stdio** (stdin/stdout).

**Module:** `github.com/rvarun11/sqlite-mcp`  
**Go version:** 1.24.4  
**CGO required:** yes (due to `mattn/go-sqlite3`)

---

## Repository Structure

```
sqlite-mcp/
├── cmd/server/main.go          # Entry point: CLI wiring, tool registration, server startup
├── internal/
│   ├── config/config.go        # Config struct and optional CLI flag validation
│   ├── handlers/
│   │   ├── mcp.go              # MCP tool handler implementations and response formatters
│   │   └── mcp_test.go         # Handler integration tests
│   ├── logger/logger.go        # Zap logger factory (prod + test variants)
│   ├── models/table.go         # Domain structs: Table, Column, Index, ForeignKey, QueryResult, ExecuteResult
│   └── repository/
│       ├── repository.go       # Repository interface
│       ├── sqlite.go           # SQLite implementation using database/sql
│       └── sqlite_test.go      # Repository unit tests
├── example.sql                 # Seed SQL for the bundled example database
├── Taskfile.yml                # Task runner (go-task)
└── Dockerfile                  # Multi-stage Docker build (CGO_ENABLED=1)
```

---

## Architecture

### Layer Responsibilities

| Layer | Package | Responsibility |
|---|---|---|
| CLI / wiring | `cmd/server` | Parse flags, construct dependencies, register MCP tools, start stdio server |
| Config | `internal/config` | Validate optional `--database` path and `--debug` flag |
| Handlers | `internal/handlers` | Translate MCP `CallToolRequest` → repository call → `CallToolResult` text |
| Repository | `internal/repository` | All database interaction; enforce read/write split; concurrent-safe DB swapping |
| Models | `internal/models` | Shared data structs passed between repository and handlers |
| Logger | `internal/logger` | `zap.SugaredLogger` factory used by all layers |

### Request Flow

```
MCP Client (stdio)
  → server.ServeStdio (mcp-go)
  → MCPHandler.{OpenDatabase,GetSchema,Query,Execute}
  → SQLiteDB.{Open,GetSchema,Query,Execute}
  → database/sql + mattn/go-sqlite3
```

---

## MCP Tools

All four tools are registered in `cmd/server/main.go` and implemented in `internal/handlers/mcp.go`.

### `open_database`
- **Purpose:** Open (or switch to) a SQLite database file, closing any existing connection.
- **Parameters:** `path` (string, required) — absolute path to an existing `.db` file.
- **Annotations:** `ReadOnly=false`, `Destructive=false`, `Idempotent=false`
- **Handler:** `MCPHandler.OpenDatabase`
- **Behaviour:**
  - Rejects concurrent `open_database` calls immediately with `IsError=true`.
  - Waits for all in-flight `get_schema`/`query`/`execute` calls to finish before swapping.
  - Rejects paths that do not point to an existing file (will not create new databases).
  - On success returns `"Opened database at <path>\n\n"` followed by the full schema output.
  - Must be called before any other tool when `--database` is not passed at startup.

### `get_schema`
- **Purpose:** List all user tables with columns, indexes, and foreign keys.
- **Parameters:** none
- **Annotations:** `ReadOnly=true`, `Destructive=false`, `Idempotent=true`
- **Handler:** `MCPHandler.GetSchema` — calls `repo.GetSchema()`, formats via `formatTablesResponse()`
- Returns `IsError=true` with `"No database is open. Call open_database first."` when no DB is open.

### `query`
- **Purpose:** Execute read-only SQL (`SELECT`, `WITH`, `EXPLAIN` only).
- **Parameters:** `sql` (string, required, 1–10 000 chars)
- **Annotations:** `ReadOnly=true`, `Destructive=false`, `Idempotent=true`
- **Handler:** `MCPHandler.Query` — validates param, calls `repo.Query()`, formats via `formatQueryResponse()`. Output is capped at **10 rows** to avoid token overflow.
- Returns `IsError=true` with `"No database is open."` when no DB is open.

### `execute`
- **Purpose:** Run DDL/DML (`INSERT`, `UPDATE`, `DELETE`, `CREATE`, `ALTER`, `DROP`, etc.).
- **Parameters:** `sql` (string, required, 1–10 000 chars)
- **Annotations:** `ReadOnly=false`, `Destructive=true`, `Idempotent=false`
- **Handler:** `MCPHandler.Execute` — validates param, calls `repo.Execute()`, formats via `formatExecuteResponse()`. Returns rows affected and last insert ID.
- Returns `IsError=true` with `"No database is open."` when no DB is open.

> `SELECT`/`WITH`/`EXPLAIN` are blocked in `execute`; write statements are blocked in `query`. Enforcement is in `repository/sqlite.go:isSelectQuery()`.

---

## Key Conventions

### Error Handling

- **Repository layer** (`sqlite.go`): Log full error detail internally; return sanitised, generic error strings to callers via `fmt.Errorf`. Do not expose raw DB errors upward.
- **Handler layer** (`mcp.go`): **Never return a Go `error`** from handler methods. Instead, return `mcp.NewToolResultError(message)` (`IsError: true`) so the MCP client receives a structured error. A Go `error` would abort the tool call entirely.
- **Startup** (`main.go`): Fatal errors before the server starts use `logger.Fatalf` or `fmt.Fprintf(os.Stderr, …) + os.Exit(1)`.

### Concurrency in `MCPHandler`

`MCPHandler` uses two synchronisation primitives for safe database swapping:

| Field | Type | Purpose |
|---|---|---|
| `gate` | `sync.RWMutex` | Data handlers (`get_schema`, `query`, `execute`) hold a **read lock** for their entire lifetime. `open_database` holds the **write lock** during the swap, which blocks until all active readers finish and prevents new ones from starting. |
| `opening` | `atomic.Int32` | CAS flag (0/1) — a second concurrent `open_database` call fails immediately with `IsError=true` without blocking on the write lock. |

Every data handler acquires `h.gate.RLock()` at the top and defers `h.gate.RUnlock()`.

### Repository Concurrency (`SQLiteDB`)

`SQLiteDB` wraps `*sql.DB` with a `sync.RWMutex`:

- `GetSchema`, `Query`, `Execute` — acquire a **read lock** to snapshot `db`, then release it before doing I/O. This allows concurrent data operations.
- `Open`, `Close` — acquire the **write lock** only to swap the pointer, not for the duration of the I/O.

### Logging

- All layers receive a `*zap.SugaredLogger` via constructor injection.
- Logs go to **stderr** only — never stdout — to avoid corrupting the MCP stdio stream.
- Use `s.logger.Debugf(…)` for per-request detail (enabled only with `--debug`).
- SQL is truncated to 100 chars before logging (see `sanitizeQuery()` in `sqlite.go`).

### Adding a New MCP Tool

1. **Define the domain model** in `internal/models/table.go` if new data structures are needed.
2. **Add a repository method** to the `Repository` interface (`repository/repository.go`) and implement it on `*SQLiteDB` (`repository/sqlite.go`). Guard against `db == nil` and snapshot `db` under `s.mu.RLock()`.
3. **Add a handler method** on `*MCPHandler` in `internal/handlers/mcp.go`. Follow the existing pattern:
   - For data-reading/writing handlers: call `h.gate.RLock()` and defer `h.gate.RUnlock()`.
   - Extract and validate params using `argsMap(request)` (safe comma-ok assertion), then assert individual fields with the comma-ok form or zero-value fallback.
   - Call the repository method; check for `"no database is open"` error specifically.
   - On error, return `mcp.NewToolResultError(…)`, not a Go error.
   - On success, format a string response and return `mcp.NewToolResultText(…)`.
4. **Add a private formatter** (`formatXxxResponse()`) in `mcp.go` if the response has structure.
5. **Register the tool** in `cmd/server/main.go` using `mcp.NewTool(…)` and `mcpServer.AddTool(tool, handler)`. Set `ReadOnly`, `Destructive`, and `Idempotent` annotations accurately.
6. **Write tests** in `handlers/mcp_test.go` covering: success, missing/empty param, invalid SQL, no-database-open case, and edge cases. Use `textFromResult(t, result)` to extract text from `mcp.CallToolResult.Content[0]` — the library stores `mcp.TextContent` as a **value type**, not a pointer.

### Repository Guidelines

- Use `database/sql` with parameterized queries for any user-supplied values. PRAGMA queries that use table names from `sqlite_master` are acceptable without parameterization (names are trusted).
- Read/write enforcement must go through `isSelectQuery()` — do not duplicate the logic.
- Always log at the repository level before returning errors.
- Use `fmt.Errorf("context: %w", err)` for error wrapping.
- Always check `db == nil` (no database open yet) and return `fmt.Errorf("no database is open")`.

### Response Formatting

- All tool responses are **plain text strings** (not JSON). Use `mcp.NewToolResultText(text)`.
- Format helpers live at the bottom of `mcp.go` and follow a consistent multiline text style (see `formatTablesResponse`, `formatQueryResponse`, `formatExecuteResponse`).
- Keep responses concise — LLM context is finite. Truncate large result sets.
- `mcp.TextContent` is a **value type** in `mcp-go`. Type-assert as `result.Content[0].(mcp.TextContent)`, never `*mcp.TextContent`.

---

## Constructors

| Function | When to use |
|---|---|
| `repository.NewSQLiteDB(logger)` | Create a repository with no database open yet (use when `--database` is omitted). |
| `repository.NewSQLiteDBFromPath(path, logger)` | Create and immediately open a database (used in startup and tests). |
| `handlers.NewMCPHandler(repo, logger)` | `repo` must be a `repository.Repository` interface, not `*SQLiteDB` directly. |

---

## Dependencies

| Package | Purpose |
|---|---|
| `github.com/mark3labs/mcp-go` | MCP protocol: tool definitions, server, stdio transport |
| `github.com/mattn/go-sqlite3` | SQLite3 driver (CGO) for `database/sql` |
| `github.com/spf13/cobra` | CLI flag parsing |
| `go.uber.org/zap` | Structured logging |

> `go-sqlite3` requires `CGO_ENABLED=1` and a C compiler (`gcc`). The Dockerfile installs `gcc` and `musl-dev` for the Alpine build stage.

---

## Testing

Tests use a **real SQLite temp file** — no mocks. The `Repository` interface enables mock injection if needed in future.

```bash
go test ./...       # run all tests
task test           # via task runner
```

- `internal/repository/sqlite_test.go` — white-box repository tests; `setupTestDB()` creates a temp `.db` file per test using `NewSQLiteDBFromPath`.
- `internal/handlers/mcp_test.go` — handler integration tests:
  - `setupTestMCPHandler()` — seeds `users` + `orders` tables with FK relationships.
  - `setupHandlerNoDB()` — returns a handler with no database open (for `open_database` tests).
- Use `logger.NewTestLogger()` in all test setups (suppresses log output at PanicLevel).
- Use `textFromResult(t, result)` helper to safely extract text from tool results.

---

## Build & Run

```bash
# Format, lint, test
task check

# Build binary (outputs to build/sqlite-mcp)
task build

# Run from source with --database (database opened at startup)
go run cmd/server/main.go --database ./build/example.db

# Run from source without --database (open_database must be called first)
go run cmd/server/main.go

# Docker
task docker-build
docker run -i --rm sqlite-mcp-server

# Docker with custom schema
docker run -i --rm -v "/path/to/schema.sql:/data/schema.sql" sqlite-mcp-server
```

### MCP Client Configuration

With a pre-specified database:
```json
{
  "mcpServers": {
    "sqlite": {
      "command": "/absolute/path/to/build/sqlite-mcp",
      "args": ["--database", "/absolute/path/to/database.db"]
    }
  }
}
```

Without a pre-specified database (agent calls `open_database` first):
```json
{
  "mcpServers": {
    "sqlite": {
      "command": "/absolute/path/to/build/sqlite-mcp"
    }
  }
}
```

Use **absolute paths** — MCP GUI clients do not expand `~` or `$HOME`.

---

## Known Limitations & TODOs

- **Stdio transport only.** SSE/HTTP transport is a noted TODO in `main.go`.
- **No parameterized user queries.** User SQL is passed directly to `database/sql`. The only guard is the `isSelectQuery` prefix check. This is intentional (the tool gives LLMs raw database access) but should be understood when expanding the tool.
- **10-row display cap** in `formatQueryResponse` (`mcp.go`). Increase if needed, but keep LLM token limits in mind.
- **`sanitizeQuery`** is a stub (truncate only) with a `// TODO` for more advanced sanitization.
- **`open_database` does not create new files.** The path must point to an existing file. To create a new database, create the file first via the filesystem, then call `open_database`.
