package ui

import (
	"time"

	"gioui.org/layout"
	"gioui.org/op"
)

// expandAnimation smoothly moves between collapsed (0) and expanded (1).
// Toggling mid-animation reverses from the current position, so it never jumps.
type expandAnimation struct {
	duration      time.Duration
	isExpanded    bool
	startProgress float32
	startedAt     time.Time
}

func newExpandAnimation(duration time.Duration, isExpanded bool) expandAnimation {
	animation := expandAnimation{duration: duration, isExpanded: isExpanded}
	if isExpanded {
		animation.startProgress = 1
	}
	return animation
}

// Toggle flips the target state, starting from wherever the animation is now.
func (a *expandAnimation) Toggle(now time.Time) {
	a.startProgress = a.linearProgress(now)
	a.isExpanded = !a.isExpanded
	a.startedAt = now
}

// Progress returns the eased progress in [0, 1] and requests another frame
// while the animation is still running.
func (a *expandAnimation) Progress(gtx layout.Context) float32 {
	linear := a.linearProgress(gtx.Now)
	if linear != a.targetProgress() {
		gtx.Execute(op.InvalidateCmd{})
	}
	return easeInOutCubic(linear)
}

func (a *expandAnimation) targetProgress() float32 {
	if a.isExpanded {
		return 1
	}
	return 0
}

func (a *expandAnimation) linearProgress(now time.Time) float32 {
	step := float32(now.Sub(a.startedAt)) / float32(a.duration)
	if a.isExpanded {
		return min(a.startProgress+step, 1)
	}
	return max(a.startProgress-step, 0)
}

func easeInOutCubic(t float32) float32 {
	if t < 0.5 {
		return 4 * t * t * t
	}
	shifted := -2*t + 2
	return 1 - shifted*shifted*shifted/2
}
