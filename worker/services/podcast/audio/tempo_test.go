package podcast_audio_service

import "testing"

func TestResolvePodcastTempoUsesJapaneseOverride(t *testing.T) {
	if got := resolvePodcastTempo("ja", 1.0, 0.85); got != 0.85 {
		t.Fatalf("expected Japanese override 0.85, got %v", got)
	}
}

func TestResolvePodcastTempoFallsBackToBase(t *testing.T) {
	if got := resolvePodcastTempo("zh", 1.0, 0.85); got != 1.0 {
		t.Fatalf("expected base tempo 1.0, got %v", got)
	}
}

func TestResolvePodcastTempoFallsBackToUnity(t *testing.T) {
	if got := resolvePodcastTempo("zh", 0, 0); got != 1.0 {
		t.Fatalf("expected fallback tempo 1.0, got %v", got)
	}
}
