package pipeline

import (
	"testing"

	dto "worker/services/podcast/model"
)

func TestMergeGeneratePayloadUsesCurrentVisualOverrides(t *testing.T) {
	saved := dto.PodcastAudioGeneratePayload{
		ProjectID:      "proj_123",
		Lang:           "zh",
		ScriptFilename: "old.json",
		BgImgFilenames: []string{"old-a.png", "old-b.png"},
		TargetPlatform: "youtube",
		AspectRatio:    "16:9",
		Resolution:     "720p",
		DesignStyle:    1,
		TTSType:        2,
		Seed:           11,
	}
	current := dto.PodcastAudioGeneratePayload{
		ProjectID:      "proj_123",
		BgImgFilenames: []string{"new-a.png"},
		TargetPlatform: "tiktok",
		AspectRatio:    "9:16",
		Resolution:     "1080p",
		DesignStyle:    2,
		BlockNums:      []int{3, 4},
		RunMode:        1,
		StartFrom:      "render",
		StopAt:         "persist",
	}

	payload, err := MergeGeneratePayload(saved, current)
	if err != nil {
		t.Fatalf("MergeGeneratePayload returned err: %v", err)
	}
	if len(payload.BgImgFilenames) != 1 || payload.BgImgFilenames[0] != "new-a.png" {
		t.Fatalf("expected current bg override, got %#v", payload.BgImgFilenames)
	}
	if payload.Resolution != "1080p" {
		t.Fatalf("expected current resolution override, got %s", payload.Resolution)
	}
	if payload.DesignStyle != 2 {
		t.Fatalf("expected current design style override, got %d", payload.DesignStyle)
	}
	if payload.TargetPlatform != "tiktok" {
		t.Fatalf("expected current target platform override, got %s", payload.TargetPlatform)
	}
	if payload.AspectRatio != "9:16" {
		t.Fatalf("expected current aspect ratio override, got %s", payload.AspectRatio)
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
	if payload.StartFrom != "render" || payload.StopAt != "persist" {
		t.Fatalf("unexpected stage range: start=%s stop=%s", payload.StartFrom, payload.StopAt)
	}
}

func TestBuildComposePayloadFromGenerateFallsBackToSavedBackgrounds(t *testing.T) {
	payload, err := BuildComposePayloadFromGenerate(dto.PodcastAudioGeneratePayload{
		ProjectID:      "proj_123",
		Lang:           "zh",
		BgImgFilenames: []string{"saved-a.png", "saved-b.png"},
		DesignStyle:    3,
		StartFrom:      "render",
	})
	if err != nil {
		t.Fatalf("BuildComposePayloadFromGenerate returned err: %v", err)
	}
	if len(payload.BgImgFilenames) != 2 {
		t.Fatalf("expected saved bg list to survive, got %#v", payload.BgImgFilenames)
	}
	if payload.DesignStyle != 1 {
		t.Fatalf("expected design style normalize to 1, got %d", payload.DesignStyle)
	}
}

func TestMergeGeneratePayloadRejectsLanguageMismatch(t *testing.T) {
	saved := dto.PodcastAudioGeneratePayload{
		ProjectID:      "proj_123",
		Lang:           "zh",
		BgImgFilenames: []string{"bg.png"},
		DesignStyle:    1,
	}
	current := dto.PodcastAudioGeneratePayload{
		ProjectID: "proj_123",
		Lang:      "ja",
	}
	if _, err := MergeGeneratePayload(saved, current); err == nil {
		t.Fatalf("expected lang mismatch error")
	}
}

func TestNextStageSkipsAlignForType2(t *testing.T) {
	next, ok, err := NextStage(2, string(StageGenerate), "")
	if err != nil {
		t.Fatalf("NextStage returned err: %v", err)
	}
	if !ok || next != string(StageRender) {
		t.Fatalf("expected type2 generate to flow into render, got next=%s ok=%v", next, ok)
	}
}

func TestNextStageStopsAtConfiguredStage(t *testing.T) {
	next, ok, err := NextStage(1, string(StageRender), string(StageRender))
	if err != nil {
		t.Fatalf("NextStage returned err: %v", err)
	}
	if ok || next != "" {
		t.Fatalf("expected stop_at to pause at render, got next=%s ok=%v", next, ok)
	}
}
