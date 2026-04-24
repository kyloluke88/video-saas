package practical_audio_service

import (
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

	shifted := applyPracticalTimelineGaps(script, []int{1400, 1200}, 600, 900, 250)

	if got := shifted.Blocks[0].StartMS; got != 0 {
		t.Fatalf("block1 start mismatch: %d", got)
	}
	if got := shifted.Blocks[0].TopicStartMS; got != 0 {
		t.Fatalf("block1 topic start mismatch: %d", got)
	}
	if got := shifted.Blocks[0].TopicEndMS; got != 1400 {
		t.Fatalf("block1 topic end mismatch: %d", got)
	}
	if got := shifted.Blocks[0].Chapters[0].StartMS; got != 1400 {
		t.Fatalf("chapter1 start mismatch: %d", got)
	}
	if got := shifted.Blocks[0].Chapters[0].Turns[0].StartMS; got != 1650 {
		t.Fatalf("chapter1 turn1 start mismatch: %d", got)
	}
	if got := shifted.Blocks[0].Chapters[1].StartMS; got != 4250 {
		t.Fatalf("chapter2 start mismatch: %d", got)
	}
	if got := shifted.Blocks[0].EndMS; got != 5500 {
		t.Fatalf("block1 end mismatch: %d", got)
	}
	if got := shifted.Blocks[1].TopicStartMS; got != 6400 {
		t.Fatalf("block2 topic start mismatch: %d", got)
	}
	if got := shifted.Blocks[1].TopicEndMS; got != 7600 {
		t.Fatalf("block2 topic end mismatch: %d", got)
	}
	if got := shifted.Blocks[1].StartMS; got != 6400 {
		t.Fatalf("block2 start mismatch: %d", got)
	}
	if got := shifted.Blocks[1].EndMS; got != 9050 {
		t.Fatalf("block2 end mismatch: %d", got)
	}
	if got := shifted.Blocks[1].Chapters[0].StartMS; got != 7600 {
		t.Fatalf("block2 chapter start mismatch: %d", got)
	}
	if got := shifted.Blocks[1].Chapters[0].Turns[0].StartMS; got != 7850 {
		t.Fatalf("block2 turn start mismatch: %d", got)
	}
	if got := shifted.Blocks[1].Chapters[0].Turns[0].EndMS; got != 9050 {
		t.Fatalf("block2 turn end mismatch: %d", got)
	}
}

func TestMapWordsToBlockTimingsUsesMatchedWindows(t *testing.T) {
	block := dto.PracticalBlock{
		BlockID: "block_01",
		Chapters: []dto.PracticalChapter{
			{
				ChapterID: "ch_01",
				Turns: []dto.PracticalTurn{
					{TurnID: "t_01", SpeechText: "hello", Text: "hello"},
					{TurnID: "t_02", SpeechText: "world", Text: "world"},
				},
			},
		},
	}

	specs, _ := buildBlockTimingSpecs("ja", block)
	aligned, ok := mapWordsToBlockTimings(block, specs, []mfa.WordTiming{
		{Text: "hello", StartMS: 0, EndMS: 820},
		{Text: "world", StartMS: 1080, EndMS: 2200},
	}, 2400)
	if !ok {
		t.Fatalf("expected matched alignment")
	}

	first := aligned.Chapters[0].Turns[0]
	second := aligned.Chapters[0].Turns[1]

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
