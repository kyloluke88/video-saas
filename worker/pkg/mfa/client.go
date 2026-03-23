package mfa

import (
	"context"
	"encoding/csv"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	services "worker/services"
)

type Config struct {
	Enabled            bool
	Command            string
	TemporaryDirectory string
	Beam               int
	RetryBeam          int

	MandarinDictionary    string
	MandarinAcousticModel string
	MandarinG2PModel      string

	JapaneseDictionary    string
	JapaneseAcousticModel string
	JapaneseG2PModel      string
}

type Client struct {
	cfg Config
}

type AlignRequest struct {
	LanguageCode string
	AudioPath    string
	Transcript   string
	WorkingDir   string
}

type WordTiming struct {
	Text    string
	StartMS int
	EndMS   int
}

func New(cfg Config) *Client {
	if strings.TrimSpace(cfg.Command) == "" {
		cfg.Command = "mfa"
	}
	return &Client{cfg: cfg}
}

func (c *Client) Enabled() bool {
	return c != nil && c.cfg.Enabled
}

// AlignWords runs MFA against a single block-sized audio file and returns the
// aligned word intervals from the generated CSV export.
func (c *Client) AlignWords(ctx context.Context, req AlignRequest) ([]WordTiming, error) {
	if !c.Enabled() {
		return nil, nil
	}
	if strings.TrimSpace(req.AudioPath) == "" {
		return nil, services.NonRetryableError{Err: fmt.Errorf("mfa audio path is required")}
	}
	if strings.TrimSpace(req.Transcript) == "" {
		return nil, services.NonRetryableError{Err: fmt.Errorf("mfa transcript is required")}
	}

	commandPath, err := exec.LookPath(strings.TrimSpace(c.cfg.Command))
	if err != nil {
		return nil, services.NonRetryableError{Err: fmt.Errorf("mfa command not found: %w", err)}
	}

	dictionary, acousticModel, g2pModel, err := c.modelsForLanguage(req.LanguageCode)
	if err != nil {
		return nil, services.NonRetryableError{Err: err}
	}

	runDir, err := os.MkdirTemp(firstNonEmpty(strings.TrimSpace(req.WorkingDir), os.TempDir()), "mfa_align_*")
	if err != nil {
		return nil, err
	}
	keepRunDir := false
	defer func() {
		if !keepRunDir {
			_ = os.RemoveAll(runDir)
		}
	}()

	corpusDir := filepath.Join(runDir, "corpus")
	if err := os.MkdirAll(corpusDir, 0o755); err != nil {
		return nil, err
	}

	audioTarget := filepath.Join(corpusDir, "block.wav")
	if err := copyFile(req.AudioPath, audioTarget); err != nil {
		return nil, err
	}
	transcriptTarget := filepath.Join(corpusDir, "block.lab")
	if err := os.WriteFile(transcriptTarget, []byte(strings.TrimSpace(req.Transcript)), 0o644); err != nil {
		return nil, err
	}

	attempts := c.alignAttempts()
	var (
		lastErr    error
		lastOutput string
		outputDir  string
	)
	for attemptIndex, attempt := range attempts {
		outputDir = filepath.Join(runDir, fmt.Sprintf("output_attempt_%d", attemptIndex+1))
		tempDir := firstNonEmpty(strings.TrimSpace(c.cfg.TemporaryDirectory), filepath.Join(runDir, fmt.Sprintf("tmp_attempt_%d", attemptIndex+1)))
		if err := os.MkdirAll(outputDir, 0o755); err != nil {
			return nil, err
		}
		if err := os.MkdirAll(tempDir, 0o755); err != nil {
			return nil, err
		}

		args := []string{
			"align",
			corpusDir,
			dictionary,
			acousticModel,
			outputDir,
			"--output_format", "csv",
			"--clean",
			"--disable_mp",
			"--single_speaker",
			"--temporary_directory", tempDir,
			"--beam", strconv.Itoa(attempt.Beam),
			"--retry_beam", strconv.Itoa(attempt.RetryBeam),
			"-q",
		}
		if strings.TrimSpace(g2pModel) != "" {
			args = append(args, "--g2p_model_path", g2pModel)
		}

		cmd := exec.CommandContext(ctx, commandPath, args...)
		raw, err := cmd.CombinedOutput()
		lastOutput = strings.TrimSpace(string(raw))
		if err == nil {
			lastErr = nil
			break
		}
		lastErr = err
		if !shouldRetryWithWiderBeam(lastOutput) || attemptIndex == len(attempts)-1 {
			break
		}
	}
	if lastErr != nil {
		keepRunDir = true
		debugPath, writeErr := preserveFailureOutput(runDir, lastOutput)
		if writeErr != nil {
			return nil, services.NonRetryableError{Err: fmt.Errorf("mfa align failed: %w output=%s debug_write_error=%v", lastErr, lastOutput, writeErr)}
		}
		return nil, services.NonRetryableError{Err: fmt.Errorf("mfa align failed: %w output=%s debug_dir=%s", lastErr, lastOutput, debugPath)}
	}

	csvPath, err := firstAlignmentCSVFile(outputDir)
	if err != nil {
		return nil, services.NonRetryableError{Err: err}
	}
	words, err := parseCSVWordTimings(csvPath)
	if err != nil {
		return nil, err
	}
	return words, nil
}

type alignAttempt struct {
	Beam      int
	RetryBeam int
}

func (c *Client) alignAttempts() []alignAttempt {
	baseBeam := c.cfg.Beam
	if baseBeam <= 0 {
		baseBeam = 10
	}
	baseRetryBeam := c.cfg.RetryBeam
	if baseRetryBeam <= 0 {
		baseRetryBeam = 40
	}

	attempts := []alignAttempt{
		{Beam: baseBeam, RetryBeam: baseRetryBeam},
	}
	if baseBeam < 100 || baseRetryBeam < 400 {
		attempts = append(attempts, alignAttempt{Beam: 100, RetryBeam: 400})
	}
	return attempts
}

func shouldRetryWithWiderBeam(output string) bool {
	value := strings.ToLower(strings.TrimSpace(output))
	return strings.Contains(value, "noalignmentserror") ||
		strings.Contains(value, "no successful alignments") ||
		strings.Contains(value, "rerunning with a larger beam")
}

func preserveFailureOutput(runDir, output string) (string, error) {
	if err := os.WriteFile(filepath.Join(runDir, "mfa_output.log"), []byte(strings.TrimSpace(output)+"\n"), 0o644); err != nil {
		return "", err
	}
	return runDir, nil
}

func (c *Client) modelsForLanguage(language string) (string, string, string, error) {
	switch strings.ToLower(strings.TrimSpace(language)) {
	case "ja", "ja-jp":
		if strings.TrimSpace(c.cfg.JapaneseDictionary) == "" || strings.TrimSpace(c.cfg.JapaneseAcousticModel) == "" {
			return "", "", "", fmt.Errorf("japanese MFA dictionary/acoustic model is required")
		}
		return strings.TrimSpace(c.cfg.JapaneseDictionary), strings.TrimSpace(c.cfg.JapaneseAcousticModel), strings.TrimSpace(c.cfg.JapaneseG2PModel), nil
	case "zh", "zh-cn":
		if strings.TrimSpace(c.cfg.MandarinDictionary) == "" || strings.TrimSpace(c.cfg.MandarinAcousticModel) == "" {
			return "", "", "", fmt.Errorf("mandarin MFA dictionary/acoustic model is required")
		}
		return strings.TrimSpace(c.cfg.MandarinDictionary), strings.TrimSpace(c.cfg.MandarinAcousticModel), strings.TrimSpace(c.cfg.MandarinG2PModel), nil
	default:
		return "", "", "", fmt.Errorf("unsupported MFA language: %s", language)
	}
}

func firstAlignmentCSVFile(root string) (string, error) {
	candidates := make([]string, 0, 4)
	err := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			return nil
		}
		if strings.EqualFold(filepath.Ext(path), ".csv") {
			candidates = append(candidates, path)
		}
		return nil
	})
	if err != nil {
		return "", err
	}

	sort.Strings(candidates)
	filtered := make([]string, 0, len(candidates))
	for _, candidate := range candidates {
		if isAnalysisCSV(candidate) {
			continue
		}
		filtered = append(filtered, candidate)
	}
	if len(filtered) == 0 {
		filtered = candidates
	}

	for _, candidate := range filtered {
		ok, err := csvHasWordTimingColumns(candidate)
		if err != nil {
			return "", err
		}
		if ok {
			return candidate, nil
		}
	}

	if len(candidates) == 0 {
		return "", fmt.Errorf("mfa output csv not found in %s", root)
	}
	return "", fmt.Errorf("mfa alignment csv with required columns not found in %s (found: %s)", root, strings.Join(candidates, ", "))
}

func isAnalysisCSV(path string) bool {
	name := strings.ToLower(strings.TrimSpace(filepath.Base(path)))
	return strings.Contains(name, "analysis") || strings.Contains(name, "confidence") || strings.Contains(name, "summary")
}

func csvHasWordTimingColumns(path string) (bool, error) {
	file, err := os.Open(path)
	if err != nil {
		return false, err
	}
	defer file.Close()

	reader := csv.NewReader(file)
	reader.FieldsPerRecord = -1
	headerRow, err := reader.Read()
	if err != nil {
		return false, err
	}

	header := make(map[string]int, len(headerRow))
	for i, value := range headerRow {
		header[strings.ToLower(strings.TrimSpace(value))] = i
	}
	return firstHeaderIndex(header, "begin", "start", "start_time") >= 0 &&
		firstHeaderIndex(header, "end", "stop", "end_time") >= 0 &&
		firstHeaderIndex(header, "label", "text", "word") >= 0, nil
}

func parseCSVWordTimings(path string) ([]WordTiming, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	reader := csv.NewReader(file)
	reader.FieldsPerRecord = -1
	rows, err := reader.ReadAll()
	if err != nil {
		return nil, err
	}
	if len(rows) < 2 {
		return nil, nil
	}

	header := make(map[string]int, len(rows[0]))
	for i, value := range rows[0] {
		header[strings.ToLower(strings.TrimSpace(value))] = i
	}

	beginIdx := firstHeaderIndex(header, "begin", "start", "start_time")
	endIdx := firstHeaderIndex(header, "end", "stop", "end_time")
	labelIdx := firstHeaderIndex(header, "label", "text", "word")
	typeIdx := firstHeaderIndex(header, "type", "tier")
	if beginIdx < 0 || endIdx < 0 || labelIdx < 0 {
		return nil, services.NonRetryableError{Err: fmt.Errorf("mfa csv missing required columns: %s", path)}
	}

	words := make([]WordTiming, 0, len(rows)-1)
	for _, row := range rows[1:] {
		if labelIdx >= len(row) || beginIdx >= len(row) || endIdx >= len(row) {
			continue
		}
		if typeIdx >= 0 && typeIdx < len(row) {
			typeValue := strings.ToLower(strings.TrimSpace(row[typeIdx]))
			if typeValue != "" && !strings.Contains(typeValue, "word") {
				continue
			}
		}

		label := strings.TrimSpace(row[labelIdx])
		if label == "" {
			continue
		}
		if isSilenceLabel(label) {
			continue
		}

		startMS, err := parseSecondsToMS(row[beginIdx])
		if err != nil {
			return nil, err
		}
		endMS, err := parseSecondsToMS(row[endIdx])
		if err != nil {
			return nil, err
		}
		if endMS <= startMS {
			endMS = startMS + 1
		}

		words = append(words, WordTiming{
			Text:    label,
			StartMS: startMS,
			EndMS:   endMS,
		})
	}
	return words, nil
}

func firstHeaderIndex(header map[string]int, keys ...string) int {
	for _, key := range keys {
		if index, ok := header[key]; ok {
			return index
		}
	}
	return -1
}

func parseSecondsToMS(value string) (int, error) {
	seconds, err := strconv.ParseFloat(strings.TrimSpace(value), 64)
	if err != nil {
		return 0, err
	}
	return int(seconds * 1000), nil
}

func isSilenceLabel(label string) bool {
	switch strings.ToLower(strings.TrimSpace(label)) {
	case "", "<eps>", "sil", "sp", "spn":
		return true
	default:
		return false
	}
}

func copyFile(src, dst string) error {
	raw, err := os.ReadFile(src)
	if err != nil {
		return err
	}
	return os.WriteFile(dst, raw, 0o644)
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}
