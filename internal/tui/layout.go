package tui

import uv "github.com/charmbracelet/ultraviolet"

// Layout breakpoints and dimensions
const (
	// CompactWidthBreakpoint is the minimum width for desktop mode
	CompactWidthBreakpoint = 100
	// CompactHeightBreakpoint is the minimum height for desktop mode
	CompactHeightBreakpoint = 25
	// SidebarWidthDesktop is the width of the sidebar in desktop mode
	SidebarWidthDesktop = 45
	// StatusHeight is the height of the status bar in rows
	StatusHeight = 1
	// FooterHeight is the height of the footer in rows
	FooterHeight = 1
)

// LayoutMode represents the layout mode based on terminal size
type LayoutMode int

const (
	// LayoutDesktop is the full layout with sidebar
	LayoutDesktop LayoutMode = iota
	// LayoutCompact is the compact layout without sidebar
	LayoutCompact
)

// Layout defines the rectangular regions for all UI components
type Layout struct {
	Mode    LayoutMode
	Area    uv.Rectangle
	Content uv.Rectangle
	Main    uv.Rectangle
	Sidebar uv.Rectangle
	Status  uv.Rectangle
	Footer  uv.Rectangle
}

// IsCompact returns true if the layout is in compact mode
func (l Layout) IsCompact() bool {
	return l.Mode == LayoutCompact
}

// CalculateLayout computes the layout rectangles based on terminal dimensions
func CalculateLayout(width, height int) Layout {
	// Determine layout mode based on breakpoints
	mode := LayoutDesktop
	if width < CompactWidthBreakpoint || height < CompactHeightBreakpoint {
		mode = LayoutCompact
	}

	// Create the full area
	area := uv.Rectangle{
		Max: uv.Position{X: width, Y: height},
	}

	// Split vertically: content | footer+status
	contentRect, rest2 := uv.SplitVertical(area, uv.Fixed(area.Dy()-StatusHeight-FooterHeight))

	// Split footer+status: status | footer
	statusRect, footerRect := uv.SplitVertical(rest2, uv.Fixed(StatusHeight))

	// Split content horizontally: main | sidebar (desktop mode only)
	var mainRect, sidebarRect uv.Rectangle
	if mode == LayoutDesktop {
		// Calculate sidebar width (max 45, or 1/3 of content width)
		sidebarWidth := SidebarWidthDesktop
		if contentRect.Dx()/3 < sidebarWidth {
			sidebarWidth = contentRect.Dx() / 3
		}

		// Split horizontally: main (flexible) | gap (1 char) | sidebar (fixed width)
		mainRect, sidebarRect = uv.SplitHorizontal(contentRect, uv.Fixed(contentRect.Dx()-sidebarWidth))
		mainRect.Max.X -= 1 // 1-char gap so header rules don't visually merge
	} else {
		// Compact mode: no sidebar
		mainRect = contentRect
		sidebarRect = uv.Rectangle{} // Empty rectangle
	}

	return Layout{
		Mode:    mode,
		Area:    area,
		Content: contentRect,
		Main:    mainRect,
		Sidebar: sidebarRect,
		Status:  statusRect,
		Footer:  footerRect,
	}
}
