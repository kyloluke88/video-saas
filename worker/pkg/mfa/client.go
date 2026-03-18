package mfa

import (
	"context"
	"encoding/csv"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"

	services "worker/services"
)

type Config struct {
	Enabled            bool
	Command            string
	TemporaryDirectory string

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
	defer os.RemoveAll(runDir)

	corpusDir := filepath.Join(runDir, "corpus")
	outputDir := filepath.Join(runDir, "output")
	tempDir := firstNonEmpty(strings.TrimSpace(c.cfg.TemporaryDirectory), filepath.Join(runDir, "tmp"))
	if err := os.MkdirAll(corpusDir, 0o755); err != nil {
		return nil, err
	}
	if err := os.MkdirAll(outputDir, 0o755); err != nil {
		return nil, err
	}
	if err := os.MkdirAll(tempDir, 0o755); err != nil {
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
		"-q",
	}
	if strings.TrimSpace(g2pModel) != "" {
		args = append(args, "--g2p_model_path", g2pModel)
	}

	cmd := exec.CommandContext(ctx, commandPath, args...)
	raw, err := cmd.CombinedOutput()
	if err != nil {
		return nil, services.NonRetryableError{Err: fmt.Errorf("mfa align failed: %w output=%s", err, strings.TrimSpace(string(raw)))}
	}

	csvPath, err := firstCSVFile(outputDir)
	if err != nil {
		return nil, services.NonRetryableError{Err: err}
	}
	words, err := parseCSVWordTimings(csvPath)
	if err != nil {
		return nil, err
	}
	return words, nil
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

func firstCSVFile(root string) (string, error) {
	var found string
	err := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			return nil
		}
		if strings.EqualFold(filepath.Ext(path), ".csv") {
			found = path
			return filepath.SkipAll
		}
		return nil
	})
	if err != nil && err != filepath.SkipAll {
		return "", err
	}
	if strings.TrimSpace(found) == "" {
		return "", fmt.Errorf("mfa output csv not found in %s", root)
	}
	return found, nil
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
