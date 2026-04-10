package podcast

import (
	"strings"
	"testing"

	dto "worker/services/podcast/model"
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

func TestJapaneseSubtitleLayout_ShrinksTopSectionWithoutMovingEnglishArea(t *testing.T) {
	layout := japaneseSubtitleLayout(1920, 1080, 2)
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

func TestJapaneseSubtitleLayout_Style1UsesConfiguredTopBandsAndColors(t *testing.T) {
	layout := japaneseSubtitleLayout(1920, 1080, 1)
	if got, want := layout.TopSectionTop, int(float64(layout.PlayH)*designType1TopBandTopRatio); got != want {
		t.Fatalf("unexpected style1 top band start: got %d want %d", got, want)
	}
	expectedEnglishTop := layout.BoxTop + int(float64(layout.BoxHeight)*designType1TopSectionRatio)
	if got := layout.BottomSectionTop; got != expectedEnglishTop {
		t.Fatalf("unexpected style1 english band start: got %d want %d", got, expectedEnglishTop)
	}
	if got, want := layout.HanziColor, assColorRGB(255, 255, 255); got != want {
		t.Fatalf("unexpected style1 base color: got %q want %q", got, want)
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
	writeJapaneseASSHeader(&b, layout)
	if !strings.Contains(b.String(), layout.HighlightColor) {
		t.Fatalf("expected style1 ass header to use configured highlight color")
	}
}

func TestJapaneseSubtitleLayout_Style2MatchesType1Typography(t *testing.T) {
	style1 := japaneseSubtitleLayout(1920, 1080, 1)
	style2 := japaneseSubtitleLayout(1920, 1080, 2)
	if style2.RubySize != style1.RubySize || style2.HanziSize != style1.HanziSize || style2.EnglishSize != style1.EnglishSize {
		t.Fatalf("expected style2 font sizes to match style1 typography")
	}
	if style2.RubyBold != style1.RubyBold || style2.HanziBold != style1.HanziBold || style2.EnglishBold != style1.EnglishBold {
		t.Fatalf("expected style2 font weights to match style1 typography")
	}
}

func TestBuildJapaneseLayoutCells_PreservesSpaceBetweenEnglishWords(t *testing.T) {
	layout := subtitleLayout{
		HanziSize:    40,
		HanziSpacing: 8,
	}
	tokens := []dto.PodcastToken{
		{Char: "I", StartMS: 100, EndMS: 200},
		{Char: " ", StartMS: 200, EndMS: 200},
		{Char: "will", StartMS: 200, EndMS: 350},
	}

	cells := buildJapaneseLayoutCells(tokens, layout)
	if len(cells) != 3 {
		t.Fatalf("unexpected cell count: got %d want %d", len(cells), 3)
	}
	if cells[0].Char != "I" {
		t.Fatalf("unexpected first cell: got %q want %q", cells[0].Char, "I")
	}
	if cells[0].Gap != 0 {
		t.Fatalf("expected english word before space to have zero gap, got %v", cells[0].Gap)
	}
	if cells[1].Char != " " {
		t.Fatalf("unexpected space cell text: got %q want single space", cells[1].Char)
	}
	if cells[1].Width <= 0 {
		t.Fatalf("expected space cell to reserve width, got %v", cells[1].Width)
	}
	if cells[2].Char != "will" {
		t.Fatalf("unexpected third cell: got %q want %q", cells[2].Char, "will")
	}
}

func TestBuildJapaneseLayoutCells_InsertsVisualSpaceBetweenAdjacentEnglishWordTokens(t *testing.T) {
	layout := subtitleLayout{
		HanziSize:    40,
		HanziSpacing: 8,
	}
	tokens := []dto.PodcastToken{
		{Char: "I", StartMS: 100, EndMS: 200},
		{Char: "will", StartMS: 200, EndMS: 350},
		{Char: "go", StartMS: 350, EndMS: 450},
	}

	cells := buildJapaneseLayoutCells(tokens, layout)
	if len(cells) != 3 {
		t.Fatalf("unexpected cell count: got %d want %d", len(cells), 3)
	}
	if cells[0].Char != "I" || cells[1].Char != "will" || cells[2].Char != "go" {
		t.Fatalf("unexpected english cells: %#v", cells)
	}
	if cells[0].Gap <= 0 || cells[1].Gap <= 0 {
		t.Fatalf("expected positive inter-word visual space, got %v and %v", cells[0].Gap, cells[1].Gap)
	}
	if cells[0].Gap >= 8 || cells[1].Gap >= 8 {
		t.Fatalf("expected compact inter-word visual space, got %v and %v", cells[0].Gap, cells[1].Gap)
	}
}

func TestBuildJapaneseLayoutCells_AsciiQuotesStickToInlineEnglish(t *testing.T) {
	layout := subtitleLayout{
		HanziSize:    40,
		HanziSpacing: 8,
	}
	tokens := []dto.PodcastToken{
		{Char: "'"},
		{Char: "will"},
		{Char: "'"},
	}

	cells := buildJapaneseLayoutCells(tokens, layout)
	if len(cells) != 3 {
		t.Fatalf("unexpected cell count: got %d want %d", len(cells), 3)
	}
	if cells[0].Char != "'" || cells[1].Char != "will" || cells[2].Char != "'" {
		t.Fatalf("unexpected cells: %#v", cells)
	}
	if cells[0].Gap != 0 {
		t.Fatalf("expected opening quote to stick to word, got gap=%v", cells[0].Gap)
	}
	if cells[1].Gap != 0 {
		t.Fatalf("expected word to stick to closing quote, got gap=%v", cells[1].Gap)
	}
}

func TestChooseJapanesePageBreak_BreaksAfterBoundaryBeforeLongWrappedSpan(t *testing.T) {
	layout := subtitleLayout{
		MaxTextWidth: 9999,
		MaxLineChars: 8,
		HanziSize:    40,
		HanziSpacing: 8,
	}
	groups := []japaneseTokenGroup{
		{Cells: []japaneseCharCell{{Char: "前", Width: 20}}},
		{Cells: []japaneseCharCell{{Char: "置", Width: 20}}},
		{Cells: []japaneseCharCell{{Char: "。", Width: 20}}},
		{Cells: []japaneseCharCell{{Char: "（", Width: 20}}},
		{Cells: []japaneseCharCell{{Char: "長", Width: 20}}},
		{Cells: []japaneseCharCell{{Char: "い", Width: 20}}},
		{Cells: []japaneseCharCell{{Char: "、", Width: 20}}},
		{Cells: []japaneseCharCell{{Char: "文", Width: 20}}},
		{Cells: []japaneseCharCell{{Char: "、", Width: 20}}},
		{Cells: []japaneseCharCell{{Char: "です", Width: 20}}},
		{Cells: []japaneseCharCell{{Char: "）", Width: 20}}},
	}
	if got, want := chooseJapanesePageBreak(groups, 0, layout), 3; got != want {
		t.Fatalf("unexpected break around boundary+long wrapped span: got %d want %d", got, want)
	}
}

func TestChooseJapanesePageBreak_DoesNotForceBoundaryBreakForShortWrappedSpan(t *testing.T) {
	layout := subtitleLayout{
		MaxTextWidth: 9999,
		MaxLineChars: 12,
		HanziSize:    40,
		HanziSpacing: 8,
	}
	groups := []japaneseTokenGroup{
		{Cells: []japaneseCharCell{{Char: "前", Width: 20}}},
		{Cells: []japaneseCharCell{{Char: "置", Width: 20}}},
		{Cells: []japaneseCharCell{{Char: "。", Width: 20}}},
		{Cells: []japaneseCharCell{{Char: "（", Width: 20}}},
		{Cells: []japaneseCharCell{{Char: "短", Width: 20}}},
		{Cells: []japaneseCharCell{{Char: "、", Width: 20}}},
		{Cells: []japaneseCharCell{{Char: "句", Width: 20}}},
		{Cells: []japaneseCharCell{{Char: "）", Width: 20}}},
		{Cells: []japaneseCharCell{{Char: "後", Width: 20}}},
	}
	if got, want := chooseJapanesePageBreak(groups, 0, layout), 8; got != want {
		t.Fatalf("unexpected forced break for short wrapped span: got %d want %d", got, want)
	}
}
