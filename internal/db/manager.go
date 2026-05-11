// Package db manages SQLite database connections and transactions.
package db

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	_ "github.com/mattn/go-sqlite3"
)

const txTimeout = 5 * time.Minute

// Transaction holds an active database transaction.
type Transaction struct {
	ID        string
	Tx        *sql.Tx
	DB        *sql.DB
	CreatedAt time.Time
}

// Manager handles database connections and in-flight transactions.
type Manager struct {
	rootDir       string
	allowAbsolute bool

	connMu sync.Mutex
	conns  map[string]*sql.DB // keyed by resolved path

	txMu sync.Mutex
	txs  map[string]*Transaction // keyed by tx_id
}

// NewManager creates a new Manager.
func NewManager(rootDir string, allowAbsolute bool) *Manager {
	m := &Manager{
		rootDir:       rootDir,
		allowAbsolute: allowAbsolute,
		conns:         make(map[string]*sql.DB),
		txs:           make(map[string]*Transaction),
	}
	go m.reapExpiredTransactions()
	return m
}

// ResolvePath converts a db parameter to an absolute filesystem path.
func (m *Manager) ResolvePath(db string) (string, error) {
	if db == "" {
		return "", fmt.Errorf("db parameter is required")
	}

	var resolved string
	if filepath.IsAbs(db) {
		if !m.allowAbsolute {
			return "", fmt.Errorf("absolute paths are not allowed (set ALLOW_ABSOLUTE_PATH=true to enable)")
		}
		resolved = db
	} else {
		resolved = filepath.Join(m.rootDir, db)
	}

	// Clean and check for path traversal
	resolved = filepath.Clean(resolved)
	if !m.allowAbsolute {
		if !strings.HasPrefix(resolved, filepath.Clean(m.rootDir)+string(os.PathSeparator)) &&
			resolved != filepath.Clean(m.rootDir) {
			return "", fmt.Errorf("path traversal detected: %q is outside ROOT_DIR", db)
		}
	}

	return resolved, nil
}

// GetConn returns (or opens) a connection to the given resolved path.
func (m *Manager) GetConn(resolvedPath string) (*sql.DB, error) {
	m.connMu.Lock()
	defer m.connMu.Unlock()

	if conn, ok := m.conns[resolvedPath]; ok {
		return conn, nil
	}

	// Ensure parent directory exists
	if err := os.MkdirAll(filepath.Dir(resolvedPath), 0755); err != nil {
		return nil, fmt.Errorf("cannot create directory for %q: %w", resolvedPath, err)
	}

	conn, err := sql.Open("sqlite3", resolvedPath+"?_journal_mode=WAL&_busy_timeout=5000")
	if err != nil {
		return nil, fmt.Errorf("cannot open database %q: %w", resolvedPath, err)
	}
	conn.SetMaxOpenConns(1) // SQLite is single-writer
	m.conns[resolvedPath] = conn
	return conn, nil
}

// OpenDB resolves the db param and returns a connection.
func (m *Manager) OpenDB(db string) (*sql.DB, string, error) {
	path, err := m.ResolvePath(db)
	if err != nil {
		return nil, "", err
	}
	conn, err := m.GetConn(path)
	if err != nil {
		return nil, "", err
	}
	return conn, path, nil
}

// BeginTransaction opens a new transaction and stores it by UUID.
func (m *Manager) BeginTransaction(db string) (string, error) {
	conn, _, err := m.OpenDB(db)
	if err != nil {
		return "", err
	}

	tx, err := conn.Begin()
	if err != nil {
		return "", fmt.Errorf("cannot begin transaction: %w", err)
	}

	id := uuid.New().String()
	m.txMu.Lock()
	m.txs[id] = &Transaction{
		ID:        id,
		Tx:        tx,
		DB:        conn,
		CreatedAt: time.Now(),
	}
	m.txMu.Unlock()

	return id, nil
}

// GetTransaction retrieves an active transaction by ID.
func (m *Manager) GetTransaction(txID string) (*Transaction, error) {
	m.txMu.Lock()
	defer m.txMu.Unlock()
	t, ok := m.txs[txID]
	if !ok {
		return nil, fmt.Errorf("transaction %q not found (may have expired or already committed/rolled back)", txID)
	}
	return t, nil
}

// CommitTransaction commits and removes a transaction.
func (m *Manager) CommitTransaction(txID string) error {
	m.txMu.Lock()
	t, ok := m.txs[txID]
	if !ok {
		m.txMu.Unlock()
		return fmt.Errorf("transaction %q not found", txID)
	}
	delete(m.txs, txID)
	m.txMu.Unlock()

	if err := t.Tx.Commit(); err != nil {
		return fmt.Errorf("commit failed: %w", err)
	}
	return nil
}

// RollbackTransaction rolls back and removes a transaction.
func (m *Manager) RollbackTransaction(txID string) error {
	m.txMu.Lock()
	t, ok := m.txs[txID]
	if !ok {
		m.txMu.Unlock()
		return fmt.Errorf("transaction %q not found", txID)
	}
	delete(m.txs, txID)
	m.txMu.Unlock()

	if err := t.Tx.Rollback(); err != nil {
		return fmt.Errorf("rollback failed: %w", err)
	}
	return nil
}

// ListDatabases returns all .db and .sqlite files under ROOT_DIR.
func (m *Manager) ListDatabases() ([]string, error) {
	var files []string
	root := filepath.Clean(m.rootDir)
	err := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil // skip unreadable dirs
		}
		if info.IsDir() {
			return nil
		}
		ext := strings.ToLower(filepath.Ext(path))
		if ext == ".db" || ext == ".sqlite" || ext == ".sqlite3" {
			rel, _ := filepath.Rel(root, path)
			files = append(files, rel)
		}
		return nil
	})
	return files, err
}

// Close closes all open database connections.
func (m *Manager) Close() {
	m.connMu.Lock()
	defer m.connMu.Unlock()
	for _, conn := range m.conns {
		conn.Close()
	}
}

// reapExpiredTransactions periodically rolls back timed-out transactions.
func (m *Manager) reapExpiredTransactions() {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()
	for range ticker.C {
		m.txMu.Lock()
		for id, t := range m.txs {
			if time.Since(t.CreatedAt) > txTimeout {
				t.Tx.Rollback() //nolint:errcheck
				delete(m.txs, id)
			}
		}
		m.txMu.Unlock()
	}
}
