package index

import (
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// StaleThreshold is the age above which a loaded index is forced to full
// rescan. Catches branch-switch and external-edit scenarios without
// fs-watch.
const StaleThreshold = 1 * time.Hour

// IndexFileName is the on-disk index filename (under stateDir/index/).
const IndexFileName = "index.json"

// BuildFull walks the entire workspace, scans every supported file, and
// returns a fresh Index. Mtimes are stamped from os.Stat; Built is set
// to time.Now().
func BuildFull(ws string) (*Index, error) {
	paths, err := WalkWorkspace(ws)
	if err != nil {
		return nil, fmt.Errorf("walk: %w", err)
	}
	idx := &Index{
		Version: CurrentSchemaVersion,
		Root:    ws,
		Built:   time.Now().UnixNano(),
		Files:   make(map[string]*FileIndex, len(paths)),
	}
	for _, p := range paths {
		fi, err := ScanFile(ws, p)
		if err != nil {
			// Best-effort: skip unreadable files but continue.
			continue
		}
		if fi == nil {
			continue
		}
		idx.Files[p] = fi
	}
	idx.RebuildLookups()
	return idx, nil
}

// LoadOrBuild loads the existing index from stateDir if present and
// healthy, otherwise builds full. Applies mtime-driven incremental update
// if the loaded index is <StaleThreshold old. Persists changes.
func LoadOrBuild(stateDir, ws string) (*Index, error) {
	idxPath := filepath.Join(stateDir, "index", IndexFileName)

	idx, err := LoadJSON(idxPath)
	if err != nil {
		if os.IsNotExist(err) || IsSchemaVersionMismatch(err) {
			// Fresh build path.
			idx, err = BuildFull(ws)
			if err != nil {
				return nil, fmt.Errorf("build full: %w", err)
			}
			if writeErr := WriteJSON(idxPath, idx); writeErr != nil {
				return nil, fmt.Errorf("persist: %w", writeErr)
			}
			return idx, nil
		}
		return nil, fmt.Errorf("load: %w", err)
	}

	// Auto-stale check.
	age := time.Since(time.Unix(0, idx.Built))
	if age > StaleThreshold {
		idx, err = BuildFull(ws)
		if err != nil {
			return nil, fmt.Errorf("rebuild stale: %w", err)
		}
		if writeErr := WriteJSON(idxPath, idx); writeErr != nil {
			return nil, fmt.Errorf("persist rebuild: %w", writeErr)
		}
		return idx, nil
	}

	// Incremental update path.
	changed, err := incrementalUpdate(idx, ws)
	if err != nil {
		return nil, fmt.Errorf("incremental: %w", err)
	}
	if changed {
		if writeErr := WriteJSON(idxPath, idx); writeErr != nil {
			return nil, fmt.Errorf("persist incremental: %w", writeErr)
		}
	}
	return idx, nil
}

// incrementalUpdate stats every file in idx.Files, rescans changed ones,
// detects new files in workspace, drops vanished files. Returns true if
// any change was made (so caller can persist). idx is mutated in place.
func incrementalUpdate(idx *Index, ws string) (bool, error) {
	changed := false

	// 1. Stat existing files; rescan if mtime newer; drop if gone.
	for path, fi := range idx.Files {
		abs := filepath.Join(ws, path)
		info, err := os.Stat(abs)
		if err != nil {
			delete(idx.Files, path)
			changed = true
			continue
		}
		if info.ModTime().UnixNano() > fi.Mtime {
			newFI, scanErr := ScanFile(ws, path)
			if scanErr != nil || newFI == nil {
				continue // keep stale rather than lose entry
			}
			idx.Files[path] = newFI
			changed = true
		}
	}

	// 2. Detect new files not yet indexed.
	allPaths, err := WalkWorkspace(ws)
	if err != nil {
		return changed, err
	}
	for _, p := range allPaths {
		if _, ok := idx.Files[p]; ok {
			continue
		}
		fi, scanErr := ScanFile(ws, p)
		if scanErr != nil || fi == nil {
			continue
		}
		idx.Files[p] = fi
		changed = true
	}

	if changed {
		idx.RebuildLookups()
	}
	return changed, nil
}
