package mfa

import (
	"context"
	"encoding/csv"
	"encoding/json"
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
	Verbose            bool
	Debug              bool
	UsePostgres        bool

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
		tempDir    string
	)
	for attemptIndex, attempt := range attempts {
		outputDir = filepath.Join(runDir, fmt.Sprintf("output_attempt_%d", attemptIndex+1))
		tempDir = firstNonEmpty(strings.TrimSpace(c.cfg.TemporaryDirectory), filepath.Join(runDir, fmt.Sprintf("tmp_attempt_%d", attemptIndex+1)))
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
		if c.cfg.Verbose {
			args = append(args, "--verbose")
		}
		if c.cfg.Debug {
			args = append(args, "--debug")
		}
		if c.cfg.UsePostgres {
			args = append(args, "--use_postgres")
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
		recoverErrMsg := ""
		if shouldAttemptArchiveRecovery(lastOutput) {
			if words, recoverErr := extractWordTimingsFromAlignmentArchive(ctx, commandPath, filepath.Join(tempDir, "corpus")); recoverErr == nil && len(words) > 0 {
				return words, nil
			} else if recoverErr != nil {
				recoverErrMsg = fmt.Sprintf(" archive_recovery_error=%v", recoverErr)
			}
		}
		keepRunDir = true
		debugPath, writeErr := preserveFailureOutput(runDir, lastOutput)
		if writeErr != nil {
			return nil, services.NonRetryableError{Err: fmt.Errorf("mfa align failed: %w output=%s debug_write_error=%v%s", lastErr, lastOutput, writeErr, recoverErrMsg)}
		}
		return nil, services.NonRetryableError{Err: fmt.Errorf("mfa align failed: %w output=%s debug_dir=%s%s", lastErr, lastOutput, debugPath, recoverErrMsg)}
	}

	csvPath, err := firstAlignmentCSVFile(outputDir)
	if err != nil {
		return nil, services.NonRetryableError{Err: err}
	}
	words, err := parseCSVWordTimings(csvPath)
	if err != nil {
		return nil, err
	}
	if len(words) == 0 {
		if recovered, recoverErr := extractWordTimingsFromAlignmentArchive(ctx, commandPath, filepath.Join(tempDir, "corpus")); recoverErr == nil && len(recovered) > 0 {
			return recovered, nil
		}
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

func shouldAttemptArchiveRecovery(output string) bool {
	value := strings.ToLower(strings.TrimSpace(output))
	return strings.Contains(value, "word_interval_temp") ||
		strings.Contains(value, "phone_interval_temp") ||
		strings.Contains(value, "collecting phone and word alignments")
}

type archiveWordTiming struct {
	Text    string `json:"text"`
	StartMS int    `json:"start_ms"`
	EndMS   int    `json:"end_ms"`
}

func extractWordTimingsFromAlignmentArchive(ctx context.Context, mfaCommandPath, corpusRoot string) ([]WordTiming, error) {
	if strings.TrimSpace(corpusRoot) == "" {
		return nil, fmt.Errorf("mfa archive recovery corpus root is required")
	}
	pythonPath, err := resolveMFAPythonPath(mfaCommandPath)
	if err != nil {
		return nil, err
	}

	scriptFile, err := os.CreateTemp("", "mfa_extract_*.py")
	if err != nil {
		return nil, err
	}
	scriptPath := scriptFile.Name()
	if err := scriptFile.Close(); err != nil {
		_ = os.Remove(scriptPath)
		return nil, err
	}
	defer os.Remove(scriptPath)

	if err := os.WriteFile(scriptPath, []byte(mfaArchiveExtractionScript), 0o644); err != nil {
		return nil, err
	}

	cmd := exec.CommandContext(ctx, pythonPath, scriptPath, corpusRoot)
	raw, err := cmd.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("mfa archive recovery failed: %w output=%s", err, strings.TrimSpace(string(raw)))
	}

	var recovered []archiveWordTiming
	if err := json.Unmarshal(raw, &recovered); err != nil {
		return nil, fmt.Errorf("mfa archive recovery parse failed: %w output=%s", err, strings.TrimSpace(string(raw)))
	}
	words := make([]WordTiming, 0, len(recovered))
	for _, item := range recovered {
		text := strings.TrimSpace(item.Text)
		if text == "" {
			continue
		}
		startMS := item.StartMS
		endMS := item.EndMS
		if endMS <= startMS {
			endMS = startMS + 1
		}
		words = append(words, WordTiming{
			Text:    text,
			StartMS: startMS,
			EndMS:   endMS,
		})
	}
	return words, nil
}

func resolveMFAPythonPath(mfaCommandPath string) (string, error) {
	candidates := make([]string, 0, 4)
	dir := filepath.Dir(strings.TrimSpace(mfaCommandPath))
	if dir != "." && dir != "" {
		candidates = append(candidates,
			filepath.Join(dir, "python"),
			filepath.Join(dir, "python3"),
		)
	}
	candidates = append(candidates, "python3", "python")

	for _, candidate := range candidates {
		if strings.TrimSpace(candidate) == "" {
			continue
		}
		if filepath.IsAbs(candidate) {
			if info, err := os.Stat(candidate); err == nil && !info.IsDir() {
				return candidate, nil
			}
			continue
		}
		path, err := exec.LookPath(candidate)
		if err == nil {
			return path, nil
		}
	}
	return "", fmt.Errorf("python executable for MFA archive recovery not found")
}

const mfaArchiveExtractionScript = `
import json
import sys
from pathlib import Path

from kalpy.gmm.data import AlignmentArchive
from kalpy.gmm.utils import read_transition_model
from montreal_forced_aligner.db import Utterance
from montreal_forced_aligner.db import Dictionary as MFADictionary
from sqlalchemy import create_engine
from sqlalchemy.orm import sessionmaker


def pick_file(root: Path, patterns):
    for pattern in patterns:
        matches = sorted(root.glob(pattern))
        for match in matches:
            if match.exists() and match.is_file():
                return match.resolve()
    return None


def main():
    corpus_root = Path(sys.argv[1]).resolve()
    db_path = corpus_root / "corpus.db"
    alignment_dir = corpus_root / "alignment"
    if not db_path.exists():
        raise FileNotFoundError(f"corpus.db not found: {db_path}")
    if not alignment_dir.exists():
        raise FileNotFoundError(f"alignment dir not found: {alignment_dir}")

    ali_path = pick_file(alignment_dir, ["ali.*.ark", "ali_first_pass.*.ark"])
    words_path = pick_file(alignment_dir, ["words.*.ark", "words_first_pass.*.ark"])
    likes_path = pick_file(alignment_dir, ["likelihoods.*.ark", "likelihoods_first_pass.*.ark"])
    model_path = alignment_dir / "final.alimdl"
    if ali_path is None or words_path is None or likes_path is None:
        raise FileNotFoundError("alignment archive files not found")
    if not model_path.exists():
        raise FileNotFoundError(f"alignment model not found: {model_path}")

    engine = create_engine("sqlite:///" + str(db_path))
    Session = sessionmaker(bind=engine)
    session = Session()
    dictionary = session.query(MFADictionary).first()
    if dictionary is None:
        raise RuntimeError("mfa dictionary row not found")

    transition_model = read_transition_model(str(model_path))
    archive = None
    out = []
    try:
        archive = AlignmentArchive(str(ali_path), words_file_name=str(words_path), likelihood_file_name=str(likes_path))
        for alignment in archive:
            intervals = alignment.generate_ctm(transition_model, dictionary.lexicon_compiler.phone_table, 0.01)
            utterance_id = int(str(alignment.utterance_id).split("-")[-1])
            text = session.query(Utterance.normalized_text).filter(Utterance.id == utterance_id).scalar()
            ctm = dictionary.lexicon_compiler.phones_to_pronunciations(
                alignment.words,
                intervals,
                transcription=False,
                text=text,
            )
            for word_interval in ctm.word_intervals:
                label = str(getattr(word_interval, "label", "")).strip()
                if label == "" or label in {"<eps>", "sil", "sp", "spn"}:
                    continue
                start_ms = int(round(float(word_interval.begin) * 1000))
                end_ms = int(round(float(word_interval.end) * 1000))
                if end_ms <= start_ms:
                    end_ms = start_ms + 1
                out.append({
                    "text": label,
                    "start_ms": start_ms,
                    "end_ms": end_ms,
                })
    finally:
        if archive is not None:
            archive.close()
        session.close()

    print(json.dumps(out, ensure_ascii=False))


if __name__ == "__main__":
    main()
`

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
