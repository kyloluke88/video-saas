package podcast_audio_service

import (
	"strings"
	"testing"

	dto "worker/services/podcast/model"
)

func TestBuildGeminiBlockPrompt_ZHUsesFixedSpeakerBible(t *testing.T) {
	prompt := buildGeminiBlockPrompt("zh", "Panpan", "Laolu")

	for _, want := range []string{
		"# AUDIO PROFILE: Panpan & Laolu",
		"## \"Natural Mandarin Learning Podcast\"",
		"## THE SCENE: Quiet Home Podcast Studio",
		"### DIRECTOR'S NOTES",
		"Style:",
		"Pace: Slow",
		"Accent: Clear standard Mandarin Chinese pronunciation",
		"Panpan is warm, natural, slightly lively, and more emotionally present.",
		"Laolu is calm, steady, thoughtful, responsive, conversational, and human.",
		"#### TRANSCRIPT",
		"Read only the dialogue supplied in this request.",
	} {
		if !strings.Contains(prompt, want) {
			t.Fatalf("expected zh prompt to contain %q, got %q", want, prompt)
		}
	}
	for _, unwanted := range []string{
		"Two-speaker Japanese learning podcast.",
		"# BLOCK TRANSCRIPT",
		"SAMPLE CONTEXT",
		"%s",
	} {
		if strings.Contains(prompt, unwanted) {
			t.Fatalf("expected zh prompt to omit %q, got %q", unwanted, prompt)
		}
	}
}

func TestBuildGeminiBlockPrompt_JAUsesFixedSpeakerBible(t *testing.T) {
	prompt := buildGeminiBlockPrompt("ja", "Yui", "Akira")

	for _, want := range []string{
		"# AUDIO PROFILE: Yui & Akira",
		"## \"Natural Japanese Learning Podcast\"",
		"## THE SCENE: Quiet Home Podcast Studio",
		"### DIRECTOR'S NOTES",
		"Style:",
		"Pace: Slow",
		"Accent: Clear standard Japanese pronunciation",
		"Yui is warm, natural, slightly lively, and more emotionally present.",
		"Akira is calm, steady, thoughtful, responsive, conversational, and human.",
		"#### TRANSCRIPT",
	} {
		if !strings.Contains(prompt, want) {
			t.Fatalf("expected ja prompt to contain %q, got %q", want, prompt)
		}
	}
	for _, unwanted := range []string{
		"Two-speaker Mandarin Chinese learning podcast.",
		"# BLOCK TRANSCRIPT",
		"SPEAKER BINDING",
		"SAMPLE CONTEXT",
		"%s",
	} {
		if strings.Contains(prompt, unwanted) {
			t.Fatalf("expected ja prompt to omit %q, got %q", unwanted, prompt)
		}
	}
}

func TestBuildElevenLabsDialoguePrompt_ContainsTagInterpretationRule(t *testing.T) {
	prompt := buildElevenLabsDialoguePrompt("zh")
	for _, want := range []string{
		"Interpret inline square-bracket emotion/action tags as performance instructions",
		"never read tag words aloud",
		"[indecisive]",
		"[laughs]",
	} {
		if !strings.Contains(prompt, want) {
			t.Fatalf("expected elevenlabs prompt to contain %q, got %q", want, prompt)
		}
	}
}

func TestSpokenTextForElevenSynthesis_PrefersSpeechText(t *testing.T) {
	seg := dto.PodcastSegment{
		Text:       "字幕文本",
		SpeechText: "[quizzically] 真的吗？",
	}
	got := spokenTextForElevenSynthesis("zh", seg)
	if got != "[quizzically] 真的吗？" {
		t.Fatalf("expected speech_text to be preferred, got %q", got)
	}
}

func TestStripElevenEmotionTags_RemovesBracketTags(t *testing.T) {
	got := stripElevenEmotionTags("[indecisive] 我  [laughs]  其实  也  不确定")
	if got != "我 其实 也 不确定" {
		t.Fatalf("unexpected cleaned text: %q", got)
	}
}
