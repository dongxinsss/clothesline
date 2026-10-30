package main

import "clothesline/internal/shelf"

// Coordinates and dimensions must use the same units. Windows passes physical
// virtual-desktop pixels, macOS passes points relative to the visible frame.
func clampBasket(p shelf.Point, area shelf.Frame, width, height int) shelf.Point {
	return shelf.Point{
		X: max(area.X, min(p.X, area.X+max(0, area.W-width))),
		Y: max(area.Y, min(p.Y, area.Y+max(0, area.H-height))),
	}
}
