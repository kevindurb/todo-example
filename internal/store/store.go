// Package store owns the SQLite database, its schema, and all SQL access.
package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	_ "modernc.org/sqlite"

	"github.com/kevindurb/todo-example/internal/todo"
)

const (
	schemaVersion = 1
	busyTimeoutMS = 5000
	openTimeout   = 10 * time.Second
	listOrder     = "done ASC, created_at DESC, id DESC"
)

// ErrNotFound is returned when an identifier has no matching todo.
var ErrNotFound = errors.New("todo not found")

// Store is a SQLite-backed todo store. It assumes a single process per
// database file: writes are serialized on one connection.
type Store struct {
	db *sql.DB
}

var schemaStatements = []string{
	`CREATE TABLE IF NOT EXISTS todos (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		title TEXT NOT NULL,
		notes TEXT NOT NULL DEFAULT '',
		done INTEGER NOT NULL DEFAULT 0,
		created_at INTEGER NOT NULL,
		updated_at INTEGER NOT NULL
	)`,
	`CREATE INDEX IF NOT EXISTS todos_list_order ON todos (done, created_at DESC, id DESC)`,
}

// Open creates any missing parent directories, opens the database, applies the
// durability pragmas, and initializes the schema.
func Open(path string) (*Store, error) {
	if path == "" {
		return nil, errors.New("database path is empty")
	}
	if dir := filepath.Dir(path); dir != "" && dir != "." {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return nil, fmt.Errorf("create database directory for %q: %w", path, err)
		}
	}

	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, fmt.Errorf("open database %q: %w", path, err)
	}
	db.SetMaxOpenConns(1)
	db.SetMaxIdleConns(1)
	db.SetConnMaxLifetime(0)

	ctx, cancel := context.WithTimeout(context.Background(), openTimeout)
	defer cancel()

	var mode string
	if err := db.QueryRowContext(ctx, "PRAGMA journal_mode = WAL").Scan(&mode); err != nil {
		db.Close()
		return nil, fmt.Errorf("enable WAL for database %q: %w", path, err)
	}
	for _, pragma := range []string{
		fmt.Sprintf("PRAGMA busy_timeout = %d", busyTimeoutMS),
		"PRAGMA foreign_keys = ON",
		"PRAGMA synchronous = NORMAL",
	} {
		if _, err := db.ExecContext(ctx, pragma); err != nil {
			db.Close()
			return nil, fmt.Errorf("configure database %q (%s): %w", path, pragma, err)
		}
	}
	if err := db.PingContext(ctx); err != nil {
		db.Close()
		return nil, fmt.Errorf("open database %q: %w", path, err)
	}
	if err := migrate(ctx, db); err != nil {
		db.Close()
		return nil, fmt.Errorf("initialize database %q: %w", path, err)
	}
	return &Store{db: db}, nil
}

func migrate(ctx context.Context, db *sql.DB) error {
	var version int
	if err := db.QueryRowContext(ctx, "PRAGMA user_version").Scan(&version); err != nil {
		return fmt.Errorf("read schema version: %w", err)
	}
	if version > schemaVersion {
		return fmt.Errorf("schema version %d is newer than supported version %d", version, schemaVersion)
	}
	if version == schemaVersion {
		return nil
	}

	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin schema transaction: %w", err)
	}
	defer tx.Rollback()

	for _, statement := range schemaStatements {
		if _, err := tx.ExecContext(ctx, statement); err != nil {
			return fmt.Errorf("apply schema: %w", err)
		}
	}
	// PRAGMA user_version cannot be parameterized; the value is a constant.
	if _, err := tx.ExecContext(ctx, fmt.Sprintf("PRAGMA user_version = %d", schemaVersion)); err != nil {
		return fmt.Errorf("record schema version: %w", err)
	}
	return tx.Commit()
}

// Close releases the database.
func (s *Store) Close() error {
	return s.db.Close()
}

// Ping reports whether the database is reachable.
func (s *Store) Ping(ctx context.Context) error {
	return s.db.PingContext(ctx)
}

// Create stores a new not-done todo using already-validated input.
func (s *Store) Create(ctx context.Context, in todo.Input) (todo.Todo, error) {
	now := time.Now().UTC().UnixNano()
	result, err := s.db.ExecContext(ctx,
		`INSERT INTO todos (title, notes, done, created_at, updated_at) VALUES (?, ?, 0, ?, ?)`,
		in.Title, in.Notes, now, now)
	if err != nil {
		return todo.Todo{}, fmt.Errorf("create todo: %w", err)
	}
	id, err := result.LastInsertId()
	if err != nil {
		return todo.Todo{}, fmt.Errorf("create todo: %w", err)
	}
	return s.Get(ctx, id)
}

// Get returns one todo or ErrNotFound.
func (s *Store) Get(ctx context.Context, id int64) (todo.Todo, error) {
	row := s.db.QueryRowContext(ctx,
		`SELECT id, title, notes, done, created_at, updated_at FROM todos WHERE id = ?`, id)
	found, err := scanTodo(row)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return todo.Todo{}, ErrNotFound
		}
		return todo.Todo{}, fmt.Errorf("get todo %d: %w", id, err)
	}
	return found, nil
}

// List returns every todo, open before done and newest first within each group.
func (s *Store) List(ctx context.Context) ([]todo.Todo, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT id, title, notes, done, created_at, updated_at FROM todos ORDER BY `+listOrder)
	if err != nil {
		return nil, fmt.Errorf("list todos: %w", err)
	}
	defer rows.Close()

	todos := make([]todo.Todo, 0)
	for rows.Next() {
		found, err := scanTodo(rows)
		if err != nil {
			return nil, fmt.Errorf("list todos: %w", err)
		}
		todos = append(todos, found)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("list todos: %w", err)
	}
	return todos, nil
}

// Update replaces the title and notes of an existing todo, preserving its
// identifier, creation timestamp, and completion state.
func (s *Store) Update(ctx context.Context, id int64, in todo.Input) (todo.Todo, error) {
	result, err := s.db.ExecContext(ctx,
		`UPDATE todos SET title = ?, notes = ?, updated_at = ? WHERE id = ?`,
		in.Title, in.Notes, time.Now().UTC().UnixNano(), id)
	if err != nil {
		return todo.Todo{}, fmt.Errorf("update todo %d: %w", id, err)
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return todo.Todo{}, fmt.Errorf("update todo %d: %w", id, err)
	}
	if affected == 0 {
		return todo.Todo{}, ErrNotFound
	}
	return s.Get(ctx, id)
}

// SetDone stores an explicit completion state. Repeating the same state leaves
// the record done or open and never creates a duplicate.
func (s *Store) SetDone(ctx context.Context, id int64, done bool) (todo.Todo, error) {
	state := 0
	if done {
		state = 1
	}
	result, err := s.db.ExecContext(ctx,
		`UPDATE todos SET done = ?, updated_at = ? WHERE id = ?`,
		state, time.Now().UTC().UnixNano(), id)
	if err != nil {
		return todo.Todo{}, fmt.Errorf("set todo %d status: %w", id, err)
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return todo.Todo{}, fmt.Errorf("set todo %d status: %w", id, err)
	}
	if affected == 0 {
		return todo.Todo{}, ErrNotFound
	}
	return s.Get(ctx, id)
}

// Delete permanently removes a todo or returns ErrNotFound.
func (s *Store) Delete(ctx context.Context, id int64) error {
	result, err := s.db.ExecContext(ctx, `DELETE FROM todos WHERE id = ?`, id)
	if err != nil {
		return fmt.Errorf("delete todo %d: %w", id, err)
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("delete todo %d: %w", id, err)
	}
	if affected == 0 {
		return ErrNotFound
	}
	return nil
}

type scanner interface {
	Scan(dest ...any) error
}

func scanTodo(row scanner) (todo.Todo, error) {
	var (
		found   todo.Todo
		done    int
		created int64
		updated int64
	)
	if err := row.Scan(&found.ID, &found.Title, &found.Notes, &done, &created, &updated); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return todo.Todo{}, ErrNotFound
		}
		return todo.Todo{}, err
	}
	found.Done = done != 0
	found.CreatedAt = time.Unix(0, created).UTC()
	found.UpdatedAt = time.Unix(0, updated).UTC()
	return found, nil
}
