package tui

import (
	"bytes"
	"fmt"
	"strings"
	"time"

	chroma "github.com/alecthomas/chroma/v2"
	"github.com/alecthomas/chroma/v2/formatters"
	"github.com/alecthomas/chroma/v2/lexers"
	"github.com/alecthomas/chroma/v2/styles"
	"github.com/charmbracelet/lipgloss"
)

// MessageItem represents a displayable message item in the agent output.
type MessageItem interface {
	// ID returns the unique identifier for this message item.
	ID() string
	// Render returns the rendered string representation at the given width.
	Render(width int) string
	// Height returns the number of lines this item occupies (0 if not yet rendered).
	Height() int
}

// Expandable is an optional interface for message items that support expand/collapse.
type Expandable interface {
	IsExpanded() bool
	ToggleExpanded()
}

// ToolStatus represents the execution status of a tool call.
type ToolStatus int

const (
	ToolStatusPending ToolStatus = iota
	ToolStatusRunning
	ToolStatusSuccess
	ToolStatusError
	ToolStatusCanceled
)

// TextMessageItem represents assistant text content.
type TextMessageItem struct {
	id           string
	content      string
	cachedRender string
	cachedWidth  int
}

// ID returns the unique identifier for this text message.
func (t *TextMessageItem) ID() string {
	return t.id
}

// Render renders the text message at the given width.
// Wraps text, applies assistant border, and caps width at min(width-2, 120).
func (t *TextMessageItem) Render(width int) string {
	// Return cached render if width matches
	if t.cachedWidth == width && t.cachedRender != "" {
		return t.cachedRender
	}

	// Cap effective width at min(width-2, 120) to prevent overly long lines
	// Subtract 2 for the left border and padding added by styleAssistantBorder
	effectiveWidth := width - 2
	if effectiveWidth > 120 {
		effectiveWidth = 120
	}
	if effectiveWidth < 1 {
		effectiveWidth = 1
	}

	// Render content as markdown with syntax highlighting
	// Falls back to plain text wrapping if glamour fails
	rendered := renderMarkdown(t.content, effectiveWidth)

	// Apply assistant border styling (left border + padding)
	result := styleAssistantBorder.Render(rendered)

	// Cache and return
	t.cachedRender = result
	t.cachedWidth = width
	return result
}

// Height returns the number of lines this text message occupies.
func (t *TextMessageItem) Height() int {
	if t.cachedRender == "" {
		return 0
	}
	// Count newlines in cached render
	lines := 1
	for _, ch := range t.cachedRender {
		if ch == '\n' {
			lines++
		}
	}
	return lines
}

// ThinkingMessageItem represents agent thinking/reasoning content.
type ThinkingMessageItem struct {
	id           string
	content      string
	collapsed    bool // default true
	duration     time.Duration
	finished     bool
	cachedRender string
	cachedWidth  int
}

// ID returns the unique identifier for this thinking message.
func (t *ThinkingMessageItem) ID() string {
	return t.id
}

// Render renders the thinking message at the given width.
// If collapsed and content has >10 lines, shows last 10 with truncation hint.
// Adds footer "Thought for Xs" if finished. Wraps in styleThinkingBox.
func (t *ThinkingMessageItem) Render(width int) string {
	// Return cached render if width matches
	if t.cachedWidth == width && t.cachedRender != "" {
		return t.cachedRender
	}

	var result strings.Builder

	// Split content into lines
	lines := strings.Split(t.content, "\n")

	// If collapsed and >10 lines, show last 10 with truncation hint
	var displayLines []string
	hiddenCount := 0
	if t.collapsed && len(lines) > 10 {
		hiddenCount = len(lines) - 10
		displayLines = lines[len(lines)-10:]
	} else {
		displayLines = lines
	}

	// Add truncation hint if we hid lines
	if hiddenCount > 0 {
		hint := styleThinkingTruncationHint.Render(
			fmt.Sprintf("… (%d lines hidden)", hiddenCount),
		)
		result.WriteString(styleThinkingContent.Render(hint))
		result.WriteString("\n")
	}

	// Render visible lines with thinking content style
	for i, line := range displayLines {
		if i > 0 || hiddenCount > 0 {
			result.WriteString("\n")
		}
		result.WriteString(styleThinkingContent.Render(line))
	}

	// Add footer with duration if finished
	if t.finished && t.duration > 0 {
		result.WriteString("\n")
		durationStr := formatDuration(t.duration)
		footer := styleThinkingFooter.Render("Thought for ") +
			styleThinkingDuration.Render(durationStr)
		result.WriteString(footer)
	}

	// Wrap entire content in thinking box style with full-width background
	boxed := styleThinkingBox.Width(width).Render(result.String())

	// Cache and return
	t.cachedRender = boxed
	t.cachedWidth = width
	return boxed
}

// Height returns the number of lines this thinking message occupies.
func (t *ThinkingMessageItem) Height() int {
	if t.cachedRender == "" {
		return 0
	}
	// Count newlines in cached render
	lines := 1
	for _, ch := range t.cachedRender {
		if ch == '\n' {
			lines++
		}
	}
	return lines
}

// IsExpanded returns whether the thinking message is expanded.
func (t *ThinkingMessageItem) IsExpanded() bool {
	return !t.collapsed
}

// ToggleExpanded toggles the expanded/collapsed state.
func (t *ThinkingMessageItem) ToggleExpanded() {
	t.collapsed = !t.collapsed
	// Invalidate cache
	t.cachedWidth = 0
}

// ToolMessageItem represents a tool call with status, input, and output.
type ToolMessageItem struct {
	id           string
	toolName     string
	status       ToolStatus
	input        map[string]any
	output       string
	expanded     bool
	maxLines     int // default 10
	cachedRender string
	cachedWidth  int
}

// ID returns the unique identifier for this tool message.
func (t *ToolMessageItem) ID() string {
	return t.id
}

// Render renders the tool message at the given width.
// Shows header with status icon, tool name, and formatted params.
// Shows output body capped at maxLines (or all if expanded).
// Code output (tools with filePath) uses syntax highlighting.
func (t *ToolMessageItem) Render(width int) string {
	// Return cached render if width matches
	if t.cachedWidth == width && t.cachedRender != "" {
		return t.cachedRender
	}

	var result strings.Builder

	// --- HEADER: [icon] [name] [params] ---
	var icon string
	var iconStyle lipgloss.Style
	switch t.status {
	case ToolStatusPending:
		icon = "●"
		iconStyle = styleToolIconPending
	case ToolStatusRunning:
		icon = "●"
		iconStyle = styleToolIconPending // Running uses same as pending
	case ToolStatusSuccess:
		icon = "✓"
		iconStyle = styleToolIconSuccess
	case ToolStatusError:
		icon = "×"
		iconStyle = styleToolIconError
	case ToolStatusCanceled:
		icon = "×"
		iconStyle = styleToolIconCanceled
	default:
		icon = "●"
		iconStyle = styleToolIconPending
	}

	// Build header: [indent] [icon] [name] [params]
	header := "  " + iconStyle.Render(icon) + " " + styleToolName.Render(t.toolName)

	// Add formatted params if present
	if len(t.input) > 0 {
		// Reserve space for icon, name, and spacing
		usedWidth := 2 + len(t.toolName) + 1 // "● " + name + " "
		paramWidth := width - usedWidth
		if paramWidth < 10 {
			paramWidth = 10
		}
		params := formatToolParams(t.input, paramWidth)
		if params != "" {
			header += " " + styleToolParams.Render(params)
		}
	}

	result.WriteString(header)

	// --- BODY: output with truncation ---
	if t.output != "" {
		result.WriteString("\n\n") // blank line between header and output

		// Split output into lines
		outputLines := strings.Split(t.output, "\n")

		// Determine visible lines based on expansion state
		var visibleLines []string
		var hiddenCount int

		if t.status == ToolStatusError {
			// Error output: show all lines styled as error
			visibleLines = outputLines
		} else if t.expanded || len(outputLines) <= t.maxLines {
			// Show all lines if expanded or within limit
			visibleLines = outputLines
		} else {
			// Show first maxLines with truncation hint
			visibleLines = outputLines[:t.maxLines]
			hiddenCount = len(outputLines) - t.maxLines
		}

		// Check if this is code output (tool has filePath param)
		isCodeOutput := false
		var fileName string
		if fp, ok := t.input["filePath"]; ok {
			isCodeOutput = true
			fileName = fmt.Sprintf("%v", fp)
		}

		// Calculate output width (available width minus indent margin)
		outputWidth := width - 2 // account for MarginLeft(2) on output styles
		if outputWidth < 1 {
			outputWidth = 1
		}

		// Render visible lines
		if t.status == ToolStatusError {
			// Error output: red styling with full-width background
			for _, line := range visibleLines {
				result.WriteString(styleToolError.Width(outputWidth).Render(line))
				result.WriteString("\n")
			}
		} else if isCodeOutput {
			// Code output: properly styled code block with line numbers and syntax highlighting
			codeBlock := strings.Join(visibleLines, "\n")
			rendered := renderCodeBlock(codeBlock, fileName, outputWidth)
			result.WriteString(rendered)
			result.WriteString("\n")
		} else {
			// Plain output: background styling with full-width fill
			for _, line := range visibleLines {
				result.WriteString(styleToolOutput.Width(outputWidth).Render(line))
				result.WriteString("\n")
			}
		}

		// Add truncation hint if lines were hidden
		if hiddenCount > 0 {
			truncMsg := fmt.Sprintf("…(%d more lines, click to expand)", hiddenCount)
			if isCodeOutput {
				// Code truncation: match code block background
				hint := styleCodeTruncation.Width(outputWidth).Render(truncMsg)
				result.WriteString(hint)
			} else {
				hint := styleToolTruncation.Render(truncMsg)
				result.WriteString(hint)
			}
			result.WriteString("\n")
		}
	}

	// Cache and return
	rendered := result.String()
	t.cachedRender = rendered
	t.cachedWidth = width
	return rendered
}

// Height returns the number of lines this tool message occupies.
func (t *ToolMessageItem) Height() int {
	if t.cachedRender == "" {
		return 0
	}
	// Count newlines in cached render
	lines := 1
	for _, ch := range t.cachedRender {
		if ch == '\n' {
			lines++
		}
	}
	return lines
}

// IsExpanded returns whether the tool message is expanded.
func (t *ToolMessageItem) IsExpanded() bool {
	return t.expanded
}

// ToggleExpanded toggles the expanded/collapsed state.
func (t *ToolMessageItem) ToggleExpanded() {
	t.expanded = !t.expanded
	// Invalidate cache
	t.cachedWidth = 0
}

// InfoMessageItem represents agent metadata (model, provider, duration).
type InfoMessageItem struct {
	id           string
	model        string
	provider     string
	duration     time.Duration
	cachedRender string
	cachedWidth  int
}

// ID returns the unique identifier for this info message.
func (i *InfoMessageItem) ID() string {
	return i.id
}

// Render renders the info message at the given width.
// Formats as "◇ Model via Provider ⏱ Duration ────────"
func (i *InfoMessageItem) Render(width int) string {
	// Return cached render if width matches
	if i.cachedWidth == width && i.cachedRender != "" {
		return i.cachedRender
	}

	// Format duration as human-readable string
	durationStr := formatDuration(i.duration)

	// Build the info string: ◇ Model via Provider ⏱ Duration
	var infoText string
	if i.model != "" && i.provider != "" {
		infoText = styleInfoIcon.Render("◇") + " " +
			styleInfoModel.Render(i.model) + " " +
			styleInfoProvider.Render("via") + " " +
			styleInfoProvider.Render(i.provider) + " " +
			styleInfoDuration.Render("⏱") + " " +
			styleInfoDuration.Render(durationStr)
	} else if i.model != "" {
		infoText = styleInfoIcon.Render("◇") + " " +
			styleInfoModel.Render(i.model) + " " +
			styleInfoDuration.Render("⏱") + " " +
			styleInfoDuration.Render(durationStr)
	} else {
		infoText = styleInfoIcon.Render("◇") + " " +
			styleInfoDuration.Render("⏱") + " " +
			styleInfoDuration.Render(durationStr)
	}

	// Add trailing horizontal rule to fill remaining width
	infoWidth := lipgloss.Width(infoText)
	remainingWidth := width - infoWidth - 1 // -1 for space before rule
	if remainingWidth > 0 {
		rule := strings.Repeat("─", remainingWidth)
		infoText = infoText + " " + styleInfoRule.Render(rule)
	}

	// Cache and return
	i.cachedRender = infoText
	i.cachedWidth = width
	return infoText
}

// Height returns the number of lines this info message occupies.
func (i *InfoMessageItem) Height() int {
	if i.cachedRender == "" {
		return 0
	}
	// Count newlines in cached render
	lines := 1
	for _, ch := range i.cachedRender {
		if ch == '\n' {
			lines++
		}
	}
	return lines
}

// DividerMessageItem represents an iteration divider.
type DividerMessageItem struct {
	id           string
	iteration    int
	cachedRender string
	cachedWidth  int
}

// ID returns the unique identifier for this divider message.
func (d *DividerMessageItem) ID() string {
	return d.id
}

// Render renders the divider at the given width.
// Shows centered iteration label with horizontal rules.
func (d *DividerMessageItem) Render(width int) string {
	// Return cached render if width matches
	if d.cachedWidth == width && d.cachedRender != "" {
		return d.cachedRender
	}

	// Create the iteration label
	label := fmt.Sprintf(" Iteration #%d ", d.iteration)
	labelWidth := len(label)

	// Calculate line widths on each side
	lineWidth := (width - labelWidth) / 2
	if lineWidth < 3 {
		lineWidth = 3
	}

	// Build the horizontal rule with centered label
	line := strings.Repeat("─", lineWidth)
	divider := line + label + line

	// Style the divider
	style := lipgloss.NewStyle().
		Foreground(colorMuted).
		Bold(true).
		MarginTop(1).
		MarginBottom(1)

	result := style.Render(divider)

	// Cache and return
	d.cachedRender = result
	d.cachedWidth = width
	return result
}

// Height returns the number of lines this divider occupies.
func (d *DividerMessageItem) Height() int {
	if d.cachedRender == "" {
		return 0
	}
	// Count newlines in cached render
	lines := 1
	for _, ch := range d.cachedRender {
		if ch == '\n' {
			lines++
		}
	}
	return lines
}

// formatDuration formats a duration as a human-readable string.
// Examples: "1.2s", "345ms", "2m30s"
func formatDuration(d time.Duration) string {
	if d < time.Second {
		return d.Round(time.Millisecond).String()
	}
	if d < time.Minute {
		return d.Round(100 * time.Millisecond).String()
	}
	return d.Round(time.Second).String()
}

// formatToolParams formats tool input parameters for display.
// Shows primary param (command/filePath) first, then remaining params as (key=val, ...).
// Truncates result to fit within maxWidth.
func formatToolParams(input map[string]any, maxWidth int) string {
	if len(input) == 0 {
		return ""
	}

	// Identify primary parameter (command or filePath)
	var primaryKey string
	var primaryVal any
	if cmd, ok := input["command"]; ok {
		primaryKey = "command"
		primaryVal = cmd
	} else if fp, ok := input["filePath"]; ok {
		primaryKey = "filePath"
		primaryVal = fp
	}

	var result strings.Builder

	// Add primary parameter if found
	if primaryKey != "" {
		result.WriteString(fmt.Sprintf("%v", primaryVal))
	}

	// Collect remaining parameters
	var remaining []string
	for key, val := range input {
		if key == primaryKey {
			continue
		}
		remaining = append(remaining, fmt.Sprintf("%s=%v", key, val))
	}

	// Add remaining params in parentheses if any
	if len(remaining) > 0 {
		if result.Len() > 0 {
			result.WriteString(" ")
		}
		result.WriteString("(")
		result.WriteString(strings.Join(remaining, ", "))
		result.WriteString(")")
	}

	// Truncate if necessary
	str := result.String()
	if len(str) > maxWidth {
		if maxWidth > 3 {
			return str[:maxWidth-3] + "..."
		}
		return str[:maxWidth]
	}

	return str
}

// renderCodeBlock parses tool output with line numbers and renders it as a
// properly styled code block with separate line number gutter and syntax-highlighted
// code content with full-width background fill.
//
// Expected input format:
//
//	<file>
//	00001| code line
//	00002| code line
//	</file>
func renderCodeBlock(content, fileName string, width int) string {
	// Strip <file> and </file> tags
	content = strings.TrimPrefix(content, "<file>")
	content = strings.TrimSuffix(content, "</file>")
	content = strings.TrimPrefix(content, "\n")
	content = strings.TrimSuffix(content, "\n")

	lines := strings.Split(content, "\n")

	// Parse line numbers and code content separately
	type codeLine struct {
		lineNum string
		code    string
	}
	var parsed []codeLine
	maxNumWidth := 0

	for _, line := range lines {
		// Try to parse "NNNNN| code" format (line number + pipe + content)
		if idx := strings.Index(line, "| "); idx > 0 && idx <= 7 {
			numStr := strings.TrimLeft(line[:idx], "0 ")
			if numStr == "" {
				numStr = "0"
			}
			parsed = append(parsed, codeLine{
				lineNum: line[:idx],
				code:    line[idx+2:], // skip "| "
			})
			if len(line[:idx]) > maxNumWidth {
				maxNumWidth = len(line[:idx])
			}
		} else if idx := strings.Index(line, "|"); idx > 0 && idx <= 7 {
			// Handle "NNNNN|" with no space (empty lines)
			parsed = append(parsed, codeLine{
				lineNum: line[:idx],
				code:    line[idx+1:],
			})
			if len(line[:idx]) > maxNumWidth {
				maxNumWidth = len(line[:idx])
			}
		} else {
			// No line number format, use as-is
			parsed = append(parsed, codeLine{code: line})
		}
	}

	if len(parsed) == 0 {
		return styleToolOutput.Width(width).Render(content)
	}

	// Extract just the code for syntax highlighting
	var codeOnly []string
	for _, p := range parsed {
		codeOnly = append(codeOnly, p.code)
	}
	highlighted := syntaxHighlight(strings.Join(codeOnly, "\n"), fileName)
	highlightedLines := strings.Split(highlighted, "\n")

	// Calculate widths (subtract 2 for the left indent)
	gutterWidth := maxNumWidth + 2 // line number + padding
	codeWidth := width - gutterWidth - 2
	if codeWidth < 10 {
		codeWidth = 10
	}

	// Render each line: [indent] [gutter] [code with bg]
	const codeIndent = "  " // 2-char indent to align with tool header
	var result []string
	for i, p := range parsed {
		// Style the line number gutter
		gutter := styleCodeLineNum.
			Width(gutterWidth).
			Render(p.lineNum)

		// Get the highlighted code line (or fallback to raw)
		var codePart string
		if i < len(highlightedLines) {
			codePart = highlightedLines[i]
		} else {
			codePart = p.code
		}

		// Apply code background with full-width fill
		styledCode := styleCodeContent.
			Width(codeWidth).
			Render(codePart)

		result = append(result, codeIndent+lipgloss.JoinHorizontal(lipgloss.Top, gutter, styledCode))
	}

	return strings.Join(result, "\n")
}

// syntaxHighlight applies syntax highlighting to source code and returns
// a string with ANSI color codes for terminal display.
//
// It uses the fileName to detect the language, falling back to content analysis,
// and finally to a plain text lexer. The output uses true color (24-bit) ANSI codes.
func syntaxHighlight(source, fileName string) string {
	// Try to detect lexer from filename
	lexer := lexers.Match(fileName)
	if lexer == nil {
		// Fall back to content-based detection
		lexer = lexers.Analyse(source)
	}
	if lexer == nil {
		// Fall back to plain text
		lexer = lexers.Fallback
	}

	// Use terminal16m formatter for true color output
	formatter := formatters.Get("terminal16m")
	if formatter == nil {
		// Fallback to terminal256 if terminal16m is not available
		formatter = formatters.Get("terminal256")
	}
	if formatter == nil {
		// Last resort: return source unchanged with background style
		return styleToolOutput.Render(source)
	}

	// Use monokai style (dark theme, similar to our UI palette)
	baseStyle := styles.Get("monokai")
	if baseStyle == nil {
		baseStyle = styles.Fallback
	}

	// Transform all token backgrounds to match our code block background (colorSurface0).
	// Without this, chroma's monokai theme uses #272822 which clashes with our #313244.
	bgColour := chroma.MustParseColour(string(colorSurface0))
	style, err := baseStyle.Builder().Transform(func(entry chroma.StyleEntry) chroma.StyleEntry {
		entry.Background = bgColour
		return entry
	}).Build()
	if err != nil {
		style = baseStyle
	}

	// Tokenize and format
	iterator, err := lexer.Tokenise(nil, source)
	if err != nil {
		return styleToolOutput.Render(source)
	}

	var buf bytes.Buffer
	err = formatter.Format(&buf, style, iterator)
	if err != nil {
		return styleToolOutput.Render(source)
	}

	// Return the highlighted output, trimming any trailing newline
	return strings.TrimRight(buf.String(), "\n")
}
