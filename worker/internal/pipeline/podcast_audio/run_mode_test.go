package podcast_audio

import (
	"testing"

	"worker/internal/dto"
)

func TestBuildComposePayloadForRunMode2UsesCurrentVisualOverrides(t *testing.T) {
	saved := dto.PodcastAudioGeneratePayload{
		ProjectID:      "proj_123",
		Lang:           "zh",
		Title:          "old title",
		BgImgFilename:  "old-bg.png",
		TargetPlatform: "youtube",
		AspectRatio:    "16:9",
		Resolution:     "720p",
		DesignStyle:    1,
	}
	current := dto.PodcastAudioGeneratePayload{
		ProjectID:     "proj_123",
		BgImgFilename: "new-bg.png",
		Resolution:    "1080p",
		DesignStyle:   3,
	}

	payload, err := buildComposePayloadForRunMode2(saved, current)
	if err != nil {
		t.Fatalf("buildComposePayloadForRunMode2 returned err: %v", err)
	}
	if payload.ProjectID != "proj_123" {
		t.Fatalf("project_id mismatch: %s", payload.ProjectID)
	}
	if payload.BgImgFilename != "new-bg.png" {
		t.Fatalf("expected current bg override, got %s", payload.BgImgFilename)
	}
	if payload.Resolution != "1080p" {
		t.Fatalf("expected current resolution override, got %s", payload.Resolution)
	}
	if payload.DesignStyle != 3 {
		t.Fatalf("expected current design style override, got %d", payload.DesignStyle)
	}
}

func TestBuildComposePayloadForRunMode2RejectsLanguageMismatch(t *testing.T) {
	saved := dto.PodcastAudioGeneratePayload{
		ProjectID:     "proj_123",
		Lang:          "zh",
		BgImgFilename: "bg.png",
		DesignStyle:   1,
	}
	current := dto.PodcastAudioGeneratePayload{
		ProjectID: "proj_123",
		Lang:      "ja",
	}

	if _, err := buildComposePayloadForRunMode2(saved, current); err == nil {
		t.Fatalf("expected lang mismatch error")
	}
}
