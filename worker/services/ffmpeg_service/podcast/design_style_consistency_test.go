package podcast

import "testing"

func TestDesignStyleKeepsChineseSubtitlePresetConsistent(t *testing.T) {
	base := chineseSubtitlePresetFor(1)
	if chineseSubtitlePresetFor(2) != base {
		t.Fatalf("expected style 2 chinese subtitle preset to match style 1")
	}
	if chineseSubtitlePresetFor(3) != base {
		t.Fatalf("expected style 3 chinese subtitle preset to match style 1")
	}
}

func TestDesignStyleKeepsJapaneseSubtitlePresetConsistent(t *testing.T) {
	base := japaneseSubtitlePresetFor(1)
	if japaneseSubtitlePresetFor(2) != base {
		t.Fatalf("expected style 2 japanese subtitle preset to match style 1")
	}
	if japaneseSubtitlePresetFor(3) != base {
		t.Fatalf("expected style 3 japanese subtitle preset to match style 1")
	}
}

func TestDesignStyleKeepsWaveformPresetConsistent(t *testing.T) {
	base := waveformPresetFor(1, "1080p")
	if waveformPresetFor(2, "1080p") != base {
		t.Fatalf("expected style 2 waveform preset to match style 1")
	}
	if waveformPresetFor(3, "1080p") != base {
		t.Fatalf("expected style 3 waveform preset to match style 1")
	}
}
