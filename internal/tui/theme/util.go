package theme

import "fmt"

// InterpolateColor blends between two hex colors based on position (0.0 to 1.0)
func InterpolateColor(colorA, colorB string, pos float64) string {
	// Parse hex colors (format: #RRGGBB)
	r1, g1, b1 := ParseHexColor(colorA)
	r2, g2, b2 := ParseHexColor(colorB)

	// Interpolate each channel
	r := uint8(float64(r1)*(1-pos) + float64(r2)*pos)
	g := uint8(float64(g1)*(1-pos) + float64(g2)*pos)
	b := uint8(float64(b1)*(1-pos) + float64(b2)*pos)

	// Return as hex color string
	return FormatHexColor(r, g, b)
}

// ParseHexColor extracts RGB values from hex color string
func ParseHexColor(hex string) (uint8, uint8, uint8) {
	// Remove # prefix if present
	if len(hex) > 0 && hex[0] == '#' {
		hex = hex[1:]
	}

	// Parse RGB values
	var r, g, b uint8
	if len(hex) == 6 {
		_, _ = fmt.Sscanf(hex, "%02x%02x%02x", &r, &g, &b)
	}

	return r, g, b
}

// FormatHexColor converts RGB values to hex color string
func FormatHexColor(r, g, b uint8) string {
	return fmt.Sprintf("#%02x%02x%02x", r, g, b)
}
