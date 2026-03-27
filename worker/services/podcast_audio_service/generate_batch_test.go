package podcast_audio_service

import (
	"errors"
	"os"
	"strings"
	"testing"

	"worker/internal/dto"
	services "worker/services"
)

func TestBuildBalancedBlockBatches_EvenlySplitsIntoThree(t *testing.T) {
	batches := buildBalancedBlockBatches(11, 3)
	if len(batches) != 3 {
		t.Fatalf("expected 3 batches, got %d", len(batches))
	}
	want := []blockBatch{
		{BatchIndex: 0, Start: 0, End: 4},
		{BatchIndex: 1, Start: 4, End: 8},
		{BatchIndex: 2, Start: 8, End: 11},
	}
	for i := range want {
		if batches[i] != want[i] {
			t.Fatalf("batch[%d] mismatch: want=%+v got=%+v", i, want[i], batches[i])
		}
	}
}

func TestBuildBalancedBlockBatches_HandlesSmallBlockCount(t *testing.T) {
	batches := buildBalancedBlockBatches(2, 3)
	if len(batches) != 2 {
		t.Fatalf("expected 2 batches, got %d", len(batches))
	}
	if batches[0].Start != 0 || batches[0].End != 1 {
		t.Fatalf("unexpected first batch: %+v", batches[0])
	}
	if batches[1].Start != 1 || batches[1].End != 2 {
		t.Fatalf("unexpected second batch: %+v", batches[1])
	}
}

func TestBuildBatchTimingWindows_ContiguousCoverage(t *testing.T) {
	bounds := []blockTimingWindow{
		{StartMS: 100, EndMS: 900},
		{StartMS: 1200, EndMS: 1800},
		{StartMS: 2100, EndMS: 2800},
	}
	windows := buildBatchTimingWindows(bounds, 3000)
	if len(windows) != 3 {
		t.Fatalf("expected 3 windows, got %d", len(windows))
	}
	if windows[0].StartMS != 0 || windows[0].EndMS != 1050 {
		t.Fatalf("unexpected first window: %+v", windows[0])
	}
	if windows[1].StartMS != 1050 || windows[1].EndMS != 1950 {
		t.Fatalf("unexpected second window: %+v", windows[1])
	}
	if windows[2].StartMS != 1950 || windows[2].EndMS != 3000 {
		t.Fatalf("unexpected third window: %+v", windows[2])
	}
	for i := 1; i < len(windows); i++ {
		if windows[i-1].EndMS != windows[i].StartMS {
			t.Fatalf("windows are not contiguous at %d: prev=%+v curr=%+v", i, windows[i-1], windows[i])
		}
	}
}

func TestEnforceBatchingHardLimit_ReturnsNonRetryableWhenActualExceedsMax(t *testing.T) {
	blocks := []dto.PodcastBlock{
		{BlockID: "b1", Segments: []dto.PodcastSegment{{Speaker: "male", Text: "a"}}},
		{BlockID: "b2", Segments: []dto.PodcastSegment{{Speaker: "female", Text: "b"}}},
		{BlockID: "b3", Segments: []dto.PodcastSegment{{Speaker: "male", Text: "c"}}},
		{BlockID: "b4", Segments: []dto.PodcastSegment{{Speaker: "female", Text: "d"}}},
		{BlockID: "b5", Segments: []dto.PodcastSegment{{Speaker: "male", Text: "e"}}},
	}
	actual := buildBalancedBlockBatches(len(blocks), 5)
	err := enforceBatchingHardLimit("zh", blocks, actual, 4)
	if err == nil {
		t.Fatalf("expected error when actual batches exceed max")
	}
	var nonRetryable services.NonRetryableError
	if !errors.As(err, &nonRetryable) {
		t.Fatalf("expected non-retryable error, got %T", err)
	}
	if !strings.Contains(err.Error(), "exceeded max batches") {
		t.Fatalf("unexpected error message: %v", err)
	}
}

func TestEnforceBatchingHardLimit_ReturnsNonRetryableWhenSegmentTextExceedsGoogleFieldLimit(t *testing.T) {
	blocks := []dto.PodcastBlock{
		{BlockID: "b1", Segments: []dto.PodcastSegment{{Speaker: "male", SegmentID: "seg_001", Text: strings.Repeat("甲", 1400)}}},
	}
	batches := []blockBatch{{BatchIndex: 0, Start: 0, End: 1}}
	err := enforceBatchingHardLimit("zh", blocks, batches, 4)
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
