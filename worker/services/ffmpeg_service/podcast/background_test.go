package podcast

import (
	"strings"
	"testing"
)

func TestBackgroundGraphForProvidesDistinctPresetGraphs(t *testing.T) {
	calm := backgroundGraphFor(1, "1080p")
	parallax := backgroundGraphFor(2, "1080p")
	glow := backgroundGraphFor(3, "1080p")

	if !strings.Contains(calm, "crop=") || !strings.Contains(calm, "[bg]") {
		t.Fatalf("calm drift graph missing expected crop/bg output: %s", calm)
	}
	if !strings.Contains(parallax, "split=2") || !strings.Contains(parallax, "gblur=sigma=22") {
		t.Fatalf("soft parallax graph missing expected split/blur: %s", parallax)
	}
	if !strings.Contains(glow, "color=c=0xF6D49A") || !strings.Contains(glow, "overlay=0:0") {
		t.Fatalf("study glow graph missing expected glow overlay: %s", glow)
	}
}
