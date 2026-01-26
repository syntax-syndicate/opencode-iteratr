package tui

import (
	"time"

	"charm.land/bubbles/v2/spinner"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/mark3labs/iteratr/internal/tui/theme"
)

// Spinner wraps bubbles spinner with convenience methods
type Spinner struct {
	model spinner.Model
}

// NewSpinner creates a new spinner with the given style
func NewSpinner(style spinner.Spinner) Spinner {
	t := theme.Current()
	s := spinner.New(
		spinner.WithSpinner(style),
		spinner.WithStyle(lipgloss.NewStyle().Foreground(lipgloss.Color(t.Primary))),
	)
	return Spinner{model: s}
}

// NewDefaultSpinner creates a spinner with MiniDot style
func NewDefaultSpinner() Spinner {
	return NewSpinner(spinner.MiniDot)
}

// Update handles spinner tick messages
func (s *Spinner) Update(msg tea.Msg) tea.Cmd {
	var cmd tea.Cmd
	s.model, cmd = s.model.Update(msg)
	return cmd
}

// View renders the current spinner frame
func (s *Spinner) View() string {
	return s.model.View()
}

// Tick returns the tick command to start animation
func (s *Spinner) Tick() tea.Cmd {
	return s.model.Tick
}

// SetStyle updates the spinner's lipgloss style
func (s *Spinner) SetStyle(style lipgloss.Style) {
	s.model.Style = style
}

// Pulse represents a pulsing animation effect
type Pulse struct {
	active   bool
	frame    int
	maxFrame int
	interval time.Duration
	lastTick time.Time
}

// PulseMsg is sent on each pulse tick
type PulseMsg struct {
	ID string
}

// NewPulse creates a new pulse animation
func NewPulse() Pulse {
	return Pulse{
		active:   false,
		frame:    0,
		maxFrame: 5, // 5 frames for fade in/out
		interval: 100 * time.Millisecond,
	}
}

// Start begins the pulse animation
func (p *Pulse) Start() tea.Cmd {
	p.active = true
	p.frame = 0
	p.lastTick = time.Now()
	return p.tick()
}

// Stop ends the pulse animation
func (p *Pulse) Stop() {
	p.active = false
	p.frame = 0
}

// Update handles pulse tick messages
func (p *Pulse) Update(msg tea.Msg) tea.Cmd {
	if !p.active {
		return nil
	}

	switch msg.(type) {
	case PulseMsg:
		now := time.Now()
		if now.Sub(p.lastTick) >= p.interval {
			p.lastTick = now
			p.frame++
			if p.frame >= p.maxFrame*2 {
				// Completed full pulse cycle (fade in + fade out)
				p.Stop()
				return nil
			}
			return p.tick()
		}
	}
	return nil
}

// tick returns a command that sends a PulseMsg after the interval
func (p *Pulse) tick() tea.Cmd {
	return tea.Tick(p.interval, func(t time.Time) tea.Msg {
		return PulseMsg{}
	})
}

// Intensity returns the current pulse intensity (0.0 to 1.0)
func (p *Pulse) Intensity() float64 {
	if !p.active {
		return 0.0
	}

	// Fade in for first half, fade out for second half
	if p.frame < p.maxFrame {
		return float64(p.frame) / float64(p.maxFrame)
	}
	return float64(p.maxFrame*2-p.frame) / float64(p.maxFrame)
}

// IsActive returns whether the pulse is currently animating
func (p *Pulse) IsActive() bool {
	return p.active
}

// GetPulseStyle returns a lipgloss style with pulse effect applied
func GetPulseStyle(base lipgloss.Style, intensity float64) lipgloss.Style {
	if intensity <= 0 {
		return base
	}

	t := theme.Current()
	// Blend between base and highlight color based on intensity
	// For now, just adjust the foreground color brightness
	if intensity > 0.5 {
		return base.Foreground(lipgloss.Color(t.Primary))
	}
	return base.Foreground(lipgloss.Color(t.Secondary))
}

// GradientSpinnerMsg is sent on each gradient spinner tick
type GradientSpinnerMsg struct{}

// GradientSpinner renders an animated gradient text spinner
type GradientSpinner struct {
	frame  int
	size   int
	colorA string
	colorB string
	label  string
}

// NewGradientSpinner creates a gradient spinner with default size
func NewGradientSpinner(colorA, colorB string, label string) GradientSpinner {
	return GradientSpinner{
		frame:  0,
		size:   15,
		colorA: colorA,
		colorB: colorB,
		label:  label,
	}
}

// View renders the gradient spinner as an animated string
func (g *GradientSpinner) View() string {
	// Create the gradient string by interpolating between colorA and colorB
	var result string

	for i := 0; i < g.size; i++ {
		// Calculate position in gradient (0.0 to 1.0), shifted by frame for animation
		pos := float64((i+g.frame)%g.size) / float64(g.size)

		// Interpolate between colorA and colorB
		colorHex := theme.InterpolateColor(g.colorA, g.colorB, pos)
		style := lipgloss.NewStyle().Foreground(lipgloss.Color(colorHex))
		result += style.Render("█")
	}

	// Prepend label if set
	if g.label != "" {
		t := theme.Current()
		labelStyle := lipgloss.NewStyle().Foreground(lipgloss.Color(t.FgBase))
		return labelStyle.Render(g.label+" ") + result
	}

	return result
}

// Tick returns a command that sends a GradientSpinnerMsg after 80ms
func (g *GradientSpinner) Tick() tea.Cmd {
	return tea.Tick(80*time.Millisecond, func(t time.Time) tea.Msg {
		return GradientSpinnerMsg{}
	})
}

// Update handles gradient spinner tick messages and advances animation
func (g *GradientSpinner) Update(msg tea.Msg) tea.Cmd {
	switch msg.(type) {
	case GradientSpinnerMsg:
		g.frame++
		// Wrap frame to prevent overflow
		if g.frame >= g.size {
			g.frame = 0
		}
		return g.Tick()
	}
	return nil
}
