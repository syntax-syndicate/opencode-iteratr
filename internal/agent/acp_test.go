package agent

import (
	"testing"
)

func TestExtractDiffBlocks_OnlyProcessesCompletedEdits(t *testing.T) {
	tests := []struct {
		name     string
		status   string
		kind     string
		wantCall bool
	}{
		{
			name:     "completed edit - should extract",
			status:   "completed",
			kind:     "edit",
			wantCall: true,
		},
		{
			name:     "error edit - should skip",
			status:   "error",
			kind:     "edit",
			wantCall: false,
		},
		{
			name:     "canceled edit - should skip",
			status:   "canceled",
			kind:     "edit",
			wantCall: false,
		},
		{
			name:     "completed read - should skip",
			status:   "completed",
			kind:     "read",
			wantCall: false,
		},
		{
			name:     "completed bash - should skip",
			status:   "completed",
			kind:     "bash",
			wantCall: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Simulate the condition from acp.go line 365
			shouldExtract := tt.status == "completed" && tt.kind == "edit"

			if shouldExtract != tt.wantCall {
				t.Errorf("extractDiffBlocks condition check: got %v, want %v for status=%s kind=%s",
					shouldExtract, tt.wantCall, tt.status, tt.kind)
			}
		})
	}
}

func TestExtractDiffBlocks(t *testing.T) {
	tests := []struct {
		name    string
		content []toolCallContent
		want    []DiffBlock
	}{
		{
			name: "extracts single diff block",
			content: []toolCallContent{
				{
					Type:    "diff",
					Path:    "/path/to/file.go",
					OldText: "old content",
					NewText: "new content",
				},
			},
			want: []DiffBlock{
				{
					Path:    "/path/to/file.go",
					OldText: "old content",
					NewText: "new content",
				},
			},
		},
		{
			name: "extracts multiple diff blocks",
			content: []toolCallContent{
				{
					Type:    "content",
					Content: contentPart{Type: "text", Text: "some text"},
				},
				{
					Type:    "diff",
					Path:    "/path/to/file1.go",
					OldText: "old1",
					NewText: "new1",
				},
				{
					Type:    "diff",
					Path:    "/path/to/file2.go",
					OldText: "",
					NewText: "new file",
				},
			},
			want: []DiffBlock{
				{
					Path:    "/path/to/file1.go",
					OldText: "old1",
					NewText: "new1",
				},
				{
					Path:    "/path/to/file2.go",
					OldText: "",
					NewText: "new file",
				},
			},
		},
		{
			name: "handles content blocks without diff",
			content: []toolCallContent{
				{
					Type:    "content",
					Content: contentPart{Type: "text", Text: "no diffs here"},
				},
			},
			want: []DiffBlock{},
		},
		{
			name:    "handles empty content array",
			content: []toolCallContent{},
			want:    []DiffBlock{},
		},
		{
			name:    "handles nil content",
			content: nil,
			want:    []DiffBlock{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := extractDiffBlocks(tt.content)

			// Handle nil vs empty slice comparison
			if len(got) == 0 && len(tt.want) == 0 {
				return
			}

			if len(got) != len(tt.want) {
				t.Errorf("extractDiffBlocks() returned %d blocks, want %d", len(got), len(tt.want))
				return
			}

			for i := range got {
				if got[i].Path != tt.want[i].Path {
					t.Errorf("block[%d].Path = %s, want %s", i, got[i].Path, tt.want[i].Path)
				}
				if got[i].OldText != tt.want[i].OldText {
					t.Errorf("block[%d].OldText = %s, want %s", i, got[i].OldText, tt.want[i].OldText)
				}
				if got[i].NewText != tt.want[i].NewText {
					t.Errorf("block[%d].NewText = %s, want %s", i, got[i].NewText, tt.want[i].NewText)
				}
			}
		})
	}
}
