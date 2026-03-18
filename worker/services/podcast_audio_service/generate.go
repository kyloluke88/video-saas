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
	ContentProfile string
	IsDirect       bool
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
	contentProfile := normalizeContentProfile(input.ContentProfile)
	if !isJapaneseLanguage(language) && contentProfile == "" {
		return GenerateResult{}, fmt.Errorf("content_profile must be daily, social_issue, or international")
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

	if input.IsDirect {
		reusable, err := loadDirectProjectArtifacts(input.ScriptFilename, artifacts.dialoguePath, artifacts.alignedPath)
		if err != nil {
			return GenerateResult{}, err
		}
		if err := validateScriptLanguage(reusable.Script.Language, language); err != nil {
			return GenerateResult{}, err
		}
		reusable.Script.Language = language
		finalScript, err := finalizeAlignedScript(input.ProjectID, artifacts.alignedPath, artifacts.dialoguePath, reusable.Script, contentProfile)
		if err != nil {
			return GenerateResult{}, err
		}
		reusable.Script = finalScript
		return *reusable, nil
	}

	script, err := loadScriptForGeneration(language, input.ScriptFilename)
	if err != nil {
		return GenerateResult{}, err
	}
	if err := writeJSON(filepath.Join(projectDir, "script_input.json"), script); err != nil {
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
	templateGapMSValue := templateGapMS()
	if templateGapMSValue > 0 {
		if err := createSilenceAudio(artifacts.templateGapPath, templateGapMSValue); err != nil {
			return GenerateResult{}, err
		}
	}

	results := make([]blockSynthesisResult, len(script.Blocks))
	for i, block := range script.Blocks {
		result, err := synthesizeOrResumeBlock(context.Background(), client, aligner, language, contentProfile, artifacts, i, block)
		if err != nil {
			return GenerateResult{}, err
		}
		results[i] = result
		script.Blocks[i] = result.AlignedBlock

		partialScript, _, _, err := assembleDialogue(
			dto.PodcastScript{
				Language:              script.Language,
				AudienceLanguage:      script.AudienceLanguage,
				DifficultyLevel:       script.DifficultyLevel,
				TargetDurationMinutes: script.TargetDurationMinutes,
				Title:                 script.Title,
				YouTube:               script.YouTube,
				Blocks:                append([]dto.PodcastBlock(nil), script.Blocks[:i+1]...),
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

	alignedScript, err := finalizeAlignedScript(input.ProjectID, artifacts.alignedPath, artifacts.dialoguePath, finalScript, contentProfile)
	if err != nil {
		return GenerateResult{}, err
	}

	return GenerateResult{
		DialogueAudioPath: artifacts.dialoguePath,
		AlignedScriptPath: artifacts.alignedPath,
		Script:            alignedScript,
	}, nil
}

func loadScriptForGeneration(language, scriptFilename string) (dto.PodcastScript, error) {
	scriptPath := scriptPathFor(scriptFilename)
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
			seg.TokenSpans = nil
			script.Blocks[i].Segments[j] = seg
		}
	}
	script.RefreshSegmentsFromBlocks()
	return script
}

func synthesizeOrResumeBlock(ctx context.Context, client *googlecloud.Client, aligner *blockAligner, language, contentProfile string, artifacts audioArtifacts, index int, block dto.PodcastBlock) (blockSynthesisResult, error) {
	audioPath, ok := existingBlockAudioPath(artifacts.blocksDir, index, block.TTSBlockID)
	if ok {
		if state, ok, err := loadBlockCheckpoint(artifacts.blockStatesDir, index, block.TTSBlockID); err == nil && ok && blockCheckpointComplete(language, state, audioPath) {
			log.Printf("♻️ podcast block resume block=%s audio=%s duration_ms=%d", block.TTSBlockID, audioPath, state.DurationMS)
			return blockSynthesisResult{
				AudioPath:    audioPath,
				DurationMS:   state.DurationMS,
				AlignedBlock: state.Block,
			}, nil
		} else if err != nil {
			return blockSynthesisResult{}, err
		}
	}

	// Each block is synthesized independently so a failed run can resume from the
	// last finished block instead of regenerating the whole episode.
	request := buildConversationRequest(language, contentProfile, block)
	log.Printf("🎙️ gemini block synth start block=%s turns=%d", block.TTSBlockID, len(request.Turns))
	result, err := client.SynthesizeConversation(ctx, request)
	if err != nil {
		return blockSynthesisResult{}, err
	}
	blockAudioPath := unitAudioPath(artifacts.blocksDir, index, block.TTSBlockID, result.Ext)
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
	log.Printf("✅ gemini block synth done block=%s audio=%s duration_ms=%d", block.TTSBlockID, blockAudioPath, durationMS)
	return blockSynthesisResult{
		AudioPath:    blockAudioPath,
		DurationMS:   durationMS,
		AlignedBlock: alignedBlock,
	}, nil
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
		block := result.AlignedBlock
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
}

func buildConversationRequest(language, contentProfile string, block dto.PodcastBlock) googlecloud.SynthesizeConversationRequest {
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
		Prompt:        buildGeminiBlockPrompt(language, contentProfile, block),
		Turns:         turns,
		MaleVoiceID:   conf.Get[string]("worker.google_tts_male_voice_id"),
		FemaleVoiceID: conf.Get[string]("worker.google_tts_female_voice_id"),
	}
}

func buildGeminiBlockPrompt(language, contentProfile string, block dto.PodcastBlock) string {
	// Gemini responds best when we keep the direction focused on relationship,
	// delivery, and emotional color instead of repeating structural rules.
	var base string
	if isJapaneseLanguage(language) {
		base = strings.TrimSpace(fmt.Sprintf(
			"Create a natural Japanese two-friend podcast block. They are longtime close friends, relaxed and unguarded, never stiff or broadcaster-like. Male speaker is steady, explanatory, and easy to follow. Female speaker reacts quickly, asks natural follow-up questions, and sounds emotionally present. Use warm everyday spoken Japanese that feels like real close-friend conversation, not textbook dialogue. When the text implies it, allow small natural laughs, sighs, surprise, hesitation, soft overlap, and playful reactions. Keep the pacing comfortable, human, and nuanced. Block purpose: %s. Content profile: %s.",
			strings.TrimSpace(block.Purpose),
			strings.TrimSpace(contentProfile),
		))
	} else {
		base = strings.TrimSpace(fmt.Sprintf(
			"Create a natural Mandarin Chinese two-friend podcast block. They are longtime close friends, relaxed and unguarded, never stiff or presenter-like. Male speaker leads with clear, easy explanations. Female speaker asks stronger follow-up questions and reacts from an everyday-life perspective. Use warm daily spoken Mandarin, not news-anchor or textbook language. Speak a little slower than casual native speed so Chinese learners can follow comfortably, while still sounding natural and human. When the text implies it, allow small natural laughs, sighs, surprise, hesitation, and playful reactions. Keep the pacing comfortable, warm, and nuanced. Block purpose: %s. Content profile: %s.",
			strings.TrimSpace(block.Purpose),
			strings.TrimSpace(contentProfile),
		))
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

func loadDirectProjectArtifacts(sourceProjectName, targetDialoguePath, targetAlignedPath string) (*GenerateResult, error) {
	sourceDir := directProjectSourceDir(sourceProjectName)
	sourceDialoguePath := filepath.Join(sourceDir, "dialogue.mp3")
	sourceAlignedPath := filepath.Join(sourceDir, "script_aligned.json")

	if _, err := os.Stat(sourceDialoguePath); err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("direct project dialogue missing: %s", sourceDialoguePath)
		}
		return nil, err
	}
	if _, err := os.Stat(sourceAlignedPath); err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("direct project aligned script missing: %s", sourceAlignedPath)
		}
		return nil, err
	}
	if err := copyFile(sourceDialoguePath, targetDialoguePath); err != nil {
		return nil, fmt.Errorf("copy direct project dialogue failed: %w", err)
	}
	if err := copyFile(sourceAlignedPath, targetAlignedPath); err != nil {
		return nil, fmt.Errorf("copy direct project aligned script failed: %w", err)
	}

	var script dto.PodcastScript
	if err := readJSON(targetAlignedPath, &script); err != nil {
		return nil, fmt.Errorf("read direct project aligned script failed: %w", err)
	}
	return &GenerateResult{
		DialogueAudioPath: targetDialoguePath,
		AlignedScriptPath: targetAlignedPath,
		Script:            script,
	}, nil
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

func normalizeContentProfile(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "daily":
		return "daily"
	case "social_issue", "international":
		return strings.ToLower(strings.TrimSpace(value))
	default:
		return ""
	}
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

func templateGapMS() int {
	return conf.Get[int]("worker.podcast_template_gap_ms")
}
