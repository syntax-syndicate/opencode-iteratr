package wizard

import (
	"os"
	"path/filepath"
	"sort"
	"strings"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/lipgloss"
)

// FileItem represents a file or directory in the file picker.
type FileItem struct {
	name  string // Name of file/directory
	path  string // Full path
	isDir bool   // True if directory
}

// ID returns a unique identifier for this item (required by ScrollItem interface).
func (f *FileItem) ID() string {
	return f.path
}

// Render returns the rendered string representation (required by ScrollItem interface).
func (f *FileItem) Render(width int) string {
	icon := "📄"
	if f.isDir {
		icon = "📁"
	}

	// Format: "icon name"
	display := icon + " " + f.name

	// Truncate if too long
	if len(display) > width-2 {
		display = display[:width-5] + "..."
	}

	return display
}

// Height returns the number of lines this item occupies (required by ScrollItem interface).
func (f *FileItem) Height() int {
	return 1
}

// FilePickerStep manages the file picker UI step.
type FilePickerStep struct {
	currentPath string      // Current directory path
	items       []*FileItem // All items in current directory
	selectedIdx int         // Index of selected item
	width       int         // Available width
	height      int         // Available height
}

// NewFilePickerStep creates a new file picker step.
func NewFilePickerStep() *FilePickerStep {
	// Start in current working directory
	cwd, err := os.Getwd()
	if err != nil {
		cwd = "."
	}

	fp := &FilePickerStep{
		currentPath: cwd,
		selectedIdx: 0,
		width:       60,
		height:      10,
	}

	// Load initial directory
	fp.loadDirectory(cwd)

	return fp
}

// loadDirectory loads files and directories from the given path.
// Filters to .md, .txt files and directories only.
func (f *FilePickerStep) loadDirectory(path string) error {
	// Read directory entries
	entries, err := os.ReadDir(path)
	if err != nil {
		return err
	}

	// Clear existing items
	f.items = make([]*FileItem, 0)

	// Add parent directory entry if not at root
	absPath, err := filepath.Abs(path)
	if err == nil && absPath != filepath.Dir(absPath) {
		// Not at root, add ".." entry
		parentPath := filepath.Dir(absPath)
		f.items = append(f.items, &FileItem{
			name:  "..",
			path:  parentPath,
			isDir: true,
		})
	}

	// Filter and add entries
	var dirs []*FileItem
	var files []*FileItem

	for _, entry := range entries {
		fullPath := filepath.Join(path, entry.Name())

		if entry.IsDir() {
			// Include all directories for navigation
			dirs = append(dirs, &FileItem{
				name:  entry.Name(),
				path:  fullPath,
				isDir: true,
			})
		} else {
			// Filter files to .md and .txt only
			ext := strings.ToLower(filepath.Ext(entry.Name()))
			if ext == ".md" || ext == ".txt" {
				files = append(files, &FileItem{
					name:  entry.Name(),
					path:  fullPath,
					isDir: false,
				})
			}
		}
	}

	// Sort directories and files separately
	sort.Slice(dirs, func(i, j int) bool {
		return strings.ToLower(dirs[i].name) < strings.ToLower(dirs[j].name)
	})
	sort.Slice(files, func(i, j int) bool {
		return strings.ToLower(files[i].name) < strings.ToLower(files[j].name)
	})

	// Combine: directories first, then files
	f.items = append(f.items, dirs...)
	f.items = append(f.items, files...)

	// Update current path
	f.currentPath = path

	// Reset selection to first item
	f.selectedIdx = 0

	return nil
}

// SetSize updates the dimensions for the file picker.
func (f *FilePickerStep) SetSize(width, height int) {
	f.width = width
	f.height = height
}

// Update handles messages for the file picker step.
func (f *FilePickerStep) Update(msg tea.Msg) tea.Cmd {
	if keyMsg, ok := msg.(tea.KeyPressMsg); ok {
		switch keyMsg.String() {
		case "up", "k":
			if len(f.items) > 0 && f.selectedIdx > 0 {
				f.selectedIdx--
			}
		case "down", "j":
			if len(f.items) > 0 && f.selectedIdx < len(f.items)-1 {
				f.selectedIdx++
			}
		case "enter":
			// Handle selection
			if len(f.items) == 0 {
				// No items to select
				return nil
			}
			if f.selectedIdx >= 0 && f.selectedIdx < len(f.items) {
				item := f.items[f.selectedIdx]
				if item.isDir {
					// Navigate into directory
					f.loadDirectory(item.path)
				} else {
					// File selected - this will be handled by parent wizard
					return func() tea.Msg {
						return FileSelectedMsg{Path: item.path}
					}
				}
			}
		case "backspace":
			// Go up one directory level
			parentPath := filepath.Dir(f.currentPath)
			if parentPath != f.currentPath {
				f.loadDirectory(parentPath)
			}
		}
	}

	return nil
}

// View renders the file picker step.
func (f *FilePickerStep) View() string {
	var b strings.Builder

	// Show current path
	b.WriteString(lipgloss.NewStyle().Foreground(lipgloss.Color("#a6adc8")).Render(f.currentPath))
	b.WriteString("\n\n")

	// Check if directory is empty (excluding parent directory entry)
	hasFiles := false
	for _, item := range f.items {
		if item.name != ".." {
			hasFiles = true
			break
		}
	}

	if len(f.items) == 0 {
		// Absolutely empty - no parent, no files (shouldn't happen but handle it)
		emptyStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#6c7086")).Italic(true)
		b.WriteString(emptyStyle.Render("Directory is empty"))
		b.WriteString("\n")
	} else if !hasFiles {
		// Show empty directory message
		emptyStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#6c7086")).Italic(true)
		b.WriteString(emptyStyle.Render("No .md or .txt files in this directory"))
		b.WriteString("\n\n")

		// Still show parent directory entry if it exists
		if len(f.items) > 0 && f.items[0].name == ".." {
			item := f.items[0]
			line := item.Render(f.width)
			line = lipgloss.NewStyle().
				Foreground(lipgloss.Color("#cba6f7")).
				Background(lipgloss.Color("#313244")).
				Bold(true).
				Render("▸ " + line)
			b.WriteString(line)
			b.WriteString("\n")
		}
	} else {
		// Show items normally
		for i, item := range f.items {
			line := item.Render(f.width)

			// Highlight selected item
			if i == f.selectedIdx {
				line = lipgloss.NewStyle().
					Foreground(lipgloss.Color("#cba6f7")).
					Background(lipgloss.Color("#313244")).
					Bold(true).
					Render("▸ " + line)
			} else {
				line = "  " + line
			}

			b.WriteString(line)
			b.WriteString("\n")
		}
	}

	// Add spacing before hint bar
	b.WriteString("\n")

	// Hint bar (context-sensitive based on directory state)
	var hintBar string
	if len(f.items) == 0 {
		// No items at all - can only go back or cancel
		hintBar = renderHintBar(
			"backspace", "go up",
			"esc", "cancel",
		)
	} else if !hasFiles && len(f.items) > 0 {
		// Empty directory with parent - show simplified hints
		hintBar = renderHintBar(
			"enter/backspace", "go up",
			"esc", "cancel",
		)
	} else {
		// Normal hints
		hintBar = renderHintBar(
			"↑↓/j/k", "navigate",
			"enter", "select",
			"backspace", "up",
			"esc", "cancel",
		)
	}
	b.WriteString(hintBar)

	return b.String()
}

// SelectedPath returns the currently selected file path (empty if directory selected).
func (f *FilePickerStep) SelectedPath() string {
	if f.selectedIdx >= 0 && f.selectedIdx < len(f.items) {
		item := f.items[f.selectedIdx]
		if !item.isDir {
			return item.path
		}
	}
	return ""
}

// FileSelectedMsg is sent when a file is selected.
type FileSelectedMsg struct {
	Path string
}
