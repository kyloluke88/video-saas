package podcast_audio_service

import (
	"testing"

	"worker/internal/dto"
)

func TestApplyJapaneseRuneTimings_ClearsRuneHighlightSpansWhenTokensPresent(t *testing.T) {
	seg := dto.PodcastSegment{
		Text: "今日はこのテーマについて、少し話し合ってみましょう。",
		Tokens: []dto.PodcastToken{
			{Char: "今日", Reading: "きょう"},
			{Char: "少", Reading: "すこ"},
			{Char: "話", Reading: "はな"},
			{Char: "合", Reading: "あ"},
		},
	}

	textRunes := []rune(seg.Text)
	runes := make([]timedRune, len(textRunes))
	for i := range textRunes {
		runes[i] = timedRune{
			StartMS: 1000 + i*100,
			EndMS:   1090 + i*100,
			Matched: true,
		}
	}

	got := applyJapaneseRuneTimings(seg, runes)

	if len(got.HighlightSpans) != 0 {
		t.Fatalf("expected highlight_spans to be cleared when tokens exist, got %d", len(got.HighlightSpans))
	}
	for i, token := range got.Tokens {
		if token.EndMS <= token.StartMS {
			t.Fatalf("expected token %d (%q) to receive timing, got start=%d end=%d", i, token.Char, token.StartMS, token.EndMS)
		}
	}
}

func TestApplyJapaneseRuneTimings_KeepsRuneHighlightSpansWithoutTokens(t *testing.T) {
	seg := dto.PodcastSegment{
		Text: "これはかなだけです。",
	}

	textRunes := []rune(seg.Text)
	runes := make([]timedRune, len(textRunes))
	for i := range textRunes {
		runes[i] = timedRune{
			StartMS: 2000 + i*80,
			EndMS:   2070 + i*80,
			Matched: true,
		}
	}

	got := applyJapaneseRuneTimings(seg, runes)

	if len(got.HighlightSpans) == 0 {
		t.Fatalf("expected rune-based highlight_spans for tokenless Japanese text")
	}
}
