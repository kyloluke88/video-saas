package model

import "testing"

func TestPracticalBlockSpeakerVoicesByRole(t *testing.T) {
	block := PracticalBlock{
		BlockID: "block-1",
		Speakers: []PracticalSpeaker{
			{SpeakerID: "female", SpeakerRole: "seller", Name: "售货员"},
			{SpeakerID: "male", SpeakerRole: "customer", Name: "顾客"},
			{SpeakerID: "male", SpeakerRole: "manager", Name: "经理"},
		},
	}

	voicesByRole, err := block.SpeakerVoicesByRole()
	if err != nil {
		t.Fatalf("SpeakerVoicesByRole returned error: %v", err)
	}
	if got := voicesByRole["seller"]; got != "female" {
		t.Fatalf("seller role mismatch: got %q", got)
	}
	if got := voicesByRole["customer"]; got != "male" {
		t.Fatalf("customer role mismatch: got %q", got)
	}
	if got := voicesByRole["manager"]; got != "male" {
		t.Fatalf("manager role mismatch: got %q", got)
	}

	names := block.SpeakerNames()
	if got := names["female"]; got != "售货员" {
		t.Fatalf("female name mismatch: got %q", got)
	}
	if got := names["male"]; got != "顾客" {
		t.Fatalf("male name mismatch: got %q", got)
	}
}

func TestPracticalScriptValidateUsesBlockSpeakers(t *testing.T) {
	script := PracticalScript{
		Language: "zh",
		Blocks: []PracticalBlock{
			{
				BlockID: "block-1",
				Topic:   "超市购物",
				Speakers: []PracticalSpeaker{
					{SpeakerID: "female", SpeakerRole: "seller", Name: "售货员"},
					{SpeakerID: "male", SpeakerRole: "customer", Name: "顾客"},
					{SpeakerID: "male", SpeakerRole: "manager", Name: "经理"},
				},
				Chapters: []PracticalChapter{
					{
						ChapterID: "chapter-1",
						Turns: []PracticalTurn{
							{TurnID: "turn-1", SpeakerRole: "seller", Text: "你好"},
							{TurnID: "turn-2", SpeakerRole: "customer", Text: "请问牛奶在哪里？"},
							{TurnID: "turn-3", SpeakerRole: "manager", Text: "这里有促销。"},
						},
					},
				},
			},
		},
	}

	if err := script.Validate(); err != nil {
		t.Fatalf("Validate returned error: %v", err)
	}
}

func TestPracticalScriptNormalizeDetectsTranslationLocales(t *testing.T) {
	script := PracticalScript{
		Language: "ja",
		Blocks: []PracticalBlock{
			{
				BlockID: "block-1",
				Speakers: []PracticalSpeaker{
					{SpeakerID: "female", SpeakerRole: "customer"},
					{SpeakerID: "male", SpeakerRole: "staff"},
				},
				Chapters: []PracticalChapter{
					{
						ChapterID: "ch-1",
						Turns: []PracticalTurn{
							{
								TurnID:      "t-1",
								SpeakerRole: "customer",
								Text:        "すみません。",
								Translations: map[string]string{
									"en": "Excuse me.",
									"ko": "실례합니다.",
								},
							},
							{
								TurnID:      "t-2",
								SpeakerRole: "staff",
								Text:        "はい。",
								Translations: map[string]string{
									"id": "Ya.",
								},
							},
						},
					},
				},
			},
		},
	}

	script.Normalize()
	if len(script.TranslationLocales) != 3 {
		t.Fatalf("unexpected translation locale length: %d", len(script.TranslationLocales))
	}
	if got := script.TranslationLocales[0]; got != "en" {
		t.Fatalf("unexpected locale[0]: %s", got)
	}
	if got := script.TranslationLocales[1]; got != "ko" {
		t.Fatalf("unexpected locale[1]: %s", got)
	}
	if got := script.TranslationLocales[2]; got != "id" {
		t.Fatalf("unexpected locale[2]: %s", got)
	}
}

func TestPracticalScriptValidateRejectsSingleVoiceChannelBlock(t *testing.T) {
	script := PracticalScript{
		Language: "ja",
		Blocks: []PracticalBlock{
			{
				BlockID: "block-1",
				Topic:   "レストランで注文する",
				Speakers: []PracticalSpeaker{
					{SpeakerID: "male", SpeakerRole: "customer"},
					{SpeakerID: "male", SpeakerRole: "waiter"},
				},
				Chapters: []PracticalChapter{
					{
						ChapterID: "ch-1",
						Turns: []PracticalTurn{
							{TurnID: "t-1", SpeakerRole: "waiter", Text: "いらっしゃいませ。"},
							{TurnID: "t-2", SpeakerRole: "customer", Text: "1人です。"},
						},
					},
				},
			},
		},
	}

	if err := script.Validate(); err == nil {
		t.Fatalf("expected validate to reject block without female speaker")
	}
}
