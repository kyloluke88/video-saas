package podcast_export_service

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestGenerateFromProjectDirCreatesExportFiles(t *testing.T) {
	if _, err := findPDFBrowserPath(); err != nil {
		t.Skipf("skip browser pdf export test: %v", err)
	}
	if err := verifyBrowserPDFExportWorks(); err != nil {
		t.Skipf("skip browser pdf export test: %v", err)
	}

	assetsDir, err := filepath.Abs(filepath.Join("..", "..", "..", "assets"))
	if err != nil {
		t.Fatalf("resolve assets dir failed: %v", err)
	}
	sourceDir := findExportFixtureProject(t)

	t.Setenv("WORKER_ASSETS_DIR", assetsDir)

	projectDir := filepath.Join(t.TempDir(), "zh_podcast_20260401165006")
	if err := copyDir(sourceDir, projectDir); err != nil {
		t.Fatalf("copy fixture project failed: %v", err)
	}

	result, err := GenerateFromProjectDir(projectDir, "zh_podcast_20260401165006")
	if err != nil {
		t.Fatalf("GenerateFromProjectDir failed: %v", err)
	}

	for _, path := range []string{result.PDFPath, result.YouTubePublishPath} {
		info, err := os.Stat(path)
		if err != nil {
			t.Fatalf("stat export file failed path=%s err=%v", path, err)
		}
		if info.Size() == 0 {
			t.Fatalf("expected export file to be non-empty: %s", path)
		}
	}
	if len(result.YouTubeTranscriptPaths) == 0 {
		t.Fatalf("expected transcript exports to be generated")
	}
	for _, path := range result.YouTubeTranscriptPaths {
		info, err := os.Stat(path)
		if err != nil {
			t.Fatalf("stat transcript export file failed path=%s err=%v", path, err)
		}
		if info.Size() == 0 {
			t.Fatalf("expected transcript export file to be non-empty: %s", path)
		}
	}
	if _, err := os.Stat(filepath.Join(projectDir, "youtube_transcript.srt")); !os.IsNotExist(err) {
		t.Fatalf("expected legacy youtube_transcript.srt to be absent, err=%v", err)
	}

	publishRaw, err := os.ReadFile(result.YouTubePublishPath)
	if err != nil {
		t.Fatalf("read publish file failed: %v", err)
	}
	if !strings.Contains(string(publishRaw), "https://podcast.lucayo.com/podcast/scripts/what-panics-first-timers-in-china") {
		t.Fatalf("expected publish file to contain canonical page url, got %q", string(publishRaw))
	}
}

func TestBuildPrintViewRendersStructuredContent(t *testing.T) {
	view, err := buildPrintView(
		"zh",
		"测试标题",
		"",
		"第一段描述。\n\n第二段描述。",
		scriptDocument{
			Sections: []sectionDocument{{
				Heading: "开场",
				Lines: []lineDocument{{
					Speaker:     "female",
					SpeakerName: "盼盼",
					Text:        "大家好",
					Ruby: []rubyToken{
						{Surface: "大", Reading: "dà"},
						{Surface: "家", Reading: "jiā"},
						{Surface: "好", Reading: "hǎo"},
					},
					Translation: "Hello everyone",
				}},
			}},
		},
		[]vocabularyItem{{
			Term:        "生活习惯",
			Tokens:      []phoneticToken{{Char: "生", Reading: "shēng"}, {Char: "活", Reading: "huó"}},
			Meaning:     "living habit",
			Explanation: "Used for daily routines.",
			Examples: []exampleDocument{{
				Text:        "每个人的生活习惯都不太一样。",
				Tokens:      []phoneticToken{{Char: "每", Reading: "měi"}},
				Translation: "Everyone has different habits.",
			}},
		}},
		[]grammarItem{{
			Pattern:     "只要 A，就 B",
			Tokens:      []phoneticToken{{Char: "只", Reading: "zhǐ"}, {Char: "要", Reading: "yào"}},
			Meaning:     "as long as A, then B",
			Explanation: "Shows a sufficient condition.",
		}},
	)
	if err != nil {
		t.Fatalf("buildPrintView failed: %v", err)
	}
	if got, want := len(view.Description), 2; got != want {
		t.Fatalf("expected %d description paragraphs, got %d", want, got)
	}
	if got, want := view.ScriptHeading, "聊天脚本"; got != want {
		t.Fatalf("expected script heading %q, got %q", want, got)
	}
	if len(view.Sections) != 1 || len(view.Sections[0].Lines) != 1 {
		t.Fatalf("expected one rendered transcript line, got %#v", view.Sections)
	}
	if got, want := view.Sections[0].Lines[0].SpeakerClass, "speaker-female"; got != want {
		t.Fatalf("expected speaker class %q, got %q", want, got)
	}
	if !strings.Contains(string(view.Sections[0].Lines[0].TextHTML), "<ruby>") {
		t.Fatalf("expected transcript line to include ruby markup, got %q", view.Sections[0].Lines[0].TextHTML)
	}
	if len(view.Vocabulary) != 1 || len(view.Vocabulary[0].Examples) != 1 {
		t.Fatalf("expected one vocabulary card with one example, got %#v", view.Vocabulary)
	}
	if len(view.Grammar) != 1 {
		t.Fatalf("expected one grammar card, got %#v", view.Grammar)
	}
}

func findExportFixtureProject(t *testing.T) string {
	t.Helper()

	projectRoot := filepath.Join("..", "..", "..", "outputs", "projects")
	preferred := filepath.Join(projectRoot, "zh_podcast_20260401165006_json")
	if info, err := os.Stat(preferred); err == nil && info.IsDir() {
		absPath, err := filepath.Abs(preferred)
		if err != nil {
			t.Fatalf("resolve fixture path failed: %v", err)
		}
		return absPath
	}

	entries, err := filepath.Glob(filepath.Join(projectRoot, "*"))
	if err != nil {
		t.Fatalf("glob fixture projects failed: %v", err)
	}
	for _, candidate := range entries {
		info, err := os.Stat(candidate)
		if err != nil || !info.IsDir() {
			continue
		}
		if hasFixtureFiles(candidate) {
			absPath, err := filepath.Abs(candidate)
			if err != nil {
				t.Fatalf("resolve fixture path failed: %v", err)
			}
			return absPath
		}
	}

	t.Skip("skip browser pdf export test: no local podcast export fixture project found")
	return ""
}

func hasFixtureFiles(projectDir string) bool {
	for _, name := range []string{"request_payload.json", "script_aligned.json"} {
		if _, err := os.Stat(filepath.Join(projectDir, name)); err != nil {
			return false
		}
	}
	return true
}

func verifyBrowserPDFExportWorks() error {
	tempDir, err := os.MkdirTemp("", "podcast-export-browser-test-*")
	if err != nil {
		return err
	}
	defer os.RemoveAll(tempDir)

	htmlPath := filepath.Join(tempDir, "smoke.html")
	pdfPath := filepath.Join(tempDir, "smoke.pdf")
	if err := os.WriteFile(htmlPath, []byte("<!DOCTYPE html><html><body><p>ok</p></body></html>"), 0o644); err != nil {
		return err
	}
	return printHTMLToPDF(htmlPath, pdfPath)
}

func copyDir(src, dst string) error {
	return filepath.Walk(src, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}
		targetPath := filepath.Join(dst, rel)
		if info.IsDir() {
			return os.MkdirAll(targetPath, 0o755)
		}
		raw, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		return os.WriteFile(targetPath, raw, info.Mode())
	})
}
