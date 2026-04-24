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
	if !strings.Contains(text, "Style: TurnSub,Maruko Gothic CJKjp Medium,38,") {
		t.Fatalf("expected practical turn font size, got: %s", text)
	}
	if !strings.Contains(text, "Style: BlockSub,Maruko Gothic CJKjp Medium,78,") {
		t.Fatalf("expected block title font size, got: %s", text)
	}
	if !strings.Contains(text, "Style: TurnRuby,Maruko Gothic CJKjp Medium,18,") {
		t.Fatalf("expected ruby style, got: %s", text)
	}
	if !strings.Contains(text, "ぎゅうにゅう") {
		t.Fatalf("expected ruby reading to be rendered, got: %s", text)
	}
	if !strings.Contains(text, ",TurnBox,,0,0,0,,{\\p1}") {
		t.Fatalf("expected vector box dialogue line, got: %s", text)
	}
	if !strings.Contains(text, ",BlockSub,,0,0,0,,{\\an8\\pos(") {
		t.Fatalf("expected centered block subtitle line, got: %s", text)
	}
}

func TestBuildPracticalRenderSegmentsUsesBlockIntroAndChapterGaps(t *testing.T) {
	script := dto.PracticalScript{
		Blocks: []dto.PracticalBlock{
			{
				BlockID:      "block_01",
				TopicStartMS: 0,
				TopicEndMS:   1600,
				Chapters: []dto.PracticalChapter{
					{
						ChapterID: "ch_01",
						Turns: []dto.PracticalTurn{
							{TurnID: "t_01", SpeakerRole: "customer", Text: "a", StartMS: 1600, EndMS: 2600},
						},
					},
					{
						ChapterID: "ch_02",
						Turns: []dto.PracticalTurn{
							{TurnID: "t_02", SpeakerRole: "staff", Text: "b", StartMS: 3200, EndMS: 4200},
						},
					},
				},
			},
			{
				BlockID:      "block_02",
				TopicStartMS: 5100,
				TopicEndMS:   6600,
				Chapters: []dto.PracticalChapter{
					{
						ChapterID: "ch_03",
						Turns: []dto.PracticalTurn{
							{TurnID: "t_03", SpeakerRole: "customer", Text: "c", StartMS: 6600, EndMS: 7600},
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
		800,
		1200,
		1000,
		1000,
	)

	if len(segments) != 5 {
		t.Fatalf("unexpected segment count: %d", len(segments))
	}
	if segments[0].BackgroundPath != "block1.png" || !approxEqual(segments[0].DurationSec, 2.6) {
		t.Fatalf("unexpected block1 intro segment: %#v", segments[0])
	}
	if segments[1].BackgroundPath != "ch1.png" || !approxEqual(segments[1].DurationSec, 2.8) {
		t.Fatalf("unexpected chapter1 segment: %#v", segments[1])
	}
	if segments[2].BackgroundPath != "ch2.png" || !approxEqual(segments[2].DurationSec, 2.0) {
		t.Fatalf("unexpected chapter2 segment: %#v", segments[2])
	}
	if segments[3].BackgroundPath != "block2.png" || !approxEqual(segments[3].DurationSec, 3.7) {
		t.Fatalf("unexpected block2 transition segment: %#v", segments[3])
	}
	if segments[4].BackgroundPath != "ch3.png" || !approxEqual(segments[4].DurationSec, 2.0) {
		t.Fatalf("unexpected chapter3 segment: %#v", segments[4])
	}
}

func approxEqual(a, b float64) bool {
	return math.Abs(a-b) < 0.0001
}
