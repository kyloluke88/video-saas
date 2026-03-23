package podcast

import (
	"testing"

	"worker/internal/dto"
)

func TestBuildJapaneseAnnotationDetails_UsesOriginalTokenIndexTiming(t *testing.T) {
	seg := dto.PodcastSegment{
		Text: "内容です",
		Tokens: []dto.PodcastToken{
			{Char: "missing", Reading: "みっしんぐ", StartMS: 10, EndMS: 20},
			{Char: "内容", Reading: "ないよう", StartMS: 200, EndMS: 420},
		},
	}

	details := buildJapaneseAnnotationDetails(seg)
	if len(details) != 1 {
		t.Fatalf("expected 1 annotation detail, got %d", len(details))
	}
	if details[0].StartMS != 200 || details[0].EndMS != 420 {
		t.Fatalf("expected surviving token timing 200-420, got %d-%d", details[0].StartMS, details[0].EndMS)
	}
	if details[0].Span.StartIndex != 0 || details[0].Span.EndIndex != 1 {
		t.Fatalf("expected ruby span over 内容, got %d-%d", details[0].Span.StartIndex, details[0].Span.EndIndex)
	}
}

func TestJapaneseSegmentTokens_DistributesMFAWindowAcrossRubySpan(t *testing.T) {
	seg := dto.PodcastSegment{
		Text:    "内容です",
		StartMS: 100,
		EndMS:   500,
		Tokens: []dto.PodcastToken{
			{Char: "内容", Reading: "ないよう", StartMS: 120, EndMS: 320},
		},
	}

	tokens := japaneseSegmentTokens(seg)
	if len(tokens) != 4 {
		t.Fatalf("expected 4 display tokens, got %d", len(tokens))
	}
	if tokens[0].StartMS != 120 || tokens[0].EndMS != 220 {
		t.Fatalf("expected first kanji to use first half of MFA window 120-220, got %d-%d", tokens[0].StartMS, tokens[0].EndMS)
	}
	if tokens[1].StartMS != 220 || tokens[1].EndMS != 320 {
		t.Fatalf("expected second kanji to use second half of MFA window 220-320, got %d-%d", tokens[1].StartMS, tokens[1].EndMS)
	}
	if tokens[2].StartMS < 320 || tokens[3].EndMS > 500 {
		t.Fatalf("expected trailing kana to be filled after ruby span within segment window, got %d-%d and %d-%d", tokens[2].StartMS, tokens[2].EndMS, tokens[3].StartMS, tokens[3].EndMS)
	}
}

func TestJapaneseSegmentTokens_UsesMFAHighlightSpanForWordInternalProgression(t *testing.T) {
	seg := dto.PodcastSegment{
		Text:    "楽しみ方",
		StartMS: 100,
		EndMS:   700,
		Tokens: []dto.PodcastToken{
			{Char: "楽", Reading: "たの", StartMS: 200, EndMS: 500},
			{Char: "方", Reading: "かた", StartMS: 200, EndMS: 500},
		},
		HighlightSpans: []dto.PodcastHighlightSpan{
			{StartIndex: 0, EndIndex: 3, StartMS: 200, EndMS: 500},
		},
	}

	tokens := japaneseSegmentTokens(seg)
	if len(tokens) != 4 {
		t.Fatalf("expected 4 display tokens, got %d", len(tokens))
	}

	expected := [][2]int{
		{200, 275},
		{275, 350},
		{350, 425},
		{425, 500},
	}
	for i, want := range expected {
		if tokens[i].StartMS != want[0] || tokens[i].EndMS != want[1] {
			t.Fatalf("expected token %d to use MFA-derived window %d-%d, got %d-%d", i, want[0], want[1], tokens[i].StartMS, tokens[i].EndMS)
		}
	}
}
