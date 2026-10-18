package ui

// loadResult carries the outcome of a background read back to the UI goroutine.
type loadResult[T any] struct {
	value T
	err   error
}

// backgroundLoad runs a slow read (disk, parsing) off the UI goroutine, one at a time.
// A Start while a read is running re-runs it once that read finishes, so the newest
// request always wins. Reads are finite, so their goroutines end on their own.
type backgroundLoad[T any] struct {
	results    chan loadResult[T]
	read       func() (T, error)
	isLoading  bool
	isOutdated bool
	invalidate func()
}

func newBackgroundLoad[T any](invalidate func()) *backgroundLoad[T] {
	// Buffered so a finished read never blocks, even if the UI stops polling.
	return &backgroundLoad[T]{results: make(chan loadResult[T], 1), invalidate: invalidate}
}

// Start runs read in the background; Result delivers its outcome.
func (l *backgroundLoad[T]) Start(read func() (T, error)) {
	l.read = read
	if l.isLoading {
		l.isOutdated = true
		return
	}
	l.isLoading = true
	go func() {
		value, err := read()
		l.results <- loadResult[T]{value: value, err: err}
		l.invalidate()
	}()
}

// Result returns the newest finished read, if one arrived since the last call.
func (l *backgroundLoad[T]) Result() (loadResult[T], bool) {
	select {
	case result := <-l.results:
		l.isLoading = false
		if l.isOutdated {
			l.isOutdated = false
			l.Start(l.read)
			return loadResult[T]{}, false
		}
		return result, true
	default:
		return loadResult[T]{}, false
	}
}

// IsLoading reports whether a read is still running.
func (l *backgroundLoad[T]) IsLoading() bool {
	return l.isLoading
}
