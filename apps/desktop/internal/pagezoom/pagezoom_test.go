package pagezoom

import "testing"

func TestNextStepsUpToTheLargestLevel(t *testing.T) {
	for current, want := range map[float64]float64{0.5: 0.75, 1: 1.15, 1.0004: 1.15, 1.2: 1.25, 3: 3, 4: 3} {
		if got := nextLevel(current); got != want {
			t.Fatalf("nextLevel(%v) = %v, want %v", current, got, want)
		}
	}
}

func TestPreviousStepsDownToTheSmallestLevel(t *testing.T) {
	for current, want := range map[float64]float64{3: 2.5, 1: 0.85, 0.9996: 0.85, 1.2: 1.15, 0.5: 0.5, 0.2: 0.5} {
		if got := previousLevel(current); got != want {
			t.Fatalf("previousLevel(%v) = %v, want %v", current, got, want)
		}
	}
}
