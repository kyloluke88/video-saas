package pipeline

import (
	"testing"

	dto "worker/services/practical/model"
)

func TestMergePayloadPrefersCurrentChapterNums(t *testing.T) {
	saved := dto.PracticalAudioGeneratePayload{
		ProjectID:      "ja_practical_20260601",
		Lang:           "ja",
		RunMode:        1,
		BlockNums:      []int{2},
		ScriptFilename: "demo.json",
	}
	current := dto.PracticalAudioGeneratePayload{
		ProjectID:   "ja_practical_20260601",
		RunMode:     1,
		StartFrom:   "align",
		ChapterNums: []int{2, 6},
	}

	got, err := MergePayload(saved, current)
	if err != nil {
		t.Fatalf("MergePayload returned err: %v", err)
	}
	if len(got.BlockNums) != 0 {
		t.Fatalf("expected current request to clear stale block_nums, got %#v", got.BlockNums)
	}
	if len(got.ChapterNums) != 2 || got.ChapterNums[0] != 2 || got.ChapterNums[1] != 6 {
		t.Fatalf("unexpected chapter_nums: %#v", got.ChapterNums)
	}
}

func TestMergePayloadKeepsBothCurrentSelectors(t *testing.T) {
	saved := dto.PracticalAudioGeneratePayload{
		ProjectID:      "ja_practical_20260601",
		Lang:           "ja",
		RunMode:        1,
		ScriptFilename: "demo.json",
	}
	current := dto.PracticalAudioGeneratePayload{
		ProjectID:   "ja_practical_20260601",
		RunMode:     1,
		StartFrom:   "generate",
		BlockNums:   []int{1},
		ChapterNums: []int{4},
	}

	got, err := MergePayload(saved, current)
	if err != nil {
		t.Fatalf("MergePayload returned err: %v", err)
	}
	if len(got.BlockNums) != 1 || got.BlockNums[0] != 1 {
		t.Fatalf("unexpected block_nums: %#v", got.BlockNums)
	}
	if len(got.ChapterNums) != 1 || got.ChapterNums[0] != 4 {
		t.Fatalf("unexpected chapter_nums: %#v", got.ChapterNums)
	}
}

func TestMergePayloadClearsSavedSelectorsWhenCurrentOmitsThem(t *testing.T) {
	saved := dto.PracticalAudioGeneratePayload{
		ProjectID:      "ja_practical_20260601",
		Lang:           "ja",
		RunMode:        1,
		BlockNums:      []int{2},
		ChapterNums:    []int{7},
		ScriptFilename: "demo.json",
	}
	current := dto.PracticalAudioGeneratePayload{
		ProjectID: "ja_practical_20260601",
		RunMode:   1,
		StartFrom: "render",
	}

	got, err := MergePayload(saved, current)
	if err != nil {
		t.Fatalf("MergePayload returned err: %v", err)
	}
	if len(got.BlockNums) != 0 {
		t.Fatalf("expected block_nums to be cleared, got %#v", got.BlockNums)
	}
	if len(got.ChapterNums) != 0 {
		t.Fatalf("expected chapter_nums to be cleared, got %#v", got.ChapterNums)
	}
}

func TestNextStageStopsAtConfiguredStage(t *testing.T) {
	next, ok, err := NextStage(string(StageImages), string(StageImages))
	if err != nil {
		t.Fatalf("NextStage returned err: %v", err)
	}
	if ok || next != "" {
		t.Fatalf("expected stop_at to pause at images, got next=%s ok=%v", next, ok)
	}
}
