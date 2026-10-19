package compose

// maxHistory bounds undo; each step is a copy of the layer list (no pixels), so it is small.
const maxHistory = 200

// History keeps earlier and undone versions of a composition for undo and redo.
type History struct {
	undo []Composition
	redo []Composition
}

// Remember saves current as the state to return to on the next Undo, and forgets redo.
func (h *History) Remember(current Composition) {
	h.undo = append(h.undo, current.Clone())
	if len(h.undo) > maxHistory {
		h.undo = h.undo[1:]
	}
	h.redo = nil
}

// Undo returns the previous version, keeping current for Redo.
func (h *History) Undo(current Composition) (Composition, bool) {
	return step(&h.undo, &h.redo, current)
}

// Redo returns the version Undo left, keeping current for Undo.
func (h *History) Redo(current Composition) (Composition, bool) {
	return step(&h.redo, &h.undo, current)
}

func step(from, to *[]Composition, current Composition) (Composition, bool) {
	last := len(*from) - 1
	if last < 0 {
		return current, false
	}
	previous := (*from)[last]
	*from = (*from)[:last]
	*to = append(*to, current.Clone())
	return previous, true
}

// CanUndo reports whether Undo has anything to return to.
func (h *History) CanUndo() bool { return len(h.undo) > 0 }

// CanRedo reports whether Redo has anything to return to.
func (h *History) CanRedo() bool { return len(h.redo) > 0 }

// Clear forgets every version, e.g. for a new canvas.
func (h *History) Clear() {
	h.undo, h.redo = nil, nil
}
