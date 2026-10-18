package ui

import (
	"testing"
	"time"
)

func TestExpandAnimationReachesTargetAfterDuration(t *testing.T) {
	start := time.Now()
	animation := newExpandAnimation(200*time.Millisecond, true)

	animation.Toggle(start)

	tests := []struct {
		name    string
		elapsed time.Duration
		want    float32
	}{
		{"at toggle time", 0, 1},
		{"halfway", 100 * time.Millisecond, 0.5},
		{"finished", 200 * time.Millisecond, 0},
		{"long after", time.Second, 0},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := animation.linearProgress(start.Add(test.elapsed)); got != test.want {
				t.Errorf("linearProgress = %v, want %v", got, test.want)
			}
		})
	}
}

func TestExpandAnimationReversesFromCurrentPosition(t *testing.T) {
	start := time.Now()
	animation := newExpandAnimation(200*time.Millisecond, true)

	animation.Toggle(start)
	reversedAt := start.Add(50 * time.Millisecond)
	animation.Toggle(reversedAt)

	if got := animation.linearProgress(reversedAt); got != 0.75 {
		t.Errorf("progress right after reversing = %v, want 0.75", got)
	}
	if got := animation.linearProgress(reversedAt.Add(50 * time.Millisecond)); got != 1 {
		t.Errorf("progress after returning = %v, want 1", got)
	}
}

func TestEaseInOutCubic(t *testing.T) {
	for input, want := range map[float32]float32{0: 0, 0.5: 0.5, 1: 1} {
		if got := easeInOutCubic(input); got != want {
			t.Errorf("easeInOutCubic(%v) = %v, want %v", input, got, want)
		}
	}
}
