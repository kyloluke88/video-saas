package podcast

import (
	"strings"
	"testing"
)

func TestWaveformPresetForStyle1UsesConfiguredTopPositionAndWhiteColor(t *testing.T) {
	preset := waveformPresetFor("1080p", 1, 1)
	if got, want := preset.Overlay, "(W-w)/2:515"; got != want {
		t.Fatalf("unexpected style1 waveform overlay: got %s want %s", got, want)
	}
	if got, want := preset.AudioGraph, "colors=0xFFFFFF"; !strings.Contains(got, want) {
		t.Fatalf("expected style1 waveform color %s in %s", want, got)
	}
	for _, want := range []string{"volume=2.40", "showwaves=s=576x51", "0.28+0.72*pow(max(0,1-abs(2*X/W-1)),1.80)", "gblur=sigma=0.30"} {
		if !strings.Contains(preset.AudioGraph, want) {
			t.Fatalf("expected style1 waveform graph to contain %q in %s", want, preset.AudioGraph)
		}
	}
}

func TestWaveformPresetForStyle2KeepsLegacyPositionAndColor(t *testing.T) {
	preset := waveformPresetFor("1080p", 2, 1)
	if got, want := preset.Overlay, "(W-w)/2:50"; got != want {
		t.Fatalf("unexpected style2 waveform overlay: got %s want %s", got, want)
	}
	if got, want := preset.AudioGraph, "colors=0x8F4BB3"; !strings.Contains(got, want) {
		t.Fatalf("expected style2 waveform color %s in %s", want, got)
	}
}
