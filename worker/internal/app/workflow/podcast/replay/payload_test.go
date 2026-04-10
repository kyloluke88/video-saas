package replay

import (
	"testing"

	dto "worker/services/podcast/model"
)

func TestBuildComposePayloadFromSavedAndCurrentUsesCurrentVisualOverrides(t *testing.T) {
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
		ProjectID:      "proj_123__rm1__20260409120000",
		BgImgFilenames: []string{"new-a.png", "new-b.png", "new-c.png"},
		Resolution:     "1080p",
		DesignStyle:    2,
	}

	replayPayload, err := BuildGeneratePayloadFromSavedAndCurrent(saved, current)
	if err != nil {
		t.Fatalf("BuildGeneratePayloadFromSavedAndCurrent returned err: %v", err)
	}
	payload, err := BuildComposePayloadFromGenerate(replayPayload)
	if err != nil {
		t.Fatalf("BuildComposePayloadFromGenerate returned err: %v", err)
	}
	if payload.ProjectID != "proj_123__rm1__20260409120000" {
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

func TestBuildGeneratePayloadFromSavedAndCurrentUsesCurrentVisualOverrides(t *testing.T) {
	saved := dto.PodcastAudioGeneratePayload{
		ProjectID:       "proj_123",
		Lang:            "zh",
		Title:           "old title",
		ScriptFilename:  "old.json",
		BgImgFilenames:  []string{"old-a.png", "old-b.png"},
		TargetPlatform:  "youtube",
		AspectRatio:     "16:9",
		Resolution:      "720p",
		DesignStyle:     1,
		TTSType:         2,
		Seed:            11,
		OnlyCurrentStep: 1,
		BlockNums:       []int{1, 2},
	}
	current := dto.PodcastAudioGeneratePayload{
		ProjectID:       "proj_123__rm1__20260409120000",
		BgImgFilenames:  []string{"new-a.png"},
		Resolution:      "1080p",
		DesignStyle:     2,
		AspectRatio:     "9:16",
		TargetPlatform:  "bilibili",
		Title:           "new title",
		BlockNums:       []int{3, 4},
		RunMode:         4,
		OnlyCurrentStep: 0,
	}

	payload, err := BuildGeneratePayloadFromSavedAndCurrent(saved, current)
	if err != nil {
		t.Fatalf("BuildGeneratePayloadFromSavedAndCurrent returned err: %v", err)
	}
	if len(payload.BgImgFilenames) != 1 || payload.BgImgFilenames[0] != "new-a.png" {
		t.Fatalf("expected current bg override, got %#v", payload.BgImgFilenames)
	}
	if payload.ProjectID != "proj_123__rm1__20260409120000" {
		t.Fatalf("expected replay target project id, got %s", payload.ProjectID)
	}
	if payload.Resolution != "1080p" {
		t.Fatalf("expected current resolution override, got %s", payload.Resolution)
	}
	if payload.DesignStyle != 2 {
		t.Fatalf("expected current design style override, got %d", payload.DesignStyle)
	}
	if payload.AspectRatio != "9:16" {
		t.Fatalf("expected current aspect ratio override, got %s", payload.AspectRatio)
	}
	if payload.TargetPlatform != "bilibili" {
		t.Fatalf("expected current target platform override, got %s", payload.TargetPlatform)
	}
	if payload.Title != "new title" {
		t.Fatalf("expected current title override, got %s", payload.Title)
	}
	if len(payload.BlockNums) != 2 || payload.BlockNums[0] != 3 {
		t.Fatalf("expected current block_nums override, got %#v", payload.BlockNums)
	}
	if payload.TTSType != 2 {
		t.Fatalf("expected saved tts_type preserved, got %d", payload.TTSType)
	}
	if payload.ScriptFilename != "old.json" {
		t.Fatalf("expected saved script filename preserved, got %s", payload.ScriptFilename)
	}
	if payload.RunMode != 4 {
		t.Fatalf("expected current run mode preserved, got %d", payload.RunMode)
	}
	if payload.OnlyCurrentStep != 0 {
		t.Fatalf("expected current only_current_step override, got %d", payload.OnlyCurrentStep)
	}
}

func TestBuildComposePayloadFromSavedAndCurrentNormalizesUnknownDesignStyleToOne(t *testing.T) {
	saved := dto.PodcastAudioGeneratePayload{
		ProjectID:      "proj_123",
		Lang:           "zh",
		BgImgFilenames: []string{"saved-a.png"},
		DesignStyle:    3,
	}

	replayPayload, err := BuildGeneratePayloadFromSavedAndCurrent(saved, dto.PodcastAudioGeneratePayload{
		ProjectID: "proj_123__rm1__20260409120000",
	})
	if err != nil {
		t.Fatalf("BuildGeneratePayloadFromSavedAndCurrent returned err: %v", err)
	}
	payload, err := BuildComposePayloadFromGenerate(replayPayload)
	if err != nil {
		t.Fatalf("BuildComposePayloadFromGenerate returned err: %v", err)
	}
	if payload.DesignStyle != 1 {
		t.Fatalf("expected unknown saved design style to normalize to 1, got %d", payload.DesignStyle)
	}
}

func TestBuildGeneratePayloadFromSavedAndCurrentRejectsLanguageMismatch(t *testing.T) {
	saved := dto.PodcastAudioGeneratePayload{
		ProjectID: "proj_123",
		Lang:      "zh",
		BgImgFilenames: []string{
			"bg.png",
		},
		DesignStyle: 1,
	}
	current := dto.PodcastAudioGeneratePayload{
		ProjectID: "proj_123__rm1__20260409120000",
		Lang:      "ja",
	}

	if _, err := BuildGeneratePayloadFromSavedAndCurrent(saved, current); err == nil {
		t.Fatalf("expected lang mismatch error")
	}
}

func TestBuildComposePayloadFromSavedAndCurrentFallsBackToBackgroundList(t *testing.T) {
	saved := dto.PodcastAudioGeneratePayload{
		ProjectID:      "proj_123",
		Lang:           "zh",
		BgImgFilenames: []string{"saved-a.png", "saved-b.png"},
		DesignStyle:    1,
	}

	replayPayload, err := BuildGeneratePayloadFromSavedAndCurrent(saved, dto.PodcastAudioGeneratePayload{
		ProjectID: "proj_123__rm1__20260409120000",
	})
	if err != nil {
		t.Fatalf("BuildGeneratePayloadFromSavedAndCurrent returned err: %v", err)
	}
	payload, err := BuildComposePayloadFromGenerate(replayPayload)
	if err != nil {
		t.Fatalf("BuildComposePayloadFromGenerate returned err: %v", err)
	}
	if len(payload.BgImgFilenames) != 2 {
		t.Fatalf("expected saved bg list to survive, got %#v", payload.BgImgFilenames)
	}
}

func TestResolveSourceProjectIDPrefersExplicitSourceProjectID(t *testing.T) {
	sourceProjectID, err := ResolveSourceProjectID(dto.PodcastAudioGeneratePayload{
		ProjectID:       "zh_podcast_20260408154607__rm3__20260409180433",
		SourceProjectID: "zh_podcast_20260408154607__rm1__20260409171630",
	})
	if err != nil {
		t.Fatalf("ResolveSourceProjectID returned err: %v", err)
	}
	if sourceProjectID != "zh_podcast_20260408154607__rm1__20260409171630" {
		t.Fatalf("unexpected source project id: %s", sourceProjectID)
	}
}
