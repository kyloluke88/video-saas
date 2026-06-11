package googlecloud

import "testing"

func TestBuildConversationGenerateContentRequestBodyUsesExplicitSpeakerVoiceConfigs(t *testing.T) {
	body := BuildConversationGenerateContentRequestBody("test-model", SynthesizeConversationRequest{
		Prompt: "sample prompt",
		Turns: []ConversationTurn{
			{Speaker: "hero", Text: "hello"},
			{Speaker: "role_a", Text: "hi"},
		},
		SpeakerNames: map[string]string{
			"hero":   "Hero",
			"role_a": "Role A",
		},
		SpeakerVoiceConfigs: []SpeakerVoiceConfig{
			{Speaker: "hero", VoiceID: "voice_hero"},
			{Speaker: "role_a", VoiceID: "voice_role_a"},
		},
	})

	generationConfig, ok := body["generationConfig"].(map[string]any)
	if !ok {
		t.Fatalf("missing generationConfig: %#v", body)
	}
	speechConfig, ok := generationConfig["speechConfig"].(map[string]any)
	if !ok {
		t.Fatalf("missing speechConfig: %#v", generationConfig)
	}
	multiSpeakerVoiceConfig, ok := speechConfig["multiSpeakerVoiceConfig"].(map[string]any)
	if !ok {
		t.Fatalf("missing multiSpeakerVoiceConfig: %#v", speechConfig)
	}
	rawConfigs, ok := multiSpeakerVoiceConfig["speakerVoiceConfigs"].([]map[string]any)
	if ok {
		if len(rawConfigs) != 2 {
			t.Fatalf("unexpected explicit speaker configs: %#v", rawConfigs)
		}
		if rawConfigs[0]["speaker"] != "Hero" || rawConfigs[1]["speaker"] != "Role A" {
			t.Fatalf("unexpected speaker names: %#v", rawConfigs)
		}
		return
	}

	configs, ok := multiSpeakerVoiceConfig["speakerVoiceConfigs"].([]any)
	if !ok || len(configs) != 2 {
		t.Fatalf("unexpected explicit speaker configs: %#v", multiSpeakerVoiceConfig["speakerVoiceConfigs"])
	}
	first, _ := configs[0].(map[string]any)
	second, _ := configs[1].(map[string]any)
	if first["speaker"] != "Hero" || second["speaker"] != "Role A" {
		t.Fatalf("unexpected speaker names: %#v", configs)
	}
}

func TestBuildConversationGenerateContentRequestBodyOmitsUnsupportedSpeakingRate(t *testing.T) {
	body := BuildConversationGenerateContentRequestBody("test-model", SynthesizeConversationRequest{
		Prompt:        "sample prompt",
		MaleVoiceID:   "male_voice",
		FemaleVoiceID: "female_voice",
		SpeakingRate:  0.85,
		Turns: []ConversationTurn{
			{Speaker: "female", Text: "hello"},
			{Speaker: "male", Text: "hi"},
		},
	})

	generationConfig, ok := body["generationConfig"].(map[string]any)
	if !ok {
		t.Fatalf("missing generationConfig: %#v", body)
	}
	speechConfig, ok := generationConfig["speechConfig"].(map[string]any)
	if !ok {
		t.Fatalf("missing speechConfig: %#v", generationConfig)
	}
	if _, exists := speechConfig["speakingRate"]; exists {
		t.Fatalf("unexpected speakingRate in speechConfig: %#v", speechConfig)
	}
}
