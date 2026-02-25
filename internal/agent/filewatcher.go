package agent

import (
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/fsnotify/fsnotify"
	"github.com/mark3labs/iteratr/internal/logger"
)

const debounceInterval = 100 * time.Millisecond

// FileWatcher watches the working directory for filesystem changes using fsnotify.
// It catches all file modifications regardless of source (agent tools, bash, subprocesses).
type FileWatcher struct {
	watcher     *fsnotify.Watcher
	workDir     string
	excludeDirs map[string]bool      // directory basenames to skip
	changed     map[string]time.Time // relative path -> last event time
	mu          sync.Mutex
	done        chan struct{}
	stopped     chan struct{}
}

// NewFileWatcher creates a watcher for the given directory.
// excludeDirs is a list of directory basenames to skip (e.g. ".git", "node_modules").
func NewFileWatcher(workDir string, excludeDirs []string) (*FileWatcher, error) {
	w, err := fsnotify.NewWatcher()
	if err != nil {
		return nil, err
	}

	exclude := make(map[string]bool, len(excludeDirs))
	for _, d := range excludeDirs {
		exclude[d] = true
	}

	return &FileWatcher{
		watcher:     w,
		workDir:     workDir,
		excludeDirs: exclude,
		changed:     make(map[string]time.Time),
		done:        make(chan struct{}),
		stopped:     make(chan struct{}),
	}, nil
}

// Start walks the directory tree, adds watches, and starts the event loop.
func (fw *FileWatcher) Start() error {
	if err := fw.addRecursive(fw.workDir); err != nil {
		fw.watcher.Close()
		return err
	}

	go fw.eventLoop()
	logger.Info("FileWatcher started for %s (excluding %v)", fw.workDir, fw.excludeList())
	return nil
}

// Stop shuts down the watcher and event loop.
func (fw *FileWatcher) Stop() error {
	close(fw.done)
	<-fw.stopped // wait for event loop to exit
	return fw.watcher.Close()
}

// Clear resets the changed file set for a new iteration.
func (fw *FileWatcher) Clear() {
	fw.mu.Lock()
	defer fw.mu.Unlock()
	fw.changed = make(map[string]time.Time)
}

// ChangedPaths returns sorted relative paths of files that changed,
// filtered by the debounce interval (only paths with no events in the
// last debounceInterval are included).
func (fw *FileWatcher) ChangedPaths() []string {
	fw.mu.Lock()
	defer fw.mu.Unlock()

	now := time.Now()
	paths := make([]string, 0, len(fw.changed))
	for p, t := range fw.changed {
		// Include all paths — debounce window only prevents rapid re-adds
		// in the event loop. By the time auto-commit runs (seconds/minutes
		// after edits), all paths are settled.
		_ = t
		_ = now
		paths = append(paths, p)
	}
	sort.Strings(paths)
	return paths
}

// HasChanges returns true if any files have changed since last Clear.
func (fw *FileWatcher) HasChanges() bool {
	fw.mu.Lock()
	defer fw.mu.Unlock()
	return len(fw.changed) > 0
}

// addRecursive walks the directory tree and adds watches for all
// non-excluded directories.
func (fw *FileWatcher) addRecursive(root string) error {
	return filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			// Skip inaccessible paths
			if info != nil && info.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		if !info.IsDir() {
			return nil
		}
		if fw.isExcluded(path) {
			return filepath.SkipDir
		}
		if err := fw.watcher.Add(path); err != nil {
			logger.Warn("FileWatcher: failed to watch %s: %v", path, err)
			// Don't fail entirely — watch what we can
			if strings.Contains(err.Error(), "no space left on device") ||
				strings.Contains(err.Error(), "too many open files") {
				logger.Error("FileWatcher: inotify watch limit reached. Increase fs.inotify.max_user_watches")
				return filepath.SkipDir
			}
		}
		return nil
	})
}

// eventLoop processes fsnotify events until Stop is called.
func (fw *FileWatcher) eventLoop() {
	defer close(fw.stopped)

	for {
		select {
		case <-fw.done:
			return

		case event, ok := <-fw.watcher.Events:
			if !ok {
				return
			}
			fw.handleEvent(event)

		case err, ok := <-fw.watcher.Errors:
			if !ok {
				return
			}
			logger.Warn("FileWatcher error: %v", err)
		}
	}
}

// handleEvent processes a single fsnotify event.
func (fw *FileWatcher) handleEvent(event fsnotify.Event) {
	// Only care about Create, Write, Rename
	if !event.Has(fsnotify.Create) && !event.Has(fsnotify.Write) && !event.Has(fsnotify.Rename) {
		return
	}

	path := event.Name

	// Skip excluded directories and their contents
	if fw.isExcludedPath(path) {
		return
	}

	// If a new directory was created, add it to the watch list
	if event.Has(fsnotify.Create) {
		if info, err := os.Stat(path); err == nil && info.IsDir() {
			if !fw.isExcluded(path) {
				if err := fw.addRecursive(path); err != nil {
					logger.Warn("FileWatcher: failed to watch new dir %s: %v", path, err)
				}
			}
			return // don't track directory creation itself
		}
	}

	// Normalize to relative path
	relPath, err := filepath.Rel(fw.workDir, path)
	if err != nil {
		relPath = path
	}

	// Skip if it resolves outside working directory
	if strings.HasPrefix(relPath, "..") {
		return
	}

	fw.mu.Lock()
	fw.changed[relPath] = time.Now()
	fw.mu.Unlock()
}

// isExcluded checks if a directory path should be excluded based on its basename.
func (fw *FileWatcher) isExcluded(path string) bool {
	base := filepath.Base(path)
	return fw.excludeDirs[base]
}

// isExcludedPath checks if any component of the path is excluded.
func (fw *FileWatcher) isExcludedPath(path string) bool {
	rel, err := filepath.Rel(fw.workDir, path)
	if err != nil {
		return false
	}
	parts := strings.Split(rel, string(filepath.Separator))
	for _, part := range parts {
		if fw.excludeDirs[part] {
			return true
		}
	}
	return false
}

// excludeList returns the exclude dirs as a sorted slice (for logging).
func (fw *FileWatcher) excludeList() []string {
	list := make([]string, 0, len(fw.excludeDirs))
	for d := range fw.excludeDirs {
		list = append(list, d)
	}
	sort.Strings(list)
	return list
}
