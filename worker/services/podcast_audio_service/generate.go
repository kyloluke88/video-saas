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

type blockBatch struct {
	BatchIndex int
	Start      int
	End        int
}

type blockTimingWindow struct {
	StartMS int
	EndMS   int
}

const googleTTSInputFieldLimitBytes = 4000

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
	targetBatchCount := podcastTTSBatchCount()
	batches := buildBalancedBlockBatches(len(script.Blocks), targetBatchCount)
	if err := enforceBatchingHardLimit(language, script.Blocks, batches, targetBatchCount); err != nil {
		return GenerateResult{}, err
	}
	log.Printf("🎛️ podcast tts batching target_batches=%d actual_batches=%d blocks=%d",
		targetBatchCount,
		len(batches),
		len(script.Blocks),
	)
	totalBatches := len(batches)
	for _, batch := range batches {
		estimatedBytes := estimateConversationBytesForBlocks(language, script.Blocks[batch.Start:batch.End])
		mergedBlock := mergeBlocksForSynthesis(script.Blocks[batch.Start:batch.End], fmt.Sprintf("batch_%03d", batch.BatchIndex+1))
		request := buildConversationRequest(language, mergedBlock)
		log.Printf("🎛️ podcast tts batch start batch=%d/%d blocks=%d range=%03d-%03d turns=%d text_bytes=%d",
			batch.BatchIndex+1,
			totalBatches,
			batch.End-batch.Start,
			batch.Start+1,
			batch.End,
			len(request.Turns),
			estimatedBytes,
		)
		batchResults, err := synthesizeOrResumeBatch(
			context.Background(),
			client,
			aligner,
			language,
			artifacts,
			script.Blocks[batch.Start:batch.End],
			batch,
			totalBatches,
		)
		if err != nil {
			return GenerateResult{}, err
		}
		for localIdx, result := range batchResults {
			globalIdx := batch.Start + localIdx
			results[globalIdx] = result
			script.Blocks[globalIdx] = result.AlignedBlock

			partialScript, _, _, err := assembleDialogue(
				dto.PodcastScript{
					Language: script.Language,
					Title:    script.Title,
					YouTube:  script.YouTube,
					Blocks:   append([]dto.PodcastBlock(nil), script.Blocks[:globalIdx+1]...),
				},
				results[:globalIdx+1],
				artifacts.blockGapPath,
				blockGapMS,
			)
			if err == nil {
				_ = writeJSON(filepath.Join(projectDir, "script_partial.json"), partialScript)
			}
		}
		lastResult := batchResults[len(batchResults)-1]
		log.Printf("✅ podcast tts batch done batch=%d/%d blocks=%d range=%03d-%03d audio=%s duration_ms=%d",
			batch.BatchIndex+1,
			totalBatches,
			batch.End-batch.Start,
			batch.Start+1,
			batch.End,
			lastResult.AudioPath,
			lastResult.DurationMS,
		)
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

func buildBalancedBlockBatches(totalBlocks int, batchCount int) []blockBatch {
	if totalBlocks <= 0 {
		return nil
	}
	if batchCount <= 0 {
		batchCount = 1
	}
	if batchCount > totalBlocks {
		batchCount = totalBlocks
	}

	base := totalBlocks / batchCount
	remainder := totalBlocks % batchCount
	out := make([]blockBatch, 0, batchCount)
	start := 0
	for i := 0; i < batchCount; i++ {
		size := base
		if i < remainder {
			size++
		}
		if size <= 0 {
			continue
		}
		end := start + size
		out = append(out, blockBatch{
			BatchIndex: i,
			Start:      start,
			End:        end,
		})
		start = end
	}
	return out
}

func enforceBatchingHardLimit(
	language string,
	blocks []dto.PodcastBlock,
	batches []blockBatch,
	maxBatches int,
) error {
	if len(batches) == 0 {
		return services.NonRetryableError{
			Err: fmt.Errorf("tts batching produced no batches"),
		}
	}
	if maxBatches > 0 && len(batches) > maxBatches {
		return services.NonRetryableError{
			Err: fmt.Errorf(
				"tts batching exceeded max batches: actual=%d max=%d; please shorten script json",
				len(batches),
				maxBatches,
			),
		}
	}
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
	for _, batch := range batches {
		if batch.Start < 0 || batch.End > len(blocks) || batch.Start >= batch.End {
			return services.NonRetryableError{
				Err: fmt.Errorf("invalid tts batch range: start=%d end=%d blocks=%d", batch.Start, batch.End, len(blocks)),
			}
		}
	}
	for blockIndex, block := range blocks {
		for segIndex, seg := range block.Segments {
			text := strings.TrimSpace(spokenTextForSynthesis(language, seg))
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

func estimateConversationBytesForBlocks(language string, blocks []dto.PodcastBlock) int {
	merged := mergeBlocksForSynthesis(blocks, "estimate")
	request := buildConversationRequest(language, merged)
	total := 0
	for _, turn := range request.Turns {
		total += len([]byte(strings.TrimSpace(turn.Text)))
	}
	return total
}

func mergeBlocksForSynthesis(blocks []dto.PodcastBlock, syntheticID string) dto.PodcastBlock {
	merged := dto.PodcastBlock{BlockID: syntheticID}
	for _, block := range blocks {
		for _, seg := range block.Segments {
			merged.Segments = append(merged.Segments, clonePodcastSegment(seg))
		}
	}
	return merged
}

func buildBatchTimingWindows(bounds []blockTimingWindow, totalDurationMS int) []blockTimingWindow {
	if len(bounds) == 0 {
		return nil
	}
	if totalDurationMS <= 0 {
		totalDurationMS = bounds[len(bounds)-1].EndMS
	}
	if totalDurationMS <= 0 {
		totalDurationMS = len(bounds)
	}

	windows := make([]blockTimingWindow, len(bounds))
	prevBoundary := 0
	for i := range bounds {
		start := prevBoundary
		var end int
		if i == len(bounds)-1 {
			end = totalDurationMS
		} else {
			leftEnd := maxInt(bounds[i].StartMS+1, bounds[i].EndMS)
			rightStart := maxInt(leftEnd+1, bounds[i+1].StartMS)
			end = (leftEnd + rightStart) / 2
			if end <= start {
				end = start + 1
			}
		}
		windows[i] = blockTimingWindow{
			StartMS: start,
			EndMS:   end,
		}
		prevBoundary = end
	}
	if windows[len(windows)-1].EndMS != totalDurationMS {
		windows[len(windows)-1].EndMS = totalDurationMS
	}
	for i := range windows {
		if windows[i].EndMS <= windows[i].StartMS {
			windows[i].EndMS = windows[i].StartMS + 1
		}
	}
	return windows
}

func extractAudioRange(sourcePath string, startMS, endMS int, outputPath string) error {
	startMS = maxInt(0, startMS)
	endMS = maxInt(startMS+1, endMS)
	startSec := fmt.Sprintf("%.3f", float64(startMS)/1000.0)
	endSec := fmt.Sprintf("%.3f", float64(endMS)/1000.0)
	return ffmpegcommon.RunFFmpeg(
		"-y",
		"-i", sourcePath,
		"-ss", startSec,
		"-to", endSec,
		"-c:a", "libmp3lame",
		"-q:a", "2",
		outputPath,
	)
}

func synthesizeOrResumeBatch(
	ctx context.Context,
	client *googlecloud.Client,
	aligner *blockAligner,
	language string,
	artifacts audioArtifacts,
	blocks []dto.PodcastBlock,
	batch blockBatch,
	totalBatches int,
) ([]blockSynthesisResult, error) {
	if len(blocks) == 0 {
		return nil, nil
	}

	results := make([]blockSynthesisResult, len(blocks))
	allReused := true
	for i, block := range blocks {
		globalIndex := batch.Start + i
		reused, ok, err := tryReuseCachedBlock(ctx, aligner, language, artifacts, globalIndex, block)
		if err != nil {
			return nil, err
		}
		if !ok {
			allReused = false
			break
		}
		results[i] = reused
	}
	if allReused {
		return results, nil
	}

	mergedBlock := mergeBlocksForSynthesis(blocks, fmt.Sprintf("batch_%03d", batch.BatchIndex+1))
	type blockSegmentRange struct {
		blockIndex int
		startSeg   int
		endSeg     int
	}
	ranges := make([]blockSegmentRange, len(blocks))
	cursorSeg := 0
	for i, block := range blocks {
		startSeg := cursorSeg
		endSeg := startSeg + len(block.Segments)
		ranges[i] = blockSegmentRange{
			blockIndex: batch.Start + i,
			startSeg:   startSeg,
			endSeg:     endSeg,
		}
		cursorSeg = endSeg
	}
	if len(mergedBlock.Segments) == 0 {
		return nil, fmt.Errorf("tts batch has no segments batch=%d", batch.BatchIndex+1)
	}
	request := buildConversationRequest(language, mergedBlock)
	ttsResult, err := client.SynthesizeConversation(ctx, request)
	if err != nil {
		return nil, err
	}
	batchExt := strings.TrimPrefix(strings.TrimSpace(ttsResult.Ext), ".")
	if batchExt == "" {
		batchExt = "mp3"
	}

	batchAudioPath := filepath.Join(
		artifacts.blocksDir,
		fmt.Sprintf("batch_%03d_%03d_%03d.%s", batch.BatchIndex+1, batch.Start+1, batch.End, batchExt),
	)
	if err := os.WriteFile(batchAudioPath, ttsResult.Audio, 0o644); err != nil {
		return nil, err
	}

	batchDurationMS, err := audioDurationMS(batchAudioPath)
	if err != nil {
		return nil, err
	}

	alignedBatch, err := aligner.AlignBlock(ctx, language, mergedBlock, batchAudioPath, batchDurationMS)
	if err != nil {
		return nil, err
	}
	if len(alignedBatch.Segments) != len(mergedBlock.Segments) {
		return nil, fmt.Errorf("aligned batch segment count mismatch batch=%d expected=%d got=%d", batch.BatchIndex+1, len(mergedBlock.Segments), len(alignedBatch.Segments))
	}

	bounds := make([]blockTimingWindow, len(ranges))
	for i, r := range ranges {
		if r.endSeg <= r.startSeg {
			return nil, fmt.Errorf("batch=%d block_index=%d has no segments", batch.BatchIndex+1, r.blockIndex)
		}
		startMS := alignedBatch.Segments[r.startSeg].StartMS
		endMS := alignedBatch.Segments[r.endSeg-1].EndMS
		if endMS <= startMS {
			return nil, fmt.Errorf("batch=%d block_index=%d has invalid segment timing start_ms=%d end_ms=%d", batch.BatchIndex+1, r.blockIndex, startMS, endMS)
		}
		bounds[i] = blockTimingWindow{StartMS: startMS, EndMS: endMS}
	}
	windows := buildBatchTimingWindows(bounds, batchDurationMS)

	for i, block := range blocks {
		globalIndex := batch.Start + i
		window := windows[i]
		blockDurationMS := window.EndMS - window.StartMS
		if blockDurationMS <= 0 {
			return nil, fmt.Errorf("batch=%d block=%s invalid window start_ms=%d end_ms=%d", batch.BatchIndex+1, block.BlockID, window.StartMS, window.EndMS)
		}

		blockAudioPath := unitAudioPath(artifacts.blocksDir, globalIndex, block.BlockID, "mp3")
		if err := extractAudioRange(batchAudioPath, window.StartMS, window.EndMS, blockAudioPath); err != nil {
			return nil, fmt.Errorf("extract block audio failed block=%s: %w", block.BlockID, err)
		}

		alignedBlock, err := aligner.AlignBlock(ctx, language, block, blockAudioPath, blockDurationMS)
		if err != nil {
			return nil, err
		}

		if err := persistBlockCheckpoint(artifacts.blockStatesDir, globalIndex, alignedBlock, blockDurationMS); err != nil {
			return nil, err
		}
		results[i] = blockSynthesisResult{
			AudioPath:    blockAudioPath,
			DurationMS:   blockDurationMS,
			AlignedBlock: alignedBlock,
		}
	}
	_ = os.Remove(batchAudioPath)
	return results, nil
}

func tryReuseCachedBlock(ctx context.Context, aligner *blockAligner, language string, artifacts audioArtifacts, index int, block dto.PodcastBlock) (blockSynthesisResult, bool, error) {
	blockID := strings.TrimSpace(block.BlockID)
	for _, candidate := range reusableBlockAudioCandidates(artifacts, index, blockID) {
		state, stateOK, err := loadBlockCheckpoint(candidate.stateDir, index, blockID)
		if err != nil {
			return blockSynthesisResult{}, false, err
		}
		if !stateOK || canReuseCachedBlockAudio(language, block, state.Block) {
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
	// Keep the base prompt language-symmetric; only the language keyword differs.
	languageLabel := "Mandarin Chinese"
	if isJapaneseLanguage(language) {
		languageLabel = "Japanese"
	}
	base := strings.TrimSpace(fmt.Sprintf(
		"Generate a natural two-speaker %s learning podcast dialogue. Use stable voice characterization and keep the overall delivery consistent. The two speakers are longtime close friends who often chat in a relaxed, easy atmosphere, with light humor, occasional self-deprecating jokes, and natural conversational laughter. Male speaker: late-20s to early-30s, calm, grounded, low-to-mid pitch, warm and steady, clear explanatory delivery, everyday spoken %s, never announcer-like, never presenter-like, never theatrical. Female speaker: mid-to-late-20s, bright but natural, mid pitch, curious and responsive, warm and conversational, everyday spoken %s, never childish, never overly cute, never exaggerated. Keep both voices relaxed, emotionally controlled, and easy to follow. Allow subtle natural warmth, light banter, and small emotional shading when the text supports it, while preserving the same voice identity and overall energy level. Naturalness must come from timing, phrasing, and clarity, not from changing character. Avoid exaggerated or theatrical laughs, sighs, gasps, breathy delivery, exaggerated surprise, performative interjections, playful overacting, or large style shifts. When explicit laughter markers appear (for example 哈哈, 呵呵, はは, ふふ, [笑], (笑), w), render a brief natural chuckle rather than reading the laughter characters literally.",
		languageLabel,
		languageLabel,
		languageLabel,
	))
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

func podcastTTSBatchCount() int {
	return firstPositive(conf.Get[int]("worker.podcast_tts_batch_count"), 4)
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
