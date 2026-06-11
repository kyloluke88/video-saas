package model

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestPracticalBlockSpeakerVoicesByRole(t *testing.T) {
	block := PracticalBlock{
		BlockID: "block-1",
		Speakers: []PracticalSpeaker{
			{SpeakerID: "female", SpeakerRole: "seller", Name: "售货员", GoogleVoiceID: "google-f-1"},
			{SpeakerID: "male", SpeakerRole: "customer", Name: "顾客", GoogleVoiceID: "google-m-1"},
			{SpeakerID: "male", SpeakerRole: "manager", Name: "经理", ElevenLabsVoiceID: "eleven-m-2"},
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
						},
					},
					{
						ChapterID: "chapter-2",
						Turns: []PracticalTurn{
							{TurnID: "turn-3", SpeakerRole: "manager", Text: "这里有促销。"},
							{TurnID: "turn-4", SpeakerRole: "customer", Text: "谢谢，我去看看。"},
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

func TestPracticalScriptValidateRejectsChapterWithMoreThanTwoActiveSpeakers(t *testing.T) {
	script := PracticalScript{
		Language: "zh",
		Blocks: []PracticalBlock{
			{
				BlockID: "block-1",
				Topic:   "我的周末",
				Speakers: []PracticalSpeaker{
					{SpeakerID: "female", SpeakerRole: "me"},
					{SpeakerID: "male", SpeakerRole: "taxi_driver"},
					{SpeakerID: "female", SpeakerRole: "station_staff"},
				},
				Chapters: []PracticalChapter{
					{
						ChapterID: "chapter-1",
						Turns: []PracticalTurn{
							{TurnID: "turn-1", SpeakerRole: "me", Text: "去机场怎么走？"},
							{TurnID: "turn-2", SpeakerRole: "taxi_driver", Text: "我送你过去。"},
							{TurnID: "turn-3", SpeakerRole: "station_staff", Text: "电车在二楼。"},
						},
					},
				},
			},
		},
	}

	if err := script.Validate(); err == nil {
		t.Fatalf("expected validate to reject chapter with more than 2 active speakers")
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

func TestPracticalScriptValidateAllowsProviderSpecificMultiVoiceBlock(t *testing.T) {
	script := PracticalScript{
		Language: "ja",
		Blocks: []PracticalBlock{
			{
				BlockID: "block-1",
				Topic:   "レストランで注文する",
				Speakers: []PracticalSpeaker{
					{SpeakerID: "female", SpeakerRole: "customer", GoogleVoiceID: "ja-JP-Neural2-B", ElevenLabsVoiceID: "voice_customer"},
					{SpeakerID: "male", SpeakerRole: "waiter", GoogleVoiceID: "ja-JP-Neural2-C", ElevenLabsVoiceID: "voice_waiter"},
					{SpeakerID: "male", SpeakerRole: "cashier", GoogleVoiceID: "ja-JP-Neural2-D", ElevenLabsVoiceID: "voice_cashier"},
				},
				Chapters: []PracticalChapter{
					{
						ChapterID: "ch-1",
						Turns: []PracticalTurn{
							{TurnID: "t-1", SpeakerRole: "waiter", Text: "いらっしゃいませ。"},
							{TurnID: "t-2", SpeakerRole: "customer", Text: "1人です。"},
						},
					},
					{
						ChapterID: "ch-2",
						Turns: []PracticalTurn{
							{TurnID: "t-3", SpeakerRole: "cashier", Text: "お会計はこちらです。"},
							{TurnID: "t-4", SpeakerRole: "customer", Text: "カードでお願いします。"},
						},
					},
				},
			},
		},
	}

	if err := script.Validate(); err != nil {
		t.Fatalf("expected validate to allow provider-specific multi-voice block, got err=%v", err)
	}
}

func TestPracticalScriptValidateRejectsMissingSpeakerID(t *testing.T) {
	script := PracticalScript{
		Language: "ja",
		Blocks: []PracticalBlock{
			{
				BlockID: "block-1",
				Topic:   "レストランで注文する",
				Speakers: []PracticalSpeaker{
					{SpeakerID: "female", SpeakerRole: "hero"},
					{SpeakerRole: "waiter"},
				},
				Chapters: []PracticalChapter{
					{
						ChapterID: "ch-1",
						Turns: []PracticalTurn{
							{TurnID: "t-1", SpeakerRole: "hero", Text: "こんにちは。"},
							{TurnID: "t-2", SpeakerRole: "waiter", Text: "いらっしゃいませ。"},
						},
					},
				},
			},
		},
	}

	if err := script.Validate(); err == nil {
		t.Fatalf("expected validate to reject missing speaker_id")
	}
}

func TestPracticalBlockResolveTurnVoiceAllowsSameGenderMultiSpeakerBlock(t *testing.T) {
	block := PracticalBlock{
		BlockID: "block-1",
		Speakers: []PracticalSpeaker{
			{SpeakerID: "female", SpeakerRole: "passenger_a"},
			{SpeakerID: "female", SpeakerRole: "passenger_b"},
			{SpeakerID: "male", SpeakerRole: "driver"},
		},
	}

	voice, err := block.ResolveTurnVoice(PracticalTurn{
		TurnID:      "turn-1",
		SpeakerRole: "passenger_b",
		Text:        "我们快到了吗？",
	})
	if err != nil {
		t.Fatalf("ResolveTurnVoice returned err: %v", err)
	}
	if voice != "female" {
		t.Fatalf("unexpected resolved voice: %s", voice)
	}
}

func TestMyDayScriptValidates(t *testing.T) {
	raw, err := os.ReadFile(filepath.Join("..", "..", "..", "assets", "practical", "scripts", "my_day.json"))
	if err != nil {
		t.Fatalf("read my_day.json failed: %v", err)
	}
	var script PracticalScript
	if err := json.Unmarshal(raw, &script); err != nil {
		t.Fatalf("unmarshal my_day.json failed: %v", err)
	}
	if err := script.Validate(); err != nil {
		t.Fatalf("my_day.json should validate, got err=%v", err)
	}
}

func TestPracticalScriptPreservesSpeakerPromptOnJSONRoundTrip(t *testing.T) {
	raw := []byte(`{
		"language":"ja",
		"blocks":[
			{
				"block_id":"block_01",
				"topic":"スーパーで買い物",
				"speakers":[
					{
						"speaker_id":"female",
						"speaker_role":"customer",
						"google_voice_id":"ja-JP-Neural2-C",
						"elevenlabs_voice_id":"eleven_customer",
						"speaker_prompt":"A young woman in a casual shopping outfit."
					},
					{
						"speaker_id":"male",
						"speaker_role":"clerk",
						"google_voice_id":"ja-JP-Neural2-D",
						"elevenlabs_voice_id":"eleven_clerk",
						"speaker_prompt":"A polite young store clerk in uniform."
					}
				],
				"chapters":[
					{
						"chapter_id":"ch_01",
						"turns":[
							{"turn_id":"t_01","speaker_role":"customer","text":"すみません。"}
						]
					}
				]
			}
		]
	}`)

	var script PracticalScript
	if err := json.Unmarshal(raw, &script); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}

	script.Normalize()
	if got := script.Blocks[0].Speakers[0].SpeakerPrompt; got != "A young woman in a casual shopping outfit." {
		t.Fatalf("speaker prompt mismatch after normalize: %q", got)
	}
	if got := script.Blocks[0].Speakers[0].GoogleVoiceID; got != "ja-JP-Neural2-C" {
		t.Fatalf("google_voice_id mismatch after normalize: %q", got)
	}
	if got := script.Blocks[0].Speakers[0].ElevenLabsVoiceID; got != "eleven_customer" {
		t.Fatalf("elevenlabs_voice_id mismatch after normalize: %q", got)
	}

	encoded, err := json.Marshal(script)
	if err != nil {
		t.Fatalf("marshal failed: %v", err)
	}
	if !json.Valid(encoded) {
		t.Fatalf("marshal output is not valid json: %s", string(encoded))
	}

	var decoded map[string]any
	if err := json.Unmarshal(encoded, &decoded); err != nil {
		t.Fatalf("decode round-trip failed: %v", err)
	}
	blocks, ok := decoded["blocks"].([]any)
	if !ok || len(blocks) != 1 {
		t.Fatalf("unexpected blocks payload: %#v", decoded["blocks"])
	}
	block, ok := blocks[0].(map[string]any)
	if !ok {
		t.Fatalf("unexpected block payload: %#v", blocks[0])
	}
	speakers, ok := block["speakers"].([]any)
	if !ok || len(speakers) != 2 {
		t.Fatalf("unexpected speakers payload: %#v", block["speakers"])
	}
	firstSpeaker, ok := speakers[0].(map[string]any)
	if !ok {
		t.Fatalf("unexpected first speaker payload: %#v", speakers[0])
	}
	if got := firstSpeaker["speaker_prompt"]; got != "A young woman in a casual shopping outfit." {
		t.Fatalf("speaker_prompt missing after round-trip: %#v", got)
	}
	if got := firstSpeaker["google_voice_id"]; got != "ja-JP-Neural2-C" {
		t.Fatalf("google_voice_id missing after round-trip: %#v", got)
	}
	if got := firstSpeaker["elevenlabs_voice_id"]; got != "eleven_customer" {
		t.Fatalf("elevenlabs_voice_id missing after round-trip: %#v", got)
	}
}
