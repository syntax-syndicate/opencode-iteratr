# Spec Wizard Manual E2E Test Plan

## Test Flow Overview
This document describes the complete end-to-end test flow for the spec wizard feature.

## Prerequisites
- iteratr binary built (`go build -o iteratr ./cmd/iteratr`)
- opencode installed and available in PATH
- Valid model configured in config
- specs/ directory exists

## Test Steps

### 1. Title Step
**Actions:**
- Run `./iteratr spec`
- Type a test title: "E2E Test Feature"
- Press Enter

**Expected:**
- Title input field appears
- Input is validated (non-empty)
- Advances to description step

### 2. Description Step
**Actions:**
- Type description: "This is an end-to-end test of the spec wizard feature. It validates the complete flow from title input through agent interview to spec generation."
- Press Enter

**Expected:**
- Multi-line textarea appears
- Can type multiple lines
- Advances to model selection step

### 3. Model Selection Step
**Actions:**
- Navigate through model options with arrow keys
- Select a model (e.g., claude-sonnet-4-5)
- Press Enter

**Expected:**
- List of available models displayed
- Selection highlights properly
- Advances to agent phase

### 4. Agent Interview Phase
**Actions:**
- Wait for agent to start
- Agent asks questions via ask-questions tool
- Answer questions using keyboard navigation
- Test tab navigation between options, custom input, and buttons
- Test "Type your own answer" option
- Submit answers

**Expected:**
- Spinner shows "Starting agent..."
- Questions appear one at a time
- Can navigate with arrow keys and tab
- Custom input field appears when "Type your own" selected
- Next/Submit buttons work
- Agent receives answers via finish-spec tool
- Progress advances through multiple questions
- Eventually completes and advances to review step

### 5. Review Step
**Actions:**
- Review the generated spec content
- Scroll through content with arrow keys
- Press 'e' to open in external editor (optional)
- Press Tab to navigate to buttons
- Navigate to Save button
- Press Enter on Save

**Expected:**
- Markdown content rendered with syntax highlighting
- Scroll viewport works properly
- External editor opens if 'e' pressed
- Button bar shows at bottom
- Tab navigation works (viewport → buttons)
- Save button triggers file write

### 6. File Overwrite (if spec exists)
**Actions:**
- If file exists, confirmation modal appears
- Press 'y' to confirm overwrite

**Expected:**
- Modal shows "Overwrite Existing Spec?" message
- 'y' confirms and saves
- 'n' or ESC cancels

### 7. Completion Step
**Actions:**
- View success message
- Note the saved file path
- Choose Exit or Start Build (when implemented)

**Expected:**
- Success message displayed
- File path shown
- Exit button available
- Start Build button (TODO: TAS-49)

### 8. Verification
**Actions:**
- Exit wizard
- Check `specs/` directory for saved spec
- Verify filename matches slugified title (e.g., `e2e-test-feature.md`)
- Verify README.md updated with new entry
- Verify spec content matches what was shown in review

**Expected:**
- Spec file exists with correct name
- README.md contains new entry in table
- Spec content is valid markdown
- All sections present from interview

## Test Variations

### Cancel Flow
**Test:** ESC during any step
**Expected:**
- Confirmation modal appears
- Can confirm or cancel the cancellation
- If confirmed, wizard exits gracefully

### Back Navigation
**Test:** Back button in question view
**Expected:**
- Confirmation modal for restart
- If confirmed, returns to agent phase start
- Previous answers preserved until restart

### Tab Navigation
**Test:** Tab key in question view
**Expected:**
- Tab cycles: options → custom input → back → next/submit
- Shift+Tab goes backwards
- ESC returns to options

### Multi-select Questions
**Test:** Questions with multi-select enabled
**Expected:**
- Can select multiple options with space/enter
- Selected items marked with checkbox
- Submit includes all selected indices

### Validation
**Test:** Submit empty answers or invalid input
**Expected:**
- Error message appears
- Cannot advance until valid input provided
- Error clears when valid input entered

## Automated Test

Run the automated validation test:
```bash
go test -v ./internal/tui/specwizard/... -run TestWizard
```

This validates the core wizard logic without requiring manual interaction.

## Success Criteria

- [ ] All steps advance correctly in sequence
- [ ] Agent interview completes with Q&A flow
- [ ] Spec content generated and displayed
- [ ] File saved to correct location
- [ ] README.md updated correctly
- [ ] Cancel/back flows work properly
- [ ] Tab navigation works in all components
- [ ] All validation works correctly
- [ ] No crashes or errors in any step
- [ ] Terminal state restored properly on exit
