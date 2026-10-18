package screenshot

import (
	"image"
)

// Picture is a rendered screenshot and where its characters landed, so the text can be
// selected and edited right on the image. Coordinates are image pixels.
type Picture struct {
	Image *image.RGBA
	rows  []textRow
}

// PositionAt returns the caret position nearest to point: the row under it (or the
// nearest row), between the two characters it falls between.
func (p *Picture) PositionAt(point image.Point) Position {
	rowIndex := max(0, min((point.Y-marginPx)/lineHeightPx, len(p.rows)-1))
	if point.Y < marginPx {
		rowIndex = 0
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

// rowEnd is the caret position after a row's last character.
func rowEnd(row textRow) Position {
	if len(row.chars) == 0 {
		return Position{Line: row.line}
	}
	return Position{Line: row.line, Index: row.chars[len(row.chars)-1].index + 1}
}

// charEdges are a character's left and right x in the image.
func charEdges(placed placedChar) (left, right int) {
	return marginPx + placed.x.Floor(), marginPx + (placed.x + placed.advance).Ceil()
}

// CaretBounds is the thin box to draw the caret at pos in; false when pos isn't drawn
// (a stale picture, or a space a line wrapped at).
func (p *Picture) CaretBounds(pos Position) (image.Rectangle, bool) {
	for rowIndex, row := range p.rows {
		x, ok := caretX(row, pos)
		if !ok {
			continue
		}
		top := marginPx + rowIndex*lineHeightPx
		return image.Rect(x, top, x+1, top+lineHeightPx), true
	}
	return image.Rectangle{}, false
}

func caretX(row textRow, pos Position) (int, bool) {
	if row.line != pos.Line {
		return 0, false
	}
	if len(row.chars) == 0 && pos.Index == 0 {
		return marginPx, true
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
		for _, placed := range row.chars {
			pos := Position{Line: row.line, Index: placed.index}
			if pos.Before(from) || !pos.Before(to) {
				continue
			}
			left, right := charEdges(placed)
			top := marginPx + rowIndex*lineHeightPx
			box = box.Union(image.Rect(left, top, right, top+lineHeightPx))
		}
		if !box.Empty() {
			boxes = append(boxes, box)
		}
	}
	return boxes
}
