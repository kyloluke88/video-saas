package podcast_audio_service

import (
	"strings"
	"testing"

	"worker/internal/dto"
)

func TestBuildGeminiBlockPrompt_ZHUsesFixedSpeakerBible(t *testing.T) {
	prompt := buildGeminiBlockPrompt("zh")

	for _, want := range []string{
		"Two-speaker Mandarin Chinese learning podcast.",
		"Male voice:",
		"Female voice:",
		"longtime close friends",
		"clear everyday Mandarin",
		"learner-friendly",
	} {
		if !strings.Contains(prompt, want) {
			t.Fatalf("expected zh prompt to contain %q, got %q", want, prompt)
		}
	}
	for _, unwanted := range []string{"Two-speaker Japanese learning podcast.", "Block purpose", "%s"} {
		if strings.Contains(prompt, unwanted) {
			t.Fatalf("expected zh prompt to omit %q, got %q", unwanted, prompt)
		}
	}
}

func TestBuildGeminiBlockPrompt_JAUsesFixedSpeakerBible(t *testing.T) {
	prompt := buildGeminiBlockPrompt("ja")

	for _, want := range []string{
		"Two-speaker Japanese learning podcast.",
		"Male voice:",
		"Female voice:",
		"longtime close friends",
		"clear everyday Japanese",
		"learner-friendly",
	} {
		if !strings.Contains(prompt, want) {
			t.Fatalf("expected ja prompt to contain %q, got %q", want, prompt)
		}
	}
	for _, unwanted := range []string{"Two-speaker Mandarin Chinese learning podcast.", "Block purpose", "%s"} {
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
