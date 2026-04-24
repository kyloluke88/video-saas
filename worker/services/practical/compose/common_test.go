package practical_compose_service

import (
	"testing"

	dto "worker/services/practical/model"
)

func TestCollectTranslationLanguagesUsesLocalesAsSourceOfTruth(t *testing.T) {
	script := dto.PracticalScript{
		TranslationLocales: []string{"en", "es-419", "en", "zh-Hans"},
		Blocks: []dto.PracticalBlock{
			{
				BlockID: "block-1",
				Speakers: []dto.PracticalSpeaker{
					{SpeakerID: "female", SpeakerRole: "customer"},
					{SpeakerID: "male", SpeakerRole: "staff"},
				},
				Chapters: []dto.PracticalChapter{
					{
						ChapterID: "ch-1",
						Turns: []dto.PracticalTurn{
							{
								TurnID:      "t-1",
								SpeakerRole: "customer",
								Text:        "hello",
								Translations: map[string]string{
									"vi": "xin chao",
									"ko": "안녕하세요",
								},
							},
						},
					},
				},
			},
		},
	}

	got := collectTranslationLanguages(script)
	if len(got) != 3 {
		t.Fatalf("unexpected locale count: %d", len(got))
	}
	if got[0] != "en" || got[1] != "es-419" || got[2] != "zh-Hans" {
		t.Fatalf("unexpected locales: %#v", got)
	}
}

func TestCollectTranslationLanguagesFallsBackToTurnTranslations(t *testing.T) {
	script := dto.PracticalScript{
		Blocks: []dto.PracticalBlock{
			{
				BlockID: "block-1",
				Speakers: []dto.PracticalSpeaker{
					{SpeakerID: "female", SpeakerRole: "customer"},
					{SpeakerID: "male", SpeakerRole: "staff"},
				},
				Chapters: []dto.PracticalChapter{
					{
						ChapterID: "ch-1",
						Turns: []dto.PracticalTurn{
							{
								TurnID:      "t-1",
								SpeakerRole: "customer",
								Text:        "hello",
								Translations: map[string]string{
									"vi":      "xin chao",
									"ko":      "안녕하세요",
									"zh-Hans": "你好",
								},
							},
						},
					},
				},
			},
		},
	}

	got := collectTranslationLanguages(script)
	if len(got) != 3 {
		t.Fatalf("unexpected locale count: %d", len(got))
	}
	if got[0] != "ko" || got[1] != "vi" || got[2] != "zh-Hans" {
		t.Fatalf("unexpected locales: %#v", got)
	}
}
