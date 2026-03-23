package podcast_audio_service

import (
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"worker/internal/dto"
	conf "worker/pkg/config"
	"worker/pkg/googlecloud"
	"worker/pkg/mfa"
	services "worker/services"
	ffmpegcommon "worker/services/ffmpeg_service/common"
)

type GenerateInput struct {
	ProjectID      string
	Language       string
	ScriptFilename string
}

type GenerateResult struct {
	DialogueAudioPath string
	AlignedScriptPath string
	Script            dto.PodcastScript
}

type blockSynthesisResult struct {
	AudioPath    string
	DurationMS   int
	AlignedBlock dto.PodcastBlock
}

func Generate(input GenerateInput) (GenerateResult, error) {
	if strings.TrimSpace(input.ProjectID) == "" {
		return GenerateResult{}, fmt.Errorf("project_id is required")
	}
	language, err := requirePodcastLanguage(input.Language)
	if err != nil {
		return GenerateResult{}, err
	}
	if strings.TrimSpace(input.ScriptFilename) == "" {
		return GenerateResult{}, fmt.Errorf("script_filename is required")
	}
	if !podcastEnabled() {
		return GenerateResult{}, fmt.Errorf("podcast generation disabled")
	}

	projectDir := projectDirFor(input.ProjectID)
	if err := os.MkdirAll(projectDir, 0o755); err != nil {
		return GenerateResult{}, err
	}
	artifacts, err := prepareAudioArtifacts(projectDir)
	if err != nil {
		return GenerateResult{}, err
	}

	script, err := loadScriptForGeneration(projectDir, language, input.ScriptFilename)
	if err != nil {
		return GenerateResult{}, err
	}

	client, err := newGoogleSpeechClient()
	if err != nil {
		return GenerateResult{}, err
	}
	alignClient := newMFAClient()
	aligner := newBlockAligner(alignClient, chunkWorkingDir(projectDir))

	blockGapMS := blockGapMS()
	if blockGapMS > 0 {
		if err := createSilenceAudio(artifacts.blockGapPath, blockGapMS); err != nil {
			return GenerateResult{}, err
		}
	}
	results := make([]blockSynthesisResult, len(script.Blocks))
	for i, block := range script.Blocks {
		result, err := synthesizeOrResumeBlock(context.Background(), client, aligner, language, artifacts, i, block)
		if err != nil {
			return GenerateResult{}, err
		}
		results[i] = result
		script.Blocks[i] = result.AlignedBlock

		partialScript, _, _, err := assembleDialogue(
			dto.PodcastScript{
				Language: script.Language,
				Title:    script.Title,
				YouTube:  script.YouTube,
				Blocks:   append([]dto.PodcastBlock(nil), script.Blocks[:i+1]...),
			},
			results[:i+1],
			artifacts.blockGapPath,
			blockGapMS,
		)
		if err == nil {
			_ = writeJSON(filepath.Join(projectDir, "script_partial.json"), partialScript)
		}
	}

	finalScript, concatPaths, _, err := assembleDialogue(script, results, artifacts.blockGapPath, blockGapMS)
	if err != nil {
		return GenerateResult{}, err
	}
	if err := concatAudioFiles(projectDir, concatPaths, artifacts.dialoguePath); err != nil {
		return GenerateResult{}, err
	}

	alignedScript, err := finalizeAlignedScript(input.ProjectID, artifacts.alignedPath, artifacts.dialoguePath, finalScript)
	if err != nil {
		return GenerateResult{}, err
	}

	return GenerateResult{
		DialogueAudioPath: artifacts.dialoguePath,
		AlignedScriptPath: artifacts.alignedPath,
		Script:            alignedScript,
	}, nil
}

func loadScriptForGeneration(projectDir, language, scriptFilename string) (dto.PodcastScript, error) {
	projectScriptPath := projectScriptInputPath(projectDir)
	if fileExists(projectScriptPath) {
		log.Printf("📘 podcast script reuse project_id=%s path=%s", filepath.Base(projectDir), projectScriptPath)
		return loadScriptFromPath(language, projectScriptPath)
	}

	scriptPath := scriptPathFor(scriptFilename)
	script, err := loadScriptFromPath(language, scriptPath)
	if err != nil {
		return dto.PodcastScript{}, err
	}
	if err := writeJSON(projectScriptPath, script); err != nil {
		return dto.PodcastScript{}, err
	}
	log.Printf("📝 podcast script cached project_id=%s source=%s path=%s", filepath.Base(projectDir), scriptPath, projectScriptPath)
	return script, nil
}

func loadScriptFromPath(language, scriptPath string) (dto.PodcastScript, error) {
	var script dto.PodcastScript
	if err := readJSON(scriptPath, &script); err != nil {
		return dto.PodcastScript{}, err
	}
	if err := validateScriptLanguage(script.Language, language); err != nil {
		return dto.PodcastScript{}, err
	}
	script.Language = language
	if isJapaneseLanguage(language) {
		if err := validateJapaneseScriptInput(script); err != nil {
			return dto.PodcastScript{}, err
		}
	} else {
		if err := validateChineseScriptInput(script); err != nil {
			return dto.PodcastScript{}, err
		}
	}
	script.RefreshSegmentsFromBlocks()
	return normalizeScriptForSpeech(script), nil
}

func normalizeScriptForSpeech(script dto.PodcastScript) dto.PodcastScript {
	for i := range script.Blocks {
		for j := range script.Blocks[i].Segments {
			seg := script.Blocks[i].Segments[j]
			seg.StartMS = 0
			seg.EndMS = 0
			for k := range seg.Tokens {
				seg.Tokens[k].StartMS = 0
				seg.Tokens[k].EndMS = 0
			}
			seg.HighlightSpans = nil
			seg.TokenSpans = nil
			script.Blocks[i].Segments[j] = seg
		}
	}
	script.RefreshSegmentsFromBlocks()
	return script
}

func synthesizeOrResumeBlock(ctx context.Context, client *googlecloud.Client, aligner *blockAligner, language string, artifacts audioArtifacts, index int, block dto.PodcastBlock) (blockSynthesisResult, error) {
	blockID := strings.TrimSpace(block.BlockID)
	for _, candidate := range reusableBlockAudioCandidates(artifacts, index, blockID) {
		state, stateOK, err := loadBlockCheckpoint(candidate.stateDir, index, blockID)
		if err != nil {
			return blockSynthesisResult{}, err
		}
		if !stateOK || canReuseCachedBlockAudio(language, block, state.Block) {
			audioPath := candidate.audioPath
			if candidate.copyToProject {
				audioPath, err = ensureProjectBlockAudio(artifacts, index, blockID, candidate.audioPath)
				if err != nil {
					return blockSynthesisResult{}, err
				}
			}
			durationMS := state.DurationMS
			if durationMS <= 0 {
				durationMS, err = audioDurationMS(candidate.audioPath)
				if err != nil {
					return blockSynthesisResult{}, err
				}
			}
			alignedBlock, err := aligner.AlignBlock(ctx, language, block, audioPath, durationMS)
			if err != nil {
				return blockSynthesisResult{}, err
			}
			if err := persistBlockCheckpoint(artifacts.blockStatesDir, index, alignedBlock, durationMS); err != nil {
				return blockSynthesisResult{}, err
			}
			log.Printf("♻️ podcast block reuse cached tts block=%s source=%s audio=%s duration_ms=%d", blockID, candidate.audioPath, audioPath, durationMS)
			return blockSynthesisResult{
				AudioPath:    audioPath,
				DurationMS:   durationMS,
				AlignedBlock: alignedBlock,
			}, nil
		}
		log.Printf("🔁 cached tts audio ignored block=%s reason=script_changed source=%s", blockID, candidate.audioPath)
	}

	// Each block is synthesized independently so a failed run can resume from the
	// last finished block instead of regenerating the whole episode.
	request := buildConversationRequest(language, block)
	log.Printf("🎙️ gemini block synth start block=%s turns=%d", blockID, len(request.Turns))
	result, err := client.SynthesizeConversation(ctx, request)
	if err != nil {
		return blockSynthesisResult{}, err
	}
	blockAudioPath := unitAudioPath(artifacts.blocksDir, index, blockID, result.Ext)
	if err := os.WriteFile(blockAudioPath, result.Audio, 0o644); err != nil {
		return blockSynthesisResult{}, err
	}

	durationMS, err := audioDurationMS(blockAudioPath)
	if err != nil {
		return blockSynthesisResult{}, err
	}

	alignedBlock, err := aligner.AlignBlock(ctx, language, block, blockAudioPath, durationMS)
	if err != nil {
		return blockSynthesisResult{}, err
	}
	if err := persistBlockCheckpoint(artifacts.blockStatesDir, index, alignedBlock, durationMS); err != nil {
		return blockSynthesisResult{}, err
	}
	log.Printf("✅ gemini block synth done block=%s audio=%s duration_ms=%d", blockID, blockAudioPath, durationMS)
	return blockSynthesisResult{
		AudioPath:    blockAudioPath,
		DurationMS:   durationMS,
		AlignedBlock: alignedBlock,
	}, nil
}

func canReuseCachedBlockAudio(language string, current, cached dto.PodcastBlock) bool {
	if len(current.Segments) != len(cached.Segments) {
		return false
	}
	for i := range current.Segments {
		currentSeg := current.Segments[i]
		cachedSeg := cached.Segments[i]
		if defaultSpeaker(currentSeg.Speaker) != defaultSpeaker(cachedSeg.Speaker) {
			return false
		}
		if strings.TrimSpace(spokenTextForSynthesis(language, currentSeg)) != strings.TrimSpace(spokenTextForSynthesis(language, cachedSeg)) {
			return false
		}
	}
	return true
}

// assembleDialogue is the single place where relative block timings become absolute
// dialogue timings. This keeps resume logic simple: block checkpoints stay local to
// each block, and final absolute timing is rebuilt every run.
func assembleDialogue(base dto.PodcastScript, results []blockSynthesisResult, gapPath string, gapMS int) (dto.PodcastScript, []string, int, error) {
	script := base
	script.Blocks = make([]dto.PodcastBlock, len(base.Blocks))
	concatPaths := make([]string, 0, len(results)*2)
	cursorMS := 0

	for i, result := range results {
		if strings.TrimSpace(result.AudioPath) == "" {
			return dto.PodcastScript{}, nil, 0, fmt.Errorf("block audio missing at index %d", i)
		}
		block := clonePodcastBlock(result.AlignedBlock)
		shiftBlockTiming(&block, cursorMS)
		script.Blocks[i] = block
		concatPaths = append(concatPaths, result.AudioPath)
		cursorMS += result.DurationMS
		if i < len(results)-1 && gapMS > 0 && strings.TrimSpace(gapPath) != "" {
			concatPaths = append(concatPaths, gapPath)
			cursorMS += gapMS
		}
	}
	script.RefreshSegmentsFromBlocks()
	return script, concatPaths, cursorMS, nil
}

func clonePodcastBlock(block dto.PodcastBlock) dto.PodcastBlock {
	out := block
	if len(block.Segments) == 0 {
		return out
	}
	out.Segments = make([]dto.PodcastSegment, len(block.Segments))
	for i, seg := range block.Segments {
		out.Segments[i] = clonePodcastSegment(seg)
	}
	return out
}

func clonePodcastSegment(seg dto.PodcastSegment) dto.PodcastSegment {
	out := seg
	if len(seg.Tokens) > 0 {
		out.Tokens = make([]dto.PodcastToken, len(seg.Tokens))
		copy(out.Tokens, seg.Tokens)
	}
	if len(seg.TokenSpans) > 0 {
		out.TokenSpans = make([]dto.PodcastTokenSpan, len(seg.TokenSpans))
		copy(out.TokenSpans, seg.TokenSpans)
	}
	if len(seg.HighlightSpans) > 0 {
		out.HighlightSpans = make([]dto.PodcastHighlightSpan, len(seg.HighlightSpans))
		copy(out.HighlightSpans, seg.HighlightSpans)
	}
	return out
}

func shiftBlockTiming(block *dto.PodcastBlock, offsetMS int) {
	if block == nil || offsetMS == 0 {
		return
	}
	for i := range block.Segments {
		shiftSegmentTiming(&block.Segments[i], offsetMS)
	}
}

func shiftSegmentTiming(seg *dto.PodcastSegment, offsetMS int) {
	if seg == nil || offsetMS == 0 {
		return
	}
	seg.StartMS += offsetMS
	seg.EndMS += offsetMS
	for i := range seg.Tokens {
		seg.Tokens[i].StartMS += offsetMS
		seg.Tokens[i].EndMS += offsetMS
	}
	for i := range seg.HighlightSpans {
		seg.HighlightSpans[i].StartMS += offsetMS
		seg.HighlightSpans[i].EndMS += offsetMS
	}
}

func buildConversationRequest(language string, block dto.PodcastBlock) googlecloud.SynthesizeConversationRequest {
	turns := make([]googlecloud.ConversationTurn, 0, len(block.Segments))
	for _, seg := range block.Segments {
		text := spokenTextForSynthesis(language, seg)
		if strings.TrimSpace(text) == "" {
			continue
		}
		turns = append(turns, googlecloud.ConversationTurn{
			Speaker: defaultSpeaker(seg.Speaker),
			Text:    text,
		})
	}
	return googlecloud.SynthesizeConversationRequest{
		LanguageCode:  language,
		Prompt:        buildGeminiBlockPrompt(language),
		Turns:         turns,
		MaleVoiceID:   conf.Get[string]("worker.google_tts_male_voice_id"),
		FemaleVoiceID: conf.Get[string]("worker.google_tts_female_voice_id"),
		SpeakingRate:  ttsSpeakingRate(language),
	}
}

func buildGeminiBlockPrompt(language string) string {
	// Keep the prompt focused on stable voice traits and avoid storyline/context
	// framing that a stateless TTS request cannot truly carry across blocks.
	var base string
	if isJapaneseLanguage(language) {
		base = strings.TrimSpace(
			"Generate a natural two-speaker Japanese learning podcast dialogue. Use stable voice characterization and keep the overall delivery consistent. Male speaker: late-20s to early-30s, calm, grounded, low-to-mid pitch, steady warmth, clear explanatory delivery, natural everyday Japanese, never announcer-like, never theatrical, never overly energetic. Female speaker: mid-to-late-20s, bright but natural, mid pitch, curious and responsive, friendly warmth, everyday spoken Japanese, never childish, never anime-like, never overly cute, never exaggerated. Keep both voices relaxed, emotionally controlled, and easy to follow. Allow subtle warmth, light conversational responsiveness, and small emotional shading when the text supports it, but keep the same voice identity and overall energy level stable. Naturalness must come from timing and clarity, not from stronger acting. Avoid dramatic laughs, audible sighs, breathy performance, exaggerated surprise, performative interjections, or large style shifts.")
	} else {
		base = strings.TrimSpace(
			"Generate a natural two-speaker Mandarin Chinese learning podcast dialogue. Use stable voice characterization and keep the overall delivery consistent. Male speaker: late-20s to early-30s, calm, grounded, low-to-mid pitch, warm, steady, clear explanatory delivery, everyday spoken Mandarin, never news-anchor-like, never presenter-like, never theatrical. Female speaker: mid-to-late-20s, bright but natural, mid pitch, curious and responsive, warm and conversational, everyday spoken Mandarin, never childish, never overly cute, never exaggerated. Keep both voices relaxed, emotionally controlled, and easy to follow. Allow subtle natural warmth, light conversational reactions, and small emotional shading when the text supports it, but keep the same voice identity and overall energy level stable. Naturalness must come from timing, phrasing, and clarity, not from changing character. Avoid dramatic laughs, sighs, gasps, breathy delivery, exaggerated surprise, playful overacting, or large style shifts.")
	}
	appendParts := []string{strings.TrimSpace(conf.Get[string]("worker.google_tts_prompt_append"))}
	if isJapaneseLanguage(language) {
		appendParts = append(appendParts, strings.TrimSpace(conf.Get[string]("worker.google_tts_ja_prompt_append")))
	} else {
		appendParts = append(appendParts, strings.TrimSpace(conf.Get[string]("worker.google_tts_zh_prompt_append")))
	}

	var extras []string
	for _, part := range appendParts {
		if part == "" {
			continue
		}
		extras = append(extras, part)
	}
	if len(extras) == 0 {
		return base
	}
	// Env-configured prompt additions are appended verbatim so we can tune
	// delivery without touching code or disturbing the default base direction.
	return strings.TrimSpace(base + " " + strings.Join(extras, " "))
}

func ttsSpeakingRate(language string) float64 {
	base := conf.Get[float64]("worker.google_tts_speaking_rate")
	if isJapaneseLanguage(language) {
		if value := conf.Get[float64]("worker.google_tts_ja_speaking_rate"); value > 0 {
			return value
		}
	}
	return base
}

func spokenTextForSynthesis(language string, seg dto.PodcastSegment) string {
	if isJapaneseLanguage(language) {
		return stripLegacySpeechTags(japaneseTTSText(seg))
	}
	return strings.TrimSpace(seg.Text)
}

var legacySpeechTagPattern = regexp.MustCompile(`\[[^\]]+\]`)

func stripLegacySpeechTags(text string) string {
	text = legacySpeechTagPattern.ReplaceAllString(text, "")
	return strings.Join(strings.Fields(strings.TrimSpace(text)), " ")
}

func newGoogleSpeechClient() (*googlecloud.Client, error) {
	return googlecloud.New(googlecloud.Config{
		ProjectID:          conf.Get[string]("worker.google_cloud_project_id"),
		UserProject:        conf.Get[string]("worker.google_user_project"),
		AccessToken:        conf.Get[string]("worker.google_access_token"),
		ServiceAccountPath: conf.Get[string]("worker.google_service_account_json_path"),
		ServiceAccountJSON: conf.Get[string]("worker.google_service_account_json"),
		TokenURL:           conf.Get[string]("worker.google_oauth_token_url"),
		TTSURL:             conf.Get[string]("worker.google_tts_url"),
		TTSModel:           conf.Get[string]("worker.google_tts_model"),
		TTSAudioEncoding:   conf.Get[string]("worker.google_tts_audio_encoding"),
		TTSSampleRateHz:    conf.Get[int]("worker.google_tts_sample_rate_hz"),
		MaleVoiceID:        conf.Get[string]("worker.google_tts_male_voice_id"),
		FemaleVoiceID:      conf.Get[string]("worker.google_tts_female_voice_id"),
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

func existingBlockAudioPath(dir string, index int, blockID string) (string, bool) {
	pattern := filepath.Join(dir, fmt.Sprintf("%03d_%s.*", index+1, sanitizeSegmentID(blockID)))
	matches, err := filepath.Glob(pattern)
	if err != nil || len(matches) == 0 {
		return "", false
	}
	return matches[0], true
}

type reusableBlockAudio struct {
	audioPath     string
	stateDir      string
	copyToProject bool
}

func reusableBlockAudioCandidates(artifacts audioArtifacts, index int, blockID string) []reusableBlockAudio {
	candidates := make([]reusableBlockAudio, 0, 2)
	if audioPath, ok := existingBlockAudioPath(artifacts.blocksDir, index, blockID); ok {
		candidates = append(candidates, reusableBlockAudio{
			audioPath: audioPath,
			stateDir:  artifacts.blockStatesDir,
		})
	}
	if artifacts.reuseBlocksDir != "" && filepath.Clean(artifacts.reuseBlocksDir) != filepath.Clean(artifacts.blocksDir) {
		if audioPath, ok := existingBlockAudioPath(artifacts.reuseBlocksDir, index, blockID); ok {
			stateDir := artifacts.reuseStatesDir
			if strings.TrimSpace(stateDir) == "" {
				stateDir = artifacts.blockStatesDir
			}
			candidates = append(candidates, reusableBlockAudio{
				audioPath:     audioPath,
				stateDir:      stateDir,
				copyToProject: true,
			})
		}
	}
	return candidates
}

func ensureProjectBlockAudio(artifacts audioArtifacts, index int, blockID, sourceAudioPath string) (string, error) {
	targetAudioPath := unitAudioPath(artifacts.blocksDir, index, blockID, filepath.Ext(sourceAudioPath))
	if filepath.Clean(sourceAudioPath) == filepath.Clean(targetAudioPath) {
		return targetAudioPath, nil
	}
	if fileExists(targetAudioPath) {
		return targetAudioPath, nil
	}
	if err := copyFile(sourceAudioPath, targetAudioPath); err != nil {
		return "", fmt.Errorf("copy cached block audio failed: %w", err)
	}
	return targetAudioPath, nil
}

func requirePodcastLanguage(value string) (string, error) {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "zh":
		return "zh", nil
	case "ja":
		return "ja", nil
	default:
		return "", fmt.Errorf("lang must be zh or ja")
	}
}

func validateScriptLanguage(scriptLanguage, payloadLanguage string) error {
	scriptLang, err := requirePodcastLanguage(scriptLanguage)
	if err != nil {
		return services.NonRetryableError{Err: fmt.Errorf("script language mismatch: script=%q payload=%q", strings.TrimSpace(scriptLanguage), payloadLanguage)}
	}
	if scriptLang != payloadLanguage {
		return services.NonRetryableError{Err: fmt.Errorf("script language mismatch: script=%q payload=%q", scriptLang, payloadLanguage)}
	}
	return nil
}

func createSilenceAudio(path string, durationMs int) error {
	if durationMs <= 0 {
		return nil
	}
	return ffmpegcommon.RunFFmpeg(
		"-y",
		"-f", "lavfi",
		"-i", "anullsrc=r=24000:cl=mono",
		"-t", fmt.Sprintf("%.3f", float64(durationMs)/1000.0),
		"-c:a", "libmp3lame",
		"-q:a", "4",
		path,
	)
}

func concatAudioFiles(projectDir string, files []string, outputPath string) error {
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
	return ffmpegcommon.RunFFmpeg(
		"-y",
		"-f", "concat",
		"-safe", "0",
		"-i", listPath,
		"-c:a", "libmp3lame",
		"-q:a", "2",
		outputPath,
	)
}

func isJapaneseLanguage(language string) bool {
	switch strings.ToLower(strings.TrimSpace(language)) {
	case "ja", "ja-jp":
		return true
	default:
		return false
	}
}

func normalizeLanguage(language string) string {
	switch strings.TrimSpace(strings.ToLower(language)) {
	case "zh":
		return "zh-CN"
	default:
		return language
	}
}

func podcastEnabled() bool {
	return conf.Get[bool]("worker.google_tts_enabled")
}

func blockGapMS() int {
	return conf.Get[int]("worker.podcast_block_gap_ms")
}
