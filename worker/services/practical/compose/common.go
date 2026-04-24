package practical_compose_service

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	conf "worker/pkg/config"
	dto "worker/services/practical/model"
)

func projectDirFor(projectID string) string {
	return filepath.Join(conf.Get[string]("worker.ffmpeg_work_dir"), "projects", strings.TrimSpace(projectID))
}

func projectScriptAlignedPath(projectDir string) string {
	return filepath.Join(projectDir, "script_aligned.json")
}

func projectDialoguePath(projectDir string) string {
	return filepath.Join(projectDir, "dialogue.wav")
}

func projectBaseVideoPath(projectDir string) string {
	return filepath.Join(projectDir, "practical_base.mp4")
}

func projectFinalVideoPath(projectDir string) string {
	return filepath.Join(projectDir, "practical_final.mp4")
}

func projectSubtitleASSPath(projectDir string) string {
	return filepath.Join(projectDir, "practical_subtitles.ass")
}

func projectYouTubeTranscriptPath(projectDir, language string) string {
	return filepath.Join(projectDir, fmt.Sprintf("youtube_transcript_%s.srt", strings.TrimSpace(language)))
}

func practicalBackgroundImagePath(filename string) string {
	base := filepath.Base(strings.TrimSpace(filename))
	candidates := []string{
		filepath.Join(conf.Get[string]("worker.worker_assets_dir"), "practical", "bg-images", base),
		filepath.Join(conf.Get[string]("worker.worker_assets_dir"), "practicle", "bg-images", base),
	}
	for _, candidate := range candidates {
		if info, err := os.Stat(candidate); err == nil && !info.IsDir() {
			return candidate
		}
	}
	return candidates[0]
}

func compactBackgroundNames(values []string) []string {
	out := make([]string, 0, len(values))
	for _, value := range values {
		if trimmed := strings.TrimSpace(value); trimmed != "" {
			out = append(out, trimmed)
		}
	}
	return out
}

func fileExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && !info.IsDir()
}

func readJSON(path string, out interface{}) error {
	raw, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	return json.Unmarshal(raw, out)
}

func writeJSON(path string, data interface{}) error {
	raw, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return os.WriteFile(path, raw, 0o644)
}

func practicalFontsDir() string {
	candidates := []string{
		filepath.Join("assets", "fonts"),
		filepath.Join("worker", "assets", "fonts"),
		"/Users/luca/go/github.com/luca/video-saas/worker/assets/fonts",
	}
	for _, candidate := range candidates {
		if info, err := os.Stat(candidate); err == nil && info.IsDir() {
			return candidate
		}
	}
	return filepath.Join("worker", "assets", "fonts")
}

func escapeFFmpegPath(path string) string {
	path = strings.ReplaceAll(path, `\`, `\\`)
	path = strings.ReplaceAll(path, ":", `\:`)
	return path
}

func practicalX264Preset() string {
	preset := strings.TrimSpace(conf.Get[string]("worker.practical_x264_preset", ""))
	if preset == "" {
		preset = strings.TrimSpace(conf.Get[string]("worker.podcast_x264_preset", "veryfast"))
	}
	if preset == "" {
		preset = "veryfast"
	}
	return preset
}

func practicalFFmpegTimeout() time.Duration {
	return time.Duration(firstPositive(conf.Get[int]("worker.ffmpeg_timeout_sec"), 300)) * time.Second
}

func practicalChapterGapMS() int {
	value := conf.Get[int]("worker.practical_chapter_gap_ms")
	if value < 0 {
		return 0
	}
	return value
}

func practicalBlockGapMS() int {
	value := conf.Get[int]("worker.practical_block_gap_ms")
	if value < 0 {
		return 0
	}
	return value
}

func practicalChapterTransitionLeadMS() int {
	value := conf.Get[int]("worker.practical_chapter_transition_lead_ms")
	if value < 0 {
		return 0
	}
	return value
}

func practicalBlockTransitionLeadMS() int {
	value := conf.Get[int]("worker.practical_block_transition_lead_ms")
	if value < 0 {
		return 0
	}
	return value
}

func practicalSubtitleLeadMS() int {
	value := conf.Get[int]("worker.practical_subtitle_lead_ms")
	if value < 0 {
		return 0
	}
	return value
}

func practicalResolutionDimensions(resolution string) (int, int) {
	switch strings.TrimSpace(strings.ToLower(resolution)) {
	case "480p":
		return 854, 480
	case "720p":
		return 1280, 720
	case "1440p":
		return 2560, 1440
	case "2000p":
		return 3556, 2000
	default:
		return 1920, 1080
	}
}

func loadAlignedScript(projectDir string, language string) (dto.PracticalScript, error) {
	var script dto.PracticalScript
	if err := readJSON(projectScriptAlignedPath(projectDir), &script); err != nil {
		return dto.PracticalScript{}, err
	}
	if err := validatePracticalScriptLanguage(script.Language, language); err != nil {
		return dto.PracticalScript{}, err
	}
	script.Language = strings.ToLower(strings.TrimSpace(language))
	script.Normalize()
	return script, nil
}

func validatePracticalScriptLanguage(scriptLanguage, payloadLanguage string) error {
	scriptLang := strings.ToLower(strings.TrimSpace(scriptLanguage))
	payloadLang := strings.ToLower(strings.TrimSpace(payloadLanguage))
	if _, err := requirePracticalLanguage(scriptLang); err != nil {
		return err
	}
	if scriptLang != payloadLang {
		return fmt.Errorf("script language mismatch: script=%q payload=%q", scriptLang, payloadLanguage)
	}
	return nil
}

func requirePracticalLanguage(value string) (string, error) {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "zh":
		return "zh", nil
	case "ja":
		return "ja", nil
	default:
		return "", fmt.Errorf("lang must be zh or ja")
	}
}

func blockStartEndMS(block dto.PracticalBlock) (int, int) {
	if block.EndMS > block.StartMS {
		return block.StartMS, block.EndMS
	}
	start := -1
	end := 0
	for _, chapter := range block.Chapters {
		for _, turn := range chapter.Turns {
			if turn.EndMS <= turn.StartMS {
				continue
			}
			if start < 0 || turn.StartMS < start {
				start = turn.StartMS
			}
			if turn.EndMS > end {
				end = turn.EndMS
			}
		}
	}
	if start < 0 {
		return 0, 0
	}
	return start, end
}

func chapterStartEndMS(chapter dto.PracticalChapter) (int, int) {
	if chapter.EndMS > chapter.StartMS {
		return chapter.StartMS, chapter.EndMS
	}
	start := -1
	end := 0
	for _, turn := range chapter.Turns {
		if turn.EndMS <= turn.StartMS {
			continue
		}
		if start < 0 || turn.StartMS < start {
			start = turn.StartMS
		}
		if turn.EndMS > end {
			end = turn.EndMS
		}
	}
	if start < 0 {
		return 0, 0
	}
	return start, end
}

func flattenChapters(script dto.PracticalScript) []struct {
	BlockIndex   int
	ChapterIndex int
	Chapter      dto.PracticalChapter
} {
	out := make([]struct {
		BlockIndex   int
		ChapterIndex int
		Chapter      dto.PracticalChapter
	}, 0)
	for blockIndex, block := range script.Blocks {
		for chapterIndex, chapter := range block.Chapters {
			out = append(out, struct {
				BlockIndex   int
				ChapterIndex int
				Chapter      dto.PracticalChapter
			}{
				BlockIndex:   blockIndex,
				ChapterIndex: chapterIndex,
				Chapter:      chapter,
			})
		}
	}
	return out
}

func collectTranslationLanguages(script dto.PracticalScript) []string {
	if locales := compactPracticalTranslationLocales(script.TranslationLocales); len(locales) > 0 {
		return locales
	}

	seen := make(map[string]struct{})
	out := make([]string, 0)
	extras := make([]string, 0)
	appendLang := func(value string) {
		lang := strings.TrimSpace(value)
		if lang == "" {
			return
		}
		if _, ok := seen[lang]; ok {
			return
		}
		seen[lang] = struct{}{}
		out = append(out, lang)
	}
	for _, block := range script.Blocks {
		for _, chapter := range block.Chapters {
			for _, turn := range chapter.Turns {
				for lang := range turn.Translations {
					trimmed := strings.TrimSpace(lang)
					if trimmed == "" {
						continue
					}
					if _, ok := seen[trimmed]; ok {
						continue
					}
					extras = append(extras, trimmed)
				}
			}
		}
	}
	if len(extras) > 0 {
		sort.Strings(extras)
		for _, lang := range extras {
			appendLang(lang)
		}
	}
	return out
}

func compactPracticalTranslationLocales(values []string) []string {
	seen := make(map[string]struct{}, len(values))
	out := make([]string, 0, len(values))
	for _, value := range values {
		lang := strings.TrimSpace(value)
		if lang == "" {
			continue
		}
		if _, ok := seen[lang]; ok {
			continue
		}
		seen[lang] = struct{}{}
		out = append(out, lang)
	}
	return out
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func firstPositive(values ...int) int {
	for _, value := range values {
		if value > 0 {
			return value
		}
	}
	return 0
}

func normalizePracticalDesignType(value int) int {
	if value == 2 {
		return 2
	}
	return 1
}

func normalizePracticalTTSType(value int) int {
	if value == 1 {
		return 1
	}
	return 1
}
