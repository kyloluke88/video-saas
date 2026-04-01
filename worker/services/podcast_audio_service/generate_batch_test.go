package podcast_audio_service

import (
	"errors"
	"os"
	"strings"
	"testing"

	"worker/internal/dto"
	services "worker/services"
)

func TestBuildRequestedBlockSet_DeduplicatesOneBasedInput(t *testing.T) {
	selected, err := buildRequestedBlockSet([]int{5, 1, 5, 3}, 6)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(selected) != 3 {
		t.Fatalf("expected 3 selected blocks, got %d", len(selected))
	}
	for _, index := range []int{0, 2, 4} {
		if _, ok := selected[index]; !ok {
			t.Fatalf("expected block index %d to be selected", index)
		}
	}
}

func TestBuildRequestedBlockSet_RejectsOutOfRangeValues(t *testing.T) {
	_, err := buildRequestedBlockSet([]int{2, 9}, 4)
	if err == nil {
		t.Fatalf("expected error when block_num exceeds block count")
	}
	var nonRetryable services.NonRetryableError
	if !errors.As(err, &nonRetryable) {
		t.Fatalf("expected non-retryable error, got %T", err)
	}
	if !strings.Contains(err.Error(), "block_nums out of range") {
		t.Fatalf("unexpected error message: %v", err)
	}
}

func TestValidateGoogleBlocks_ReturnsNonRetryableWhenSegmentTextExceedsGoogleFieldLimit(t *testing.T) {
	blocks := []dto.PodcastBlock{
		{BlockID: "b1", Segments: []dto.PodcastSegment{{Speaker: "male", SegmentID: "seg_001", Text: strings.Repeat("甲", 1400)}}},
	}
	err := validateGoogleBlocks("zh", blocks)
	if err == nil {
		t.Fatalf("expected error when segment text exceeds google limit")
	}
	var nonRetryable services.NonRetryableError
	if !errors.As(err, &nonRetryable) {
		t.Fatalf("expected non-retryable error, got %T", err)
	}
	if !strings.Contains(err.Error(), "exceeds google 4000-byte limit") {
		t.Fatalf("unexpected error message: %v", err)
	}
}

func TestMarkScriptLoadNonRetryable_MissingFile(t *testing.T) {
	err := markScriptLoadNonRetryable("/tmp/missing.json", os.ErrNotExist)
	var nonRetryable services.NonRetryableError
	if !errors.As(err, &nonRetryable) {
		t.Fatalf("expected non-retryable error, got %T", err)
	}
	if !strings.Contains(err.Error(), "script file not found") {
		t.Fatalf("unexpected error message: %v", err)
	}
}
