package podcast

import (
	"strings"
	"testing"

	dto "worker/services/podcast/model"
)

func TestBuildTokenCells_PreservesSpaceBetweenEnglishWords(t *testing.T) {
	layout := subtitleLayout{
		HanziSize:    40,
		HanziSpacing: 8,
	}
	tokens := []dto.PodcastToken{
		{Char: "I", StartMS: 100, EndMS: 200},
		{Char: " ", StartMS: 200, EndMS: 200},
		{Char: "will", StartMS: 200, EndMS: 350},
	}

	cells := buildTokenCells(tokens, layout)
	if len(cells) != 3 {
		t.Fatalf("unexpected cell count: got %d want %d", len(cells), 3)
	}
	if cells[0].Hanzi != "I" {
		t.Fatalf("unexpected first cell: got %q want %q", cells[0].Hanzi, "I")
	}
	if cells[0].Gap != 0 {
		t.Fatalf("expected english word before space to have zero gap, got %v", cells[0].Gap)
	}
	if cells[1].Hanzi != " " {
		t.Fatalf("unexpected space cell text: got %q want single space", cells[1].Hanzi)
	}
	if cells[1].Width <= 0 {
		t.Fatalf("expected space cell to reserve width, got %v", cells[1].Width)
	}
	if cells[2].Hanzi != "will" {
		t.Fatalf("unexpected third cell: got %q want %q", cells[2].Hanzi, "will")
	}
}

func TestBuildTokenCells_InsertsVisualSpaceBetweenAdjacentEnglishWordTokens(t *testing.T) {
	layout := subtitleLayout{
		HanziSize:    40,
		HanziSpacing: 8,
	}
	tokens := []dto.PodcastToken{
		{Char: "I", StartMS: 100, EndMS: 200},
		{Char: "will", StartMS: 200, EndMS: 350},
		{Char: "go", StartMS: 350, EndMS: 450},
	}

	cells := buildTokenCells(tokens, layout)
	if len(cells) != 3 {
		t.Fatalf("unexpected cell count: got %d want %d", len(cells), 3)
	}
	if cells[0].Hanzi != "I" || cells[1].Hanzi != "will" || cells[2].Hanzi != "go" {
		t.Fatalf("unexpected english cells: %#v", cells)
	}
	if cells[0].Gap <= 0 || cells[1].Gap <= 0 {
		t.Fatalf("expected positive inter-word visual space, got %v and %v", cells[0].Gap, cells[1].Gap)
	}
	if cells[0].Gap >= 8 || cells[1].Gap >= 8 {
		t.Fatalf("expected compact inter-word visual space, got %v and %v", cells[0].Gap, cells[1].Gap)
	}
}

func TestBuildTokenCells_AsciiQuotesStickToInlineEnglish(t *testing.T) {
	layout := subtitleLayout{
		HanziSize:    40,
		HanziSpacing: 8,
	}
	tokens := []dto.PodcastToken{
		{Char: "'"},
		{Char: "will"},
		{Char: "'"},
	}

	cells := buildTokenCells(tokens, layout)
	if len(cells) != 3 {
		t.Fatalf("unexpected cell count: got %d want %d", len(cells), 3)
	}
	if cells[0].Hanzi != "'" || cells[1].Hanzi != "will" || cells[2].Hanzi != "'" {
		t.Fatalf("unexpected cells: %#v", cells)
	}
	if cells[0].Gap != 0 {
		t.Fatalf("expected opening quote to stick to word, got gap=%v", cells[0].Gap)
	}
	if cells[1].Gap != 0 {
		t.Fatalf("expected word to stick to closing quote, got gap=%v", cells[1].Gap)
	}
}

func TestChooseChinesePageBreak_EnglishWordCountsAsSingleUnit(t *testing.T) {
	layout := subtitleLayout{
		MaxTextWidth: 9999,
		MaxLineChars: 2,
		HanziSize:    40,
		HanziSpacing: 8,
	}
	cells := []tokenCell{
		{Hanzi: "I", Width: 20},
		{Hanzi: " ", Width: 10},
		{Hanzi: "will", Width: 60},
		{Hanzi: "中", Width: 40},
	}

	if got, want := chooseChinesePageBreak(cells, 0, layout), 4; got != want {
		t.Fatalf("unexpected page break: got %d want %d", got, want)
	}
}

func TestChooseChinesePageBreak_AsciiQuotedBlockCanBreakWhenOverLimit(t *testing.T) {
	layout := subtitleLayout{
		MaxTextWidth: 9999,
		MaxLineChars: 4,
		HanziSize:    40,
		HanziSpacing: 8,
	}
	cells := []tokenCell{
		{Hanzi: "'"},
		{Hanzi: "I"},
		{Hanzi: "will"},
		{Hanzi: "go"},
		{Hanzi: "to"},
		{Hanzi: "the"},
		{Hanzi: "supermarket"},
		{Hanzi: "tomorrow"},
		{Hanzi: "'"},
	}
	if got, want := chooseChinesePageBreak(cells, 0, layout), 9; got != want {
		t.Fatalf("unexpected page break for over-limit quoted block: got %d want %d", got, want)
	}
}

func TestChooseChinesePageBreak_AsciiQuotedBlockStaysTogetherWhenWithinLimit(t *testing.T) {
	layout := subtitleLayout{
		MaxTextWidth: 9999,
		MaxLineChars: 10,
		HanziSize:    40,
		HanziSpacing: 8,
	}
	cells := []tokenCell{
		{Hanzi: "'"},
		{Hanzi: "I"},
		{Hanzi: "will"},
		{Hanzi: "'"},
		{Hanzi: "。"},
	}
	if got, want := chooseChinesePageBreak(cells, 0, layout), 5; got != want {
		t.Fatalf("unexpected page break for short quoted block: got %d want %d", got, want)
	}
}

func TestChooseChinesePageBreak_BreaksAfterCommaBeforeLongQuotedSpan(t *testing.T) {
	layout := subtitleLayout{
		MaxTextWidth: 9999,
		MaxLineChars: 6,
		HanziSize:    40,
		HanziSpacing: 8,
	}
	cells := []tokenCell{
		{Hanzi: "比"},
		{Hanzi: "如"},
		{Hanzi: "，"},
		{Hanzi: "'"},
		{Hanzi: "I"},
		{Hanzi: "will"},
		{Hanzi: "go"},
		{Hanzi: "to"},
		{Hanzi: "the"},
		{Hanzi: "supermarket"},
		{Hanzi: "tomorrow"},
		{Hanzi: "'"},
	}
	if got, want := chooseChinesePageBreak(cells, 0, layout), 3; got != want {
		t.Fatalf("unexpected page break around comma+long quote: got %d want %d", got, want)
	}
}

func TestChooseChinesePageBreak_DoesNotForceCommaBreakForShortQuotedSpan(t *testing.T) {
	layout := subtitleLayout{
		MaxTextWidth: 9999,
		MaxLineChars: 10,
		HanziSize:    40,
		HanziSpacing: 8,
	}
	cells := []tokenCell{
		{Hanzi: "比"},
		{Hanzi: "如"},
		{Hanzi: "，"},
		{Hanzi: "'"},
		{Hanzi: "I"},
		{Hanzi: "'"},
		{Hanzi: "。"},
	}
	if got, want := chooseChinesePageBreak(cells, 0, layout), 7; got != want {
		t.Fatalf("unexpected forced break for short quote: got %d want %d", got, want)
	}
}

func TestNormalizeEnglishSubtitleSpacing_RemovesQuotePadding(t *testing.T) {
	if got, want := normalizeEnglishSubtitleSpacing("supermarket   tomorrow   '  ."), "supermarket tomorrow'."; got != want {
		t.Fatalf("unexpected normalized english subtitle spacing: got %q want %q", got, want)
	}
}

func TestChineseSubtitleLayout_UsesTwentyCharLimit(t *testing.T) {
	layout := chineseSubtitleLayout(1920, 1080, 2)
	if got, want := layout.MaxLineChars, 20; got != want {
		t.Fatalf("unexpected char limit: got %d want %d", got, want)
	}
}

func TestChineseSubtitleLayout_ShrinksTopSectionWithoutMovingEnglishArea(t *testing.T) {
	layout := chineseSubtitleLayout(1920, 1080, 2)
	boxHeight := int(float64(layout.PlayH) * 0.4029)
	boxTop := int(float64(layout.PlayH) * 0.5561)
	oldBottomTop := boxTop + int(float64(boxHeight)*0.7301)
	if got := layout.BottomSectionTop; got != oldBottomTop {
		t.Fatalf("english area moved: got bottom top %d want %d", got, oldBottomTop)
	}
	expectedTop := boxTop + int(float64(boxHeight)*0.02) + int(float64(layout.PlayH)*0.03)
	if got := layout.TopSectionTop; got != expectedTop {
		t.Fatalf("top section did not shift down: got %d want %d", got, expectedTop)
	}
}

func TestChineseSubtitleLayout_Style1UsesConfiguredTopBandsAndColors(t *testing.T) {
	layout := chineseSubtitleLayout(1920, 1080, 1)
	if got, want := layout.TopSectionTop, int(float64(layout.PlayH)*designType1TopBandTopRatio); got != want {
		t.Fatalf("unexpected style1 top band start: got %d want %d", got, want)
	}
	expectedEnglishTop := layout.BoxTop + int(float64(layout.BoxHeight)*designType1TopSectionRatio)
	if got := layout.BottomSectionTop; got != expectedEnglishTop {
		t.Fatalf("unexpected style1 english band start: got %d want %d", got, expectedEnglishTop)
	}
	if got, want := layout.HanziColor, assColorRGB(255, 255, 255); got != want {
		t.Fatalf("unexpected style1 hanzi color: got %q want %q", got, want)
	}
	if got, want := layout.HighlightColor, assColorRGB(196, 236, 121); got != want {
		t.Fatalf("unexpected style1 highlight color: got %q want %q", got, want)
	}
	if got, want := layout.EnglishColor, assColorRGB(183, 236, 70); got != want {
		t.Fatalf("unexpected style1 english color: got %q want %q", got, want)
	}
	if got, want := layout.RubyBold, 0; got != want {
		t.Fatalf("unexpected style1 ruby bold: got %d want %d", got, want)
	}

	var b strings.Builder
	writeASSHeader(&b, layout)
	if !strings.Contains(b.String(), layout.HighlightColor) {
		t.Fatalf("expected style1 ass header to use configured highlight color")
	}
}

func TestChineseSubtitleLayout_Style2MatchesType1Typography(t *testing.T) {
	style1 := chineseSubtitleLayout(1920, 1080, 1)
	style2 := chineseSubtitleLayout(1920, 1080, 2)
	if style2.RubySize != style1.RubySize || style2.HanziSize != style1.HanziSize || style2.EnglishSize != style1.EnglishSize {
		t.Fatalf("expected style2 font sizes to match style1 typography")
	}
	if style2.RubyBold != style1.RubyBold || style2.HanziBold != style1.HanziBold || style2.EnglishBold != style1.EnglishBold {
		t.Fatalf("expected style2 font weights to match style1 typography")
	}
}
