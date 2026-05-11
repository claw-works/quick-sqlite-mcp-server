# quick-sqlite-mcp-server

A SQLite MCP (Model Context Protocol) server for **[Amazon Quick](https://docs.aws.amazon.com/quick/latest/userguide/amazon-quick-desktop.html)** (desktop) and other MCP-compatible AI tools. Runs as a Docker container with your database files mounted in — no SQLite bindings needed in the client.

> **What is Amazon Quick?** Amazon Quick is a native desktop AI assistant for macOS and Windows that supports MCP servers as custom tool integrations. This server gives Quick the ability to read and write SQLite databases directly from chat.

## Features

- 🗄️ **Multi-database**: every tool accepts a `db` parameter — operate on as many databases as you need
- 🔒 **Path safety**: relative paths resolve against `ROOT_DIR`; absolute paths require opt-in
- 💾 **Transactions**: explicit `begin / commit / rollback` with 5-minute auto-expiry
- 🐳 **Docker-first**: mount any directory, zero host dependencies
- ⚡ **WAL mode**: all databases opened with WAL journal for better concurrency

## Quick Start

```bash
# Pull and run
docker run --rm -i \
  -v /your/data:/data \
  ghcr.io/claw-works/quick-sqlite-mcp-server
```

## Amazon Quick Configuration

Amazon Quick uses a UI-based MCP setup. Go to **Settings → Capabilities → MCP → + Add MCP** and choose **Local**:

| Field | Value |
|-------|-------|
| Name | `SQLite` |
| Command | `docker` |
| Arguments | `run --rm -i -v /your/data:/data -e ROOT_DIR=/data ghcr.io/claw-works/quick-sqlite-mcp-server` |
| Description | `Read and write SQLite databases. Supports multiple databases, transactions, and schema inspection.` |

> Replace `/your/data` with the directory containing your `.db` files.

Alternatively, if you prefer a config file (e.g. for Kiro or Claude Code import), use:

```json
{
  "mcpServers": {
    "sqlite": {
      "command": "docker",
      "args": [
        "run", "--rm", "-i",
        "-v", "/your/data:/data",
        "-e", "ROOT_DIR=/data",
        "ghcr.io/claw-works/quick-sqlite-mcp-server"
      ]
    }
  }
}
```

Quick can import this format via **+ Add MCP → Import** by pointing to the config file path.

## Environment Variables

| Variable | Default | Description |
|----------|---------|-------------|
| `ROOT_DIR` | `/data` | Base directory for relative database paths |
| `ALLOW_ABSOLUTE_PATH` | `false` | Allow absolute paths in the `db` parameter |

## Tools Reference

### `sqlite_list_databases`
List all `.db` / `.sqlite` / `.sqlite3` files under `ROOT_DIR`.

```json
{}
```

---

### `sqlite_query`
Execute a SELECT statement and return results as a JSON array.

```json
{
  "db": "myapp.db",
  "sql": "SELECT * FROM users WHERE age > ?",
  "params": [18]
}
```

Returns: `{ "columns": [...], "rows": [...], "row_count": N }`

---

### `sqlite_execute`
Execute an INSERT, UPDATE, or DELETE statement.

```json
{
  "db": "myapp.db",
  "sql": "INSERT INTO users (name, age) VALUES (?, ?)",
  "params": ["Alice", 30]
}
```

Returns: `{ "rows_affected": N, "last_insert_id": N }`

---

### `sqlite_execute_batch`
Execute multiple SQL statements in a single atomic transaction. Perfect for schema migrations.

```json
{
  "db": "myapp.db",
  "statements": [
    "CREATE TABLE IF NOT EXISTS users (id INTEGER PRIMARY KEY, name TEXT NOT NULL, age INTEGER)",
    "CREATE INDEX IF NOT EXISTS idx_users_age ON users(age)"
  ]
}
```

Returns: `{ "statements_executed": N, "total_rows_affected": N }`

---

### `sqlite_list_tables`
List all user-defined tables in a database.

```json
{ "db": "myapp.db" }
```

Returns: `{ "tables": ["users", "posts", ...], "count": N }`

---

### `sqlite_describe_table`
Get the schema of a table (columns, types, constraints).

```json
{
  "db": "myapp.db",
  "table": "users"
}
```

Returns column info including `name`, `type`, `not_null`, `default_value`, `primary_key`.

---

### `sqlite_begin_transaction`
Start an explicit transaction. Returns a `tx_id` for use with commit/rollback.

```json
{ "db": "myapp.db" }
```

Returns: `{ "tx_id": "uuid-...", "message": "..." }`

> Transactions expire automatically after **5 minutes**.

---

### `sqlite_commit`
Commit an active transaction.

```json
{ "tx_id": "uuid-..." }
```

---

### `sqlite_rollback`
Roll back an active transaction.

```json
{ "tx_id": "uuid-..." }
```

---

## Path Resolution

| `db` value | `ALLOW_ABSOLUTE_PATH` | Resolved path |
|------------|----------------------|---------------|
| `myapp.db` | any | `$ROOT_DIR/myapp.db` |
| `sub/data.db` | any | `$ROOT_DIR/sub/data.db` |
| `/tmp/other.db` | `true` | `/tmp/other.db` |
| `/tmp/other.db` | `false` | ❌ Error |
| `../../etc/passwd` | `false` | ❌ Path traversal error |

## Building Locally

```bash
# Requires Go 1.23+ and gcc (for CGO/sqlite3)
go build -o quick-sqlite-mcp-server .

# Or with Docker
docker build -t quick-sqlite-mcp-server .
```

## License

MIT
