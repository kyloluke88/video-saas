package podcast

import (
	"testing"

	"worker/internal/dto"
)

func TestAdjustSubtitlePageBreak_AttachesTrailingPunctuation(t *testing.T) {
	texts := []string{"现在", "年轻人", "，", "为什么"}
	if got, want := adjustSubtitlePageBreak(texts, 0, 2), 3; got != want {
		t.Fatalf("unexpected page break: got %d want %d", got, want)
	}
}

func TestAdjustSubtitlePageBreak_KeepsQuotedSpanTogether(t *testing.T) {
	texts := []string{"他说", "“", "没关系", "”", "。", "后来"}
	if got, want := adjustSubtitlePageBreak(texts, 0, 3), 5; got != want {
		t.Fatalf("unexpected quoted break: got %d want %d", got, want)
	}
}

func TestInlineLatinWordTokenRun_MergesWordAndConnector(t *testing.T) {
	tokens := []dto.PodcastToken{
		{Char: "H"},
		{Char: "S"},
		{Char: "K"},
		{Char: "-"},
		{Char: "3"},
		{Char: "中"},
	}
	end, ok := inlineLatinWordTokenRun(tokens, 0)
	if !ok {
		t.Fatal("expected latin word run")
	}
	if end != 4 {
		t.Fatalf("unexpected run end: got %d want %d", end, 4)
	}
}

func TestJapaneseBuildLayoutCells_MergesInlineEnglishWord(t *testing.T) {
	layout := subtitleLayout{
		HanziSize: 40,
		BaseGap:   8,
	}
	tokens := []dto.PodcastToken{
		{Char: "A", StartMS: 100, EndMS: 200},
		{Char: "I", StartMS: 200, EndMS: 300},
		{Char: "で", StartMS: 300, EndMS: 400},
	}
	cells := buildJapaneseLayoutCells(tokens, layout)
	if len(cells) != 2 {
		t.Fatalf("unexpected cell count: got %d want %d", len(cells), 2)
	}
	if cells[0].Char != "AI" {
		t.Fatalf("unexpected english cell text: got %q want %q", cells[0].Char, "AI")
	}
	if cells[0].StartIndex != 0 || cells[0].EndIndex != 1 {
		t.Fatalf("unexpected english cell range: got %d-%d want 0-1", cells[0].StartIndex, cells[0].EndIndex)
	}
}
