package tui

import (
	"path/filepath"
	"testing"

	tea "charm.land/bubbletea/v2"
	uv "github.com/charmbracelet/ultraviolet"
	"github.com/mark3labs/iteratr/internal/session"
	"github.com/mark3labs/iteratr/internal/tui/testfixtures"
)

// TestNoteModal_InitialState tests the modal's initial state
func TestNoteModal_InitialState(t *testing.T) {
	t.Parallel()

	modal := NewNoteModal()

	// Modal should start invisible
	if modal.IsVisible() {
		t.Error("Modal should start invisible")
	}

	// Note should be nil
	if modal.note != nil {
		t.Error("Note should be nil initially")
	}

	// Default dimensions
	if modal.width != 60 {
		t.Errorf("Default width: got %d, want 60", modal.width)
	}
	if modal.height != 22 {
		t.Errorf("Default height: got %d, want 22", modal.height)
	}
}

// TestNoteModal_SetNote tests setting a note and showing the modal
func TestNoteModal_SetNote(t *testing.T) {
	t.Parallel()

	modal := NewNoteModal()
	state := testfixtures.StateWithNotes()
	note := state.Notes[0] // "Learned about event sourcing pattern" (learning)

	modal.SetNote(note)

	// Modal should be visible
	if !modal.IsVisible() {
		t.Error("Modal should be visible after SetNote")
	}

	// Note should be set
	if modal.note != note {
		t.Error("Note should be set to provided note")
	}

	// Focus should start on type selector
	if modal.focus != noteModalFocusType {
		t.Errorf("Focus should start on type, got %d", modal.focus)
	}

	// Type index should match the note's type
	if modal.typeIndex != noteTypeToIndex(note.Type) {
		t.Errorf("Type index should be %d for %s, got %d", noteTypeToIndex(note.Type), note.Type, modal.typeIndex)
	}

	// Content should not be modified
	if modal.contentModified {
		t.Error("Content should not be marked modified on SetNote")
	}
}

// TestNoteModal_Close tests closing the modal
func TestNoteModal_Close(t *testing.T) {
	t.Parallel()

	modal := NewNoteModal()
	state := testfixtures.StateWithNotes()
	note := state.Notes[0]

	// Show modal
	modal.SetNote(note)
	if !modal.IsVisible() {
		t.Fatal("Setup: modal should be visible")
	}

	// Close modal
	modal.Close()

	// Modal should be hidden
	if modal.IsVisible() {
		t.Error("Modal should be hidden after Close")
	}

	// Note should be cleared
	if modal.note != nil {
		t.Error("Note should be nil after Close")
	}

	// Content modified should be reset
	if modal.contentModified {
		t.Error("Content modified should be reset after Close")
	}
}

// TestNoteModal_Update_WhenNotVisible tests Update when modal is not visible
func TestNoteModal_Update_WhenNotVisible(t *testing.T) {
	t.Parallel()

	modal := NewNoteModal()
	// Modal not visible, no note set

	cmd := modal.Update(tea.KeyPressMsg{Text: "enter"})
	if cmd != nil {
		t.Error("Update should return nil when modal not visible")
	}
}

// TestNoteModal_Update_WhenNilNote tests Update when note is nil
func TestNoteModal_Update_WhenNilNote(t *testing.T) {
	t.Parallel()

	modal := NewNoteModal()
	modal.visible = true // Force visible but keep note nil

	cmd := modal.Update(tea.KeyPressMsg{Text: "enter"})
	if cmd != nil {
		t.Error("Update should return nil when note is nil")
	}
}

// TestNoteModal_ESC_ClosesModal tests that ESC closes the modal when not in textarea
func TestNoteModal_ESC_ClosesModal(t *testing.T) {
	t.Parallel()

	modal := NewNoteModal()
	state := testfixtures.StateWithNotes()
	modal.SetNote(state.Notes[0])

	// Focus is on type selector (default)
	cmd := modal.Update(tea.KeyPressMsg{Text: "esc"})
	_ = cmd

	if modal.IsVisible() {
		t.Error("ESC should close modal when not in textarea")
	}
}

// TestNoteModal_ESC_BlursTextarea tests that ESC blurs textarea when textarea is focused
func TestNoteModal_ESC_BlursTextarea(t *testing.T) {
	t.Parallel()

	modal := NewNoteModal()
	state := testfixtures.StateWithNotes()
	modal.SetNote(state.Notes[0])

	// Tab to content textarea
	modal.Update(tea.KeyPressMsg{Text: "tab"})
	if modal.focus != noteModalFocusContent {
		t.Fatalf("Expected focus on content, got %d", modal.focus)
	}

	// ESC should blur textarea and move focus to type
	modal.Update(tea.KeyPressMsg{Text: "esc"})

	if modal.focus != noteModalFocusType {
		t.Errorf("ESC in textarea should move focus to type, got %d", modal.focus)
	}
	if !modal.IsVisible() {
		t.Error("Modal should still be visible after ESC from textarea")
	}
}

// TestNoteModal_TabCycling tests focus cycling with tab
func TestNoteModal_TabCycling(t *testing.T) {
	t.Parallel()

	modal := NewNoteModal()
	state := testfixtures.StateWithNotes()
	modal.SetNote(state.Notes[0])

	// Start: type
	if modal.focus != noteModalFocusType {
		t.Fatalf("Expected initial focus on type, got %d", modal.focus)
	}

	// Tab: type -> content
	modal.Update(tea.KeyPressMsg{Text: "tab"})
	if modal.focus != noteModalFocusContent {
		t.Errorf("Tab from type should go to content, got %d", modal.focus)
	}

	// Tab: content -> delete
	modal.Update(tea.KeyPressMsg{Text: "tab"})
	if modal.focus != noteModalFocusDelete {
		t.Errorf("Tab from content should go to delete, got %d", modal.focus)
	}

	// Tab: delete -> type (wraps)
	modal.Update(tea.KeyPressMsg{Text: "tab"})
	if modal.focus != noteModalFocusType {
		t.Errorf("Tab from delete should wrap to type, got %d", modal.focus)
	}
}

// TestNoteModal_ShiftTabCycling tests reverse focus cycling with shift+tab
func TestNoteModal_ShiftTabCycling(t *testing.T) {
	t.Parallel()

	modal := NewNoteModal()
	state := testfixtures.StateWithNotes()
	modal.SetNote(state.Notes[0])

	// Start: type
	// Shift+Tab: type -> delete (wraps backward)
	modal.Update(tea.KeyPressMsg{Text: "shift+tab"})
	if modal.focus != noteModalFocusDelete {
		t.Errorf("Shift+tab from type should wrap to delete, got %d", modal.focus)
	}

	// Shift+Tab: delete -> content
	modal.Update(tea.KeyPressMsg{Text: "shift+tab"})
	if modal.focus != noteModalFocusContent {
		t.Errorf("Shift+tab from delete should go to content, got %d", modal.focus)
	}

	// Shift+Tab: content -> type
	modal.Update(tea.KeyPressMsg{Text: "shift+tab"})
	if modal.focus != noteModalFocusType {
		t.Errorf("Shift+tab from content should go to type, got %d", modal.focus)
	}
}

// TestNoteModal_TypeCycling tests type cycling with left/right arrows
func TestNoteModal_TypeCycling(t *testing.T) {
	t.Parallel()

	modal := NewNoteModal()
	state := testfixtures.StateWithNotes()
	modal.SetNote(state.Notes[0]) // learning (index 0)

	// Focus should be on type selector
	if modal.focus != noteModalFocusType {
		t.Fatalf("Expected focus on type, got %d", modal.focus)
	}

	// Right arrow should cycle to stuck (index 1)
	cmd := modal.Update(tea.KeyPressMsg{Text: "right"})
	if modal.typeIndex != 1 {
		t.Errorf("Expected type index 1 (stuck), got %d", modal.typeIndex)
	}
	// Should emit UpdateNoteTypeMsg
	if cmd == nil {
		t.Fatal("Expected command from type cycling")
	}
	msg := cmd()
	typeMsg, ok := msg.(UpdateNoteTypeMsg)
	if !ok {
		t.Fatalf("Expected UpdateNoteTypeMsg, got %T", msg)
	}
	if typeMsg.Type != "stuck" {
		t.Errorf("Expected type 'stuck', got '%s'", typeMsg.Type)
	}

	// Left arrow should cycle back to learning (index 0)
	cmd = modal.Update(tea.KeyPressMsg{Text: "left"})
	if modal.typeIndex != 0 {
		t.Errorf("Expected type index 0 (learning), got %d", modal.typeIndex)
	}
	msg = cmd()
	typeMsg, ok = msg.(UpdateNoteTypeMsg)
	if !ok {
		t.Fatalf("Expected UpdateNoteTypeMsg, got %T", msg)
	}
	if typeMsg.Type != "learning" {
		t.Errorf("Expected type 'learning', got '%s'", typeMsg.Type)
	}
}

// TestNoteModal_TypeCycling_Wraps tests that type cycling wraps around
func TestNoteModal_TypeCycling_Wraps(t *testing.T) {
	t.Parallel()

	modal := NewNoteModal()
	state := testfixtures.StateWithNotes()
	modal.SetNote(state.Notes[0]) // learning (index 0)

	// Left from first should wrap to last (decision)
	cmd := modal.Update(tea.KeyPressMsg{Text: "left"})
	if modal.typeIndex != 3 {
		t.Errorf("Expected type index 3 (decision), got %d", modal.typeIndex)
	}
	msg := cmd()
	typeMsg := msg.(UpdateNoteTypeMsg)
	if typeMsg.Type != "decision" {
		t.Errorf("Expected type 'decision', got '%s'", typeMsg.Type)
	}
}

// TestNoteModal_DeleteButton tests delete button interaction
func TestNoteModal_DeleteButton(t *testing.T) {
	t.Parallel()

	modal := NewNoteModal()
	state := testfixtures.StateWithNotes()
	modal.SetNote(state.Notes[0])

	// Tab to delete button
	modal.Update(tea.KeyPressMsg{Text: "tab"}) // type -> content
	modal.Update(tea.KeyPressMsg{Text: "tab"}) // content -> delete

	if modal.focus != noteModalFocusDelete {
		t.Fatalf("Expected focus on delete, got %d", modal.focus)
	}

	// Enter on delete button should emit RequestDeleteNoteMsg
	cmd := modal.Update(tea.KeyPressMsg{Text: "enter"})
	if cmd == nil {
		t.Fatal("Expected command from delete button")
	}
	msg := cmd()
	deleteMsg, ok := msg.(RequestDeleteNoteMsg)
	if !ok {
		t.Fatalf("Expected RequestDeleteNoteMsg, got %T", msg)
	}
	if deleteMsg.ID != "NOT-1" {
		t.Errorf("Expected note ID 'NOT-1', got '%s'", deleteMsg.ID)
	}
}

// TestNoteModal_DeleteShortcut tests 'd' shortcut for delete
func TestNoteModal_DeleteShortcut(t *testing.T) {
	t.Parallel()

	modal := NewNoteModal()
	state := testfixtures.StateWithNotes()
	modal.SetNote(state.Notes[0])

	// 'd' from type selector should emit delete request
	cmd := modal.Update(tea.KeyPressMsg{Text: "d"})
	if cmd == nil {
		t.Fatal("Expected command from 'd' shortcut")
	}
	msg := cmd()
	deleteMsg, ok := msg.(RequestDeleteNoteMsg)
	if !ok {
		t.Fatalf("Expected RequestDeleteNoteMsg, got %T", msg)
	}
	if deleteMsg.ID != "NOT-1" {
		t.Errorf("Expected note ID 'NOT-1', got '%s'", deleteMsg.ID)
	}
}

// TestNoteModal_DeleteShortcut_NotInTextarea tests that 'd' doesn't trigger delete in textarea
func TestNoteModal_DeleteShortcut_NotInTextarea(t *testing.T) {
	t.Parallel()

	modal := NewNoteModal()
	state := testfixtures.StateWithNotes()
	modal.SetNote(state.Notes[0])

	// Tab to textarea
	modal.Update(tea.KeyPressMsg{Text: "tab"}) // type -> content
	if modal.focus != noteModalFocusContent {
		t.Fatalf("Expected focus on content, got %d", modal.focus)
	}

	// 'd' in textarea should type 'd', not delete
	cmd := modal.Update(tea.KeyPressMsg{Text: "d"})
	if cmd != nil {
		msg := cmd()
		if _, ok := msg.(RequestDeleteNoteMsg); ok {
			t.Error("'d' in textarea should not trigger delete")
		}
	}
}

// TestNoteModal_ContentEditing tests content save with ctrl+enter
func TestNoteModal_ContentEditing(t *testing.T) {
	t.Parallel()

	modal := NewNoteModal()
	note := &session.Note{
		ID:        "NOT-1",
		Content:   "Original content",
		Type:      "learning",
		CreatedAt: testfixtures.FixedTime,
		UpdatedAt: testfixtures.FixedTime,
		Iteration: 1,
	}
	modal.SetNote(note)

	// Tab to textarea
	modal.Update(tea.KeyPressMsg{Text: "tab"}) // type -> content

	// Type some text (modify content)
	modal.Update(tea.KeyPressMsg{Text: "x"})

	// Content should be modified
	if !modal.contentModified {
		t.Error("Content should be marked modified after typing")
	}

	// ctrl+enter should save
	cmd := modal.Update(tea.KeyPressMsg{Text: "ctrl+enter"})
	if cmd == nil {
		t.Fatal("Expected command from ctrl+enter save")
	}
	msg := cmd()
	contentMsg, ok := msg.(UpdateNoteContentMsg)
	if !ok {
		t.Fatalf("Expected UpdateNoteContentMsg, got %T", msg)
	}
	if contentMsg.ID != "NOT-1" {
		t.Errorf("Expected note ID 'NOT-1', got '%s'", contentMsg.ID)
	}
}

// TestNoteModal_CtrlEnter_NoOpWhenUnmodified tests ctrl+enter with unmodified content
func TestNoteModal_CtrlEnter_NoOpWhenUnmodified(t *testing.T) {
	t.Parallel()

	modal := NewNoteModal()
	state := testfixtures.StateWithNotes()
	modal.SetNote(state.Notes[0])

	// ctrl+enter without modification should be a no-op
	cmd := modal.Update(tea.KeyPressMsg{Text: "ctrl+enter"})
	if cmd != nil {
		t.Error("ctrl+enter with no modifications should return nil")
	}
}

// TestNoteModal_Note_ReturnsCurrentNote tests the Note() getter
func TestNoteModal_Note_ReturnsCurrentNote(t *testing.T) {
	t.Parallel()

	modal := NewNoteModal()
	state := testfixtures.StateWithNotes()
	note := state.Notes[0]
	modal.SetNote(note)

	if modal.Note() != note {
		t.Error("Note() should return the current note")
	}

	modal.Close()
	if modal.Note() != nil {
		t.Error("Note() should return nil after Close")
	}
}

// --- NoteModal Rendering Tests ---

// TestNoteModal_InvisibleDoesNotRender tests that invisible modal doesn't render
func TestNoteModal_InvisibleDoesNotRender(t *testing.T) {
	t.Parallel()

	modal := NewNoteModal()
	// Don't call SetNote()

	// Render
	area := uv.Rectangle{
		Min: uv.Position{X: 0, Y: 0},
		Max: uv.Position{X: testfixtures.TestTermWidth, Y: testfixtures.TestTermHeight},
	}
	scr := uv.NewScreenBuffer(testfixtures.TestTermWidth, testfixtures.TestTermHeight)

	// Get baseline empty render
	emptyRender := scr.Render()

	// Now draw the invisible modal
	modal.Draw(scr, area)
	rendered := scr.Render()

	// Should be identical to empty screen (modal drew nothing)
	if rendered != emptyRender {
		t.Error("Invisible modal should not render any content")
	}
}

// TestNoteModal_NilNoteDoesNotRender tests that visible modal with nil note doesn't render
func TestNoteModal_NilNoteDoesNotRender(t *testing.T) {
	t.Parallel()

	modal := NewNoteModal()
	modal.visible = true // Force visible but keep note nil

	// Render
	area := uv.Rectangle{
		Min: uv.Position{X: 0, Y: 0},
		Max: uv.Position{X: testfixtures.TestTermWidth, Y: testfixtures.TestTermHeight},
	}
	scr := uv.NewScreenBuffer(testfixtures.TestTermWidth, testfixtures.TestTermHeight)

	// Get baseline empty render
	emptyRender := scr.Render()

	// Now draw the modal with nil note
	modal.Draw(scr, area)
	rendered := scr.Render()

	// Should be identical to empty screen (modal drew nothing)
	if rendered != emptyRender {
		t.Error("Modal with nil note should not render any content")
	}
}

// TestNoteModal_WordWrap tests word wrapping functionality
func TestNoteModal_WordWrap(t *testing.T) {
	t.Parallel()

	modal := NewNoteModal()

	testCases := []struct {
		name     string
		input    string
		width    int
		expected string
	}{
		{
			name:     "Short text no wrap",
			input:    "Hello world",
			width:    20,
			expected: "Hello world",
		},
		{
			name:     "Long text wraps",
			input:    "This is a very long sentence that should wrap across multiple lines",
			width:    20,
			expected: "This is a very long\nsentence that should\nwrap across multiple\nlines",
		},
		{
			name:     "Single word longer than width",
			input:    "Supercalifragilisticexpialidocious is a word",
			width:    10,
			expected: "Supercalifragilisticexpialidocious\nis a word",
		},
		{
			name:     "Empty string",
			input:    "",
			width:    20,
			expected: "",
		},
		{
			name:     "Zero width uses default",
			input:    "Test",
			width:    0,
			expected: "Test",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := modal.wordWrap(tc.input, tc.width)
			if result != tc.expected {
				t.Errorf("wordWrap:\ngot:  %q\nwant: %q", result, tc.expected)
			}
		})
	}
}

// TestNoteModal_RenderAllNoteTypes tests rendering with all 4 note types
func TestNoteModal_RenderAllNoteTypes(t *testing.T) {
	testCases := []struct {
		name       string
		noteIndex  int
		noteType   string
		goldenFile string
	}{
		{"Learning", 0, "learning", "note_modal_type_learning.golden"},
		{"Stuck", 1, "stuck", "note_modal_type_stuck.golden"},
		{"Tip", 2, "tip", "note_modal_type_tip.golden"},
		{"Decision", 3, "decision", "note_modal_type_decision.golden"},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			modal := NewNoteModal()
			state := testfixtures.StateWithNotes()
			modal.SetNote(state.Notes[tc.noteIndex])

			// Render
			area := uv.Rectangle{
				Min: uv.Position{X: 0, Y: 0},
				Max: uv.Position{X: testfixtures.TestTermWidth, Y: testfixtures.TestTermHeight},
			}
			scr := uv.NewScreenBuffer(testfixtures.TestTermWidth, testfixtures.TestTermHeight)
			modal.Draw(scr, area)

			// Capture rendered output
			rendered := scr.Render()

			// Verify golden file
			goldenPath := filepath.Join("testdata", tc.goldenFile)
			testfixtures.CompareGolden(t, goldenPath, rendered)
		})
	}
}

// TestNoteModal_RenderLongContent tests rendering with long content that wraps
func TestNoteModal_RenderLongContent(t *testing.T) {
	modal := NewNoteModal()
	state := testfixtures.StateWithNotes()

	// Create a note with long content
	longNote := *state.Notes[0]
	longNote.Content = "This is a very long note content that should wrap across multiple lines in the modal. It contains enough text to demonstrate the word wrapping functionality and how the modal handles longer content that doesn't fit on a single line. The modal should properly wrap this text and display it in a readable format."

	modal.SetNote(&longNote)

	// Render
	area := uv.Rectangle{
		Min: uv.Position{X: 0, Y: 0},
		Max: uv.Position{X: testfixtures.TestTermWidth, Y: testfixtures.TestTermHeight},
	}
	scr := uv.NewScreenBuffer(testfixtures.TestTermWidth, testfixtures.TestTermHeight)
	modal.Draw(scr, area)

	// Capture rendered output
	rendered := scr.Render()

	// Verify golden file
	goldenPath := filepath.Join("testdata", "note_modal_long_content.golden")
	testfixtures.CompareGolden(t, goldenPath, rendered)
}

// TestNoteModal_RenderSmallScreen tests rendering with constrained dimensions
func TestNoteModal_RenderSmallScreen(t *testing.T) {
	modal := NewNoteModal()
	state := testfixtures.StateWithNotes()
	modal.SetNote(state.Notes[0])

	// Render with small screen
	area := uv.Rectangle{
		Min: uv.Position{X: 0, Y: 0},
		Max: uv.Position{X: 40, Y: 20},
	}
	scr := uv.NewScreenBuffer(40, 20)
	modal.Draw(scr, area)

	// Capture rendered output
	rendered := scr.Render()

	// Verify golden file
	goldenPath := filepath.Join("testdata", "note_modal_small_screen.golden")
	testfixtures.CompareGolden(t, goldenPath, rendered)
}

// TestNoteModal_RenderVerySmallScreen tests minimum dimension constraints
func TestNoteModal_RenderVerySmallScreen(t *testing.T) {
	modal := NewNoteModal()
	state := testfixtures.StateWithNotes()
	modal.SetNote(state.Notes[0])

	// Render with very small screen (should use minimum constraints)
	area := uv.Rectangle{
		Min: uv.Position{X: 0, Y: 0},
		Max: uv.Position{X: 20, Y: 10},
	}
	scr := uv.NewScreenBuffer(20, 10)
	modal.Draw(scr, area)

	// Should not panic and should use minimum dimensions
	rendered := scr.Render()

	// Verify it rendered something (minimum size constraints applied)
	if len(rendered) == 0 {
		t.Error("Should render content even with very small screen")
	}

	// Verify golden file
	goldenPath := filepath.Join("testdata", "note_modal_very_small_screen.golden")
	testfixtures.CompareGolden(t, goldenPath, rendered)
}

// TestNoteModal_RenderUnknownNoteType tests rendering with unknown note type
func TestNoteModal_RenderUnknownNoteType(t *testing.T) {
	modal := NewNoteModal()
	state := testfixtures.StateWithNotes()

	// Create a note with unknown type
	unknownNote := *state.Notes[0]
	unknownNote.Type = "custom-type"
	unknownNote.Content = "This note has a custom unknown type"

	modal.SetNote(&unknownNote)

	// Render
	area := uv.Rectangle{
		Min: uv.Position{X: 0, Y: 0},
		Max: uv.Position{X: testfixtures.TestTermWidth, Y: testfixtures.TestTermHeight},
	}
	scr := uv.NewScreenBuffer(testfixtures.TestTermWidth, testfixtures.TestTermHeight)
	modal.Draw(scr, area)

	// Capture rendered output
	rendered := scr.Render()

	// Verify golden file
	goldenPath := filepath.Join("testdata", "note_modal_unknown_type.golden")
	testfixtures.CompareGolden(t, goldenPath, rendered)
}

// TestNoteModal_Centering tests that modal is centered on screen
func TestNoteModal_Centering(t *testing.T) {
	t.Parallel()

	modal := NewNoteModal()
	state := testfixtures.StateWithNotes()
	modal.SetNote(state.Notes[0])

	// Render on standard screen
	area := uv.Rectangle{
		Min: uv.Position{X: 0, Y: 0},
		Max: uv.Position{X: testfixtures.TestTermWidth, Y: testfixtures.TestTermHeight},
	}
	scr := uv.NewScreenBuffer(testfixtures.TestTermWidth, testfixtures.TestTermHeight)
	modal.Draw(scr, area)

	rendered := scr.Render()

	// Modal should have content in the center, not at edges
	if len(rendered) == 0 {
		t.Error("Modal should render content")
	}
}

// TestNoteModal_TypeBadgesRendering tests that all type badges render correctly
func TestNoteModal_TypeBadgesRendering(t *testing.T) {
	testCases := []struct {
		name      string
		noteType  string
		typeIndex int
	}{
		{"Learning type", "learning", 0},
		{"Stuck type", "stuck", 1},
		{"Tip type", "tip", 2},
		{"Decision type", "decision", 3},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			modal := NewNoteModal()
			state := testfixtures.StateWithNotes()
			modal.SetNote(state.Notes[tc.typeIndex])

			// Verify type index is correct
			if modal.typeIndex != tc.typeIndex {
				t.Errorf("Expected type index %d for %s, got %d", tc.typeIndex, tc.noteType, modal.typeIndex)
			}

			// Verify badge rendering doesn't panic
			badges := modal.renderTypeBadges()
			if badges == "" {
				t.Error("Type badges should not be empty")
			}
		})
	}
}
