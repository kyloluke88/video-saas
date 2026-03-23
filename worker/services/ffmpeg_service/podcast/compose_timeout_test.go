package podcast

import (
	"testing"
	"time"
)

func TestComputePodcastComposeTimeout_UsesConfiguredOverride(t *testing.T) {
	got := computePodcastComposeTimeout(45*time.Minute, 5*time.Minute, 977.0)
	if got != 45*time.Minute {
		t.Fatalf("unexpected timeout: got %s want %s", got, 45*time.Minute)
	}
}

func TestComputePodcastComposeTimeout_ExpandsForLongAudio(t *testing.T) {
	got := computePodcastComposeTimeout(0, 5*time.Minute, 977.0)
	want := time.Duration(977.0*float64(time.Second))*2 + 10*time.Minute
	if got != want {
		t.Fatalf("unexpected timeout: got %s want %s", got, want)
	}
}

func TestComputePodcastComposeTimeout_HasMinimumFloor(t *testing.T) {
	got := computePodcastComposeTimeout(0, 5*time.Minute, 30.0)
	if got != 20*time.Minute {
		t.Fatalf("unexpected timeout floor: got %s want %s", got, 20*time.Minute)
	}
}
