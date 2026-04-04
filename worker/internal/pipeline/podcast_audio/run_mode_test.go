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
		BgImgFilenames: []string{"old-a.png", "old-b.png"},
		TargetPlatform: "youtube",
		AspectRatio:    "16:9",
		Resolution:     "720p",
		DesignStyle:    1,
	}
	current := dto.PodcastAudioGeneratePayload{
		ProjectID:      "proj_123",
		BgImgFilenames: []string{"new-a.png", "new-b.png", "new-c.png"},
		Resolution:     "1080p",
		DesignStyle:    2,
	}

	payload, err := buildComposePayloadForRunMode2(saved, current)
	if err != nil {
		t.Fatalf("buildComposePayloadForRunMode2 returned err: %v", err)
	}
	if payload.ProjectID != "proj_123" {
		t.Fatalf("project_id mismatch: %s", payload.ProjectID)
	}
	if len(payload.BgImgFilenames) != 3 || payload.BgImgFilenames[0] != "new-a.png" {
		t.Fatalf("expected current bg list override, got %#v", payload.BgImgFilenames)
	}
	if payload.Resolution != "1080p" {
		t.Fatalf("expected current resolution override, got %s", payload.Resolution)
	}
	if payload.DesignStyle != 2 {
		t.Fatalf("expected current design style override, got %d", payload.DesignStyle)
	}
}

func TestBuildComposePayloadForRunMode2NormalizesUnknownDesignStyleToOne(t *testing.T) {
	saved := dto.PodcastAudioGeneratePayload{
		ProjectID:      "proj_123",
		Lang:           "zh",
		BgImgFilenames: []string{"saved-a.png"},
		DesignStyle:    3,
	}

	payload, err := buildComposePayloadForRunMode2(saved, dto.PodcastAudioGeneratePayload{ProjectID: "proj_123"})
	if err != nil {
		t.Fatalf("buildComposePayloadForRunMode2 returned err: %v", err)
	}
	if payload.DesignStyle != 1 {
		t.Fatalf("expected unknown saved design style to normalize to 1, got %d", payload.DesignStyle)
	}
}

func TestBuildComposePayloadForRunMode2RejectsLanguageMismatch(t *testing.T) {
	saved := dto.PodcastAudioGeneratePayload{
		ProjectID: "proj_123",
		Lang:      "zh",
		BgImgFilenames: []string{
			"bg.png",
		},
		DesignStyle: 1,
	}
	current := dto.PodcastAudioGeneratePayload{
		ProjectID: "proj_123",
		Lang:      "ja",
	}

	if _, err := buildComposePayloadForRunMode2(saved, current); err == nil {
		t.Fatalf("expected lang mismatch error")
	}
}

func TestBuildComposePayloadForRunMode2FallsBackToBackgroundList(t *testing.T) {
	saved := dto.PodcastAudioGeneratePayload{
		ProjectID:      "proj_123",
		Lang:           "zh",
		BgImgFilenames: []string{"saved-a.png", "saved-b.png"},
		DesignStyle:    1,
	}

	payload, err := buildComposePayloadForRunMode2(saved, dto.PodcastAudioGeneratePayload{ProjectID: "proj_123"})
	if err != nil {
		t.Fatalf("buildComposePayloadForRunMode2 returned err: %v", err)
	}
	if len(payload.BgImgFilenames) != 2 {
		t.Fatalf("expected saved bg list to survive, got %#v", payload.BgImgFilenames)
	}
}
