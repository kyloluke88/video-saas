package podcast_audio_service

import (
	"context"
	"errors"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	conf "worker/pkg/config"
	"worker/pkg/elevenlabs"
	"worker/pkg/googlecloud"
	"worker/pkg/mfa"
	services "worker/services"
	ffmpegcommon "worker/services/media/ffmpeg/common"
	dto "worker/services/podcast/model"
)

type GenerateInput struct {
	ProjectID      string
	Language       string
	TTSType        int
	Seed           int
	BlockNums      []int
	ScriptFilename string
}

type AlignInput struct {
	ProjectID string
	Language  string
	BlockNums []int
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

const googleTTSInputFieldLimitBytes = 4000

const (
	podcastTTSTypeGoogle     = 1
	podcastTTSTypeElevenLabs = 2
)

func Generate(ctx context.Context, input GenerateInput) (GenerateResult, error) {
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
	ttsType := normalizePodcastTTSType(input.TTSType)
	if !podcastEnabled(ttsType) {
		return GenerateResult{}, fmt.Errorf("podcast generation disabled for tts_type=%d (%s)", ttsType, podcastTTSTypeLabel(ttsType))
	}

	projectDir := projectDirFor(input.ProjectID)
	if err := os.MkdirAll(projectDir, 0o755); err != nil {
		return GenerateResult{}, err
	}
	switch ttsType {
	case podcastTTSTypeGoogle:
		if err := GenerateGoogleAudio(ctx, input); err != nil {
			return GenerateResult{}, err
		}
		return AlignGoogle(ctx, AlignInput{
			ProjectID: input.ProjectID,
			Language:  language,
			BlockNums: input.BlockNums,
		})
	case podcastTTSTypeElevenLabs:
		artifacts, err := prepareAudioArtifacts(projectDir)
		if err != nil {
			return GenerateResult{}, err
		}
		script, err := loadScriptForGeneration(projectDir, language, input.ScriptFilename)
		if err != nil {
			return GenerateResult{}, err
		}
		requestedBlocks, err := buildRequestedBlockSet(input.BlockNums, len(script.Blocks))
		if err != nil {
			return GenerateResult{}, err
		}
		blockGapMS := blockGapMS()
		if blockGapMS > 0 {
			if err := createSilenceAudio(ctx, artifacts.blockGapPath, blockGapMS); err != nil {
				return GenerateResult{}, err
			}
		}
		elevenClient, err := newElevenLabsSpeechClient()
		if err != nil {
			return GenerateResult{}, err
		}
		results, err := synthesizeWithElevenLabs(
			ctx,
			elevenClient,
			input.ProjectID,
			language,
			projectDir,
			artifacts,
			script,
			blockGapMS,
			input.Seed,
			requestedBlocks,
		)
		if err != nil {
			return GenerateResult{}, err
		}
		for i := range results {
			if i >= len(script.Blocks) {
				break
			}
			script.Blocks[i] = results[i].AlignedBlock
		}

		finalScript, concatPaths, _, err := assembleDialogue(script, results, artifacts.blockGapPath, blockGapMS)
		if err != nil {
			return GenerateResult{}, err
		}
		if err := concatAudioFiles(ctx, projectDir, concatPaths, artifacts.dialoguePath); err != nil {
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
	default:
		return GenerateResult{}, fmt.Errorf("unsupported podcast tts_type=%d", ttsType)
	}
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
		return dto.PodcastScript{}, markScriptLoadNonRetryable(scriptPath, err)
	}
	if err := validateScriptLanguage(script.Language, language); err != nil {
		return dto.PodcastScript{}, err
	}
	script.Language = language
	script = sanitizeScriptTokens(script)
	if isJapaneseLanguage(language) {
		if err := validateJapaneseScriptInput(script); err != nil {
			return dto.PodcastScript{}, markScriptInputNonRetryable(err)
		}
	} else {
		if err := validateChineseScriptInput(script); err != nil {
			return dto.PodcastScript{}, markScriptInputNonRetryable(err)
		}
	}
	script.RefreshSegmentsFromBlocks()
	return normalizeScriptForSpeech(script), nil
}

func markScriptLoadNonRetryable(scriptPath string, err error) error {
	if err == nil {
		return nil
	}
	if os.IsNotExist(err) {
		return services.NonRetryableError{
			Err: fmt.Errorf("script file not found: %s", strings.TrimSpace(scriptPath)),
		}
	}
	return err
}

func markScriptInputNonRetryable(err error) error {
	if err == nil {
		return nil
	}
	var nonRetryable services.NonRetryableError
	if errors.As(err, &nonRetryable) {
		return err
	}
	return services.NonRetryableError{Err: err}
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

func validateGoogleBlocks(language string, blocks []dto.PodcastBlock) error {
	promptBytes := len([]byte(strings.TrimSpace(buildGeminiBlockPrompt(language))))
	if promptBytes > googleTTSInputFieldLimitBytes {
		return services.NonRetryableError{
			Err: fmt.Errorf(
				"tts prompt exceeds google 4000-byte limit: prompt_bytes=%d limit=%d",
				promptBytes,
				googleTTSInputFieldLimitBytes,
			),
		}
	}
	for blockIndex, block := range blocks {
		for segIndex, seg := range block.Segments {
			text := strings.TrimSpace(spokenTextForGoogleSynthesis(language, seg))
			if text == "" {
				continue
			}
			textBytes := len([]byte(text))
			if textBytes <= googleTTSInputFieldLimitBytes {
				continue
			}
			segID := strings.TrimSpace(seg.SegmentID)
			if segID == "" {
				segID = fmt.Sprintf("segment_%03d", segIndex+1)
			}
			return services.NonRetryableError{
				Err: fmt.Errorf(
					"segment text exceeds google 4000-byte limit: block=%s block_index=%d segment=%s segment_index=%d text_bytes=%d limit=%d",
					strings.TrimSpace(block.BlockID),
					blockIndex+1,
					segID,
					segIndex+1,
					textBytes,
					googleTTSInputFieldLimitBytes,
				),
			}
		}
	}
	return nil
}

func estimateConversationBytes(request googlecloud.SynthesizeConversationRequest) int {
	total := 0
	for _, turn := range request.Turns {
		total += len([]byte(strings.TrimSpace(turn.Text)))
	}
	return total
}

func tryReuseCachedBlock(ctx context.Context, aligner *blockAligner, language string, artifacts audioArtifacts, index int, block dto.PodcastBlock) (blockSynthesisResult, bool, error) {
	blockID := strings.TrimSpace(block.BlockID)
	for _, candidate := range reusableBlockAudioCandidates(artifacts, index, blockID) {
		state, stateOK, err := loadBlockCheckpoint(candidate.stateDir, index, blockID)
		if err != nil {
			return blockSynthesisResult{}, false, err
		}
		if !stateOK || canReuseCachedBlockAudio(podcastTTSTypeGoogle, language, block, state.Block) {
			audioPath := candidate.audioPath
			if candidate.copyToProject {
				audioPath, err = ensureProjectBlockAudio(artifacts, index, blockID, candidate.audioPath)
				if err != nil {
					return blockSynthesisResult{}, false, err
				}
			}
			durationMS := state.DurationMS
			if durationMS <= 0 {
				durationMS, err = audioDurationMS(candidate.audioPath)
				if err != nil {
					return blockSynthesisResult{}, false, err
				}
			}
			alignedBlock, err := aligner.AlignBlock(ctx, language, block, audioPath, durationMS)
			if err != nil {
				return blockSynthesisResult{}, false, err
			}
			if err := persistBlockCheckpoint(artifacts.blockStatesDir, index, alignedBlock, durationMS); err != nil {
				return blockSynthesisResult{}, false, err
			}
			log.Printf("♻️ podcast block reuse cached tts block=%s source=%s audio=%s duration_ms=%d", blockID, candidate.audioPath, audioPath, durationMS)
			return blockSynthesisResult{
				AudioPath:    audioPath,
				DurationMS:   durationMS,
				AlignedBlock: alignedBlock,
			}, true, nil
		}
		log.Printf("🔁 cached tts audio ignored block=%s reason=script_changed source=%s", blockID, candidate.audioPath)
	}
	return blockSynthesisResult{}, false, nil
}

func tryReuseCompletedBlockWithoutMFA(
	ttsType int,
	providerLabel string,
	language string,
	artifacts audioArtifacts,
	index int,
	block dto.PodcastBlock,
) (blockSynthesisResult, bool, error) {
	blockID := strings.TrimSpace(block.BlockID)
	for _, candidate := range reusableBlockAudioCandidates(artifacts, index, blockID) {
		state, stateOK, err := loadBlockCheckpoint(candidate.stateDir, index, blockID)
		if err != nil {
			return blockSynthesisResult{}, false, err
		}
		if !stateOK || !blockCheckpointComplete(language, state, candidate.audioPath) {
			continue
		}
		if !canReuseCachedBlockAudio(ttsType, language, block, state.Block) {
			log.Printf("🔁 cached tts audio ignored block=%s reason=script_changed source=%s", blockID, candidate.audioPath)
			continue
		}

		audioPath := candidate.audioPath
		if candidate.copyToProject {
			audioPath, err = ensureProjectBlockAudio(artifacts, index, blockID, candidate.audioPath)
			if err != nil {
				return blockSynthesisResult{}, false, err
			}
		}
		durationMS := state.DurationMS
		if durationMS <= 0 {
			durationMS, err = audioDurationMS(audioPath)
			if err != nil {
				return blockSynthesisResult{}, false, err
			}
		}
		if err := persistBlockCheckpoint(artifacts.blockStatesDir, index, state.Block, durationMS); err != nil {
			return blockSynthesisResult{}, false, err
		}
		log.Printf("♻️ podcast block reuse cached tts provider=%s block=%s source=%s audio=%s duration_ms=%d",
			providerLabel, blockID, candidate.audioPath, audioPath, durationMS)
		return blockSynthesisResult{
			AudioPath:    audioPath,
			DurationMS:   durationMS,
			AlignedBlock: state.Block,
		}, true, nil
	}
	return blockSynthesisResult{}, false, nil
}

func buildRequestedBlockSet(blockNums []int, totalBlocks int) (map[int]struct{}, error) {
	if len(blockNums) == 0 {
		return nil, nil
	}
	selected := make(map[int]struct{}, len(blockNums))
	for _, value := range blockNums {
		if value <= 0 || value > totalBlocks {
			return nil, services.NonRetryableError{
				Err: fmt.Errorf("block_nums out of range: block_num=%d total_blocks=%d", value, totalBlocks),
			}
		}
		selected[value-1] = struct{}{}
	}
	if len(selected) == 0 {
		return nil, nil
	}
	return selected, nil
}

func hasRequestedBlocks(requested map[int]struct{}) bool {
	return len(requested) > 0
}

func isRequestedBlock(requested map[int]struct{}, index int) bool {
	if len(requested) == 0 {
		return false
	}
	_, ok := requested[index]
	return ok
}

func canReuseCachedBlockAudio(ttsType int, language string, current, cached dto.PodcastBlock) bool {
	if len(current.Segments) != len(cached.Segments) {
		return false
	}
	for i := range current.Segments {
		currentSeg := current.Segments[i]
		cachedSeg := cached.Segments[i]
		if defaultSpeaker(currentSeg.Speaker) != defaultSpeaker(cachedSeg.Speaker) {
			return false
		}
		if strings.TrimSpace(synthesisTextForProvider(ttsType, language, currentSeg)) != strings.TrimSpace(synthesisTextForProvider(ttsType, language, cachedSeg)) {
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
	maleVoiceID, femaleVoiceID := googleVoiceIDs(language)
	turns := make([]googlecloud.ConversationTurn, 0, len(block.Segments))
	for _, seg := range block.Segments {
		text := spokenTextForGoogleSynthesis(language, seg)
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
		MaleVoiceID:   maleVoiceID,
		FemaleVoiceID: femaleVoiceID,
		SpeakingRate:  ttsSpeakingRate(language),
	}
}

func buildGeminiBlockPrompt(language string) string {
	base := strings.TrimSpace(geminiJapaneseBasePrompt())
	if !isJapaneseLanguage(language) {
		base = strings.TrimSpace(geminiChineseBasePrompt())
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
	prompt := base
	maxBytes := ttsPromptMaxBytes()
	for _, part := range extras {
		next := strings.TrimSpace(prompt + " " + part)
		if maxBytes > 0 && len([]byte(next)) > maxBytes {
			log.Printf("⚠️ google tts prompt append truncated lang=%s max_bytes=%d", language, maxBytes)
			break
		}
		prompt = next
	}
	if maxBytes > 0 && len([]byte(prompt)) > maxBytes {
		return truncateUTF8ByBytes(prompt, maxBytes)
	}
	return prompt
}

func geminiChineseBasePrompt() string {
	return `Two-speaker Mandarin Chinese learning podcast. The speakers are longtime close friends having a relaxed, lightly humorous conversation. Male voice: calm, steady, clear everyday Mandarin. Female voice: bright but natural, clear everyday Mandarin. Keep each speaker's voice identity stable and consistent throughout the entire block. Do not let the same speaker's timbre, age impression, or vocal placement drift mid-block. Keep the delivery natural, easy to follow, and learner-friendly, with clear sentence boundaries and natural pauses. Allow light warmth, light humor, and natural emotional movement when the text supports it. Natural laughter is allowed and should sound relaxed, human, and spontaneous. If the text contains laughter markers such as 哈哈, 呵呵, [笑], or (笑), render a short natural laugh or amused chuckle instead of reading the markers literally.`
}

func geminiJapaneseBasePrompt() string {
	return `Two-speaker Japanese learning podcast. The speakers are longtime close friends having a relaxed, lightly humorous conversation. Male voice: calm, steady, clear everyday Japanese. Female voice: bright but natural, clear everyday Japanese. Keep each speaker's voice identity stable and consistent throughout the entire block. Do not let the same speaker's timbre, age impression, or vocal placement drift mid-block. Keep the delivery natural, easy to follow, and learner-friendly, with clear sentence boundaries and natural pauses. Allow light warmth, light humor, and natural emotional movement when the text supports it. Natural laughter is allowed and should sound relaxed, human, and spontaneous. If the text contains laughter markers such as はは, ふふ, [笑], (笑), or w, render a short natural laugh or amused chuckle instead of reading the markers literally.`
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

func ttsPromptMaxBytes() int {
	return firstPositive(conf.Get[int]("worker.google_tts_prompt_max_bytes"), 1200)
}

func truncateUTF8ByBytes(text string, maxBytes int) string {
	if maxBytes <= 0 {
		return ""
	}
	text = strings.TrimSpace(text)
	if len([]byte(text)) <= maxBytes {
		return text
	}
	runes := []rune(text)
	for len(runes) > 0 {
		runes = runes[:len(runes)-1]
		candidate := strings.TrimSpace(string(runes))
		if len([]byte(candidate)) <= maxBytes {
			return candidate
		}
	}
	return ""
}

func synthesisTextForProvider(ttsType int, language string, seg dto.PodcastSegment) string {
	if normalizePodcastTTSType(ttsType) == podcastTTSTypeElevenLabs {
		return spokenTextForElevenSynthesis(language, seg)
	}
	return spokenTextForGoogleSynthesis(language, seg)
}

func spokenTextForGoogleSynthesis(language string, seg dto.PodcastSegment) string {
	if isJapaneseLanguage(language) {
		return stripLegacySpeechTags(japaneseTTSText(seg))
	}
	return strings.TrimSpace(seg.Text)
}

func spokenTextForElevenSynthesis(language string, seg dto.PodcastSegment) string {
	if text := strings.TrimSpace(seg.SpeechText); text != "" {
		return text
	}
	if isJapaneseLanguage(language) {
		return strings.TrimSpace(japaneseTTSText(seg))
	}
	return strings.TrimSpace(seg.Text)
}

var legacySpeechTagPattern = regexp.MustCompile(`\[[^\]]+\]`)
var elevenEmotionTagPattern = regexp.MustCompile(`[\[\(（【][^\]\)）】]{1,48}[\]\)）】]`)

func stripLegacySpeechTags(text string) string {
	text = legacySpeechTagPattern.ReplaceAllString(text, "")
	return strings.Join(strings.Fields(strings.TrimSpace(text)), " ")
}

func stripElevenEmotionTags(text string) string {
	text = elevenEmotionTagPattern.ReplaceAllString(text, "")
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
		HTTPTimeoutSeconds: firstPositive(conf.Get[int]("worker.ffmpeg_timeout_sec"), 300),
	})
}

func googleVoiceIDs(language string) (string, string) {
	if isJapaneseLanguage(language) {
		return strings.TrimSpace(conf.Get[string]("worker.google_tts_ja_male_voice_id")),
			strings.TrimSpace(conf.Get[string]("worker.google_tts_ja_female_voice_id"))
	}
	return strings.TrimSpace(conf.Get[string]("worker.google_tts_zh_male_voice_id")),
		strings.TrimSpace(conf.Get[string]("worker.google_tts_zh_female_voice_id"))
}

func newElevenLabsSpeechClient() (*elevenlabs.Client, error) {
	client, err := elevenlabs.New(elevenlabs.Config{
		BaseURL:            conf.Get[string]("worker.elevenlabs_base_url"),
		DialoguePath:       conf.Get[string]("worker.elevenlabs_dialogue_path"),
		APIKey:             conf.Get[string]("worker.elevenlabs_api_key"),
		ModelID:            conf.Get[string]("worker.elevenlabs_tts_model"),
		OutputFormat:       conf.Get[string]("worker.elevenlabs_output_format"),
		HTTPTimeoutSeconds: firstPositive(conf.Get[int]("worker.ffmpeg_timeout_sec"), 300),
	})
	if err != nil {
		return nil, services.NonRetryableError{
			Err: fmt.Errorf("elevenlabs client init failed: %w", err),
		}
	}
	return client, nil
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
		"-c:a", "libmp3lame",
		"-q:a", "4",
		path,
	)
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
	return ffmpegcommon.RunFFmpegContext(
		ctx,
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

func podcastEnabled(ttsType int) bool {
	switch ttsType {
	case podcastTTSTypeElevenLabs:
		return conf.Get[bool]("worker.elevenlabs_tts_enabled")
	default:
		return conf.Get[bool]("worker.google_tts_enabled")
	}
}

func normalizePodcastTTSType(value int) int {
	if value == podcastTTSTypeElevenLabs {
		return podcastTTSTypeElevenLabs
	}
	return podcastTTSTypeGoogle
}

func podcastTTSTypeLabel(value int) string {
	switch normalizePodcastTTSType(value) {
	case podcastTTSTypeElevenLabs:
		return "elevenlabs"
	default:
		return "google"
	}
}

func blockGapMS() int {
	return conf.Get[int]("worker.podcast_block_gap_ms")
}
