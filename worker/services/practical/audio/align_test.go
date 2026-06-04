package practical_audio_service

import (
	"path/filepath"
	"strings"
	"testing"

	"worker/pkg/mfa"
	dto "worker/services/practical/model"
)

func TestApplyPracticalTimelineGapsAppliesTopicChapterAndBlockOffsets(t *testing.T) {
	script := dto.PracticalScript{
		Blocks: []dto.PracticalBlock{
			{
				BlockID: "block_01",
				Chapters: []dto.PracticalChapter{
					{
						ChapterID: "ch_01",
						Turns: []dto.PracticalTurn{
							{TurnID: "t_01", StartMS: 0, EndMS: 1000},
							{TurnID: "t_02", StartMS: 1000, EndMS: 2000},
						},
					},
					{
						ChapterID: "ch_02",
						Turns: []dto.PracticalTurn{
							{TurnID: "t_03", StartMS: 2200, EndMS: 3200},
						},
					},
				},
			},
			{
				BlockID: "block_02",
				Chapters: []dto.PracticalChapter{
					{
						ChapterID: "ch_03",
						Turns: []dto.PracticalTurn{
							{TurnID: "t_04", StartMS: 0, EndMS: 1200},
						},
					},
				},
			},
		},
	}

	shifted := applyPracticalTimelineGaps(script, []int{1400, 1200}, 600, 900, 250, 400)

	if got := shifted.Blocks[0].StartMS; got != 0 {
		t.Fatalf("block1 start mismatch: %d", got)
	}
	if got := shifted.Blocks[0].TopicStartMS; got != 0 {
		t.Fatalf("block1 topic start mismatch: %d", got)
	}
	if got := shifted.Blocks[0].TopicEndMS; got != 1400 {
		t.Fatalf("block1 topic end mismatch: %d", got)
	}
	if got := shifted.Blocks[0].Chapters[0].StartMS; got != 1800 {
		t.Fatalf("chapter1 start mismatch: %d", got)
	}
	if got := shifted.Blocks[0].Chapters[0].Turns[0].StartMS; got != 1800 {
		t.Fatalf("chapter1 turn1 start mismatch: %d", got)
	}
	if got := shifted.Blocks[0].Chapters[1].StartMS; got != 4400 {
		t.Fatalf("chapter2 start mismatch: %d", got)
	}
	if got := shifted.Blocks[0].Chapters[1].Turns[0].StartMS; got != 4650 {
		t.Fatalf("chapter2 turn1 start mismatch: %d", got)
	}
	if got := shifted.Blocks[0].EndMS; got != 5650 {
		t.Fatalf("block1 end mismatch: %d", got)
	}
	if got := shifted.Blocks[1].TopicStartMS; got != 6550 {
		t.Fatalf("block2 topic start mismatch: %d", got)
	}
	if got := shifted.Blocks[1].TopicEndMS; got != 7750 {
		t.Fatalf("block2 topic end mismatch: %d", got)
	}
	if got := shifted.Blocks[1].StartMS; got != 6550 {
		t.Fatalf("block2 start mismatch: %d", got)
	}
	if got := shifted.Blocks[1].EndMS; got != 9350 {
		t.Fatalf("block2 end mismatch: %d", got)
	}
	if got := shifted.Blocks[1].Chapters[0].StartMS; got != 8150 {
		t.Fatalf("block2 chapter start mismatch: %d", got)
	}
	if got := shifted.Blocks[1].Chapters[0].Turns[0].StartMS; got != 8150 {
		t.Fatalf("block2 turn start mismatch: %d", got)
	}
	if got := shifted.Blocks[1].Chapters[0].Turns[0].EndMS; got != 9350 {
		t.Fatalf("block2 turn end mismatch: %d", got)
	}
}

func TestMapWordsToChapterTimingsUsesMatchedWindows(t *testing.T) {
	chapter := dto.PracticalChapter{
		ChapterID: "ch_01",
		Turns: []dto.PracticalTurn{
			{TurnID: "t_01", SpeechText: "hello", Text: "hello"},
			{TurnID: "t_02", SpeechText: "world", Text: "world"},
		},
	}

	specs, _ := buildChapterTimingSpecs("ja", chapter)
	aligned, ok := mapWordsToChapterTimings(chapter, specs, []mfa.WordTiming{
		{Text: "hello", StartMS: 0, EndMS: 820},
		{Text: "world", StartMS: 1080, EndMS: 2200},
	}, 2400)
	if !ok {
		t.Fatalf("expected matched alignment")
	}

	first := aligned.Turns[0]
	second := aligned.Turns[1]

	if first.StartMS != 0 {
		t.Fatalf("unexpected first turn start: %d", first.StartMS)
	}
	if first.EndMS != 820 {
		t.Fatalf("unexpected first turn end: %d", first.EndMS)
	}
	if second.StartMS != 1080 {
		t.Fatalf("unexpected second turn start: %d", second.StartMS)
	}
	if second.EndMS != 2200 {
		t.Fatalf("unexpected second turn end: %d", second.EndMS)
	}
}

func TestBuildConcatListContentUsesAbsolutePaths(t *testing.T) {
	content, err := buildConcatListContent([]string{
		filepath.Join("outputs", "projects", "demo", "blocks", "block_01_topic.wav"),
	})
	if err != nil {
		t.Fatalf("buildConcatListContent returned err: %v", err)
	}

	expected, err := filepath.Abs(filepath.Join("outputs", "projects", "demo", "blocks", "block_01_topic.wav"))
	if err != nil {
		t.Fatalf("filepath.Abs returned err: %v", err)
	}
	if !strings.Contains(content, expected) {
		t.Fatalf("expected concat content to contain absolute path %q, got %q", expected, content)
	}
}

func TestBuildChapterTimingSpecsUsesRawJapaneseTranscriptForMFA(t *testing.T) {
	chapter := dto.PracticalChapter{
		ChapterID: "ch_01",
		Turns: []dto.PracticalTurn{
			{TurnID: "t_01", SpeechText: "すみません。お米はどこにありますか。"},
			{TurnID: "t_02", SpeechText: "じゃあ、これを一つ買います。"},
		},
	}

	_, transcript := buildChapterTimingSpecs("ja", chapter)

	expected := "すみません。お米はどこにありますか。\nじゃあ、これを一つ買います。"
	if transcript != expected {
		t.Fatalf("unexpected japanese transcript: %q", transcript)
	}
}

func TestApplyPracticalTurnGapToChapterTimingsPreservesRawTimelineAndAddsGap(t *testing.T) {
	chapter := dto.PracticalChapter{
		ChapterID: "ch_01",
		Turns: []dto.PracticalTurn{
			{TurnID: "t_01", StartMS: 120, EndMS: 820},
			{TurnID: "t_02", StartMS: 990, EndMS: 1720},
			{TurnID: "t_03", StartMS: 2100, EndMS: 2600},
		},
	}

	shifted, slices := applyPracticalTurnGapToChapterTimings(chapter, 280, 3000)

	if len(slices) != 3 {
		t.Fatalf("unexpected slice count: %d", len(slices))
	}
	if shifted.Turns[0].StartMS != 120 || shifted.Turns[0].EndMS != 820 {
		t.Fatalf("unexpected turn1 timing: %#v", shifted.Turns[0])
	}
	if shifted.Turns[1].StartMS != 1270 || shifted.Turns[1].EndMS != 2000 {
		t.Fatalf("unexpected turn2 timing: %#v", shifted.Turns[1])
	}
	if shifted.Turns[2].StartMS != 2660 || shifted.Turns[2].EndMS != 3160 {
		t.Fatalf("unexpected turn3 timing: %#v", shifted.Turns[2])
	}
	if shifted.StartMS != 0 || shifted.EndMS != 3560 {
		t.Fatalf("unexpected chapter audio range: start=%d end=%d", shifted.StartMS, shifted.EndMS)
	}
	if slices[0].StartMS != 0 || slices[0].EndMS != 820 {
		t.Fatalf("unexpected source slice for turn1: %#v", slices[0])
	}
	if slices[1].StartMS != 820 || slices[1].EndMS != 1720 {
		t.Fatalf("unexpected source slice for turn2: %#v", slices[1])
	}
	if slices[2].StartMS != 1720 || slices[2].EndMS != 3000 {
		t.Fatalf("unexpected source slice for turn3: %#v", slices[2])
	}
}

func TestLocalizePracticalChapterTimingsSubtractsAbsoluteChapterStart(t *testing.T) {
	chapter := dto.PracticalChapter{
		ChapterID: "ch_01",
		StartMS:   1800,
		EndMS:     3000,
		Turns: []dto.PracticalTurn{
			{TurnID: "t_01", StartMS: 1900, EndMS: 3000},
		},
	}

	got := localizePracticalChapterTimings(chapter, 0)

	if got.StartMS != 0 || got.EndMS != 1200 {
		t.Fatalf("unexpected localized chapter range: start=%d end=%d", got.StartMS, got.EndMS)
	}
	if got.Turns[0].StartMS != 100 || got.Turns[0].EndMS != 1200 {
		t.Fatalf("unexpected localized turn timing: %#v", got.Turns[0])
	}
}

func TestLocalizePracticalChapterTimingsSubtractsChapterLeadForReusedChapter(t *testing.T) {
	chapter := dto.PracticalChapter{
		ChapterID: "ch_02",
		StartMS:   4400,
		EndMS:     5650,
		Turns: []dto.PracticalTurn{
			{TurnID: "t_03", StartMS: 4650, EndMS: 5650},
		},
	}

	got := localizePracticalChapterTimings(chapter, 250)

	if got.StartMS != 0 || got.EndMS != 1000 {
		t.Fatalf("unexpected localized chapter range: start=%d end=%d", got.StartMS, got.EndMS)
	}
	if got.Turns[0].StartMS != 0 || got.Turns[0].EndMS != 1000 {
		t.Fatalf("unexpected localized turn timing: %#v", got.Turns[0])
	}
}

func TestMergeReusableLocalChapterTimingsPreservesScriptContentAndCopiesTimings(t *testing.T) {
	script := dto.PracticalScript{
		Blocks: []dto.PracticalBlock{
			{
				BlockID: "block_01",
				Chapters: []dto.PracticalChapter{
					{
						ChapterID: "ch_01",
						Scene:     "new scene",
						Turns: []dto.PracticalTurn{
							{TurnID: "t_01", Text: "new text"},
						},
					},
				},
			},
		},
	}
	reusable := dto.PracticalScript{
		Blocks: []dto.PracticalBlock{
			{
				BlockID: "block_01",
				Chapters: []dto.PracticalChapter{
					{
						ChapterID: "ch_01",
						StartMS:   0,
						EndMS:     1320,
						Turns: []dto.PracticalTurn{
							{TurnID: "t_01", StartMS: 120, EndMS: 1320},
						},
					},
				},
			},
		},
	}

	got, err := mergeReusableLocalChapterTimings(script, reusable)
	if err != nil {
		t.Fatalf("mergeReusableLocalChapterTimings returned err: %v", err)
	}

	chapter := got.Blocks[0].Chapters[0]
	if chapter.Scene != "new scene" || chapter.Turns[0].Text != "new text" {
		t.Fatalf("expected script content to be preserved: %#v", chapter)
	}
	if chapter.StartMS != 0 || chapter.EndMS != 1320 {
		t.Fatalf("unexpected merged chapter timing: %#v", chapter)
	}
	if chapter.Turns[0].StartMS != 120 || chapter.Turns[0].EndMS != 1320 {
		t.Fatalf("unexpected merged turn timing: %#v", chapter.Turns[0])
	}
}
