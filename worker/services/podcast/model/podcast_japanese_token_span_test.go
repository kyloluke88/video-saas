package model

import "testing"

func TestBuildJapaneseTokenSpanRefs_KeepsAdjacentKanjiTokensSeparate(t *testing.T) {
	text := "春の給与交渉で"
	tokens := []PodcastToken{
		{Char: "春", Reading: "はる"},
		{Char: "給与", Reading: "きゅうよ"},
		{Char: "交渉", Reading: "こうしょう"},
	}

	refs := BuildJapaneseTokenSpanRefs(text, tokens)
	if len(refs) != 3 {
		t.Fatalf("expected 3 token span refs, got %d", len(refs))
	}

	want := []PodcastTokenSpan{
		{StartIndex: 0, EndIndex: 0, Reading: "はる"},
		{StartIndex: 2, EndIndex: 3, Reading: "きゅうよ"},
		{StartIndex: 4, EndIndex: 5, Reading: "こうしょう"},
	}
	for i := range want {
		if refs[i].Span != want[i] {
			t.Fatalf("span[%d] mismatch: want=%+v got=%+v", i, want[i], refs[i].Span)
		}
	}
}

func TestBuildJapaneseTokenSpanRefs_KeepsSplitCompoundKanjiTokensSeparate(t *testing.T) {
	text := "大企業を中心に"
	tokens := []PodcastToken{
		{Char: "大", Reading: "だい"},
		{Char: "企業", Reading: "きぎょう"},
		{Char: "中心", Reading: "ちゅうしん"},
	}

	refs := BuildJapaneseTokenSpanRefs(text, tokens)
	if len(refs) != 3 {
		t.Fatalf("expected 3 token span refs, got %d", len(refs))
	}

	want := []PodcastTokenSpan{
		{StartIndex: 0, EndIndex: 0, Reading: "だい"},
		{StartIndex: 1, EndIndex: 2, Reading: "きぎょう"},
		{StartIndex: 4, EndIndex: 5, Reading: "ちゅうしん"},
	}
	for i := range want {
		if refs[i].Span != want[i] {
			t.Fatalf("span[%d] mismatch: want=%+v got=%+v", i, want[i], refs[i].Span)
		}
	}
}
