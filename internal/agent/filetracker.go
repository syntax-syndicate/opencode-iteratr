package agent

import (
	"path/filepath"
	"sort"
	"sync"
)

// FileChange represents a single file modification
type FileChange struct {
	Path      string // Relative path from working directory
	AbsPath   string // Absolute path
	IsNew     bool   // True if file was created (oldText was empty at extraction)
	Additions int    // Lines added (from metadata, 0 if unknown)
	Deletions int    // Lines deleted (from metadata, 0 if unknown)
}

// FileTracker tracks files modified during an iteration
type FileTracker struct {
	workDir string
	changes map[string]*FileChange // keyed by relative path
	mu      sync.Mutex
}

// NewFileTracker creates a new FileTracker for the given working directory
func NewFileTracker(workDir string) *FileTracker {
	return &FileTracker{
		workDir: workDir,
		changes: make(map[string]*FileChange),
	}
}

// RecordChange records a file modification
func (ft *FileTracker) RecordChange(absPath string, isNew bool, additions, deletions int) {
	ft.mu.Lock()
	defer ft.mu.Unlock()

	// Normalize to relative path
	relPath, err := filepath.Rel(ft.workDir, absPath)
	if err != nil {
		// If we can't make it relative, use the absolute path as-is
		relPath = absPath
	}

	// Store or update the change
	ft.changes[relPath] = &FileChange{
		Path:      relPath,
		AbsPath:   absPath,
		IsNew:     isNew,
		Additions: additions,
		Deletions: deletions,
	}
}

// Get returns the FileChange for the given relative path, or nil if not found
func (ft *FileTracker) Get(relPath string) *FileChange {
	ft.mu.Lock()
	defer ft.mu.Unlock()

	return ft.changes[relPath]
}

// Changes returns a sorted list of all file changes
func (ft *FileTracker) Changes() []*FileChange {
	ft.mu.Lock()
	defer ft.mu.Unlock()

	result := make([]*FileChange, 0, len(ft.changes))
	for _, change := range ft.changes {
		result = append(result, change)
	}

	// Sort by relative path for consistent ordering
	sort.Slice(result, func(i, j int) bool {
		return result[i].Path < result[j].Path
	})

	return result
}

// ModifiedPaths returns just the file paths, sorted
func (ft *FileTracker) ModifiedPaths() []string {
	ft.mu.Lock()
	defer ft.mu.Unlock()

	paths := make([]string, 0, len(ft.changes))
	for path := range ft.changes {
		paths = append(paths, path)
	}

	sort.Strings(paths)
	return paths
}

// Clear resets the tracker for a new iteration
func (ft *FileTracker) Clear() {
	ft.mu.Lock()
	defer ft.mu.Unlock()

	ft.changes = make(map[string]*FileChange)
}

// Count returns the number of unique files modified
func (ft *FileTracker) Count() int {
	ft.mu.Lock()
	defer ft.mu.Unlock()

	return len(ft.changes)
}

// HasChanges returns true if any files have been modified
func (ft *FileTracker) HasChanges() bool {
	ft.mu.Lock()
	defer ft.mu.Unlock()

	return len(ft.changes) > 0
}
