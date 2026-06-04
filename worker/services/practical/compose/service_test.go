package practical_compose_service

import (
	"strings"
	"testing"
)

func TestBuildPracticalVideoFilterUsesHardCuts(t *testing.T) {
	segments := []practicalRenderSegment{
		{BackgroundPath: "a.png", DurationSec: 1.25},
		{BackgroundPath: "b.png", DurationSec: 2.50},
		{BackgroundPath: "c.png", DurationSec: 3.75},
	}

	filter, finalLabel, err := buildPracticalVideoFilter(segments)
	if err != nil {
		t.Fatalf("buildPracticalVideoFilter returned error: %v", err)
	}
	if finalLabel != "[vout]" {
		t.Fatalf("unexpected final label: %s", finalLabel)
	}
	if strings.Contains(filter, "xfade") {
		t.Fatalf("filter should not contain xfade: %s", filter)
	}
	if !strings.Contains(filter, "concat=n=3:v=1:a=0[vout]") {
		t.Fatalf("filter should contain concat hard cut: %s", filter)
	}
	if !strings.Contains(filter, "fps=24") {
		t.Fatalf("filter should contain fps=24: %s", filter)
	}
	if strings.Contains(filter, "scale=") {
		t.Fatalf("filter should not contain scale: %s", filter)
	}
	if strings.Contains(filter, "crop=") {
		t.Fatalf("filter should not contain crop: %s", filter)
	}
}

func TestBuildPracticalVideoFilterSingleSegment(t *testing.T) {
	filter, finalLabel, err := buildPracticalVideoFilter([]practicalRenderSegment{
		{BackgroundPath: "a.png", DurationSec: 1.25},
	})
	if err != nil {
		t.Fatalf("buildPracticalVideoFilter returned error: %v", err)
	}
	if finalLabel != "[v0]" {
		t.Fatalf("unexpected final label: %s", finalLabel)
	}
	if strings.Contains(filter, "concat=") {
		t.Fatalf("single segment filter should not contain concat: %s", filter)
	}
}
