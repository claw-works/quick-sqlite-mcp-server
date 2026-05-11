# quick-sqlite-mcp-server

A SQLite MCP (Model Context Protocol) server for **Amazon Q Developer** and other MCP-compatible AI tools. Runs as a Docker container with your database files mounted in — no SQLite bindings needed in the client.

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

## Amazon Q Developer Configuration

Add to your MCP settings (`.amazonq/mcp.json` or via Q Developer UI):

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

> **Tip**: Replace `/your/data` with the directory containing your `.db` files.

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
# Requires Go 1.22+ and gcc (for CGO/sqlite3)
go build -o quick-sqlite-mcp-server .

# Or with Docker
docker build -t quick-sqlite-mcp-server .
```

## License

MIT
