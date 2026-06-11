package practical_page_service

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	dto "worker/services/practical/model"
)

func TestBuildYouTubeChapterLinesUsesChapterStartTimes(t *testing.T) {
	script := dto.PracticalScript{
		Language: "ja",
		Blocks: []dto.PracticalBlock{
			{
				BlockID: "block_01",
				Topic:   "Everyday Scenarios",
				Chapters: []dto.PracticalChapter{
					{ChapterID: "ch_01", Scene: "Restaurant", StartMS: 0},
					{ChapterID: "ch_02", Scene: "Station", StartMS: 132000},
				},
			},
		},
	}
	meta := youtubeMetadata{
		Chapters: []youtubeChapter{
			{ChapterID: "ch_01", TitleEN: "Ordering Food at a Japanese Restaurant"},
			{ChapterID: "ch_02", TitleEN: "Buying a Train Ticket in Japan"},
		},
	}

	got := buildYouTubeChapterLines(script, meta)
	want := []string{
		"00:00 Ordering Food at a Japanese Restaurant",
		"02:12 Buying a Train Ticket in Japan",
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("unexpected chapter lines:\n got %#v\nwant %#v", got, want)
	}
}

func TestBuildYouTubeChapterLinesFallsBackToSingleBlockChapterOrder(t *testing.T) {
	script := dto.PracticalScript{
		Language: "ja",
		Blocks: []dto.PracticalBlock{
			{
				BlockID: "block_01",
				Topic:   "Everyday Scenarios",
				Chapters: []dto.PracticalChapter{
					{ChapterID: "ch_01", Scene: "Restaurant", StartMS: 0},
					{ChapterID: "ch_02", Scene: "Station", StartMS: 132000},
				},
			},
		},
	}
	meta := youtubeMetadata{
		Chapters: []youtubeChapter{
			{BlockID: "block_01", TitleEN: "Ordering Food at a Japanese Restaurant"},
			{BlockID: "block_02", TitleEN: "Buying a Train Ticket in Japan"},
		},
	}

	got := buildYouTubeChapterLines(script, meta)
	want := []string{
		"00:00 Ordering Food at a Japanese Restaurant",
		"02:12 Buying a Train Ticket in Japan",
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("unexpected fallback chapter lines:\n got %#v\nwant %#v", got, want)
	}
}

func TestParseYouTubeMetadataKeepsChapterID(t *testing.T) {
	raw := json.RawMessage(`{
		"chapters": [
			{
				"chapter_id": "ch_01",
				"title_en": "Ordering Food at a Japanese Restaurant"
			}
		]
	}`)

	meta := parseYouTubeMetadata(raw)
	if got := meta.Chapters[0].ChapterID; got != "ch_01" {
		t.Fatalf("expected chapter_id to be parsed, got %q", got)
	}
}

func TestGenerateYouTubeTranscriptsIncludesSourceLanguage(t *testing.T) {
	projectDir := t.TempDir()
	script := dto.PracticalScript{
		Language:           "ja",
		TranslationLocales: []string{"en"},
		Blocks: []dto.PracticalBlock{
			{
				BlockID: "block_01",
				Topic:   "Topic",
				Chapters: []dto.PracticalChapter{
					{
						ChapterID: "ch_01",
						Turns: []dto.PracticalTurn{
							{
								TurnID:       "t_01",
								Text:         "ありがとうございます。",
								SpeechText:   "ありがとうございます。",
								StartMS:      1200,
								EndMS:        2400,
								Translations: map[string]string{"en": "Thank you very much."},
							},
						},
					},
				},
			},
		},
	}

	paths, err := generateYouTubeTranscripts(projectDir, script)
	if err != nil {
		t.Fatalf("generateYouTubeTranscripts returned err: %v", err)
	}

	wantPaths := []string{
		filepath.Join(projectDir, "youtube_transcript_ja.srt"),
		filepath.Join(projectDir, "youtube_transcript_en.srt"),
	}
	if !reflect.DeepEqual(paths, wantPaths) {
		t.Fatalf("unexpected transcript paths:\n got %#v\nwant %#v", paths, wantPaths)
	}

	jaRaw, err := os.ReadFile(wantPaths[0])
	if err != nil {
		t.Fatalf("read ja srt failed: %v", err)
	}
	if !strings.Contains(string(jaRaw), "00:00:01,200 --> 00:00:02,400") {
		t.Fatalf("expected ja srt to use turn timings, got %q", string(jaRaw))
	}
	if !strings.Contains(string(jaRaw), "ありがとうございます。") {
		t.Fatalf("expected ja srt to contain source text, got %q", string(jaRaw))
	}
}

func TestGenerateYouTubeTranscriptsUsesTurnTextForSourceLanguage(t *testing.T) {
	projectDir := t.TempDir()
	script := dto.PracticalScript{
		Language: "ja",
		Blocks: []dto.PracticalBlock{
			{
				BlockID: "block_01",
				Topic:   "Topic",
				Chapters: []dto.PracticalChapter{
					{
						ChapterID: "ch_01",
						Turns: []dto.PracticalTurn{
							{
								TurnID:     "t_01",
								Text:       "すみません、注文をお願いします。",
								SpeechText: "[happy] すみません、注文をお願いします。",
								StartMS:    1200,
								EndMS:      2400,
							},
						},
					},
				},
			},
		},
	}

	paths, err := generateYouTubeTranscripts(projectDir, script)
	if err != nil {
		t.Fatalf("generateYouTubeTranscripts returned err: %v", err)
	}
	if len(paths) != 1 {
		t.Fatalf("unexpected transcript path count: %d", len(paths))
	}

	raw, err := os.ReadFile(paths[0])
	if err != nil {
		t.Fatalf("read srt failed: %v", err)
	}
	text := string(raw)
	if !strings.Contains(text, "すみません、注文をお願いします。") {
		t.Fatalf("expected source transcript to use turn text, got %q", text)
	}
	if strings.Contains(text, "[happy]") {
		t.Fatalf("expected source transcript to omit speech_text markup, got %q", text)
	}
}

func TestGenerateYouTubeTranscriptsPreservesExistingFilesWhenNoLocalesGenerated(t *testing.T) {
	projectDir := t.TempDir()
	existingPath := filepath.Join(projectDir, "youtube_transcript_ja.srt")
	if err := os.WriteFile(existingPath, []byte("existing"), 0o644); err != nil {
		t.Fatalf("write existing transcript failed: %v", err)
	}

	paths, err := generateYouTubeTranscripts(projectDir, dto.PracticalScript{})
	if err != nil {
		t.Fatalf("generateYouTubeTranscripts returned err: %v", err)
	}
	if !reflect.DeepEqual(paths, []string{existingPath}) {
		t.Fatalf("unexpected transcript paths:\n got %#v\nwant %#v", paths, []string{existingPath})
	}

	raw, err := os.ReadFile(existingPath)
	if err != nil {
		t.Fatalf("read existing transcript failed: %v", err)
	}
	if string(raw) != "existing" {
		t.Fatalf("expected existing transcript to be preserved, got %q", string(raw))
	}
}

func TestGenerateYouTubePublishTextPreservesExistingFileWhenNewContentEmpty(t *testing.T) {
	projectDir := t.TempDir()
	existingPath := filepath.Join(projectDir, youtubePublishFilename)
	if err := os.WriteFile(existingPath, []byte("existing"), 0o644); err != nil {
		t.Fatalf("write existing publish text failed: %v", err)
	}

	path, err := generateYouTubePublishText(projectDir, dto.PracticalScript{})
	if err != nil {
		t.Fatalf("generateYouTubePublishText returned err: %v", err)
	}
	if path != existingPath {
		t.Fatalf("unexpected publish path: got %q want %q", path, existingPath)
	}

	raw, err := os.ReadFile(existingPath)
	if err != nil {
		t.Fatalf("read existing publish text failed: %v", err)
	}
	if string(raw) != "existing" {
		t.Fatalf("expected existing publish text to be preserved, got %q", string(raw))
	}
}
