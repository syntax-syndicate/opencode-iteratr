package agent

import (
	"path/filepath"
	"sync"
	"testing"
)

func TestNewFileTracker(t *testing.T) {
	workDir := "/home/user/project"
	ft := NewFileTracker(workDir)

	if ft.workDir != workDir {
		t.Errorf("expected workDir %q, got %q", workDir, ft.workDir)
	}

	if ft.changes == nil {
		t.Error("expected changes map to be initialized")
	}

	if ft.Count() != 0 {
		t.Errorf("expected empty tracker, got count %d", ft.Count())
	}
}

func TestRecordChange_PathNormalization(t *testing.T) {
	workDir := "/home/user/project"
	ft := NewFileTracker(workDir)

	tests := []struct {
		name        string
		absPath     string
		expectedRel string
	}{
		{
			name:        "file in workdir",
			absPath:     "/home/user/project/main.go",
			expectedRel: "main.go",
		},
		{
			name:        "file in subdirectory",
			absPath:     "/home/user/project/internal/agent/filetracker.go",
			expectedRel: "internal/agent/filetracker.go",
		},
		{
			name:        "file outside workdir",
			absPath:     "/etc/config.txt",
			expectedRel: "../../../etc/config.txt",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ft.Clear()
			ft.RecordChange(tt.absPath, false, 10, 5)

			change := ft.Get(tt.expectedRel)
			if change == nil {
				t.Fatalf("expected change for path %q, got nil", tt.expectedRel)
			}

			if change.Path != tt.expectedRel {
				t.Errorf("expected relative path %q, got %q", tt.expectedRel, change.Path)
			}

			if change.AbsPath != tt.absPath {
				t.Errorf("expected absolute path %q, got %q", tt.absPath, change.AbsPath)
			}
		})
	}
}

func TestRecordChange_Deduplication(t *testing.T) {
	workDir := "/home/user/project"
	ft := NewFileTracker(workDir)

	absPath := filepath.Join(workDir, "main.go")

	// Record the same file multiple times with different stats
	ft.RecordChange(absPath, true, 5, 0)
	if ft.Count() != 1 {
		t.Errorf("expected count 1 after first record, got %d", ft.Count())
	}

	ft.RecordChange(absPath, false, 10, 3)
	if ft.Count() != 1 {
		t.Errorf("expected count 1 after second record (dedup), got %d", ft.Count())
	}

	// Verify the latest values are stored
	change := ft.Get("main.go")
	if change == nil {
		t.Fatal("expected change for main.go, got nil")
	}

	if change.IsNew {
		t.Error("expected IsNew to be false (updated value)")
	}

	if change.Additions != 10 {
		t.Errorf("expected additions 10, got %d", change.Additions)
	}

	if change.Deletions != 3 {
		t.Errorf("expected deletions 3, got %d", change.Deletions)
	}
}

func TestRecordChange_MultipleFiles(t *testing.T) {
	workDir := "/home/user/project"
	ft := NewFileTracker(workDir)

	files := []string{
		filepath.Join(workDir, "main.go"),
		filepath.Join(workDir, "internal/agent/filetracker.go"),
		filepath.Join(workDir, "cmd/build.go"),
	}

	for i, f := range files {
		ft.RecordChange(f, i == 0, i+1, i)
	}

	if ft.Count() != 3 {
		t.Errorf("expected count 3, got %d", ft.Count())
	}

	if !ft.HasChanges() {
		t.Error("expected HasChanges to be true")
	}
}

func TestGet_NotFound(t *testing.T) {
	workDir := "/home/user/project"
	ft := NewFileTracker(workDir)

	change := ft.Get("nonexistent.go")
	if change != nil {
		t.Errorf("expected nil for nonexistent file, got %+v", change)
	}
}

func TestChanges_Sorted(t *testing.T) {
	workDir := "/home/user/project"
	ft := NewFileTracker(workDir)

	// Add files in non-alphabetical order
	files := []string{"z.go", "a.go", "m.go", "b.go"}
	for _, f := range files {
		ft.RecordChange(filepath.Join(workDir, f), false, 1, 0)
	}

	changes := ft.Changes()
	if len(changes) != 4 {
		t.Fatalf("expected 4 changes, got %d", len(changes))
	}

	// Verify alphabetical sorting
	expected := []string{"a.go", "b.go", "m.go", "z.go"}
	for i, exp := range expected {
		if changes[i].Path != exp {
			t.Errorf("expected changes[%d].Path = %q, got %q", i, exp, changes[i].Path)
		}
	}
}

func TestModifiedPaths_Sorted(t *testing.T) {
	workDir := "/home/user/project"
	ft := NewFileTracker(workDir)

	// Add files in non-alphabetical order
	files := []string{"z.go", "a.go", "m.go", "b.go"}
	for _, f := range files {
		ft.RecordChange(filepath.Join(workDir, f), false, 1, 0)
	}

	paths := ft.ModifiedPaths()
	if len(paths) != 4 {
		t.Fatalf("expected 4 paths, got %d", len(paths))
	}

	// Verify alphabetical sorting
	expected := []string{"a.go", "b.go", "m.go", "z.go"}
	for i, exp := range expected {
		if paths[i] != exp {
			t.Errorf("expected paths[%d] = %q, got %q", i, exp, paths[i])
		}
	}
}

func TestClear(t *testing.T) {
	workDir := "/home/user/project"
	ft := NewFileTracker(workDir)

	// Add some files
	ft.RecordChange(filepath.Join(workDir, "a.go"), false, 5, 2)
	ft.RecordChange(filepath.Join(workDir, "b.go"), true, 10, 0)

	if ft.Count() != 2 {
		t.Fatalf("expected count 2 before clear, got %d", ft.Count())
	}

	// Clear
	ft.Clear()

	if ft.Count() != 0 {
		t.Errorf("expected count 0 after clear, got %d", ft.Count())
	}

	if ft.HasChanges() {
		t.Error("expected HasChanges to be false after clear")
	}

	if len(ft.Changes()) != 0 {
		t.Errorf("expected 0 changes after clear, got %d", len(ft.Changes()))
	}

	if len(ft.ModifiedPaths()) != 0 {
		t.Errorf("expected 0 paths after clear, got %d", len(ft.ModifiedPaths()))
	}
}

func TestHasChanges(t *testing.T) {
	workDir := "/home/user/project"
	ft := NewFileTracker(workDir)

	if ft.HasChanges() {
		t.Error("expected HasChanges to be false for empty tracker")
	}

	ft.RecordChange(filepath.Join(workDir, "main.go"), false, 1, 0)

	if !ft.HasChanges() {
		t.Error("expected HasChanges to be true after recording change")
	}

	ft.Clear()

	if ft.HasChanges() {
		t.Error("expected HasChanges to be false after clear")
	}
}

func TestFileChange_Fields(t *testing.T) {
	workDir := "/home/user/project"
	ft := NewFileTracker(workDir)

	absPath := filepath.Join(workDir, "internal/agent/filetracker.go")
	ft.RecordChange(absPath, true, 100, 0)

	change := ft.Get("internal/agent/filetracker.go")
	if change == nil {
		t.Fatal("expected change, got nil")
	}

	if change.Path != "internal/agent/filetracker.go" {
		t.Errorf("expected Path %q, got %q", "internal/agent/filetracker.go", change.Path)
	}

	if change.AbsPath != absPath {
		t.Errorf("expected AbsPath %q, got %q", absPath, change.AbsPath)
	}

	if !change.IsNew {
		t.Error("expected IsNew to be true")
	}

	if change.Additions != 100 {
		t.Errorf("expected Additions 100, got %d", change.Additions)
	}

	if change.Deletions != 0 {
		t.Errorf("expected Deletions 0, got %d", change.Deletions)
	}
}

func TestConcurrentAccess(t *testing.T) {
	workDir := "/home/user/project"
	ft := NewFileTracker(workDir)

	const numGoroutines = 10
	const numOpsPerGoroutine = 100

	var wg sync.WaitGroup
	wg.Add(numGoroutines)

	// Concurrent writes
	for i := 0; i < numGoroutines; i++ {
		go func(id int) {
			defer wg.Done()
			for j := 0; j < numOpsPerGoroutine; j++ {
				filename := filepath.Join(workDir, "file.go")
				ft.RecordChange(filename, false, id*100+j, id)
			}
		}(i)
	}

	wg.Wait()

	// Should only have one entry (deduplicated)
	if ft.Count() != 1 {
		t.Errorf("expected count 1 after concurrent writes, got %d", ft.Count())
	}

	// Concurrent reads
	wg.Add(numGoroutines)
	for i := 0; i < numGoroutines; i++ {
		go func() {
			defer wg.Done()
			for j := 0; j < numOpsPerGoroutine; j++ {
				_ = ft.Get("file.go")
				_ = ft.Changes()
				_ = ft.ModifiedPaths()
				_ = ft.Count()
				_ = ft.HasChanges()
			}
		}()
	}

	wg.Wait()
}

func TestPathNormalization_EdgeCases(t *testing.T) {
	workDir := "/home/user/project"
	ft := NewFileTracker(workDir)

	tests := []struct {
		name        string
		absPath     string
		description string
	}{
		{
			name:        "trailing slash in path",
			absPath:     "/home/user/project/main.go",
			description: "should normalize correctly",
		},
		{
			name:        "dot segments",
			absPath:     "/home/user/project/./internal/../main.go",
			description: "filepath.Rel should clean",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ft.Clear()
			ft.RecordChange(tt.absPath, false, 1, 0)

			if ft.Count() != 1 {
				t.Errorf("expected 1 file recorded, got %d", ft.Count())
			}
		})
	}
}

func TestChanges_EmptyTracker(t *testing.T) {
	workDir := "/home/user/project"
	ft := NewFileTracker(workDir)

	changes := ft.Changes()
	if changes == nil {
		t.Error("expected non-nil slice, got nil")
	}

	if len(changes) != 0 {
		t.Errorf("expected empty slice, got length %d", len(changes))
	}
}

func TestModifiedPaths_EmptyTracker(t *testing.T) {
	workDir := "/home/user/project"
	ft := NewFileTracker(workDir)

	paths := ft.ModifiedPaths()
	if paths == nil {
		t.Error("expected non-nil slice, got nil")
	}

	if len(paths) != 0 {
		t.Errorf("expected empty slice, got length %d", len(paths))
	}
}
