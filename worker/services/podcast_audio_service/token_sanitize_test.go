package podcast_audio_service

import (
	"errors"
	"testing"

	"worker/internal/dto"
	services "worker/services"
)

func TestSanitizeSegmentTokensDropsWhitespaceAndEmptyTokens(t *testing.T) {
	seg := dto.PodcastSegment{
		Text: "英 文 'I'",
		Tokens: []dto.PodcastToken{
			{Char: "英", Reading: "yīng"},
			{Char: " ", Reading: ""},
			{Char: "文", Reading: "wén"},
			{Char: "", Reading: ""},
			{Char: "'", Reading: ""},
			{Char: "I", Reading: ""},
			{Char: "'", Reading: ""},
		},
	}

	seg = sanitizeSegmentTokens(seg)
	if len(seg.Tokens) != 6 {
		t.Fatalf("expected whitespace token kept and empty token removed, got %#v", seg.Tokens)
	}
	if seg.Tokens[1].Char != " " {
		t.Fatalf("expected whitespace token to be normalized to single space, got %#v", seg.Tokens[1])
	}
}

func TestValidateChineseScriptInputRejectsCoverageMismatch(t *testing.T) {
	script := dto.PodcastScript{
		Blocks: []dto.PodcastBlock{
			{
				BlockID: "block_001",
				Segments: []dto.PodcastSegment{
					{
						SegmentID: "seg_001",
						Text:      "我去。",
						Tokens: []dto.PodcastToken{
							{Char: "我", Reading: "wǒ"},
							{Char: "去", Reading: "qù"},
						},
					},
				},
			},
		},
	}
	err := validateChineseScriptInput(script)
	if err == nil {
		t.Fatalf("expected chinese token coverage mismatch error")
	}
}

func TestMarkScriptInputNonRetryableWrapsValidationErrors(t *testing.T) {
	err := markScriptInputNonRetryable(errors.New("bad script"))
	var nonRetryable services.NonRetryableError
	if !errors.As(err, &nonRetryable) {
		t.Fatalf("expected non-retryable error, got %T", err)
	}
}

func TestValidateChineseScriptInputAllowsEnglishWordTokensWithSpaces(t *testing.T) {
	script := dto.PodcastScript{
		Blocks: []dto.PodcastBlock{
			{
				BlockID: "block_001",
				Segments: []dto.PodcastSegment{
					{
						SegmentID: "seg_001",
						Text:      "我想说 I will。",
						Tokens: []dto.PodcastToken{
							{Char: "我", Reading: "wǒ"},
							{Char: "想", Reading: "xiǎng"},
							{Char: "说", Reading: "shuō"},
							{Char: " ", Reading: ""},
							{Char: "I", Reading: ""},
							{Char: " ", Reading: ""},
							{Char: "will", Reading: ""},
							{Char: "。", Reading: ""},
						},
					},
				},
			},
		},
	}
	if err := validateChineseScriptInput(script); err != nil {
		t.Fatalf("expected word-level english tokens with spaces to validate, got %v", err)
	}
}
