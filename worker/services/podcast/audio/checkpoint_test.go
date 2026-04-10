package podcast_audio_service

import (
	"errors"
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
