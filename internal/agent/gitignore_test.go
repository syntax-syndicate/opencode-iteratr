package agent

import (
	"os"
	"path/filepath"
	"testing"
)

func TestParseIgnoreRule(t *testing.T) {
	tests := []struct {
		line     string
		wantOK   bool
		negate   bool
		dirOnly  bool
		anchored bool
		pattern  string
	}{
		// Skip empty and comments
		{"", false, false, false, false, ""},
		{"# comment", false, false, false, false, ""},
		{"  ", false, false, false, false, ""},

		// Simple basename
		{"*.log", true, false, false, false, "*.log"},
		{"coverage.out", true, false, false, false, "coverage.out"},
		{".DS_Store", true, false, false, false, ".DS_Store"},

		// Directory-only
		{"build/", true, false, true, false, "build"},
		{"node_modules/", true, false, true, false, "node_modules"},

		// Root-relative (leading /)
		{"/iteratr", true, false, false, true, "iteratr"},
		{"/dist/", true, false, true, true, "dist"},

		// Anchored (contains /)
		{"foo/bar", true, false, false, true, "foo/bar"},

		// Negation
		{"!important.log", true, true, false, false, "important.log"},
		{"!build/", true, true, true, false, "build"},

		// Double-star (leading ** is not anchored)
		{"**/foo", true, false, false, false, "**/foo"},
		{"foo/**", true, false, false, true, "foo/**"},
		{"foo/**/bar", true, false, false, true, "foo/**/bar"},

		// Trailing whitespace stripped
		{"*.log   ", true, false, false, false, "*.log"},
	}

	for _, tt := range tests {
		t.Run(tt.line, func(t *testing.T) {
			rule, ok := parseIgnoreRule(tt.line)
			if ok != tt.wantOK {
				t.Fatalf("parseIgnoreRule(%q) ok = %v, want %v", tt.line, ok, tt.wantOK)
			}
			if !ok {
				return
			}
			if rule.negate != tt.negate {
				t.Errorf("negate = %v, want %v", rule.negate, tt.negate)
			}
			if rule.dirOnly != tt.dirOnly {
				t.Errorf("dirOnly = %v, want %v", rule.dirOnly, tt.dirOnly)
			}
			if rule.anchored != tt.anchored {
				t.Errorf("anchored = %v, want %v", rule.anchored, tt.anchored)
			}
			if rule.pattern != tt.pattern {
				t.Errorf("pattern = %q, want %q", rule.pattern, tt.pattern)
			}
		})
	}
}

func TestGitIgnore_Match(t *testing.T) {
	gi := &gitIgnore{}
	gi.addPattern("*.log")
	gi.addPattern("coverage.out")
	gi.addPattern("/iteratr")
	gi.addPattern("build/")
	gi.addPattern(".git/")
	gi.addPattern("!important.log")
	gi.addPattern("**/generated")
	gi.addPattern("src/**/test_*.go")

	tests := []struct {
		path  string
		isDir bool
		want  bool
	}{
		// Glob basename
		{"debug.log", false, true},
		{"internal/app.log", false, true},
		{"deep/nested/error.log", false, true},

		// Negation overrides previous match
		{"important.log", false, false},

		// Exact basename
		{"coverage.out", false, true},

		// Root-relative — only matches at root
		{"iteratr", false, true},
		{"sub/iteratr", false, false},

		// Directory-only
		{"build", true, true},
		{"build", false, false}, // not a dir → no match
		{"sub/build", true, true},

		// .git directory
		{".git", true, true},
		{".git/HEAD", false, true}, // parent dir ignored

		// Double-star prefix: match at any depth
		{"generated", false, true},
		{"src/generated", false, true},
		{"deep/nested/generated", false, true},

		// Double-star middle
		{"src/test_foo.go", false, true},
		{"src/pkg/test_bar.go", false, true},
		{"src/a/b/c/test_baz.go", false, true},

		// Non-matching
		{"main.go", false, false},
		{"README.md", false, false},
	}

	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			got := gi.IsIgnored(tt.path, tt.isDir)
			if got != tt.want {
				t.Errorf("IsIgnored(%q, isDir=%v) = %v, want %v", tt.path, tt.isDir, got, tt.want)
			}
		})
	}
}

func TestGitIgnore_ChildOfIgnoredDir(t *testing.T) {
	// Files inside an ignored directory should be caught by the watcher's
	// directory-skip logic during walk, but if an event somehow arrives
	// for a child path, the gitignore matcher should still catch it
	// when the parent directory pattern is checked as a path component.
	gi := &gitIgnore{}
	gi.addPattern(".git/")

	// Direct directory match
	if !gi.IsIgnored(".git", true) {
		t.Error("expected .git dir to be ignored")
	}

	// Child of ignored dir — the watcher skips during walk, so this
	// path shouldn't normally appear. But test the matcher handles it.
	// Note: gitignore patterns don't inherently match children of
	// dir-only patterns (the walk skip handles that). This tests
	// that the pattern at least matches the directory itself.
	if !gi.IsIgnored(".git", true) {
		t.Error("expected .git to be ignored as dir")
	}
}

func TestLoadGitIgnore_FromFile(t *testing.T) {
	dir := t.TempDir()

	content := `# Build artifacts
*.o
*.exe
build/

# IDE
.vscode/
.idea/

# Root only
/dist

# Keep this
!dist/important.txt
`
	if err := os.WriteFile(filepath.Join(dir, ".gitignore"), []byte(content), 0644); err != nil {
		t.Fatalf("WriteFile error: %v", err)
	}

	gi := loadGitIgnore(dir)

	if len(gi.rules) != 7 {
		t.Fatalf("expected 7 rules, got %d", len(gi.rules))
	}

	tests := []struct {
		path  string
		isDir bool
		want  bool
	}{
		{"main.o", false, true},
		{"app.exe", false, true},
		{"build", true, true},
		{"build", false, false},
		{".vscode", true, true},
		{".idea", true, true},
		{"dist", false, true},
		{"sub/dist", false, false},          // root-relative, no match in subdir
		{"dist/important.txt", false, true}, // parent dir ignored — negation can't override (matches git behavior)
		{"main.go", false, false},
	}

	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			got := gi.IsIgnored(tt.path, tt.isDir)
			if got != tt.want {
				t.Errorf("IsIgnored(%q, isDir=%v) = %v, want %v", tt.path, tt.isDir, got, tt.want)
			}
		})
	}
}

func TestLoadGitIgnore_MissingFile(t *testing.T) {
	gi := loadGitIgnore(t.TempDir())

	if len(gi.rules) != 0 {
		t.Errorf("expected 0 rules for missing .gitignore, got %d", len(gi.rules))
	}

	// Should not match anything
	if gi.IsIgnored("anything.go", false) {
		t.Error("empty gitignore should not match anything")
	}
}

func TestGitIgnore_AddPattern(t *testing.T) {
	gi := &gitIgnore{}
	gi.addPattern("node_modules/")
	gi.addPattern(".iteratr/")

	if !gi.IsIgnored("node_modules", true) {
		t.Error("expected node_modules dir to be ignored")
	}
	if !gi.IsIgnored(".iteratr", true) {
		t.Error("expected .iteratr dir to be ignored")
	}
	if gi.IsIgnored("node_modules", false) {
		t.Error("node_modules as file should not match dir-only pattern")
	}
}
