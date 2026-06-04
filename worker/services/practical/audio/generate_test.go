package practical_audio_service

import (
	"strings"
	"testing"

	dto "worker/services/practical/model"
)

func TestBuildPracticalTTSPromptJapaneseKeepsFinalSyllables(t *testing.T) {
	block := dto.PracticalBlock{
		Topic: "買い物する",
		Speakers: []dto.PracticalSpeaker{
			{SpeakerID: "female", SpeakerRole: "customer"},
			{SpeakerID: "male", SpeakerRole: "clerk"},
		},
	}
	chapter := dto.PracticalChapter{
		Scene:       "レジで支払う",
		ScenePrompt: "A checkout counter, @[customer] paying while @[clerk] smiles politely.",
	}

	prompt := buildPracticalChapterTTSPrompt("ja", block, chapter)

	for _, expected := range []string{
		"Language: Japanese.",
		"Do not swallow, clip, or drop the ending of a word or sentence.",
		"Fully pronounce final verb endings and final morae such as ru, u, ku, tsu, and i.",
		"Keep particles and polite endings audible and complete, including desu, masu, and dictionary-form endings.",
		"Topic: 買い物する",
		"Scene: レジで支払う",
		"Visual scene: A checkout counter, @[customer] paying while @[clerk] smiles politely.",
		"Timing pauses are handled by the audio pipeline.",
	} {
		if !strings.Contains(prompt, expected) {
			t.Fatalf("prompt missing %q: %s", expected, prompt)
		}
	}
}

func TestBuildPracticalBlockTopicPromptJapaneseRequiresCompletePronunciation(t *testing.T) {
	block := dto.PracticalBlock{
		Topic:       "食事中のやり取り",
		BlockPrompt: "A family restaurant in Tokyo.",
	}

	prompt := buildPracticalBlockTopicPrompt("ja", block)

	for _, expected := range []string{
		"Read only the title text exactly as written, once.",
		"The title may be short. Even if it is short, pronounce every word, character, and ending completely before stopping.",
		"Do not skip, swallow, clip, merge, paraphrase, or add any word.",
		"Language: Japanese.",
		"Pronounce every Japanese word completely and clearly.",
	} {
		if !strings.Contains(prompt, expected) {
			t.Fatalf("block topic prompt missing %q: %s", expected, prompt)
		}
	}
	if strings.Contains(prompt, "Visual scene:") {
		t.Fatalf("block topic prompt should not include visual scene context: %s", prompt)
	}
}

func TestNormalizePracticalNarrationTextCollapsesWhitespace(t *testing.T) {
	got := normalizePracticalNarrationText("  食事中の\nやり取り \t ")
	if got != "食事中の やり取り" {
		t.Fatalf("unexpected normalized narration text: %q", got)
	}
}

func TestBuildRequestedChapterSetAcceptsKnownChapterNums(t *testing.T) {
	script := dto.PracticalScript{
		Blocks: []dto.PracticalBlock{
			{Chapters: []dto.PracticalChapter{{ChapterID: "ch_01"}, {ChapterID: "ch_02"}}},
			{Chapters: []dto.PracticalChapter{{ChapterID: "ch_03"}, {ChapterID: "ch_04"}}},
		},
	}

	got, err := buildRequestedChapterSet(script, []int{4, 2, 4})
	if err != nil {
		t.Fatalf("buildRequestedChapterSet returned err: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("expected 2 chapter nums, got %#v", got)
	}
	if _, ok := got[2]; !ok {
		t.Fatalf("expected chapter 2 in set: %#v", got)
	}
	if _, ok := got[4]; !ok {
		t.Fatalf("expected chapter 4 in set: %#v", got)
	}
}

func TestBuildRequestedChapterSetRejectsOutOfRangeChapterNum(t *testing.T) {
	script := dto.PracticalScript{
		Blocks: []dto.PracticalBlock{
			{Chapters: []dto.PracticalChapter{{ChapterID: "ch_01"}}},
		},
	}

	if _, err := buildRequestedChapterSet(script, []int{2}); err == nil {
		t.Fatalf("expected out-of-range chapter_num validation error")
	}
}

func TestPracticalSelectedChapterIndexesReturnsAllForFullGenerate(t *testing.T) {
	block := dto.PracticalBlock{
		Chapters: []dto.PracticalChapter{{ChapterID: "ch_01"}, {ChapterID: "ch_02"}, {ChapterID: "ch_03"}},
	}

	got := practicalSelectedChapterIndexes(block, nil, true, 0)
	if len(got) != 3 || got[0] != 0 || got[1] != 1 || got[2] != 2 {
		t.Fatalf("unexpected full chapter selection: %#v", got)
	}
}

func TestPracticalSelectedChapterIndexesUsesGlobalChapterNums(t *testing.T) {
	block := dto.PracticalBlock{
		Chapters: []dto.PracticalChapter{{ChapterID: "ch_03"}, {ChapterID: "ch_04"}, {ChapterID: "ch_05"}},
	}

	got := practicalSelectedChapterIndexes(block, map[int]struct{}{4: {}, 5: {}}, false, 2)
	if len(got) != 2 || got[0] != 1 || got[1] != 2 {
		t.Fatalf("unexpected requested chapter selection: %#v", got)
	}
}

func TestPracticalSelectedChapterIndexesDoesNotSelectWithoutChapterNums(t *testing.T) {
	block := dto.PracticalBlock{
		Chapters: []dto.PracticalChapter{{ChapterID: "ch_01"}, {ChapterID: "ch_02"}},
	}

	got := practicalSelectedChapterIndexes(block, nil, false, 0)
	if len(got) != 0 {
		t.Fatalf("expected no chapter selection without chapter_nums, got %#v", got)
	}
}
