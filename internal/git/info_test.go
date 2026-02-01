package git

import (
	"testing"
)

func TestGetInfo_CurrentRepo(t *testing.T) {
	// Test against the current repository (iteratr itself)
	info, err := GetInfo("../..")
	if err != nil {
		t.Fatalf("GetInfo failed: %v", err)
	}
	if info == nil {
		t.Fatal("Expected info for git repo, got nil")
	}

	// Should have a branch name
	if info.Branch == "" {
		t.Error("Expected non-empty branch name")
	}

	// Should have a hash
	if info.Hash == "" {
		t.Error("Expected non-empty hash")
	}
	if len(info.Hash) != 7 {
		t.Errorf("Expected 7-char hash, got %d chars: %s", len(info.Hash), info.Hash)
	}

	t.Logf("Branch: %s, Hash: %s, Dirty: %v, Ahead: %d, Behind: %d",
		info.Branch, info.Hash, info.Dirty, info.Ahead, info.Behind)
}

func TestGetInfo_NonGitDir(t *testing.T) {
	// /tmp is typically not a git repository
	info, err := GetInfo("/tmp")
	if err != nil {
		t.Fatalf("GetInfo failed: %v", err)
	}
	if info != nil {
		t.Error("Expected nil for non-git directory")
	}
}

func TestGetInfo_NoUpstream(t *testing.T) {
	// Create a temp git repo with no upstream
	dir := t.TempDir()

	// Initialize git repo
	if _, err := runGit(dir, "init"); err != nil {
		t.Fatalf("git init failed: %v", err)
	}

	// Configure user for commits
	if _, err := runGit(dir, "config", "user.email", "test@test.com"); err != nil {
		t.Fatalf("git config email failed: %v", err)
	}
	if _, err := runGit(dir, "config", "user.name", "Test"); err != nil {
		t.Fatalf("git config name failed: %v", err)
	}

	// Create initial commit (required for HEAD to exist)
	if _, err := runGit(dir, "commit", "--allow-empty", "-m", "initial"); err != nil {
		t.Fatalf("git commit failed: %v", err)
	}

	// Get info - should succeed with ahead/behind = 0 (no upstream)
	info, err := GetInfo(dir)
	if err != nil {
		t.Fatalf("GetInfo failed: %v", err)
	}
	if info == nil {
		t.Fatal("Expected info, got nil")
	}

	// Ahead/behind should be 0 when no upstream is configured
	if info.Ahead != 0 {
		t.Errorf("Expected Ahead=0 with no upstream, got %d", info.Ahead)
	}
	if info.Behind != 0 {
		t.Errorf("Expected Behind=0 with no upstream, got %d", info.Behind)
	}

	// Branch should be master or main (depends on git version)
	if info.Branch != "master" && info.Branch != "main" {
		t.Errorf("Expected branch master or main, got %s", info.Branch)
	}

	t.Logf("Branch: %s, Hash: %s, Ahead: %d, Behind: %d",
		info.Branch, info.Hash, info.Ahead, info.Behind)
}
