package checkpoint

import (
	"database/sql"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"sync"
	"time"

	_ "modernc.org/sqlite" // Pure Go SQLite driver
)

// SQLiteStore persists checkpoints to SQLite.
// It is suitable for single-process production use.
type SQLiteStore struct {
	db     *sql.DB
	mu     sync.RWMutex
	closed bool
}

// NewSQLiteStore creates a new SQLite checkpoint store.
// The path should be a file path (e.g., "./checkpoints.db") or ":memory:" for testing.
//
// The database file is created with restrictive permissions (0600) to protect
// checkpoint data which may contain sensitive state.
func NewSQLiteStore(path string) (*SQLiteStore, error) {
	// Create file with restrictive permissions BEFORE sql.Open touches it.
	// This prevents a TOCTOU race where the file is briefly world-readable.
	if path != ":memory:" {
		if _, err := os.Stat(path); os.IsNotExist(err) {
			f, createErr := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
			if createErr == nil {
				if closeErr := f.Close(); closeErr != nil {
					// Log but continue - file exists, sql.Open will handle any real issues
					slog.Warn("failed to close checkpoint file after creation",
						slog.String("path", path),
						slog.String("error", closeErr.Error()))
				}
			}
			// Ignore createErr - file might have been created between Stat and OpenFile (TOCTOU)
		}
	}

	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, fmt.Errorf("open database: %w", err)
	}

	// Enable WAL mode for better concurrent read performance
	if _, err := db.Exec("PRAGMA journal_mode=WAL"); err != nil {
		db.Close()
		return nil, fmt.Errorf("enable WAL mode: %w", err)
	}

	// Create table and index
	if _, err := db.Exec(`
		CREATE TABLE IF NOT EXISTS checkpoints (
			run_id TEXT NOT NULL,
			node_id TEXT NOT NULL,
			sequence INTEGER NOT NULL,
			timestamp TEXT NOT NULL,
			data BLOB NOT NULL,
			PRIMARY KEY (run_id, node_id)
		)
	`); err != nil {
		db.Close()
		return nil, fmt.Errorf("create table: %w", err)
	}

	if _, err := db.Exec(`
		CREATE INDEX IF NOT EXISTS idx_checkpoints_run_id
		ON checkpoints(run_id)
	`); err != nil {
		db.Close()
		return nil, fmt.Errorf("create index: %w", err)
	}

	// Ensure permissions are correct for existing files
	if path != ":memory:" {
		if err := os.Chmod(path, 0600); err != nil {
			// SECURITY WARNING: chmod failure means checkpoint file may be readable by others.
			// This can happen on network filesystems, read-only mounts, or permission issues.
			slog.Warn("failed to set restrictive permissions on checkpoint file",
				slog.String("path", path),
				slog.String("error", err.Error()),
				slog.String("security_note", "checkpoint data may be readable by other users"))
		}
	}

	return &SQLiteStore{db: db}, nil
}

// Save implements Store.
func (s *SQLiteStore) Save(runID, nodeID string, data []byte) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.closed {
		return ErrStoreClosed
	}

	// Use INSERT OR REPLACE to handle updates
	// Calculate sequence as max + 1 for this run
	_, err := s.db.Exec(`
		INSERT INTO checkpoints (run_id, node_id, sequence, timestamp, data)
		VALUES (
			?, ?,
			COALESCE((SELECT MAX(sequence) FROM checkpoints WHERE run_id = ?), 0) + 1,
			?, ?
		)
		ON CONFLICT(run_id, node_id) DO UPDATE SET
			sequence = (SELECT MAX(sequence) FROM checkpoints WHERE run_id = excluded.run_id) + 1,
			timestamp = excluded.timestamp,
			data = excluded.data
	`, runID, nodeID, runID, time.Now().UTC().Format(time.RFC3339Nano), data)

	if err != nil {
		return fmt.Errorf("save checkpoint: %w", err)
	}
	return nil
}

// Load implements Store.
func (s *SQLiteStore) Load(runID, nodeID string) ([]byte, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if s.closed {
		return nil, ErrStoreClosed
	}

	var data []byte
	err := s.db.QueryRow(`
		SELECT data FROM checkpoints
		WHERE run_id = ? AND node_id = ?
	`, runID, nodeID).Scan(&data)

	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("load checkpoint: %w", err)
	}
	return data, nil
}

// List implements Store.
func (s *SQLiteStore) List(runID string) ([]Info, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if s.closed {
		return nil, ErrStoreClosed
	}

	rows, err := s.db.Query(`
		SELECT node_id, sequence, timestamp, LENGTH(data)
		FROM checkpoints
		WHERE run_id = ?
		ORDER BY sequence
	`, runID)
	if err != nil {
		return nil, fmt.Errorf("list checkpoints: %w", err)
	}
	defer rows.Close()

	var infos []Info
	for rows.Next() {
		var info Info
		var timestamp string
		if err := rows.Scan(&info.NodeID, &info.Sequence, &timestamp, &info.Size); err != nil {
			return nil, fmt.Errorf("scan checkpoint info: %w", err)
		}
		info.RunID = runID
		var parseErr error
		info.Timestamp, parseErr = time.Parse(time.RFC3339Nano, timestamp)
		if parseErr != nil {
			// Log but continue - timestamp is metadata, not critical for resume
			slog.Warn("failed to parse checkpoint timestamp",
				slog.String("run_id", runID),
				slog.String("node_id", info.NodeID),
				slog.String("raw_timestamp", timestamp),
				slog.String("error", parseErr.Error()))
			// info.Timestamp will be zero time
		}
		infos = append(infos, info)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate checkpoints: %w", err)
	}

	return infos, nil
}

// Delete implements Store.
func (s *SQLiteStore) Delete(runID, nodeID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.closed {
		return ErrStoreClosed
	}

	_, err := s.db.Exec(`
		DELETE FROM checkpoints
		WHERE run_id = ? AND node_id = ?
	`, runID, nodeID)
	if err != nil {
		return fmt.Errorf("delete checkpoint: %w", err)
	}
	return nil
}

// DeleteRun implements Store.
func (s *SQLiteStore) DeleteRun(runID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.closed {
		return ErrStoreClosed
	}

	_, err := s.db.Exec(`
		DELETE FROM checkpoints WHERE run_id = ?
	`, runID)
	if err != nil {
		return fmt.Errorf("delete run checkpoints: %w", err)
	}
	return nil
}

// Close implements Store.
func (s *SQLiteStore) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.closed {
		return nil
	}

	s.closed = true
	return s.db.Close()
}
