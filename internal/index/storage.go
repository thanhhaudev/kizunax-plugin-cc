package index

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

// ErrSchemaVersionMismatch is returned by LoadJSON when the persisted
// index's Version != CurrentSchemaVersion. Callers should rebuild.
var ErrSchemaVersionMismatch = errors.New("index schema version mismatch")

// IsSchemaVersionMismatch reports whether err is a wrapped
// ErrSchemaVersionMismatch.
func IsSchemaVersionMismatch(err error) bool {
	return errors.Is(err, ErrSchemaVersionMismatch)
}

// WriteJSON serializes idx to path via atomic temp+rename. The temp file
// is created in the same directory as path so the rename is on the same
// filesystem. Mode 0600. Parent directory is created if missing.
func WriteJSON(path string, idx *Index) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return fmt.Errorf("mkdir: %w", err)
	}
	tmp, err := os.CreateTemp(filepath.Dir(path), ".index-*.tmp")
	if err != nil {
		return fmt.Errorf("create temp: %w", err)
	}
	tmpPath := tmp.Name()
	// Ensure cleanup on early return.
	defer func() {
		if tmpPath != "" {
			_ = os.Remove(tmpPath)
		}
	}()

	enc := json.NewEncoder(tmp)
	enc.SetIndent("", "")
	if err := enc.Encode(idx); err != nil {
		tmp.Close()
		return fmt.Errorf("encode: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("close temp: %w", err)
	}
	if err := os.Chmod(tmpPath, 0o600); err != nil {
		return fmt.Errorf("chmod: %w", err)
	}
	if err := os.Rename(tmpPath, path); err != nil {
		return fmt.Errorf("rename: %w", err)
	}
	tmpPath = "" // skip cleanup, rename succeeded
	return nil
}

// LoadJSON reads and parses an index from disk. Returns
// ErrSchemaVersionMismatch if the persisted Version != CurrentSchemaVersion.
// On success, the returned Index has bySymbol populated.
func LoadJSON(path string) (*Index, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var idx Index
	if err := json.Unmarshal(data, &idx); err != nil {
		return nil, fmt.Errorf("unmarshal index: %w", err)
	}
	if idx.Version != CurrentSchemaVersion {
		return nil, fmt.Errorf("%w: file=%d current=%d", ErrSchemaVersionMismatch, idx.Version, CurrentSchemaVersion)
	}
	idx.RebuildLookups()
	return &idx, nil
}
