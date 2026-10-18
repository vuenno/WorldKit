package ui

// lineSelection is a range of whole lines picked with the mouse: a press sets the anchor,
// dragging or a Shift+press moves the other end. Lines are indexes into the visible lines.
type lineSelection struct {
	anchor   int
	end      int
	hasLines bool
}

// Press starts a new selection at line, or extends the current one when isExtending.
func (s *lineSelection) Press(line int, isExtending bool) {
	if !isExtending || !s.hasLines {
		s.anchor = line
	}
	s.end = line
	s.hasLines = true
}

// DragTo moves the free end of an active selection to line.
func (s *lineSelection) DragTo(line int) {
	if s.hasLines {
		s.end = line
	}
}

// SelectAll selects lines 0 to count-1; with no lines it clears the selection.
func (s *lineSelection) SelectAll(count int) {
	*s = lineSelection{anchor: 0, end: count - 1, hasLines: count > 0}
}

// Clear selects nothing.
func (s *lineSelection) Clear() {
	*s = lineSelection{}
}

// Range returns the first and last selected line, in order.
func (s lineSelection) Range() (first, last int, ok bool) {
	if !s.hasLines {
		return 0, 0, false
	}
	return min(s.anchor, s.end), max(s.anchor, s.end), true
}

// Contains reports whether line is selected.
func (s lineSelection) Contains(line int) bool {
	first, last, ok := s.Range()
	return ok && line >= first && line <= last
}
