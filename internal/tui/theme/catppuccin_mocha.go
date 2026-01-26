package theme

// NewCatppuccinMocha creates the default Catppuccin Mocha theme.
// For tracer bullet, this contains only 3 colors: Primary, BgBase, FgBase.
func NewCatppuccinMocha() *Theme {
	return &Theme{
		Name:   "catppuccin-mocha",
		IsDark: true,

		// Semantic colors
		Primary: "#cba6f7", // Mauve

		// Background hierarchy
		BgBase: "#1e1e2e", // Base background

		// Foreground hierarchy
		FgBase: "#cdd6f4", // Main text color

		// Diff colors
		DiffInsertBg:  "#303a30", // Green-tinted background for insertions
		DiffDeleteBg:  "#3a3030", // Red-tinted background for deletions
		DiffEqualBg:   "#1e1e2e", // Neutral background for context lines
		DiffMissingBg: "#181825", // Dim background for empty sides
	}
}
