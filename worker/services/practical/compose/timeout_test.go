package practical_compose_service

import (
	"testing"
	"time"
)

func TestComputePracticalComposeTimeoutUsesConfiguredValue(t *testing.T) {
	got := computePracticalComposeTimeout(45*time.Minute, 5*time.Minute, 377.0)
	if got != 45*time.Minute {
		t.Fatalf("unexpected timeout: got=%s want=%s", got, 45*time.Minute)
	}
}

func TestComputePracticalComposeTimeoutScalesWithAudioDuration(t *testing.T) {
	got := computePracticalComposeTimeout(0, 5*time.Minute, 377.0)
	want := 22*time.Minute + 34*time.Second
	if got != want {
		t.Fatalf("unexpected timeout: got=%s want=%s", got, want)
	}
}

func TestComputePracticalComposeTimeoutHasMinimumFloor(t *testing.T) {
	got := computePracticalComposeTimeout(0, 5*time.Minute, 30.0)
	want := 20 * time.Minute
	if got != want {
		t.Fatalf("unexpected timeout: got=%s want=%s", got, want)
	}
}
