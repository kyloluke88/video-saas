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
		"This content is for beginners.",
		"Keep the overall speaking pace very slow and easy to follow.",
		"Topic: 買い物する",
		"Scene: レジで支払う",
		"Visual scene: A checkout counter, @[customer] paying while @[clerk] smiles politely.",
		"Use clearly noticeable, wide pauses between turns so beginners can easily hear the speaker change.",
		"Leave a large, consistent, audible gap after each turn before the next speaker begins.",
		"At every speaker change, insert [extended pause, about 800ms] between the two turns.",
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

func TestResolvePracticalSpeakerVoiceAssignmentsUsesHeroAndPersistsMapping(t *testing.T) {
	projectDir := t.TempDir()
	block := dto.PracticalBlock{
		BlockID: "block_1",
		Speakers: []dto.PracticalSpeaker{
			{SpeakerID: "female", SpeakerRole: "hero"},
			{SpeakerID: "female", SpeakerRole: "role_a"},
			{SpeakerID: "male", SpeakerRole: "role_b"},
		},
	}

	assignments, err := resolvePracticalSpeakerVoiceAssignments(
		projectDir,
		block,
		map[string]string{"female": "hero-f", "male": "hero-m"},
		nil,
		map[string][]string{
			"female": {"f-1", "f-2"},
			"male":   {"m-1", "m-2"},
		},
	)
	if err != nil {
		t.Fatalf("resolvePracticalSpeakerVoiceAssignments returned err: %v", err)
	}
	if got := assignments["hero"]; got != "hero-f" {
		t.Fatalf("unexpected hero voice: %s", got)
	}
	if got := assignments["role_a"]; got != "f-1" {
		t.Fatalf("unexpected role_a voice: %s", got)
	}
	if got := assignments["role_b"]; got != "m-1" {
		t.Fatalf("unexpected role_b voice: %s", got)
	}

	var stored practicalVoiceAssignmentFile
	if err := readJSON(projectSpeakerVoiceMapPath(projectDir), &stored); err != nil {
		t.Fatalf("read voice map: %v", err)
	}
	if got := stored.Google["block_1"]["role_a"]; got != "f-1" {
		t.Fatalf("unexpected stored role_a voice: %s", got)
	}

	reused, err := resolvePracticalSpeakerVoiceAssignments(
		projectDir,
		block,
		map[string]string{"female": "hero-f", "male": "hero-m"},
		nil,
		map[string][]string{
			"female": nil,
			"male":   nil,
		},
	)
	if err != nil {
		t.Fatalf("resolvePracticalSpeakerVoiceAssignments reused returned err: %v", err)
	}
	if got := reused["role_a"]; got != "f-1" {
		t.Fatalf("unexpected reused role_a voice: %s", got)
	}
	if got := reused["role_b"]; got != "m-1" {
		t.Fatalf("unexpected reused role_b voice: %s", got)
	}
}

func TestResolvePracticalSpeakerVoiceAssignmentsErrorsWhenPoolIsTooSmall(t *testing.T) {
	block := dto.PracticalBlock{
		BlockID: "block_1",
		Speakers: []dto.PracticalSpeaker{
			{SpeakerID: "female", SpeakerRole: "hero"},
			{SpeakerID: "female", SpeakerRole: "role_a"},
			{SpeakerID: "female", SpeakerRole: "role_b"},
		},
	}

	if _, err := resolvePracticalSpeakerVoiceAssignments(
		t.TempDir(),
		block,
		map[string]string{"female": "hero-f", "male": "hero-m"},
		nil,
		map[string][]string{
			"female": {"f-1"},
		},
	); err == nil {
		t.Fatalf("expected pool exhaustion error")
	}
}

func TestResolvePracticalSpeakerVoiceAssignmentsPrefersExplicitOverride(t *testing.T) {
	block := dto.PracticalBlock{
		BlockID: "block_1",
		Speakers: []dto.PracticalSpeaker{
			{SpeakerID: "male", SpeakerRole: "hero"},
			{SpeakerID: "male", SpeakerRole: "driver", GoogleVoiceID: "driver-override"},
			{SpeakerID: "male", SpeakerRole: "station_staff"},
		},
	}

	assignments, err := resolvePracticalSpeakerVoiceAssignments(
		t.TempDir(),
		block,
		map[string]string{"female": "hero-f", "male": "hero-m"},
		func(speaker dto.PracticalSpeaker) string {
			return speaker.GoogleVoiceID
		},
		map[string][]string{
			"male": {"m-1", "m-2"},
		},
	)
	if err != nil {
		t.Fatalf("resolvePracticalSpeakerVoiceAssignments returned err: %v", err)
	}
	if got := assignments["hero"]; got != "hero-m" {
		t.Fatalf("unexpected hero voice: %s", got)
	}
	if got := assignments["driver"]; got != "driver-override" {
		t.Fatalf("unexpected driver voice: %s", got)
	}
	if got := assignments["station_staff"]; got != "m-1" {
		t.Fatalf("unexpected station_staff voice: %s", got)
	}
}

func TestPracticalGoogleSpeakerConfigsUsesChapterRolesAndFallbackSpeaker(t *testing.T) {
	block := dto.PracticalBlock{
		Speakers: []dto.PracticalSpeaker{
			{SpeakerID: "female", SpeakerRole: "hero", Name: "Hero"},
			{SpeakerID: "male", SpeakerRole: "role_a", Name: "Role A"},
		},
	}
	chapter := dto.PracticalChapter{
		ChapterID: "ch_01",
		Turns: []dto.PracticalTurn{
			{TurnID: "t_01", SpeakerRole: "hero", Text: "hello"},
		},
	}

	configs, names, err := practicalGoogleSpeakerConfigs(block, chapter, map[string]string{
		"hero":   "hero-f",
		"role_a": "role-a-m",
	})
	if err != nil {
		t.Fatalf("practicalGoogleSpeakerConfigs returned err: %v", err)
	}
	if len(configs) != 2 {
		t.Fatalf("expected 2 configs, got %#v", configs)
	}
	if configs[0].Speaker != "hero" || configs[0].VoiceID != "hero-f" {
		t.Fatalf("unexpected first config: %#v", configs[0])
	}
	if configs[1].Speaker != "role_a" || configs[1].VoiceID != "role-a-m" {
		t.Fatalf("unexpected second config: %#v", configs[1])
	}
	if names["hero"] != "Hero" || names["role_a"] != "Role A" {
		t.Fatalf("unexpected speaker names: %#v", names)
	}
}

func TestPracticalSpeakerVoiceMappingPromptIncludesRoleGenderAndVoice(t *testing.T) {
	block := dto.PracticalBlock{
		Speakers: []dto.PracticalSpeaker{
			{SpeakerID: "female", SpeakerRole: "hero"},
			{SpeakerID: "male", SpeakerRole: "friend"},
		},
	}
	chapter := dto.PracticalChapter{
		ChapterID: "ch_07",
		Turns: []dto.PracticalTurn{
			{TurnID: "t_131", SpeakerRole: "friend", Text: "hello"},
			{TurnID: "t_132", SpeakerRole: "hero", Text: "hi"},
		},
	}

	prompt := practicalSpeakerVoiceMappingPrompt(block, chapter, map[string]string{
		"hero":   "Leda",
		"friend": "Achird",
	})

	for _, expected := range []string{
		"Speaker mapping for this chapter:",
		"- friend: male voice Achird.",
		"- hero: female voice Leda.",
		"Every line labeled \"friend:\" must be spoken only with the male voice Achird.",
		"Every line labeled \"hero:\" must be spoken only with the female voice Leda.",
		"Never merge two speakers into one voice.",
	} {
		if !strings.Contains(prompt, expected) {
			t.Fatalf("prompt missing %q: %s", expected, prompt)
		}
	}
}
