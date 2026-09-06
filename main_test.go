package main

import "testing"

func TestMemoDrawsEachFrameOnce(t *testing.T) {
	calls := 0
	draw := memo(func(angle float64) string {
		calls++
		return "frame"
	})
	for turn := 0; turn < 3; turn++ {
		for step := 0; step < turnSteps; step++ {
			if got := draw(step); got != "frame" {
				t.Fatalf("step %d returned %q", step, got)
			}
		}
	}
	if calls != turnSteps {
		t.Errorf("drew %d frames over three turns, want %d", calls, turnSteps)
	}
}

// The animation has to close for memoing to be correct: the outlet's pulse
// must complete a whole number of cycles per turn, or frame 0 of the second
// turn would differ from frame 0 of the first.
func TestAnimationIsPeriodic(t *testing.T) {
	for _, g := range []glyphSet{sextants, halfBlocks} {
		if a, b := Render(0, g), Render(2*3.141592653589793, g); a != b {
			t.Errorf("%s: a full turn does not return to the same frame", g.name)
		}
	}
}
