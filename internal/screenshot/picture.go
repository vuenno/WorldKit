package screenshot

import (
	"image"
	"math"
)

// Picture is a rendered screenshot and where its characters landed, so the text can be
// selected and edited right on the image. Coordinates are image pixels.
type Picture struct {
	Image *image.RGBA
	Look  Look // the font and chat scale it was drawn with
	rows  []textRow
	grid  RowGrid
}

// PositionAt returns the caret position nearest to point: the row under it (or the
// nearest row), between the two characters it falls between.
func (p *Picture) PositionAt(point image.Point) Position {
	rowIndex := 0
	for rowIndex < len(p.rows)-1 {
		if _, bottom := p.grid.rowSpan(rowIndex); point.Y < bottom {
			break
		}
		rowIndex++
	}
	row := p.rows[rowIndex]
	for _, placed := range row.chars {
		left, right := charEdges(placed)
		if point.X < (left+right)/2 {
			return Position{Line: row.line, Index: placed.index}
		}
	}
	return rowEnd(row)
}

// LineCount returns how many lines the picture shows; every line has at least one row.
func (p *Picture) LineCount() int {
	if len(p.rows) == 0 {
		return 0
	}
	return p.rows[len(p.rows)-1].line + 1
}

// LineBounds is the band the line's rows cover, across the image's full width; false when
// the picture has no such line.
func (p *Picture) LineBounds(line int) (image.Rectangle, bool) {
	var bounds image.Rectangle
	for rowIndex, row := range p.rows {
		if row.line != line {
			continue
		}
		top, bottom := p.grid.rowSpan(rowIndex)
		bounds = bounds.Union(image.Rect(0, top, p.Image.Bounds().Dx(), bottom))
	}
	return bounds, !bounds.Empty()
}

// rowEnd is the caret position after a row's last character.
func rowEnd(row textRow) Position {
	if len(row.chars) == 0 {
		return Position{Line: row.line}
	}
	return Position{Line: row.line, Index: row.chars[len(row.chars)-1].index + 1}
}

// charEdges are a character's left and right x in the image.
func charEdges(placed placedChar) (left, right int) {
	return int(math.Floor(textOriginX + placed.x)), int(math.Ceil(textOriginX + placed.x + placed.advance))
}

// CaretBounds is the thin box to draw the caret at pos in; false when pos isn't drawn
// (a stale picture, or a space a line wrapped at).
func (p *Picture) CaretBounds(pos Position) (image.Rectangle, bool) {
	for rowIndex, row := range p.rows {
		x, ok := caretX(row, pos)
		if !ok {
			continue
		}
		top, bottom := p.grid.rowSpan(rowIndex)
		return image.Rect(x, top, x+1, bottom), true
	}
	return image.Rectangle{}, false
}

func caretX(row textRow, pos Position) (int, bool) {
	if row.line != pos.Line {
		return 0, false
	}
	if len(row.chars) == 0 && pos.Index == 0 {
		return int(math.Floor(textOriginX)), true
	}
	for i, placed := range row.chars {
		left, right := charEdges(placed)
		isLast := i == len(row.chars)-1
		switch {
		case pos.Index == placed.index:
			return left, true
		case pos.Index == placed.index+1 && isLast:
			return right, true
		}
	}
	return 0, false
}

// SelectionBounds returns one box per row covering the selected characters on it.
func (p *Picture) SelectionBounds(sel Selection) []image.Rectangle {
	from, to := sel.Ordered()
	var boxes []image.Rectangle
	for rowIndex, row := range p.rows {
		box := image.Rectangle{}
		top, bottom := p.grid.rowSpan(rowIndex)
		for _, placed := range row.chars {
			pos := Position{Line: row.line, Index: placed.index}
			if pos.Before(from) || !pos.Before(to) {
				continue
			}
			left, right := charEdges(placed)
			box = box.Union(image.Rect(left, top, right, bottom))
		}
		if !box.Empty() {
			boxes = append(boxes, box)
		}
	}
	return boxes
}
