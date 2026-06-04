package replay

import (
	"testing"

	dto "worker/services/practical/model"
)

func TestBuildGeneratePayloadFromSavedAndCurrentPrefersCurrentChapterNums(t *testing.T) {
	saved := dto.PracticalAudioGeneratePayload{
		ProjectID:      "ja_practical_20260601",
		Lang:           "ja",
		RunMode:        1,
		BlockNums:      []int{2},
		ScriptFilename: "demo.json",
	}
	current := dto.PracticalAudioGeneratePayload{
		ProjectID:   "ja_practical_20260601__rm1__20260603120000",
		RunMode:     1,
		ChapterNums: []int{2, 6},
	}

	got, err := BuildGeneratePayloadFromSavedAndCurrent(saved, current)
	if err != nil {
		t.Fatalf("BuildGeneratePayloadFromSavedAndCurrent returned err: %v", err)
	}
	if len(got.BlockNums) != 0 {
		t.Fatalf("expected current request to clear stale block_nums, got %#v", got.BlockNums)
	}
	if len(got.ChapterNums) != 2 || got.ChapterNums[0] != 2 || got.ChapterNums[1] != 6 {
		t.Fatalf("unexpected chapter_nums: %#v", got.ChapterNums)
	}
}

func TestBuildGeneratePayloadFromSavedAndCurrentKeepsBothCurrentSelectors(t *testing.T) {
	saved := dto.PracticalAudioGeneratePayload{
		ProjectID:      "ja_practical_20260601",
		Lang:           "ja",
		RunMode:        1,
		ScriptFilename: "demo.json",
	}
	current := dto.PracticalAudioGeneratePayload{
		ProjectID:   "ja_practical_20260601__rm1__20260603120000",
		RunMode:     1,
		BlockNums:   []int{1},
		ChapterNums: []int{4},
	}

	got, err := BuildGeneratePayloadFromSavedAndCurrent(saved, current)
	if err != nil {
		t.Fatalf("BuildGeneratePayloadFromSavedAndCurrent returned err: %v", err)
	}
	if len(got.BlockNums) != 1 || got.BlockNums[0] != 1 {
		t.Fatalf("unexpected block_nums: %#v", got.BlockNums)
	}
	if len(got.ChapterNums) != 1 || got.ChapterNums[0] != 4 {
		t.Fatalf("unexpected chapter_nums: %#v", got.ChapterNums)
	}
}

func TestBuildGeneratePayloadFromSavedAndCurrentClearsSavedSelectorsWhenCurrentOmitsThem(t *testing.T) {
	saved := dto.PracticalAudioGeneratePayload{
		ProjectID:      "ja_practical_20260601",
		Lang:           "ja",
		RunMode:        1,
		BlockNums:      []int{2},
		ChapterNums:    []int{7},
		ScriptFilename: "demo.json",
	}
	current := dto.PracticalAudioGeneratePayload{
		ProjectID: "ja_practical_20260601__rm1__20260603120000",
		RunMode:   1,
	}

	got, err := BuildGeneratePayloadFromSavedAndCurrent(saved, current)
	if err != nil {
		t.Fatalf("BuildGeneratePayloadFromSavedAndCurrent returned err: %v", err)
	}
	if len(got.BlockNums) != 0 {
		t.Fatalf("expected block_nums to be cleared, got %#v", got.BlockNums)
	}
	if len(got.ChapterNums) != 0 {
		t.Fatalf("expected chapter_nums to be cleared, got %#v", got.ChapterNums)
	}
}
