package app

import (
	"fmt"
	"math"
	"strings"
	"unicode/utf8"
)

// planSchModuleFrame is shared presentation geometry, independent of circuit
// topology. All coordinates (including fontSize) are in 0.01 inch, y-UP.
// The title has its own band above the content; no Notes primitive is generated.
func planSchModuleFrame(id, title string, content, sheet layoutBBox) (schFrameSpec, error) {
	frame, err := measureSchModuleFrame(id, title, content)
	if err != nil {
		return frame, err
	}
	if !plBoxValid(sheet) || !boxInside(frame.Rect, sheet) {
		return schFrameSpec{}, fmt.Errorf("module %s frame/title outside sheet; revise the input layout (no automatic pagination)", id)
	}
	return frame, nil
}

func measureSchModuleFrame(id, title string, content layoutBBox) (schFrameSpec, error) {
	const padding, titleInset, fontSize = 15.0, 10.0, 20.0
	if strings.TrimSpace(id) == "" || strings.TrimSpace(title) == "" || !plBoxValid(content) {
		return schFrameSpec{}, fmt.Errorf("module frame requires an id, title and finite content bounds")
	}
	frame := schFrameSpec{ID: id, Title: title, FontSize: fontSize, Color: "#AA00AA", LineType: 1}
	frame.Rect = layoutBBox{
		MinX: plFloor(content.MinX - padding), MinY: plFloor(content.MinY - padding),
		MaxX: plCeil(content.MaxX + padding), MaxY: plCeil(content.MaxY + padding + fontSize + 2*titleInset),
	}
	// Full em per rune is deliberately conservative for mixed Chinese/Latin
	// titles. This is an estimate, not an API-measured rendered text bbox.
	titleWidth := float64(utf8.RuneCountInString(title)) * fontSize
	frame.Rect.MaxX = math.Max(frame.Rect.MaxX, plCeil(frame.Rect.MinX+2*titleInset+titleWidth))
	frame.TitleX, frame.TitleY = frame.Rect.MinX+titleInset, frame.Rect.MaxY-titleInset
	return frame, nil
}

// powerLayoutContentBounds includes pins and wire paths, and reserves the
// measured symbol bodies plus conservative label/marker extents. Rendered
// graphic bounds are verified after conversion; a body-only box is insufficient.
func powerLayoutContentBounds(plan *powerLayoutPlan) layoutBBox {
	b := layoutBBox{MinX: math.Inf(1), MinY: math.Inf(1), MaxX: math.Inf(-1), MaxY: math.Inf(-1)}
	include := func(x, y float64) {
		b.MinX, b.MinY = math.Min(b.MinX, x), math.Min(b.MinY, y)
		b.MaxX, b.MaxY = math.Max(b.MaxX, x), math.Max(b.MaxY, y)
	}
	for _, c := range plan.Placements {
		include(c.BBox.MinX, c.BBox.MinY-20)
		include(c.BBox.MaxX, c.BBox.MaxY+15)
		labelWidth := math.Max(plPowerTextWidth(c.Designator), plPowerTextWidth(c.Value))
		include(c.BBox.MaxX+10+labelWidth, c.BBox.MaxY)
		for _, p := range c.Pins {
			include(p.X, p.Y)
		}
	}
	for _, wire := range plan.Wires {
		for _, p := range wire.Points {
			include(p[0], p[1])
		}
	}
	for _, f := range plan.Flags {
		x, y := f.PinX, f.PinY
		switch f.Direction {
		case "left":
			x -= f.Offset
		case "right":
			x += f.Offset
		case "up":
			y += f.Offset
		case "down":
			y -= f.Offset
		}
		// Actual current ground glyph reaches 31.5 units beyond its anchor;
		// use 40 plus the net-name width, including rotated local VOUT labels.
		halfWidth := math.Max(15, plPowerTextWidth(f.Net)/2)
		dx, dy := halfWidth, 40.0
		if f.Direction == "left" || f.Direction == "right" {
			dx, dy = dy, dx
		}
		include(x-dx, y-dy)
		include(x+dx, y+dy)
	}
	return b
}
