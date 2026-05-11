// Package tools implements all SQLite MCP tools.
package tools

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/claw-works/quick-sqlite-mcp-server/internal/db"
	"github.com/claw-works/quick-sqlite-mcp-server/internal/mcp"
)

// Registry holds all tool definitions and handlers.
type Registry struct {
	manager *db.Manager
	tools   []mcp.ToolDef
	handlers map[string]func(json.RawMessage) (interface{}, error)
}

// NewRegistry creates and registers all tools.
func NewRegistry(manager *db.Manager) *Registry {
	r := &Registry{
		manager:  manager,
		handlers: make(map[string]func(json.RawMessage) (interface{}, error)),
	}
	r.register()
	return r
}

// ListTools returns all registered tool definitions.
func (r *Registry) ListTools() []mcp.ToolDef {
	return r.tools
}

// CallTool dispatches a tool call by name.
func (r *Registry) CallTool(name string, args json.RawMessage) (interface{}, error) {
	h, ok := r.handlers[name]
	if !ok {
		return nil, fmt.Errorf("unknown tool: %q", name)
	}
	return h(args)
}

func (r *Registry) add(def mcp.ToolDef, handler func(json.RawMessage) (interface{}, error)) {
	r.tools = append(r.tools, def)
	r.handlers[def.Name] = handler
}

// dbParam is the common "db" property definition.
var dbParam = mcp.Property{
	Type:        "string",
	Description: `Path to the SQLite database file. Use a relative path (e.g. "myapp.db" or "subdir/data.db") to resolve against ROOT_DIR, or an absolute path if ALLOW_ABSOLUTE_PATH=true. If the file does not exist, it will be created automatically.`,
}

func (r *Registry) register() {
	// ── sqlite_query ──────────────────────────────────────────────────────────
	r.add(mcp.ToolDef{
		Name:        "sqlite_query",
		Description: "Execute a SELECT statement against a SQLite database and return the results as a JSON array of row objects. Use this for all read-only queries. Supports parameterized queries to prevent SQL injection. The database file is created automatically if it does not exist.",
		InputSchema: mcp.InputSchema{
			Type: "object",
			Properties: map[string]mcp.Property{
				"db":     dbParam,
				"sql":    {Type: "string", Description: "The SELECT SQL statement to execute."},
				"params": {Type: "array", Description: "Optional list of parameter values for positional placeholders (?) in the SQL statement.", Items: &mcp.Items{Type: "string"}},
			},
			Required: []string{"db", "sql"},
		},
	}, r.sqliteQuery)

	// ── sqlite_execute ────────────────────────────────────────────────────────
	r.add(mcp.ToolDef{
		Name:        "sqlite_execute",
		Description: "Execute a single INSERT, UPDATE, or DELETE statement against a SQLite database. Returns the number of rows affected and the last inserted row ID (for INSERT). Supports parameterized queries. The database file is created automatically if it does not exist.",
		InputSchema: mcp.InputSchema{
			Type: "object",
			Properties: map[string]mcp.Property{
				"db":     dbParam,
				"sql":    {Type: "string", Description: "The INSERT, UPDATE, or DELETE SQL statement to execute."},
				"params": {Type: "array", Description: "Optional list of parameter values for positional placeholders (?) in the SQL statement.", Items: &mcp.Items{Type: "string"}},
			},
			Required: []string{"db", "sql"},
		},
	}, r.sqliteExecute)

	// ── sqlite_execute_batch ──────────────────────────────────────────────────
	r.add(mcp.ToolDef{
		Name:        "sqlite_execute_batch",
		Description: "Execute multiple SQL statements in a single atomic transaction. Ideal for creating a new database schema, running migrations, or bulk inserts. The database file is created automatically if it does not exist. If any statement fails, all changes are rolled back automatically.",
		InputSchema: mcp.InputSchema{
			Type: "object",
			Properties: map[string]mcp.Property{
				"db":         dbParam,
				"statements": {Type: "array", Description: "List of SQL statements to execute in order within a single transaction.", Items: &mcp.Items{Type: "string"}},
			},
			Required: []string{"db", "statements"},
		},
	}, r.sqliteExecuteBatch)

	// ── sqlite_list_tables ────────────────────────────────────────────────────
	r.add(mcp.ToolDef{
		Name:        "sqlite_list_tables",
		Description: "List all user-defined tables in a SQLite database. Returns table names only (excludes SQLite internal tables). Use this to explore an unfamiliar database before querying.",
		InputSchema: mcp.InputSchema{
			Type: "object",
			Properties: map[string]mcp.Property{
				"db": dbParam,
			},
			Required: []string{"db"},
		},
	}, r.sqliteListTables)

	// ── sqlite_describe_table ─────────────────────────────────────────────────
	r.add(mcp.ToolDef{
		Name:        "sqlite_describe_table",
		Description: "Get the schema of a specific table: column names, data types, nullability, default values, and primary key flags. Use this before writing INSERT/UPDATE statements to understand the table structure.",
		InputSchema: mcp.InputSchema{
			Type: "object",
			Properties: map[string]mcp.Property{
				"db":    dbParam,
				"table": {Type: "string", Description: "The name of the table to describe."},
			},
			Required: []string{"db", "table"},
		},
	}, r.sqliteDescribeTable)

	// ── sqlite_begin_transaction ──────────────────────────────────────────────
	r.add(mcp.ToolDef{
		Name:        "sqlite_begin_transaction",
		Description: "Begin an explicit database transaction. Returns a tx_id that must be passed to sqlite_commit or sqlite_rollback. Use this when you need to execute multiple write statements atomically across separate tool calls. Transactions expire automatically after 5 minutes of inactivity. The database file is created automatically if it does not exist.",
		InputSchema: mcp.InputSchema{
			Type: "object",
			Properties: map[string]mcp.Property{
				"db": dbParam,
			},
			Required: []string{"db"},
		},
	}, r.sqliteBeginTransaction)

	// ── sqlite_commit ─────────────────────────────────────────────────────────
	r.add(mcp.ToolDef{
		Name:        "sqlite_commit",
		Description: "Commit an active transaction, making all changes permanent. The tx_id is invalidated after this call.",
		InputSchema: mcp.InputSchema{
			Type: "object",
			Properties: map[string]mcp.Property{
				"tx_id": {Type: "string", Description: "The transaction ID returned by sqlite_begin_transaction."},
			},
			Required: []string{"tx_id"},
		},
	}, r.sqliteCommit)

	// ── sqlite_rollback ───────────────────────────────────────────────────────
	r.add(mcp.ToolDef{
		Name:        "sqlite_rollback",
		Description: "Roll back an active transaction, discarding all changes made since sqlite_begin_transaction. The tx_id is invalidated after this call.",
		InputSchema: mcp.InputSchema{
			Type: "object",
			Properties: map[string]mcp.Property{
				"tx_id": {Type: "string", Description: "The transaction ID returned by sqlite_begin_transaction."},
			},
			Required: []string{"tx_id"},
		},
	}, r.sqliteRollback)

	// ── sqlite_list_databases ─────────────────────────────────────────────────
	r.add(mcp.ToolDef{
		Name:        "sqlite_list_databases",
		Description: "List all SQLite database files (.db, .sqlite, .sqlite3) available under the configured ROOT_DIR. Returns relative paths that can be used directly as the 'db' parameter in other tools.",
		InputSchema: mcp.InputSchema{
			Type:       "object",
			Properties: map[string]mcp.Property{},
		},
	}, r.sqliteListDatabases)
}

// ── Tool implementations ──────────────────────────────────────────────────────

type queryArgs struct {
	DB     string        `json:"db"`
	SQL    string        `json:"sql"`
	Params []interface{} `json:"params"`
}

func (r *Registry) sqliteQuery(raw json.RawMessage) (interface{}, error) {
	var args queryArgs
	if err := json.Unmarshal(raw, &args); err != nil {
		return nil, fmt.Errorf("invalid arguments: %w", err)
	}
	conn, _, err := r.manager.OpenDB(args.DB)
	if err != nil {
		return nil, err
	}

	rows, err := conn.Query(args.SQL, args.Params...)
	if err != nil {
		return nil, fmt.Errorf("query failed: %w", err)
	}
	defer rows.Close()

	cols, err := rows.Columns()
	if err != nil {
		return nil, fmt.Errorf("cannot get columns: %w", err)
	}

	var results []map[string]interface{}
	for rows.Next() {
		vals := make([]interface{}, len(cols))
		ptrs := make([]interface{}, len(cols))
		for i := range vals {
			ptrs[i] = &vals[i]
		}
		if err := rows.Scan(ptrs...); err != nil {
			return nil, fmt.Errorf("scan error: %w", err)
		}
		row := make(map[string]interface{}, len(cols))
		for i, col := range cols {
			// Convert []byte to string for readability
			if b, ok := vals[i].([]byte); ok {
				row[col] = string(b)
			} else {
				row[col] = vals[i]
			}
		}
		results = append(results, row)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("rows error: %w", err)
	}

	if results == nil {
		results = []map[string]interface{}{}
	}

	return map[string]interface{}{
		"columns":   cols,
		"rows":      results,
		"row_count": len(results),
	}, nil
}

type execArgs struct {
	DB     string        `json:"db"`
	SQL    string        `json:"sql"`
	Params []interface{} `json:"params"`
}

func (r *Registry) sqliteExecute(raw json.RawMessage) (interface{}, error) {
	var args execArgs
	if err := json.Unmarshal(raw, &args); err != nil {
		return nil, fmt.Errorf("invalid arguments: %w", err)
	}
	conn, _, err := r.manager.OpenDB(args.DB)
	if err != nil {
		return nil, err
	}

	result, err := conn.Exec(args.SQL, args.Params...)
	if err != nil {
		return nil, fmt.Errorf("execute failed: %w", err)
	}

	rowsAffected, _ := result.RowsAffected()
	lastInsertID, _ := result.LastInsertId()

	return map[string]interface{}{
		"rows_affected":  rowsAffected,
		"last_insert_id": lastInsertID,
	}, nil
}

type batchArgs struct {
	DB         string   `json:"db"`
	Statements []string `json:"statements"`
}

func (r *Registry) sqliteExecuteBatch(raw json.RawMessage) (interface{}, error) {
	var args batchArgs
	if err := json.Unmarshal(raw, &args); err != nil {
		return nil, fmt.Errorf("invalid arguments: %w", err)
	}
	if len(args.Statements) == 0 {
		return nil, fmt.Errorf("statements array is empty")
	}

	conn, _, err := r.manager.OpenDB(args.DB)
	if err != nil {
		return nil, err
	}

	tx, err := conn.Begin()
	if err != nil {
		return nil, fmt.Errorf("cannot begin transaction: %w", err)
	}

	var totalAffected int64
	for i, stmt := range args.Statements {
		stmt = strings.TrimSpace(stmt)
		if stmt == "" {
			continue
		}
		result, err := tx.Exec(stmt)
		if err != nil {
			tx.Rollback() //nolint:errcheck
			return nil, fmt.Errorf("statement %d failed: %w\nSQL: %s", i+1, err, stmt)
		}
		n, _ := result.RowsAffected()
		totalAffected += n
	}

	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("commit failed: %w", err)
	}

	return map[string]interface{}{
		"statements_executed": len(args.Statements),
		"total_rows_affected": totalAffected,
	}, nil
}

type dbOnlyArgs struct {
	DB string `json:"db"`
}

func (r *Registry) sqliteListTables(raw json.RawMessage) (interface{}, error) {
	var args dbOnlyArgs
	if err := json.Unmarshal(raw, &args); err != nil {
		return nil, fmt.Errorf("invalid arguments: %w", err)
	}
	conn, _, err := r.manager.OpenDB(args.DB)
	if err != nil {
		return nil, err
	}

	rows, err := conn.Query(`SELECT name FROM sqlite_master WHERE type='table' AND name NOT LIKE 'sqlite_%' ORDER BY name`)
	if err != nil {
		return nil, fmt.Errorf("query failed: %w", err)
	}
	defer rows.Close()

	var tables []string
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			return nil, err
		}
		tables = append(tables, name)
	}
	if tables == nil {
		tables = []string{}
	}

	return map[string]interface{}{
		"tables": tables,
		"count":  len(tables),
	}, nil
}

type describeArgs struct {
	DB    string `json:"db"`
	Table string `json:"table"`
}

type ColumnInfo struct {
	CID          int         `json:"cid"`
	Name         string      `json:"name"`
	Type         string      `json:"type"`
	NotNull      bool        `json:"not_null"`
	DefaultValue interface{} `json:"default_value"`
	PrimaryKey   int         `json:"primary_key"`
}

func (r *Registry) sqliteDescribeTable(raw json.RawMessage) (interface{}, error) {
	var args describeArgs
	if err := json.Unmarshal(raw, &args); err != nil {
		return nil, fmt.Errorf("invalid arguments: %w", err)
	}
	conn, _, err := r.manager.OpenDB(args.DB)
	if err != nil {
		return nil, err
	}

	rows, err := conn.Query(fmt.Sprintf("PRAGMA table_info(%s)", args.Table))
	if err != nil {
		return nil, fmt.Errorf("PRAGMA table_info failed: %w", err)
	}
	defer rows.Close()

	var columns []ColumnInfo
	for rows.Next() {
		var col ColumnInfo
		var notNull int
		if err := rows.Scan(&col.CID, &col.Name, &col.Type, &notNull, &col.DefaultValue, &col.PrimaryKey); err != nil {
			return nil, err
		}
		col.NotNull = notNull == 1
		columns = append(columns, col)
	}

	if len(columns) == 0 {
		return nil, fmt.Errorf("table %q not found", args.Table)
	}

	return map[string]interface{}{
		"table":   args.Table,
		"columns": columns,
	}, nil
}

func (r *Registry) sqliteBeginTransaction(raw json.RawMessage) (interface{}, error) {
	var args dbOnlyArgs
	if err := json.Unmarshal(raw, &args); err != nil {
		return nil, fmt.Errorf("invalid arguments: %w", err)
	}

	txID, err := r.manager.BeginTransaction(args.DB)
	if err != nil {
		return nil, err
	}

	return map[string]interface{}{
		"tx_id":   txID,
		"message": "Transaction started. Use tx_id with sqlite_commit or sqlite_rollback. Expires in 5 minutes.",
	}, nil
}

type txArgs struct {
	TxID string `json:"tx_id"`
}

func (r *Registry) sqliteCommit(raw json.RawMessage) (interface{}, error) {
	var args txArgs
	if err := json.Unmarshal(raw, &args); err != nil {
		return nil, fmt.Errorf("invalid arguments: %w", err)
	}
	if err := r.manager.CommitTransaction(args.TxID); err != nil {
		return nil, err
	}
	return map[string]interface{}{"message": "Transaction committed successfully."}, nil
}

func (r *Registry) sqliteRollback(raw json.RawMessage) (interface{}, error) {
	var args txArgs
	if err := json.Unmarshal(raw, &args); err != nil {
		return nil, fmt.Errorf("invalid arguments: %w", err)
	}
	if err := r.manager.RollbackTransaction(args.TxID); err != nil {
		return nil, err
	}
	return map[string]interface{}{"message": "Transaction rolled back successfully."}, nil
}

func (r *Registry) sqliteListDatabases(raw json.RawMessage) (interface{}, error) {
	files, err := r.manager.ListDatabases()
	if err != nil {
		return nil, fmt.Errorf("cannot list databases: %w", err)
	}
	if files == nil {
		files = []string{}
	}
	return map[string]interface{}{
		"databases": files,
		"count":     len(files),
		"note":      "These relative paths can be used directly as the 'db' parameter in other tools.",
	}, nil
}

// Ensure Registry implements mcp.Registry
var _ interface {
	ListTools() []mcp.ToolDef
	CallTool(name string, args json.RawMessage) (interface{}, error)
} = (*Registry)(nil)

// sqliteQueryWithTx executes a query within an existing transaction.
func sqliteQueryWithTx(tx *sql.Tx, query string, params []interface{}) (interface{}, error) {
	rows, err := tx.Query(query, params...)
	if err != nil {
		return nil, fmt.Errorf("query failed: %w", err)
	}
	defer rows.Close()

	cols, _ := rows.Columns()
	var results []map[string]interface{}
	for rows.Next() {
		vals := make([]interface{}, len(cols))
		ptrs := make([]interface{}, len(cols))
		for i := range vals {
			ptrs[i] = &vals[i]
		}
		rows.Scan(ptrs...) //nolint:errcheck
		row := make(map[string]interface{}, len(cols))
		for i, col := range cols {
			if b, ok := vals[i].([]byte); ok {
				row[col] = string(b)
			} else {
				row[col] = vals[i]
			}
		}
		results = append(results, row)
	}
	return results, rows.Err()
}
