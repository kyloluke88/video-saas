package practical_compose_service

import (
	"math"
	"os"
	"path/filepath"
	"strings"
	"testing"

	dto "worker/services/practical/model"
)

func TestWritePracticalASSUsesPodcastType1TypographyAndBox(t *testing.T) {
	projectDir := t.TempDir()
	script := dto.PracticalScript{
		Language: "ja",
		Blocks: []dto.PracticalBlock{
			{
				BlockID:      "block_01",
				Topic:        "スーパーで買い物",
				TopicStartMS: 0,
				TopicEndMS:   1600,
				Chapters: []dto.PracticalChapter{
					{
						ChapterID: "ch_01",
						Scene:     "レジで支払う",
						StartMS:   0,
						EndMS:     1800,
						Turns: []dto.PracticalTurn{
							{
								TurnID:      "t_01",
								SpeakerRole: "customer",
								Text:        "すみません。牛乳はどこですか？",
								Tokens:      []byte(`[{"char":"牛乳","reading":"ぎゅうにゅう"}]`),
								StartMS:     0,
								EndMS:       1800,
							},
						},
					},
				},
			},
		},
	}

	assPath, err := writePracticalASS(script, projectDir, "1080p", 1)
	if err != nil {
		t.Fatalf("writePracticalASS returned error: %v", err)
	}
	if filepath.Base(assPath) != "practical_subtitles.ass" {
		t.Fatalf("unexpected ass path: %s", assPath)
	}

	raw, err := os.ReadFile(assPath)
	if err != nil {
		t.Fatalf("read ass failed: %v", err)
	}
	text := string(raw)
	if !strings.Contains(text, "Style: TurnBox,Maruko Gothic CJKjp Medium,20,") {
		t.Fatalf("expected podcast-style box typography, got: %s", text)
	}
	if !strings.Contains(text, "Style: TurnBoxFemale,Maruko Gothic CJKjp Medium,20,") {
		t.Fatalf("expected female turn box style, got: %s", text)
	}
	if !strings.Contains(text, "Style: BlockSub,Maruko Gothic CJKjp Medium,90,") {
		t.Fatalf("expected block title font size, got: %s", text)
	}
	if !strings.Contains(text, "Style: SceneSub,Maruko Gothic CJKjp Medium,63,") {
		t.Fatalf("expected scene title font size to be turn font size + 5, got: %s", text)
	}
	if !strings.Contains(text, "Style: TurnSub,Maruko Gothic CJKjp Medium,58,") {
		t.Fatalf("expected practical turn font size, got: %s", text)
	}
	if !strings.Contains(text, "Style: TurnRuby,Maruko Gothic CJKjp Medium,30,") {
		t.Fatalf("expected ruby style, got: %s", text)
	}
	if !strings.Contains(text, "Style: TurnRuby,Maruko Gothic CJKjp Medium,30,&H00000000,&H00000000,&H00000000,&H00000000,0,0,0,0,100,100,-1.33,0,1,0,0,8,0,0,0,1") {
		t.Fatalf("expected podcast-style ruby spacing, got: %s", text)
	}
	if !strings.Contains(text, "ぎゅうにゅう") {
		t.Fatalf("expected ruby reading to be rendered, got: %s", text)
	}
	if !strings.Contains(text, ",TurnBox,,0,0,0,,{\\an7\\pos(0,0)\\p1}") {
		t.Fatalf("expected vector box dialogue line, got: %s", text)
	}
	if !strings.Contains(text, ",SceneBox,,0,0,0,,{\\an7\\pos(0,0)\\p1}") {
		t.Fatalf("expected scene box to use absolute positioning, got: %s", text)
	}
	if !strings.Contains(text, ",BlockSub,,0,0,0,,{\\an5\\pos(") {
		t.Fatalf("expected centered block subtitle line, got: %s", text)
	}
	if !strings.Contains(text, ",SceneSub,,0,0,0,,{\\an7\\pos(") {
		t.Fatalf("expected top-left scene subtitle line, got: %s", text)
	}
	if !strings.Contains(text, "レジで支払う") {
		t.Fatalf("expected scene title to be rendered, got: %s", text)
	}
}

func TestBuildPracticalJapaneseLineLayoutsKeepsRubyTokenOnSingleLine(t *testing.T) {
	layouts := buildPracticalJapaneseLineLayouts(
		"ja",
		"あああああ大丈夫です",
		[]byte(`[{"char":"大丈夫","reading":"だいじょうぶ"}]`),
		practicalSubtitleStyleFor("ja", 1),
		960,
		6,
		2,
	)

	if len(layouts) != 2 {
		t.Fatalf("unexpected layout count: %d", len(layouts))
	}
	if layouts[0].Line.Text != "あああああ" {
		t.Fatalf("unexpected first line: %#v", layouts[0].Line)
	}
	if layouts[1].Line.Text != "大丈夫です" {
		t.Fatalf("unexpected second line: %#v", layouts[1].Line)
	}
	if len(layouts[0].Spans) != 0 {
		t.Fatalf("expected first line to have no ruby spans, got %#v", layouts[0].Spans)
	}
	if len(layouts[1].Spans) != 1 {
		t.Fatalf("expected second line to keep the ruby token intact, got %#v", layouts[1].Spans)
	}
	if layouts[1].Spans[0].StartRune != 5 || layouts[1].Spans[0].EndRune != 8 {
		t.Fatalf("unexpected ruby span range: %#v", layouts[1].Spans[0])
	}

	cells := layoutPracticalLineCells(layouts[1].Cells, 960, 960)
	centerX, ok := practicalRubyCenter(layouts[1].Spans[0], cells)
	if !ok || centerX <= 0 {
		t.Fatalf("expected ruby center for grouped token, got center=%d ok=%v", centerX, ok)
	}
}

func TestBuildPracticalJapaneseCellsUsePodcastWidthModel(t *testing.T) {
	style := practicalSubtitleStyleFor("ja", 1)
	cells := buildPracticalJapaneseCells([]rune("日。"), style)
	if len(cells) != 2 {
		t.Fatalf("unexpected cell count: %d", len(cells))
	}
	if cells[0].Gap >= 1 {
		t.Fatalf("expected podcast-style near-zero normal char gap, got %v", cells[0].Gap)
	}
	if cells[1].Width > float64(style.TurnFontSize)*0.49 {
		t.Fatalf("expected punctuation width to be compressed like podcast layout, got %v", cells[1].Width)
	}
}

func TestBuildPracticalTurnSubtitleWindowsAvoidsOverlap(t *testing.T) {
	windows := buildPracticalTurnSubtitleWindows([]practicalTurnEntry{
		{Turn: dto.PracticalTurn{TurnID: "t_01", StartMS: 1000, EndMS: 2000}},
		{Turn: dto.PracticalTurn{TurnID: "t_02", StartMS: 2000, EndMS: 3000}},
	}, 200)

	if len(windows) != 2 {
		t.Fatalf("unexpected window count: %d", len(windows))
	}
	if windows[0].StartMS != 800 || windows[0].EndMS != 2000 {
		t.Fatalf("unexpected first window: %#v", windows[0])
	}
	if windows[1].StartMS != 2000 || windows[1].EndMS != 3000 {
		t.Fatalf("unexpected second window: %#v", windows[1])
	}
}

func TestPracticalSceneTitleWindowPersistsForEntireChapter(t *testing.T) {
	chapter := dto.PracticalChapter{
		ChapterID: "ch_01",
		StartMS:   5000,
		EndMS:     12000,
		Turns: []dto.PracticalTurn{
			{TurnID: "t_01", StartMS: 6200, EndMS: 7600},
		},
	}

	startMS, endMS := practicalSceneTitleWindow(chapter)
	if startMS != 5000 {
		t.Fatalf("unexpected scene start: %d", startMS)
	}
	if endMS != 12000 {
		t.Fatalf("expected scene title to persist for the whole chapter, got %d", endMS)
	}
}

func TestPracticalTurnPanelGrowsWithLargerTypography(t *testing.T) {
	style := practicalSubtitleStyleFor("ja", 1)
	panel := buildPracticalTurnPanelWithLineWidths(1920, 1080, style, []float64{estimatePracticalTextWidth("すみません。牛乳はどこですか？", style.TurnFontSize)})

	if style.TurnFontSize != 58 {
		t.Fatalf("unexpected turn font size: %d", style.TurnFontSize)
	}
	if style.RubyFontSize != 30 {
		t.Fatalf("unexpected ruby font size: %d", style.RubyFontSize)
	}
	if panel.Height < 100 {
		t.Fatalf("expected larger subtitle panel height, got %d", panel.Height)
	}
	if panel.Width < 560 {
		t.Fatalf("expected larger subtitle panel width, got %d", panel.Width)
	}
}

func TestBuildPracticalTopLeftPanelUsesTighterSceneWidth(t *testing.T) {
	style := practicalSubtitleStyleFor("ja", 1)
	lines := buildPracticalSubtitleLines("公園で友達にばったり会う", style.SceneMaxLineChars, style.SceneMaxLines)
	panel := buildPracticalTopLeftPanel(1920, 1080, style, lines)

	looseWidth := int(math.Ceil(estimatePracticalTextWidth(lines[0].Text, style.SceneFontSize))) + style.SceneBoxPaddingX*2
	tightWidth := int(math.Ceil(estimatePracticalSceneTextWidth(lines[0].Text, style.SceneFontSize))) + style.SceneBoxPaddingX*2

	if panel.Width != tightWidth {
		t.Fatalf("expected scene panel width %d, got %d", tightWidth, panel.Width)
	}
	if panel.Width >= looseWidth {
		t.Fatalf("expected tighter scene width than loose subtitle estimate, got panel=%d loose=%d", panel.Width, looseWidth)
	}
}

func TestBuildPracticalRenderSegmentsUsesTimelineBoundariesForBlockAndChapterGaps(t *testing.T) {
	script := dto.PracticalScript{
		Blocks: []dto.PracticalBlock{
			{
				BlockID:      "block_01",
				TopicStartMS: 0,
				TopicEndMS:   1400,
				Chapters: []dto.PracticalChapter{
					{
						ChapterID: "ch_01",
						StartMS:   2400,
						EndMS:     4650,
						Turns: []dto.PracticalTurn{
							{TurnID: "t_01", SpeakerRole: "customer", Text: "a", StartMS: 2400, EndMS: 3400},
							{TurnID: "t_02", SpeakerRole: "staff", Text: "b", StartMS: 3400, EndMS: 4650},
						},
					},
					{
						ChapterID: "ch_02",
						StartMS:   5250,
						EndMS:     6500,
						Turns: []dto.PracticalTurn{
							{TurnID: "t_03", SpeakerRole: "customer", Text: "c", StartMS: 5500, EndMS: 6500},
						},
					},
				},
			},
			{
				BlockID:      "block_02",
				TopicStartMS: 7400,
				TopicEndMS:   8600,
				Chapters: []dto.PracticalChapter{
					{
						ChapterID: "ch_03",
						StartMS:   9600,
						EndMS:     11050,
						Turns: []dto.PracticalTurn{
							{TurnID: "t_04", SpeakerRole: "customer", Text: "d", StartMS: 9600, EndMS: 11050},
						},
					},
				},
			},
		},
	}

	segments := buildPracticalRenderSegments(
		script,
		[]string{"ch1.png", "ch2.png", "ch3.png"},
		[]string{"block1.png", "block2.png"},
		600,
		900,
	)

	if len(segments) != 5 {
		t.Fatalf("unexpected segment count: %d", len(segments))
	}
	if segments[0].BackgroundPath != "block1.png" || !approxEqual(segments[0].DurationSec, 2.4) {
		t.Fatalf("unexpected block1 intro segment: %#v", segments[0])
	}
	if segments[1].BackgroundPath != "ch1.png" || !approxEqual(segments[1].DurationSec, 2.85) {
		t.Fatalf("unexpected chapter1 segment: %#v", segments[1])
	}
	if segments[2].BackgroundPath != "ch2.png" || !approxEqual(segments[2].DurationSec, 2.15) {
		t.Fatalf("unexpected chapter2 segment: %#v", segments[2])
	}
	if segments[3].BackgroundPath != "block2.png" || !approxEqual(segments[3].DurationSec, 2.2) {
		t.Fatalf("unexpected block2 transition segment: %#v", segments[3])
	}
	if segments[4].BackgroundPath != "ch3.png" || !approxEqual(segments[4].DurationSec, 1.45) {
		t.Fatalf("unexpected chapter3 segment: %#v", segments[4])
	}
}

func approxEqual(a, b float64) bool {
	return math.Abs(a-b) < 0.0001
}
