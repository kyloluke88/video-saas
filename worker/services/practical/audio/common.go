package practical_audio_service

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"strings"
	"time"
	"unicode"

	conf "worker/pkg/config"
	"worker/pkg/googlecloud"
	"worker/pkg/mfa"
	ffmpegcommon "worker/services/media/ffmpeg/common"
)

func scriptPathFor(filename string) string {
	base := filepath.Base(strings.TrimSpace(filename))
	candidates := []string{
		filepath.Join(conf.Get[string]("worker.worker_assets_dir"), "practical", "scripts", base),
		filepath.Join(conf.Get[string]("worker.worker_assets_dir"), "practicle", "scripts", base),
		filepath.Join("worker", "doc", "practical", base),
		filepath.Join("worker", "doc", "practicle", base),
		filepath.Join("worker", "doc", "life", base),
		filepath.Join("doc", "practicle", base),
		filepath.Join("doc", "life", base),
	}
	for _, candidate := range candidates {
		if info, err := os.Stat(candidate); err == nil && !info.IsDir() {
			return candidate
		}
	}
	return candidates[0]
}

func projectDirFor(projectID string) string {
	return filepath.Join(conf.Get[string]("worker.ffmpeg_work_dir"), "projects", strings.TrimSpace(projectID))
}

func projectScriptInputPath(projectDir string) string {
	return filepath.Join(projectDir, "script_input.json")
}

func projectScriptAlignedPath(projectDir string) string {
	return filepath.Join(projectDir, "script_aligned.json")
}

func projectDialoguePath(projectDir string) string {
	return filepath.Join(projectDir, "dialogue.wav")
}

func projectChapterGapPath(projectDir string) string {
	return filepath.Join(projectDir, "chapter_gap.wav")
}

func projectBlockGapPath(projectDir string) string {
	return filepath.Join(projectDir, "block_gap.wav")
}

func projectChapterTransitionLeadPath(projectDir string) string {
	return filepath.Join(projectDir, "chapter_transition_lead.wav")
}

func projectBlockTransitionLeadPath(projectDir string) string {
	return filepath.Join(projectDir, "block_transition_lead.wav")
}

func blocksDir(projectDir string) string {
	return filepath.Join(projectDir, "blocks")
}

func blockAudioPath(projectDir, blockID string, blockIndex int) string {
	name := sanitizePracticalID(blockID)
	if name == "" {
		name = fmt.Sprintf("block_%02d", maxInt(1, blockIndex))
	}
	return filepath.Join(blocksDir(projectDir), name+".wav")
}

func blockIntroAudioPath(projectDir, blockID string, blockIndex int) string {
	name := sanitizePracticalID(blockID)
	if name == "" {
		name = fmt.Sprintf("block_%02d", maxInt(1, blockIndex))
	}
	return filepath.Join(blocksDir(projectDir), name+"_topic.wav")
}

func sanitizePracticalID(value string) string {
	raw := strings.TrimSpace(value)
	if raw == "" {
		return ""
	}
	raw = strings.ReplaceAll(raw, "/", "_")
	raw = strings.ReplaceAll(raw, "\\", "_")
	raw = strings.ReplaceAll(raw, " ", "_")
	return raw
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

func readJSON(path string, out interface{}) error {
	raw, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	return json.Unmarshal(raw, out)
}

func fileExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && !info.IsDir()
}

func createSilenceAudio(ctx context.Context, path string, durationMs int) error {
	if durationMs <= 0 {
		return nil
	}
	return ffmpegcommon.RunFFmpegContext(
		ctx,
		"-y",
		"-f", "lavfi",
		"-i", "anullsrc=r=24000:cl=mono",
		"-t", fmt.Sprintf("%.3f", float64(durationMs)/1000.0),
		"-c:a", "pcm_s16le",
		path,
	)
}

func applyAudioTempoToFile(ctx context.Context, path string, tempo float64) error {
	if tempo <= 0 || math.Abs(tempo-1.0) <= 0.001 {
		return nil
	}
	filter := buildAtempoFilter(tempo)
	if strings.TrimSpace(filter) == "" {
		return nil
	}

	dir := filepath.Dir(path)
	ext := filepath.Ext(path)
	if strings.TrimSpace(ext) == "" {
		ext = ".wav"
	}
	tmpFile, err := os.CreateTemp(dir, "tempo_*.tmp"+ext)
	if err != nil {
		return err
	}
	tmpPath := tmpFile.Name()
	if err := tmpFile.Close(); err != nil {
		_ = os.Remove(tmpPath)
		return err
	}

	if err := ffmpegcommon.RunFFmpegContext(
		ctx,
		"-y",
		"-i", path,
		"-filter:a", filter,
		"-c:a", "pcm_s16le",
		tmpPath,
	); err != nil {
		_ = os.Remove(tmpPath)
		return err
	}
	if err := os.Rename(tmpPath, path); err != nil {
		_ = os.Remove(tmpPath)
		return err
	}
	return nil
}

func buildAtempoFilter(speed float64) string {
	if speed <= 0 {
		return ""
	}
	speed = math.Round(speed*1000) / 1000
	if speed <= 0 || math.Abs(speed-1.0) <= 0.001 {
		return ""
	}

	parts := make([]string, 0, 4)
	remaining := speed
	for remaining < 0.5 {
		parts = append(parts, "atempo=0.5")
		remaining /= 0.5
	}
	for remaining > 2.0 {
		parts = append(parts, "atempo=2.0")
		remaining /= 2.0
	}
	parts = append(parts, fmt.Sprintf("atempo=%.3f", remaining))
	return strings.Join(parts, ",")
}

func concatAudioFiles(ctx context.Context, projectDir string, files []string, outputPath string) error {
	if len(files) == 0 {
		return fmt.Errorf("no audio files to concat")
	}
	listPath := filepath.Join(projectDir, fmt.Sprintf("audio_concat_%d.txt", time.Now().UnixNano()))
	var b strings.Builder
	for _, file := range files {
		b.WriteString("file '")
		b.WriteString(strings.ReplaceAll(file, "'", "'\\''"))
		b.WriteString("'\n")
	}
	if err := os.WriteFile(listPath, []byte(b.String()), 0o644); err != nil {
		return err
	}
	defer os.Remove(listPath)

	return ffmpegcommon.RunFFmpegContext(
		ctx,
		"-y",
		"-f", "concat",
		"-safe", "0",
		"-i", listPath,
		"-c", "copy",
		outputPath,
	)
}

func extractAudioChunk(ctx context.Context, sourcePath, outputPath string, startMS, endMS int) error {
	startMS = maxInt(0, startMS)
	endMS = maxInt(startMS+1, endMS)
	if err := os.MkdirAll(filepath.Dir(outputPath), 0o755); err != nil {
		return err
	}
	return ffmpegcommon.RunFFmpegContext(
		ctx,
		"-y",
		"-i", sourcePath,
		"-ss", fmt.Sprintf("%.3f", float64(startMS)/1000.0),
		"-to", fmt.Sprintf("%.3f", float64(endMS)/1000.0),
		"-c:a", "pcm_s16le",
		outputPath,
	)
}

func newGoogleSpeechClient() (*googlecloud.Client, error) {
	return googlecloud.New(googlecloud.Config{
		ProjectID:          conf.Get[string]("worker.google_cloud_project_id"),
		UserProject:        conf.Get[string]("worker.google_user_project"),
		AccessToken:        conf.Get[string]("worker.google_access_token"),
		ServiceAccountPath: conf.Get[string]("worker.google_service_account_json_path"),
		ServiceAccountJSON: conf.Get[string]("worker.google_service_account_json"),
		TokenURL:           conf.Get[string]("worker.google_oauth_token_url"),
		TTSURL:             "https://generativelanguage.googleapis.com/v1beta/models/%s:generateContent",
		TTSModel:           googlecloud.DefaultTTSModel,
		HTTPTimeoutSeconds: firstPositive(conf.Get[int]("worker.ffmpeg_timeout_sec"), 300),
	})
}

func newMFAClient() *mfa.Client {
	return mfa.New(mfa.Config{
		Enabled:               conf.Get[bool]("worker.mfa_enabled"),
		Command:               conf.Get[string]("worker.mfa_command"),
		TemporaryDirectory:    conf.Get[string]("worker.mfa_temporary_directory"),
		Beam:                  conf.Get[int]("worker.mfa_beam"),
		RetryBeam:             conf.Get[int]("worker.mfa_retry_beam"),
		MandarinDictionary:    conf.Get[string]("worker.mfa_zh_dictionary"),
		MandarinAcousticModel: conf.Get[string]("worker.mfa_zh_acoustic_model"),
		MandarinG2PModel:      conf.Get[string]("worker.mfa_zh_g2p_model"),
		JapaneseDictionary:    conf.Get[string]("worker.mfa_ja_dictionary"),
		JapaneseAcousticModel: conf.Get[string]("worker.mfa_ja_acoustic_model"),
		JapaneseG2PModel:      conf.Get[string]("worker.mfa_ja_g2p_model"),
	})
}

func practicalSpeakingRate(language string) float64 {
	if strings.EqualFold(strings.TrimSpace(language), "ja") {
		if value := conf.Get[float64]("worker.google_tts_ja_speaking_rate"); value > 0 {
			return value
		}
		return 0.85
	}
	if value := conf.Get[float64]("worker.google_tts_speaking_rate"); value > 0 {
		return value
	}
	return 1.0
}

func practicalTTSVoiceIDs(language string) (string, string) {
	if strings.EqualFold(strings.TrimSpace(language), "ja") {
		return strings.TrimSpace(conf.Get[string]("worker.google_tts_ja_male_voice_id")),
			strings.TrimSpace(conf.Get[string]("worker.google_tts_ja_female_voice_id"))
	}
	return strings.TrimSpace(conf.Get[string]("worker.google_tts_zh_male_voice_id")),
		strings.TrimSpace(conf.Get[string]("worker.google_tts_zh_female_voice_id"))
}

func practicalNarratorVoiceID() string {
	return strings.TrimSpace(conf.Get[string]("worker.google_tts_narrator_voice_id"))
}

func practicalTempo() float64 {
	value := conf.Get[float64]("worker.practical_tts_tempo")
	if value <= 0 {
		return 0.8
	}
	return value
}

func practicalTurnTempo() float64 {
	return 0.8
}

func practicalBlockTempo() float64 {
	return 0.8
}

func practicalNarratorSpeakingRate(language string) float64 {
	_ = language
	rate := 1.0
	if rate <= 0 {
		return 1.0
	}
	return rate
}

func practicalChapterGapMS() int {
	value := conf.Get[int]("worker.practical_chapter_gap_ms")
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

func RequireLanguageForValidation(value string) (string, error) {
	return requirePracticalLanguage(value)
}

func practicalBlockGapMS() int {
	value := conf.Get[int]("worker.practical_block_gap_ms")
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

func practicalAlignmentUnits(language, text string) []string {
	trimmed := strings.TrimSpace(text)
	if trimmed == "" {
		return nil
	}
	if strings.EqualFold(strings.TrimSpace(language), "ja") {
		if fields := strings.Fields(trimmed); len(fields) > 0 {
			return fields
		}
	}
	return splitPracticalCJKUnits(trimmed)
}

func splitPracticalCJKUnits(text string) []string {
	runes := []rune(text)
	units := make([]string, 0, len(runes))
	for _, r := range runes {
		if unicode.IsSpace(r) {
			continue
		}
		if isPracticalPunctuationRune(r) {
			continue
		}
		units = append(units, string(r))
	}
	if len(units) > 0 {
		return units
	}
	return []string{text}
}

func isPracticalPunctuationRune(r rune) bool {
	return unicode.IsPunct(r) || strings.ContainsRune("，。！？；：“”‘’（）《》、…,.!?;:()[]{}\"'", r)
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

func compactPositiveInts(values []int) []int {
	seen := make(map[int]struct{})
	out := make([]int, 0, len(values))
	for _, value := range values {
		if value <= 0 {
			continue
		}
		if _, exists := seen[value]; exists {
			continue
		}
		seen[value] = struct{}{}
		out = append(out, value)
	}
	return out
}

func compactNonEmptyStrings(values []string) []string {
	out := make([]string, 0, len(values))
	for _, value := range values {
		if trimmed := strings.TrimSpace(value); trimmed != "" {
			out = append(out, trimmed)
		}
	}
	return out
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if trimmed := strings.TrimSpace(value); trimmed != "" {
			return trimmed
		}
	}
	return ""
}
