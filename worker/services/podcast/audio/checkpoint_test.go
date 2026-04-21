package podcast_audio_service

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	services "worker/services"
	dto "worker/services/podcast/model"
)

func TestValidateAlignedTimeline_NonMonotonicIsNonRetryable(t *testing.T) {
	script := dto.PodcastScript{
		Segments: []dto.PodcastSegment{
			{SegmentID: "seg_001", StartMS: 100, EndMS: 300},
			{SegmentID: "seg_002", StartMS: 250, EndMS: 400},
		},
	}

	err := validateAlignedTimeline(script, "")
	if err == nil {
		t.Fatalf("expected non-monotonic timeline error")
	}
	var nonRetryable services.NonRetryableError
	if !errors.As(err, &nonRetryable) {
		t.Fatalf("expected non-retryable error, got %T", err)
	}
}

func TestValidateAlignedTimeline_InvalidSegmentWindowIsNonRetryable(t *testing.T) {
	script := dto.PodcastScript{
		Segments: []dto.PodcastSegment{
			{SegmentID: "seg_001", StartMS: 100, EndMS: 100},
		},
	}

	err := validateAlignedTimeline(script, "")
	if err == nil {
		t.Fatalf("expected invalid timeline error")
	}
	var nonRetryable services.NonRetryableError
	if !errors.As(err, &nonRetryable) {
		t.Fatalf("expected non-retryable error, got %T", err)
	}
}

func TestBlockCheckpointCompleteRejectsMissingTokenTiming(t *testing.T) {
	dir := t.TempDir()
	audioPath := filepath.Join(dir, "001_block_001.wav")
	if err := os.WriteFile(audioPath, []byte("audio"), 0o644); err != nil {
		t.Fatalf("failed to write audio file: %v", err)
	}

	state := blockCheckpoint{
		Block: dto.PodcastBlock{
			Segments: []dto.PodcastSegment{
				{
					SegmentID: "seg_001",
					StartMS:   0,
					EndMS:     1000,
					Tokens: []dto.PodcastToken{
						{Char: "你"},
					},
				},
			},
		},
		DurationMS: 1000,
	}

	if blockCheckpointComplete("zh", state, audioPath) {
		t.Fatalf("expected checkpoint with missing token timing to be rejected")
	}
}

func TestBlockCheckpointCompleteAcceptsJapaneseHighlightTiming(t *testing.T) {
	dir := t.TempDir()
	audioPath := filepath.Join(dir, "001_block_001.wav")
	if err := os.WriteFile(audioPath, []byte("audio"), 0o644); err != nil {
		t.Fatalf("failed to write audio file: %v", err)
	}

	state := blockCheckpoint{
		Block: dto.PodcastBlock{
			Segments: []dto.PodcastSegment{
				{
					SegmentID: "seg_001",
					StartMS:   0,
					EndMS:     1000,
					HighlightSpans: []dto.PodcastHighlightSpan{
						{StartIndex: 0, EndIndex: 1, StartMS: 10, EndMS: 100},
					},
				},
			},
		},
		DurationMS: 1000,
	}

	if !blockCheckpointComplete("ja", state, audioPath) {
		t.Fatalf("expected checkpoint with highlight timings to be accepted")
	}
}
