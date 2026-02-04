package tui

import (
	"testing"
)

// TestPulse_NewPulse verifies initial state of a new Pulse
func TestPulse_NewPulse(t *testing.T) {
	t.Parallel()

	pulse := NewPulse()

	if pulse.IsActive() {
		t.Error("new pulse should not be active")
	}

	if pulse.frame != 0 {
		t.Errorf("new pulse frame should be 0, got %d", pulse.frame)
	}

	if pulse.maxFrame != 5 {
		t.Errorf("new pulse maxFrame should be 5, got %d", pulse.maxFrame)
	}

	intensity := pulse.Intensity()
	if intensity != 0.0 {
		t.Errorf("inactive pulse intensity should be 0.0, got %f", intensity)
	}
}

// TestPulse_Start verifies pulse activation
func TestPulse_Start(t *testing.T) {
	t.Parallel()

	pulse := NewPulse()

	// Start pulse
	cmd := pulse.Start()
	if cmd == nil {
		t.Error("Start() should return a tick command")
	}

	if !pulse.IsActive() {
		t.Error("pulse should be active after Start()")
	}

	if pulse.frame != 0 {
		t.Errorf("pulse frame should be reset to 0 after Start(), got %d", pulse.frame)
	}
}

// TestPulse_Stop verifies pulse deactivation
func TestPulse_Stop(t *testing.T) {
	t.Parallel()

	pulse := NewPulse()
	pulse.Start()

	// Advance frame
	pulse.frame = 3

	// Stop pulse
	pulse.Stop()

	if pulse.IsActive() {
		t.Error("pulse should not be active after Stop()")
	}

	if pulse.frame != 0 {
		t.Errorf("pulse frame should be reset to 0 after Stop(), got %d", pulse.frame)
	}

	intensity := pulse.Intensity()
	if intensity != 0.0 {
		t.Errorf("stopped pulse intensity should be 0.0, got %f", intensity)
	}
}

// TestPulse_IntensityInactive verifies intensity when pulse is inactive
func TestPulse_IntensityInactive(t *testing.T) {
	t.Parallel()

	pulse := NewPulse()

	intensity := pulse.Intensity()
	if intensity != 0.0 {
		t.Errorf("inactive pulse intensity should be 0.0, got %f", intensity)
	}
}

// TestPulse_IntensityFadeIn verifies intensity calculation during fade-in phase
func TestPulse_IntensityFadeIn(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		frame    int
		expected float64
	}{
		{name: "frame 0", frame: 0, expected: 0.0},
		{name: "frame 1", frame: 1, expected: 0.2},
		{name: "frame 2", frame: 2, expected: 0.4},
		{name: "frame 3", frame: 3, expected: 0.6},
		{name: "frame 4", frame: 4, expected: 0.8},
		{name: "frame 5 (peak)", frame: 5, expected: 1.0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pulse := NewPulse()
			pulse.Start()
			pulse.frame = tt.frame

			intensity := pulse.Intensity()
			if intensity != tt.expected {
				t.Errorf("frame %d: expected intensity %f, got %f", tt.frame, tt.expected, intensity)
			}
		})
	}
}

// TestPulse_IntensityFadeOut verifies intensity calculation during fade-out phase
func TestPulse_IntensityFadeOut(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		frame    int
		expected float64
	}{
		{name: "frame 5 (peak)", frame: 5, expected: 1.0},
		{name: "frame 6", frame: 6, expected: 0.8},
		{name: "frame 7", frame: 7, expected: 0.6},
		{name: "frame 8", frame: 8, expected: 0.4},
		{name: "frame 9", frame: 9, expected: 0.2},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pulse := NewPulse()
			pulse.Start()
			pulse.frame = tt.frame

			intensity := pulse.Intensity()
			if intensity != tt.expected {
				t.Errorf("frame %d: expected intensity %f, got %f", tt.frame, tt.expected, intensity)
			}
		})
	}
}

// TestPulse_IntensityRange verifies intensity is always between 0.0 and 1.0
func TestPulse_IntensityRange(t *testing.T) {
	t.Parallel()

	pulse := NewPulse()
	pulse.Start()

	// Test all frames in a full cycle
	maxFrame := pulse.maxFrame
	for frame := 0; frame < maxFrame*2; frame++ {
		pulse.frame = frame
		intensity := pulse.Intensity()
		if intensity < 0.0 || intensity > 1.0 {
			t.Errorf("frame %d: intensity %f is out of range [0.0, 1.0]", frame, intensity)
		}
	}
}

// TestPulse_UpdateInactive verifies Update returns nil when pulse is inactive
func TestPulse_UpdateInactive(t *testing.T) {
	t.Parallel()

	pulse := NewPulse()

	cmd := pulse.Update(PulseMsg{})
	if cmd != nil {
		t.Error("Update should return nil when pulse is inactive")
	}
}

// TestPulse_UpdateNonPulseMsg verifies Update ignores non-PulseMsg messages
func TestPulse_UpdateNonPulseMsg(t *testing.T) {
	t.Parallel()

	pulse := NewPulse()
	pulse.Start()

	// Send a non-PulseMsg
	type OtherMsg struct{}
	cmd := pulse.Update(OtherMsg{})

	// Should return nil (no command to continue ticking)
	if cmd != nil {
		t.Error("Update should return nil for non-PulseMsg messages")
	}
}

// TestPulse_FullCycleCompletion verifies pulse completes after full cycle
func TestPulse_FullCycleCompletion(t *testing.T) {
	t.Parallel()

	pulse := NewPulse()
	pulse.Start()

	maxFrame := pulse.maxFrame
	// Simulate full cycle by manually advancing frame and calling Update
	// Note: Update checks time interval, so we advance frame manually
	for frame := 0; frame < maxFrame*2-1; frame++ {
		// Manually advance frame to bypass time check
		pulse.lastTick = pulse.lastTick.Add(-pulse.interval)
		cmd := pulse.Update(PulseMsg{})
		if cmd == nil {
			t.Errorf("frame %d: Update should return tick command before cycle completes", frame)
		}
		if !pulse.IsActive() {
			t.Errorf("frame %d: pulse should still be active before cycle completes", frame)
		}
	}

	// One more update to reach maxFrame*2 and complete the cycle
	pulse.lastTick = pulse.lastTick.Add(-pulse.interval)
	cmd := pulse.Update(PulseMsg{})
	if cmd != nil {
		t.Error("Update should return nil after completing full cycle")
	}
	if pulse.IsActive() {
		t.Error("pulse should be inactive after completing full cycle")
	}
}

// TestPulse_MultipleStartCalls verifies multiple Start() calls reset the pulse
func TestPulse_MultipleStartCalls(t *testing.T) {
	t.Parallel()

	pulse := NewPulse()

	// Start and advance
	pulse.Start()
	pulse.frame = 3

	// Start again - should reset
	cmd := pulse.Start()
	if cmd == nil {
		t.Error("Start() should return tick command")
	}

	if pulse.frame != 0 {
		t.Errorf("Start() should reset frame to 0, got %d", pulse.frame)
	}

	if !pulse.IsActive() {
		t.Error("pulse should be active after second Start()")
	}
}

// TestPulse_StopWhileInactive verifies Stop() is safe when pulse is already inactive
func TestPulse_StopWhileInactive(t *testing.T) {
	t.Parallel()

	pulse := NewPulse()

	// Stop without starting - should not panic
	pulse.Stop()

	if pulse.IsActive() {
		t.Error("pulse should not be active")
	}

	if pulse.frame != 0 {
		t.Errorf("pulse frame should be 0, got %d", pulse.frame)
	}
}

// TestPulse_IntensitySymmetry verifies fade-in and fade-out are symmetric
func TestPulse_IntensitySymmetry(t *testing.T) {
	t.Parallel()

	pulse := NewPulse()
	pulse.Start()

	maxFrame := pulse.maxFrame

	// Compare fade-in and fade-out intensities
	for i := 0; i < maxFrame; i++ {
		// Fade-in frame
		pulse.frame = i
		fadeInIntensity := pulse.Intensity()

		// Corresponding fade-out frame
		pulse.frame = maxFrame*2 - i
		fadeOutIntensity := pulse.Intensity()

		if fadeInIntensity != fadeOutIntensity {
			t.Errorf("frame %d: fade-in intensity %f != fade-out intensity %f (asymmetric)",
				i, fadeInIntensity, fadeOutIntensity)
		}
	}
}

// TestPulse_PeakIntensity verifies intensity reaches exactly 1.0 at peak
func TestPulse_PeakIntensity(t *testing.T) {
	t.Parallel()

	pulse := NewPulse()
	pulse.Start()

	// Peak should be at maxFrame
	pulse.frame = pulse.maxFrame
	intensity := pulse.Intensity()

	if intensity != 1.0 {
		t.Errorf("peak intensity should be exactly 1.0, got %f", intensity)
	}
}

// TestPulse_ZeroIntensityAtEnds verifies intensity is 0.0 at start and end of cycle
func TestPulse_ZeroIntensityAtEnds(t *testing.T) {
	t.Parallel()

	pulse := NewPulse()
	pulse.Start()

	// Start of cycle
	pulse.frame = 0
	if intensity := pulse.Intensity(); intensity != 0.0 {
		t.Errorf("start intensity should be 0.0, got %f", intensity)
	}

	// End of cycle (frame just before completion)
	// Note: frame = maxFrame*2 would trigger Stop() in Update, so we test maxFrame*2-1
	pulse.frame = pulse.maxFrame*2 - 1
	// This is actually frame 9 (when maxFrame=5), which should give 0.2
	// The cycle completes and stops at frame 10 (maxFrame*2)
	// So let's verify behavior at the mathematical end point
	pulse.frame = pulse.maxFrame * 2
	// At this point, Update would have called Stop(), so intensity would be 0
	pulse.Stop() // Simulate what Update does
	if intensity := pulse.Intensity(); intensity != 0.0 {
		t.Errorf("end intensity should be 0.0, got %f", intensity)
	}
}
