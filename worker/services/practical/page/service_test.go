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
