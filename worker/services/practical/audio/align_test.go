package practical_audio_service

import (
	"context"
	"os"
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
			{TurnID: "t_01", SpeechText: "[happy] ignored", Text: "hello"},
			{TurnID: "t_02", SpeechText: "[calm] ignored", Text: "world"},
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
			{TurnID: "t_01", Text: "すみません。お米はどこにありますか。", SpeechText: "[happy] すみません。お米はどこにありますか。"},
			{TurnID: "t_02", Text: "じゃあ、これを一つ買います。", SpeechText: "[calm] じゃあ、これを一つ買います。"},
		},
	}

	_, transcript := buildChapterTimingSpecs("ja", chapter)

	expected := "すみません。お米はどこにありますか。\nじゃあ、これを一つ買います。"
	if transcript != expected {
		t.Fatalf("unexpected japanese transcript: %q", transcript)
	}
}

func TestStabilizePracticalChapterTimingsAnchorsChapterDurationAndExtendsTurnEndToSilence(t *testing.T) {
	chapter := dto.PracticalChapter{
		ChapterID: "ch_01",
		Turns: []dto.PracticalTurn{
			{TurnID: "t_01", StartMS: 120, EndMS: 820},
			{TurnID: "t_02", StartMS: 1200, EndMS: 1720},
		},
	}

	got := stabilizePracticalChapterTimings(chapter, []practicalSilenceInterval{
		{StartMS: 930, EndMS: 1080},
		{StartMS: 1760, EndMS: 1880},
	}, 2200)

	if got.StartMS != 0 || got.EndMS != 2200 {
		t.Fatalf("unexpected chapter audio range: start=%d end=%d", got.StartMS, got.EndMS)
	}
	if got.Turns[0].StartMS != 120 || got.Turns[0].EndMS != 930 {
		t.Fatalf("unexpected turn1 timing: %#v", got.Turns[0])
	}
	if got.Turns[1].StartMS != 1200 || got.Turns[1].EndMS != 1760 {
		t.Fatalf("unexpected turn2 timing: %#v", got.Turns[1])
	}
}

func TestStabilizePracticalChapterTimingsDoesNotCrossNextTurnStart(t *testing.T) {
	chapter := dto.PracticalChapter{
		ChapterID: "ch_01",
		Turns: []dto.PracticalTurn{
			{TurnID: "t_01", StartMS: 0, EndMS: 1000},
			{TurnID: "t_02", StartMS: 1080, EndMS: 1800},
		},
	}

	got := stabilizePracticalChapterTimings(chapter, []practicalSilenceInterval{
		{StartMS: 1140, EndMS: 1240},
	}, 2200)

	if got.Turns[0].EndMS != 1000 {
		t.Fatalf("unexpected first turn end: %#v", got.Turns[0])
	}
}

func TestParsePracticalSilenceIntervalsClosesTrailingSilenceAtEOF(t *testing.T) {
	output := strings.Join([]string{
		"[silencedetect @ 0x1] silence_start: 1.230",
		"[silencedetect @ 0x1] silence_end: 1.480 | silence_duration: 0.250",
		"[silencedetect @ 0x1] silence_start: 2.500",
	}, "\n")

	got := parsePracticalSilenceIntervals(output, 3200)
	if len(got) != 2 {
		t.Fatalf("unexpected interval count: %#v", got)
	}
	if got[0].StartMS != 1230 || got[0].EndMS != 1480 {
		t.Fatalf("unexpected first interval: %#v", got[0])
	}
	if got[1].StartMS != 2500 || got[1].EndMS != 3200 {
		t.Fatalf("unexpected trailing interval: %#v", got[1])
	}
}

func TestMaterializeAlignedChapterAudioCopiesTempoAudioAndKeepsRealDuration(t *testing.T) {
	ctx := context.Background()
	projectDir := t.TempDir()
	sourcePath := filepath.Join(projectDir, "source.wav")
	outputPath := filepath.Join(projectDir, "out.wav")
	sourceContent := []byte("tempo-audio")
	if err := os.WriteFile(sourcePath, sourceContent, 0o644); err != nil {
		t.Fatalf("os.WriteFile returned err: %v", err)
	}

	got, err := materializeAlignedChapterAudio(ctx, sourcePath, outputPath, dto.PracticalChapter{
		ChapterID: "ch_01",
		Turns: []dto.PracticalTurn{
			{TurnID: "t_01", StartMS: 120, EndMS: 820},
		},
	}, 2200)
	if err != nil {
		t.Fatalf("materializeAlignedChapterAudio returned err: %v", err)
	}

	raw, err := os.ReadFile(outputPath)
	if err != nil {
		t.Fatalf("os.ReadFile returned err: %v", err)
	}
	if string(raw) != string(sourceContent) {
		t.Fatalf("unexpected copied audio content: %q", string(raw))
	}
	if got.StartMS != 0 || got.EndMS != 2200 {
		t.Fatalf("unexpected chapter timing: start=%d end=%d", got.StartMS, got.EndMS)
	}
	if got.Turns[0].StartMS != 120 || got.Turns[0].EndMS != 820 {
		t.Fatalf("unexpected turn timing: %#v", got.Turns[0])
	}
}

func TestCreateSilenceAudioLikeUsesReferenceSampleRate(t *testing.T) {
	ctx := context.Background()
	projectDir := t.TempDir()
	referencePath := filepath.Join(projectDir, "reference.wav")
	if err := createSilenceAudioWithStreamInfo(ctx, referencePath, 120, practicalAudioStreamInfo{
		SampleRate: 44100,
		Channels:   1,
	}); err != nil {
		t.Skipf("ffmpeg unavailable for audio format test: %v", err)
	}

	gapPath := filepath.Join(projectDir, "gap.wav")
	if err := createSilenceAudioLike(ctx, gapPath, 600, referencePath); err != nil {
		t.Fatalf("createSilenceAudioLike returned err: %v", err)
	}

	referenceInfo, err := audioStreamInfo(ctx, referencePath)
	if err != nil {
		t.Fatalf("audioStreamInfo(reference) returned err: %v", err)
	}
	gapInfo, err := audioStreamInfo(ctx, gapPath)
	if err != nil {
		t.Fatalf("audioStreamInfo(gap) returned err: %v", err)
	}
	if gapInfo.SampleRate != referenceInfo.SampleRate {
		t.Fatalf("unexpected gap sample rate: got=%d want=%d", gapInfo.SampleRate, referenceInfo.SampleRate)
	}
	if gapInfo.Channels != referenceInfo.Channels {
		t.Fatalf("unexpected gap channels: got=%d want=%d", gapInfo.Channels, referenceInfo.Channels)
	}
	durationMS, err := audioDurationMS(ctx, gapPath)
	if err != nil {
		t.Fatalf("audioDurationMS returned err: %v", err)
	}
	if durationMS < 590 || durationMS > 610 {
		t.Fatalf("unexpected gap duration: %dms", durationMS)
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

func TestLoadReusableLocalAlignedScriptReturnsNotFoundWithoutError(t *testing.T) {
	projectDir := t.TempDir()

	got, ok, err := loadReusableLocalAlignedScript(projectDir, "ja")
	if err != nil {
		t.Fatalf("loadReusableLocalAlignedScript returned err: %v", err)
	}
	if ok {
		t.Fatalf("expected aligned script to be reported missing, got ok=%v script=%#v", ok, got)
	}
}

func TestPracticalCanRecoverFullAlignFromLocalAudioRequiresAllRawAssets(t *testing.T) {
	projectDir := t.TempDir()
	script := dto.PracticalScript{
		Blocks: []dto.PracticalBlock{
			{
				BlockID: "block_01",
				Chapters: []dto.PracticalChapter{
					{ChapterID: "ch_01"},
					{ChapterID: "ch_02"},
				},
			},
		},
	}

	if err := os.MkdirAll(filepath.Dir(blockIntroRawAudioPath(projectDir, "block_01", 1)), 0o755); err != nil {
		t.Fatalf("mkdir topic dir failed: %v", err)
	}
	if err := os.WriteFile(blockIntroRawAudioPath(projectDir, "block_01", 1), []byte("topic"), 0o644); err != nil {
		t.Fatalf("write topic raw failed: %v", err)
	}
	if err := os.MkdirAll(filepath.Dir(chapterRawAudioPath(projectDir, "block_01", "ch_01", 1, 1)), 0o755); err != nil {
		t.Fatalf("mkdir chapter dir failed: %v", err)
	}
	if err := os.WriteFile(chapterRawAudioPath(projectDir, "block_01", "ch_01", 1, 1), []byte("ch1"), 0o644); err != nil {
		t.Fatalf("write ch1 raw failed: %v", err)
	}

	if practicalCanRecoverFullAlignFromLocalAudio(projectDir, script) {
		t.Fatal("expected recovery to fail when any chapter raw audio is missing")
	}

	if err := os.WriteFile(chapterRawAudioPath(projectDir, "block_01", "ch_02", 1, 2), []byte("ch2"), 0o644); err != nil {
		t.Fatalf("write ch2 raw failed: %v", err)
	}
	if !practicalCanRecoverFullAlignFromLocalAudio(projectDir, script) {
		t.Fatal("expected recovery to succeed when all raw assets exist")
	}
}
