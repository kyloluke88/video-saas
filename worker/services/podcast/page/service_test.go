package podcast_page_service

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	services "worker/services"
	dto "worker/services/podcast/model"
)

func TestBuildPageUpsertFromProjectDir(t *testing.T) {
	projectID := "zh_podcast_20260401165006_json"
	projectDir := filepath.Clean(filepath.Join("..", "..", "..", "outputs", "projects", projectID))

	upsert, err := BuildPageUpsertFromProjectDir(projectDir, PersistInput{
		ProjectID: projectID,
		VideoURL:  "https://cdn.example.com/projects/zh_podcast_20260401165006_json/final.mp4",
	})
	if err != nil {
		t.Fatalf("BuildPageUpsertFromProjectDir failed: %v", err)
	}

	if upsert.ProjectID != projectID {
		t.Fatalf("unexpected project_id: %s", upsert.ProjectID)
	}
	if upsert.Title == "" || !strings.Contains(upsert.Title, "外国人第一次来中国") {
		t.Fatalf("unexpected title: %s", upsert.Title)
	}
	if upsert.VideoURL == "" || !strings.Contains(upsert.VideoURL, "cdn.example.com") {
		t.Fatalf("unexpected video_url: %s", upsert.VideoURL)
	}
	if len(upsert.Script) == 0 || !strings.Contains(string(upsert.Script), "\"sections\"") {
		t.Fatalf("script json was not built correctly: %s", string(upsert.Script))
	}
	if upsert.Slug != "what-panics-first-timers-in-china" {
		t.Fatalf("unexpected slug: %s", upsert.Slug)
	}
}

func TestBuildPageUpsertFromProjectDirIncludesVocabularyAndGrammar(t *testing.T) {
	projectDir := t.TempDir()

	requestPayload := `{
		"lang":"zh",
		"title":"为什么中国人总说多喝热水？",
		"script_filename":"chinese-hot-water-test.json"
	}`
	scriptInput := `{
		"language":"zh",
		"title":"为什么中国人总说多喝热水？",
		"en_title":"Why Do Chinese People Always Say Drink More Hot Water",
		"vocabulary":[
			{
				"term":"多喝热水",
				"tokens":[
					{"char":"多","reading":"duō"},
					{"char":"喝","reading":"hē"},
					{"char":"热","reading":"rè"},
					{"char":"水","reading":"shuǐ"}
				],
				"meaning":"drink more hot water",
				"explanation":"A common caring phrase in Chinese.",
				"examples":[
					{"text":"不舒服就多喝热水。","tokens":[{"char":"不","reading":"bù"},{"char":"舒","reading":"shū"},{"char":"服","reading":"fu"},{"char":"就","reading":"jiù"},{"char":"多","reading":"duō"},{"char":"喝","reading":"hē"},{"char":"热","reading":"rè"},{"char":"水","reading":"shuǐ"},{"char":"。","reading":""}],"translation":"If you feel unwell, drink more hot water."},
					{"text":"她总是叫我多喝热水。","tokens":[{"char":"她","reading":"tā"},{"char":"总","reading":"zǒng"},{"char":"是","reading":"shì"},{"char":"叫","reading":"jiào"},{"char":"我","reading":"wǒ"},{"char":"多","reading":"duō"},{"char":"喝","reading":"hē"},{"char":"热","reading":"rè"},{"char":"水","reading":"shuǐ"},{"char":"。","reading":""}],"translation":"She always tells me to drink more hot water."}
				]
			}
		],
		"grammar":[
			{
				"pattern":"A 的时候，B 也...",
				"tokens":[
					{"char":"的","reading":"de"},
					{"char":"时","reading":"shí"},
					{"char":"候","reading":"hou"},
					{"char":"也","reading":"yě"}
				],
				"meaning":"when A happens, B also happens",
				"explanation":"Used to list multiple situations where the same thing applies.",
				"examples":[
					{"text":"感冒的时候说，肚子不舒服的时候也说。","tokens":[{"char":"感","reading":"gǎn"},{"char":"冒","reading":"mào"},{"char":"的","reading":"de"},{"char":"时","reading":"shí"},{"char":"候","reading":"hou"},{"char":"说","reading":"shuō"},{"char":"，","reading":""},{"char":"肚","reading":"dù"},{"char":"子","reading":"zi"},{"char":"不","reading":"bù"},{"char":"舒","reading":"shū"},{"char":"服","reading":"fu"},{"char":"的","reading":"de"},{"char":"时","reading":"shí"},{"char":"候","reading":"hou"},{"char":"也","reading":"yě"},{"char":"说","reading":"shuō"},{"char":"。","reading":""}],"translation":"They say it when you have a cold, and also when your stomach feels bad."},
					{"text":"忙的时候不回，开会的时候也不回。","tokens":[{"char":"忙","reading":"máng"},{"char":"的","reading":"de"},{"char":"时","reading":"shí"},{"char":"候","reading":"hou"},{"char":"不","reading":"bù"},{"char":"回","reading":"huí"},{"char":"，","reading":""},{"char":"开","reading":"kāi"},{"char":"会","reading":"huì"},{"char":"的","reading":"de"},{"char":"时","reading":"shí"},{"char":"候","reading":"hou"},{"char":"也","reading":"yě"},{"char":"不","reading":"bù"},{"char":"回","reading":"huí"},{"char":"。","reading":""}],"translation":"He does not reply when he is busy, and he also does not reply when he is in a meeting."}
				]
			}
		],
		"segments":[
			{
				"segment_id":"seg_001",
				"speaker":"female",
				"speaker_name":"盼盼",
				"text":"你有没有发现，中国人很喜欢说多喝热水？",
				"en":"Have you noticed that Chinese people really like saying, drink more hot water?",
				"tokens":[
					{"char":"你","reading":"nǐ"},
					{"char":"有","reading":"yǒu"},
					{"char":"没","reading":"méi"},
					{"char":"有","reading":"yǒu"},
					{"char":"发","reading":"fā"},
					{"char":"现","reading":"xiàn"},
					{"char":"，","reading":""},
					{"char":"中","reading":"zhōng"},
					{"char":"国","reading":"guó"},
					{"char":"人","reading":"rén"},
					{"char":"很","reading":"hěn"},
					{"char":"喜","reading":"xǐ"},
					{"char":"欢","reading":"huān"},
					{"char":"说","reading":"shuō"},
					{"char":"多","reading":"duō"},
					{"char":"喝","reading":"hē"},
					{"char":"热","reading":"rè"},
					{"char":"水","reading":"shuǐ"},
					{"char":"？","reading":""}
				]
			}
		]
	}`

	if err := os.WriteFile(filepath.Join(projectDir, "request_payload.json"), []byte(requestPayload), 0o644); err != nil {
		t.Fatalf("write request_payload.json failed: %v", err)
	}
	if err := os.WriteFile(filepath.Join(projectDir, "script_input.json"), []byte(scriptInput), 0o644); err != nil {
		t.Fatalf("write script_input.json failed: %v", err)
	}

	upsert, err := BuildPageUpsertFromProjectDir(projectDir, PersistInput{
		ProjectID: "podcast-page-test-001",
	})
	if err != nil {
		t.Fatalf("BuildPageUpsertFromProjectDir failed: %v", err)
	}

	if len(upsert.Vocabulary) == 0 {
		t.Fatal("expected vocabulary to be populated from script_input.json")
	}
	if len(upsert.Grammar) == 0 {
		t.Fatal("expected grammar to be populated from script_input.json")
	}
	if !strings.Contains(string(upsert.Script), `"speaker_name":"盼盼"`) {
		t.Fatalf("expected script json to preserve speaker_name, got: %s", string(upsert.Script))
	}

	var vocabulary []map[string]any
	if err := json.Unmarshal(upsert.Vocabulary, &vocabulary); err != nil {
		t.Fatalf("unmarshal vocabulary failed: %v", err)
	}
	if len(vocabulary) != 1 {
		t.Fatalf("unexpected vocabulary length: %d", len(vocabulary))
	}

	var grammar []map[string]any
	if err := json.Unmarshal(upsert.Grammar, &grammar); err != nil {
		t.Fatalf("unmarshal grammar failed: %v", err)
	}
	if len(grammar) != 1 {
		t.Fatalf("unexpected grammar length: %d", len(grammar))
	}
}

func TestBuildPageUpsertFromProjectDirRequiresEnTitleForSlug(t *testing.T) {
	projectDir := t.TempDir()

	requestPayload := `{
		"lang":"zh",
		"title":"测试标题",
		"script_filename":"test.json"
	}`
	scriptInput := `{
		"language":"zh",
		"title":"测试标题",
		"segments":[
			{
				"segment_id":"seg_001",
				"speaker":"female",
				"speaker_name":"盼盼",
				"text":"测试内容",
				"en":"test"
			}
		]
	}`

	if err := os.WriteFile(filepath.Join(projectDir, "request_payload.json"), []byte(requestPayload), 0o644); err != nil {
		t.Fatalf("write request_payload.json failed: %v", err)
	}
	if err := os.WriteFile(filepath.Join(projectDir, "script_input.json"), []byte(scriptInput), 0o644); err != nil {
		t.Fatalf("write script_input.json failed: %v", err)
	}

	_, err := BuildPageUpsertFromProjectDir(projectDir, PersistInput{
		ProjectID: "podcast-page-test-002",
	})
	if err == nil || !strings.Contains(err.Error(), "en_title is required") {
		t.Fatalf("expected en_title requirement error, got: %v", err)
	}
	var nonRetryable services.NonRetryableError
	if !errors.As(err, &nonRetryable) {
		t.Fatalf("expected non-retryable error, got: %T %v", err, err)
	}
}

func TestBuildPageSlugStripsPunctuationFromEnTitle(t *testing.T) {
	slug := buildPageSlug(dto.PodcastScript{
		EnTitle: "How Are Ne, Yo, and Yone Different?",
	})
	if slug != "how-are-ne-yo-and-yone-different" {
		t.Fatalf("unexpected slug: %s", slug)
	}
}
