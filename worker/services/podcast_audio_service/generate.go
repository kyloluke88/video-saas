package podcast_audio_service

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"
	"unicode"

	"worker/internal/dto"
	conf "worker/pkg/config"
	"worker/pkg/tts"
	"worker/pkg/tts/elevenlabs"
	services "worker/services"
	ffmpegcommon "worker/services/ffmpeg_service/common"
)

type GenerateInput struct {
	ProjectID       string
	Language        string
	ContentProfile  string
	IsDirect        bool
	ScriptFilename  string
	MaleVoiceType   *int64
	FemaleVoiceType *int64
}

type GenerateResult struct {
	DialogueAudioPath string
	AlignedScriptPath string
	Script            dto.PodcastScript
}

type speakerProfile struct {
	VoiceType        int64
	VoiceID          string
	Speed            float64
	SampleRate       int64
	EmotionCategory  string
	EmotionIntensity int64
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
	log.Printf("🎧 podcast audio generate start project_id=%s script_filename=%s", input.ProjectID, input.ScriptFilename)

	projectDir := projectDirFor(input.ProjectID)
	if err := os.MkdirAll(projectDir, 0o755); err != nil {
		return GenerateResult{}, err
	}
	artifacts, err := prepareAudioArtifacts(projectDir)
	if err != nil {
		return GenerateResult{}, err
	}

	if strings.TrimSpace(input.ScriptFilename) == "" {
		return GenerateResult{}, fmt.Errorf("script_filename is required")
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
		log.Printf("♻️ podcast direct reuse project_id=%s language=%s source_project=%s dialogue=%s script=%s",
			input.ProjectID, language, filepath.Base(strings.TrimSpace(input.ScriptFilename)), reusable.DialogueAudioPath, reusable.AlignedScriptPath)
		reusable.Script = finalScript
		return *reusable, nil
	}

	scriptPath := scriptPathFor(input.ScriptFilename)
	var script dto.PodcastScript
	if err := readJSON(scriptPath, &script); err != nil {
		return GenerateResult{}, err
	}
	if err := validateScriptLanguage(script.Language, language); err != nil {
		return GenerateResult{}, err
	}
	script.Language = language
	if isJapaneseLanguage(language) {
		if err := validateJapaneseScriptInput(script); err != nil {
			return GenerateResult{}, err
		}
		script.RefreshSegmentsFromBlocks()
	} else {
		if err := validateChineseScriptInput(script); err != nil {
			return GenerateResult{}, err
		}
		script.RefreshSegmentsFromBlocks()
	}
	if !podcastEnabledForLanguage(language) {
		return GenerateResult{}, fmt.Errorf("podcast generation disabled for language %s", language)
	}
	log.Printf("📝 podcast script loaded project_id=%s segments=%d language=%s path=%s", input.ProjectID, len(script.Segments), language, scriptPath)
	if err := writeJSON(filepath.Join(projectDir, "script_input.json"), script); err != nil {
		return GenerateResult{}, err
	}
	script = loadResumableScript(script, artifacts.alignedPath)
	script.Language = language

	maleProfile := resolveSpeakerProfile(language, contentProfile, "male", input.MaleVoiceType)
	femaleProfile := resolveSpeakerProfile(language, contentProfile, "female", input.FemaleVoiceType)
	if isJapaneseLanguage(language) {
		log.Printf("🗣️ podcast speaker profiles project_id=%s male_voice_id=%s female_voice_id=%s",
			input.ProjectID, maleProfile.VoiceID, femaleProfile.VoiceID)
	} else {
		log.Printf("🗣️ podcast speaker profiles project_id=%s male_voice=%d female_voice=%d male_voice_id=%s female_voice_id=%s male_speed=%.2f female_speed=%.2f sample_rate=%d",
			input.ProjectID, maleProfile.VoiceType, femaleProfile.VoiceType, maleProfile.VoiceID, femaleProfile.VoiceID, maleProfile.Speed, femaleProfile.Speed, maleProfile.SampleRate)
	}

	gapMs := segmentGapMSForLanguage(language)
	sameSpeakerGapMs := sameSpeakerGapMSForLanguage(language)
	if gapMs > 0 {
		if err := createSilenceAudio(artifacts.silencePath, gapMs); err != nil {
			return GenerateResult{}, err
		}
		log.Printf("⏸️ podcast segment gap prepared project_id=%s gap_ms=%d path=%s", input.ProjectID, gapMs, artifacts.silencePath)
	}
	if sameSpeakerGapMs > 0 && sameSpeakerGapMs != gapMs {
		if err := createSilenceAudio(artifacts.shortSilencePath, sameSpeakerGapMs); err != nil {
			return GenerateResult{}, err
		}
		log.Printf("⏸️ podcast same-speaker gap prepared project_id=%s gap_ms=%d path=%s", input.ProjectID, sameSpeakerGapMs, artifacts.shortSilencePath)
	}

	if isJapaneseLanguage(language) {
		return generateJapaneseDialogue(input.ProjectID, script, artifacts, gapMs, sameSpeakerGapMs, maleProfile, femaleProfile)
	}

	provider, err := newProvider(language)
	if err != nil {
		return GenerateResult{}, err
	}
	adapter := adapterFor(language)

	concatPaths := make([]string, 0, len(script.Segments)*2)
	cursorMs := 0
	for i, seg := range script.Segments {
		text := strings.TrimSpace(adapter.SegmentText(seg))
		if text == "" {
			continue
		}
		seg = adapter.NormalizeSegment(seg)
		if existingPath, ok := existingUnitAudioPath(artifacts.segmentsDir, i, seg.SegmentID, "mp3"); ok && segmentCheckpointComplete(language, seg, existingPath) {
			log.Printf("♻️ segment resume project_id=%s segment_id=%s audio=%s window_ms=%d-%d",
				input.ProjectID, seg.SegmentID, existingPath, seg.StartMS, seg.EndMS)
			nextGapMs, nextGapPath := chineseGapAfterSegment(script.Segments, i, gapMs, sameSpeakerGapMs, artifacts.silencePath, artifacts.shortSilencePath)
			concatPaths = appendConcatPath(concatPaths, existingPath, nextGapMs > 0 && nextGapPath != "", nextGapPath)
			cursorMs = seg.EndMS
			if nextGapMs > 0 {
				cursorMs += nextGapMs
			}
			script.Segments[i] = seg
			continue
		}

		profile := maleProfile
		if strings.EqualFold(strings.TrimSpace(seg.Speaker), "female") {
			profile = femaleProfile
		}
		log.Printf("🔊 segment tts start project_id=%s segment_id=%s speaker=%s chars=%d voice=%d speed=%.2f emotion=%s intensity=%d",
			input.ProjectID, seg.SegmentID, defaultSpeaker(seg.Speaker), adapter.CharacterCount(seg), profile.VoiceType, profile.Speed, profile.EmotionCategory, profile.EmotionIntensity)

		result, synthErr := provider.Synthesize(context.Background(), tts.Request{
			Text:             text,
			Language:         normalizeLanguage(language),
			VoiceType:        int64Ptr(profile.VoiceType),
			VoiceID:          stringPtr(profile.VoiceID),
			Speed:            float64Ptr(profile.Speed),
			SampleRate:       int64Ptr(profile.SampleRate),
			EmotionCategory:  profile.EmotionCategory,
			EmotionIntensity: int64Ptr(profile.EmotionIntensity),
			EnableSubtitle:   boolPtr(true),
		})
		if synthErr != nil {
			return GenerateResult{}, synthErr
		}
		if len(result.Audio) == 0 {
			return GenerateResult{}, fmt.Errorf("tts returned empty audio for segment %s", seg.SegmentID)
		}

		ext := strings.TrimSpace(result.Ext)
		if ext == "" {
			ext = "mp3"
		}
		segmentPath := unitAudioPath(artifacts.segmentsDir, i, seg.SegmentID, ext)
		if err := os.WriteFile(segmentPath, result.Audio, 0o644); err != nil {
			return GenerateResult{}, err
		}
		if len(result.RawResponse) > 0 {
			responsePath := unitAudioPath(artifacts.ttsResponsesDir, i, seg.SegmentID, "json")
			if err := os.WriteFile(responsePath, result.RawResponse, 0o644); err != nil {
				return GenerateResult{}, err
			}
		}
		durationSec, err := ffmpegcommon.AudioDurationSec(segmentPath)
		if err != nil {
			return GenerateResult{}, err
		}
		durationMs := int(durationSec * 1000)
		seg.StartMS = cursorMs
		seg.EndMS = cursorMs + durationMs
		seg = adapter.ApplyAlignment(seg, result.Subtitles)
		matchedTokens, totalTokens := adapter.AlignmentStats(seg)
		log.Printf("✅ segment tts done project_id=%s segment_id=%s audio=%s duration_ms=%d subtitle_marks=%d token_timed=%d/%d window_ms=%d-%d",
			input.ProjectID, seg.SegmentID, segmentPath, durationMs, len(result.Subtitles), matchedTokens, totalTokens, seg.StartMS, seg.EndMS)
		script.Segments[i] = seg
		if err := persistAlignedCheckpoint(artifacts.alignedPath, script); err != nil {
			return GenerateResult{}, err
		}
		cursorMs = seg.EndMS

		nextGapMs, nextGapPath := chineseGapAfterSegment(script.Segments, i, gapMs, sameSpeakerGapMs, artifacts.silencePath, artifacts.shortSilencePath)
		concatPaths = appendConcatPath(concatPaths, segmentPath, nextGapMs > 0 && nextGapPath != "", nextGapPath)
		if nextGapMs > 0 {
			cursorMs += nextGapMs
		}
	}

	if err := concatAudioFiles(projectDir, concatPaths, artifacts.dialoguePath); err != nil {
		return GenerateResult{}, err
	}
	log.Printf("🎼 dialogue audio ready project_id=%s path=%s parts=%d total_ms=%d", input.ProjectID, artifacts.dialoguePath, len(concatPaths), cursorMs)
	alignedScript, err := finalizeAlignedScript(input.ProjectID, artifacts.alignedPath, artifacts.dialoguePath, script, contentProfile)
	if err != nil {
		return GenerateResult{}, err
	}

	return GenerateResult{
		DialogueAudioPath: artifacts.dialoguePath,
		AlignedScriptPath: artifacts.alignedPath,
		Script:            alignedScript,
	}, nil
}

func generateJapaneseDialogue(projectID string, script dto.PodcastScript, artifacts audioArtifacts, gapMs, sameSpeakerGapMs int, maleProfile, femaleProfile speakerProfile) (GenerateResult, error) {
	cfg := providerConfigForLanguage(script.Language)
	provider, err := elevenlabs.New(cfg)
	if err != nil {
		return GenerateResult{}, err
	}
	dialogueStability := conf.Get[float64]("worker.elevenlabs_podcast_dialogue_stability", 0.47)

	blockAudioPaths := make([]string, len(script.Blocks))
	for blockIndex, block := range script.Blocks {
		if len(block.Segments) == 0 {
			continue
		}
		if existingPath, ok := existingUnitAudioPath(artifacts.segmentsDir, blockIndex, block.TTSBlockID, "mp3"); ok && blockCheckpointComplete(block, existingPath) {
			log.Printf("♻️ dialogue block resume project_id=%s block=%s audio=%s window_end=%d",
				projectID, block.TTSBlockID, existingPath, blockEndMS(block))
			blockAudioPaths[blockIndex] = existingPath
			continue
		}

		inputs := make([]elevenlabs.DialogueInput, 0, len(block.Segments))
		for _, seg := range block.Segments {
			text := strings.TrimSpace(japaneseTTSText(seg))
			if text == "" {
				continue
			}
			profile := maleProfile
			if strings.EqualFold(strings.TrimSpace(seg.Speaker), "female") {
				profile = femaleProfile
			}
			inputs = append(inputs, elevenlabs.DialogueInput{
				Text:    text,
				VoiceID: profile.VoiceID,
			})
		}
		if len(inputs) == 0 {
			continue
		}

		log.Printf("🎭 dialogue block start project_id=%s block=%s turns=%d",
			projectID, block.TTSBlockID, len(inputs))

		result, err := provider.SynthesizeDialogue(context.Background(), inputs, normalizeLanguage(script.Language), float64Ptr(dialogueStability))
		if err != nil {
			return GenerateResult{}, err
		}
		if len(result.Audio) == 0 {
			return GenerateResult{}, fmt.Errorf("dialogue api returned empty audio for block %s", block.TTSBlockID)
		}

		ext := strings.TrimSpace(result.Ext)
		if ext == "" {
			ext = "mp3"
		}
		blockPath := unitAudioPath(artifacts.segmentsDir, blockIndex, block.TTSBlockID, ext)
		if err := os.WriteFile(blockPath, result.Audio, 0o644); err != nil {
			return GenerateResult{}, err
		}
		durationSec, err := ffmpegcommon.AudioDurationSec(blockPath)
		if err != nil {
			return GenerateResult{}, err
		}
		durationMs := int(durationSec * 1000)
		assignDialogueBlockTimes(&block, result, 0, durationMs)
		script.Blocks[blockIndex] = block
		blockAudioPaths[blockIndex] = blockPath
		script.RefreshSegmentsFromBlocks()
		if err := persistAlignedCheckpoint(artifacts.alignedPath, script); err != nil {
			return GenerateResult{}, err
		}
		log.Printf("✅ dialogue block done project_id=%s block=%s audio=%s duration_ms=%d timed_segments=%d",
			projectID, block.TTSBlockID, blockPath, durationMs, blockTimedSegments(block))
	}

	concatPaths, totalMS, err := rebuildJapaneseDialogueWithSegmentGaps(script, blockAudioPaths, artifacts, gapMs, sameSpeakerGapMs)
	if err != nil {
		return GenerateResult{}, err
	}
	if err := concatAudioFiles(artifacts.projectDir, concatPaths, artifacts.dialoguePath); err != nil {
		return GenerateResult{}, err
	}
	script.RefreshSegmentsFromBlocks()
	log.Printf("🎼 dialogue audio ready project_id=%s path=%s parts=%d total_ms=%d", projectID, artifacts.dialoguePath, len(concatPaths), totalMS)
	finalScript, err := finalizeAlignedScript(projectID, artifacts.alignedPath, artifacts.dialoguePath, script, "")
	if err != nil {
		return GenerateResult{}, err
	}

	return GenerateResult{
		DialogueAudioPath: artifacts.dialoguePath,
		AlignedScriptPath: artifacts.alignedPath,
		Script:            finalScript,
	}, nil
}

type japaneseSegmentRef struct {
	blockIndex   int
	segmentIndex int
	globalIndex  int
}

func rebuildJapaneseDialogueWithSegmentGaps(script dto.PodcastScript, blockAudioPaths []string, artifacts audioArtifacts, gapMs, sameSpeakerGapMs int) ([]string, int, error) {
	refs := make([]japaneseSegmentRef, 0, len(script.Segments))
	globalIndex := 0
	for blockIndex := range script.Blocks {
		for segmentIndex := range script.Blocks[blockIndex].Segments {
			refs = append(refs, japaneseSegmentRef{
				blockIndex:   blockIndex,
				segmentIndex: segmentIndex,
				globalIndex:  globalIndex,
			})
			globalIndex++
		}
	}

	concatPaths := make([]string, 0, len(refs)*2)
	cursorMS := 0
	for idx, ref := range refs {
		blockAudioPath := ""
		if ref.blockIndex >= 0 && ref.blockIndex < len(blockAudioPaths) {
			blockAudioPath = blockAudioPaths[ref.blockIndex]
		}
		if strings.TrimSpace(blockAudioPath) == "" || !fileExists(blockAudioPath) {
			return nil, 0, fmt.Errorf("dialogue block audio missing for block index %d", ref.blockIndex)
		}

		seg := script.Blocks[ref.blockIndex].Segments[ref.segmentIndex]
		if seg.EndMS <= seg.StartMS {
			return nil, 0, fmt.Errorf("japanese segment timing missing for %s", seg.SegmentID)
		}
		clipPath := unitAudioPath(artifacts.dialogueClipsDir, ref.globalIndex, seg.SegmentID, "mp3")
		if err := extractAudioClip(blockAudioPath, clipPath, seg.StartMS, seg.EndMS); err != nil {
			return nil, 0, err
		}

		shiftSegmentTiming(&seg, cursorMS-seg.StartMS)
		script.Blocks[ref.blockIndex].Segments[ref.segmentIndex] = seg
		concatPaths = append(concatPaths, clipPath)
		cursorMS = seg.EndMS

		if idx == len(refs)-1 {
			continue
		}
		next := script.Blocks[refs[idx+1].blockIndex].Segments[refs[idx+1].segmentIndex]
		nextGapMS, nextGapPath := japaneseGapAfterSegment(seg, next, gapMs, sameSpeakerGapMs, artifacts.silencePath, artifacts.shortSilencePath)
		if nextGapMS > 0 && strings.TrimSpace(nextGapPath) != "" {
			concatPaths = append(concatPaths, nextGapPath)
			cursorMS += nextGapMS
		}
	}

	script.RefreshSegmentsFromBlocks()
	return concatPaths, cursorMS, nil
}

func japaneseGapAfterSegment(current, next dto.PodcastSegment, defaultGapMs, sameSpeakerGapMs int, defaultGapPath, sameSpeakerGapPath string) (int, string) {
	if strings.EqualFold(strings.TrimSpace(current.Speaker), strings.TrimSpace(next.Speaker)) {
		if sameSpeakerGapMs > 0 && strings.TrimSpace(sameSpeakerGapPath) != "" {
			return sameSpeakerGapMs, sameSpeakerGapPath
		}
	}
	if defaultGapMs > 0 && strings.TrimSpace(defaultGapPath) != "" {
		return defaultGapMs, defaultGapPath
	}
	return 0, ""
}

func extractAudioClip(inputPath, outputPath string, startMS, endMS int) error {
	if strings.TrimSpace(inputPath) == "" || strings.TrimSpace(outputPath) == "" {
		return fmt.Errorf("audio clip path is required")
	}
	if endMS <= startMS {
		return fmt.Errorf("invalid audio clip window start=%d end=%d", startMS, endMS)
	}
	startSec := float64(startMS) / 1000.0
	durationSec := float64(endMS-startMS) / 1000.0
	return ffmpegcommon.RunFFmpeg(
		"-y",
		"-i", inputPath,
		"-ss", fmt.Sprintf("%.3f", startSec),
		"-t", fmt.Sprintf("%.3f", durationSec),
		"-c:a", "libmp3lame",
		"-q:a", "2",
		outputPath,
	)
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

func newProvider(language string) (tts.Provider, error) {
	return tts.NewProvider(providerConfigForLanguage(language))
}

func assignDialogueBlockTimes(block *dto.PodcastBlock, result elevenlabs.DialogueResult, blockStartMS, blockDurationMS int) {
	if block == nil || len(block.Segments) == 0 {
		return
	}
	voiceSegments := result.VoiceSegments
	if dialogueVoiceSegmentsUsable(voiceSegments, len(block.Segments)) {
		for i := range block.Segments {
			start := clampMS(blockStartMS+voiceSegments[i].StartTime, blockStartMS, blockStartMS+blockDurationMS)
			end := clampMS(blockStartMS+voiceSegments[i].EndTime, start+1, blockStartMS+blockDurationMS)
			block.Segments[i].StartMS = start
			block.Segments[i].EndMS = end
			block.Segments[i] = assignJapaneseDialogueChars(block.Segments[i], result.NormalizedAlignment, voiceSegments[i], blockStartMS)
			if len(block.Segments[i].Chars) == 0 {
				block.Segments[i] = normalizeJapaneseSegment(block.Segments[i])
			}
		}
		return
	}

	totalWeight := 0
	weights := make([]int, len(block.Segments))
	for i, seg := range block.Segments {
		weight := maxInt(1, len([]rune(japaneseTTSText(seg))))
		weights[i] = weight
		totalWeight += weight
	}
	cursor := blockStartMS
	for i := range block.Segments {
		span := int(float64(blockDurationMS) * float64(weights[i]) / float64(maxInt(totalWeight, 1)))
		if i == len(block.Segments)-1 {
			span = blockStartMS + blockDurationMS - cursor
		}
		if span <= 0 {
			span = 1
		}
		block.Segments[i].StartMS = cursor
		block.Segments[i].EndMS = cursor + span
		block.Segments[i] = normalizeJapaneseSegment(block.Segments[i])
		cursor += span
	}
}

func dialogueVoiceSegmentsUsable(segments []elevenlabs.DialogueVoiceSegment, expected int) bool {
	if len(segments) != expected || expected == 0 {
		return false
	}
	valid := 0
	for idx, seg := range segments {
		if seg.EndTime > seg.StartTime && seg.DialogueInputIndex == idx {
			valid++
		}
	}
	return valid == expected
}

func assignJapaneseDialogueChars(seg dto.PodcastSegment, alignment []tts.Subtitle, voiceSegment elevenlabs.DialogueVoiceSegment, blockStartMS int) dto.PodcastSegment {
	seg.RubySpans = buildRubySpansFromTokens(japaneseDisplayText(seg), seg.RubyTokens)
	seg.RubyTokens = nil

	displayRunes := []rune(japaneseDisplayText(seg))
	if len(displayRunes) == 0 {
		seg.Chars = nil
		return seg
	}
	start := maxInt(0, voiceSegment.CharacterStartIndex)
	end := voiceSegment.CharacterEndIndex
	if end < start {
		end = start
	}
	if end > len(alignment) {
		end = len(alignment)
	}
	charSubs := make([]tts.Subtitle, 0, end-start)
	for i := start; i < end; i++ {
		if i >= 0 && i < len(alignment) {
			charSubs = append(charSubs, alignment[i])
		}
	}
	if len(charSubs) == 0 {
		return seg
	}

	if chars, ok := mapJapaneseDialogueChars(displayRunes, charSubs, seg.StartMS, seg.EndMS, blockStartMS); ok {
		seg.Chars = chars
		return seg
	}

	return normalizeJapaneseSegment(seg)
}

func mapJapaneseDialogueChars(displayRunes []rune, charSubs []tts.Subtitle, segmentStartMS, segmentEndMS, blockStartMS int) ([]dto.PodcastCharToken, bool) {
	if len(displayRunes) == 0 || len(charSubs) == 0 {
		return nil, false
	}
	subRunes := make([]rune, 0, len(charSubs))
	for _, sub := range charSubs {
		runes := []rune(sub.Text)
		if len(runes) != 1 {
			return nil, false
		}
		subRunes = append(subRunes, runes[0])
	}

	mapping := longestCommonSubsequenceMap(displayRunes, subRunes)
	matched := 0
	chars := make([]dto.PodcastCharToken, len(displayRunes))
	for i, r := range displayRunes {
		chars[i] = dto.PodcastCharToken{Index: i, Char: string(r)}
		if mapping[i] < 0 {
			continue
		}
		sub := charSubs[mapping[i]]
		chars[i].StartMS = blockStartMS + sub.BeginTime
		chars[i].EndMS = blockStartMS + maxInt(sub.EndTime, sub.BeginTime+1)
		matched++
	}
	if matched == 0 {
		return nil, false
	}

	fillJapaneseCharTimingGaps(chars, segmentStartMS, segmentEndMS)
	return chars, true
}

func longestCommonSubsequenceMap(displayRunes, subRunes []rune) []int {
	dp := make([][]int, len(displayRunes)+1)
	for i := range dp {
		dp[i] = make([]int, len(subRunes)+1)
	}
	for i := len(displayRunes) - 1; i >= 0; i-- {
		for j := len(subRunes) - 1; j >= 0; j-- {
			if japaneseDialogueRuneEqual(displayRunes[i], subRunes[j]) {
				dp[i][j] = dp[i+1][j+1] + 1
				continue
			}
			if dp[i+1][j] >= dp[i][j+1] {
				dp[i][j] = dp[i+1][j]
			} else {
				dp[i][j] = dp[i][j+1]
			}
		}
	}

	out := make([]int, len(displayRunes))
	for i := range out {
		out[i] = -1
	}
	i, j := 0, 0
	for i < len(displayRunes) && j < len(subRunes) {
		if japaneseDialogueRuneEqual(displayRunes[i], subRunes[j]) {
			out[i] = j
			i++
			j++
			continue
		}
		if dp[i+1][j] >= dp[i][j+1] {
			i++
		} else {
			j++
		}
	}
	return out
}

func japaneseDialogueRuneEqual(a, b rune) bool {
	if a == b {
		return true
	}
	if unicode.IsSpace(a) && unicode.IsSpace(b) {
		return true
	}
	return japaneseDialogueRuneClass(a) != "" && japaneseDialogueRuneClass(a) == japaneseDialogueRuneClass(b)
}

func japaneseDialogueRuneClass(r rune) string {
	switch r {
	case '、', '，', ',', '､':
		return "comma"
	case '。', '．', '.', '｡':
		return "period"
	case '？', '?':
		return "question"
	case '！', '!':
		return "exclaim"
	case 'ー', '−', '-':
		return "dash"
	default:
		return ""
	}
}

func fillJapaneseCharTimingGaps(chars []dto.PodcastCharToken, segmentStartMS, segmentEndMS int) {
	if len(chars) == 0 {
		return
	}
	start := maxInt(0, segmentStartMS)
	end := maxInt(start+1, segmentEndMS)

	for i := 0; i < len(chars); {
		if chars[i].EndMS > chars[i].StartMS {
			i++
			continue
		}
		j := i
		for j < len(chars) && chars[j].EndMS <= chars[j].StartMS {
			j++
		}

		windowStart := start
		if i > 0 && chars[i-1].EndMS > chars[i-1].StartMS {
			windowStart = chars[i-1].EndMS
		}
		windowEnd := end
		if j < len(chars) && chars[j].EndMS > chars[j].StartMS {
			windowEnd = chars[j].StartMS
		}
		if windowEnd <= windowStart {
			windowEnd = windowStart + (j - i)
		}
		step := maxInt(1, (windowEnd-windowStart)/maxInt(1, j-i))
		cursor := windowStart
		for k := i; k < j; k++ {
			chars[k].StartMS = cursor
			if k == j-1 {
				chars[k].EndMS = maxInt(cursor+1, windowEnd)
			} else {
				chars[k].EndMS = maxInt(cursor+1, cursor+step)
			}
			cursor = chars[k].EndMS
		}
		i = j
	}
}

func clampMS(value, minValue, maxValue int) int {
	if value < minValue {
		return minValue
	}
	if value > maxValue {
		return maxValue
	}
	return value
}

func resolveSpeakerProfile(language, contentProfile, speaker string, overrideVoice *int64) speakerProfile {
	key := "male"
	if strings.EqualFold(strings.TrimSpace(speaker), "female") {
		key = "female"
	}

	if isJapaneseLanguage(language) {
		return speakerProfile{
			VoiceID: conf.Get[string]("worker.elevenlabs_podcast_" + key + "_voice_id"),
		}
	}

	voiceProfilePrefix := chineseVoiceProfilePrefix(contentProfile)
	profile := speakerProfile{
		VoiceType:        conf.Get[int64]("worker.tencent_podcast_" + voiceProfilePrefix + "_" + key + "_voice_type"),
		Speed:            conf.Get[float64]("worker.tencent_podcast_" + voiceProfilePrefix + "_" + key + "_speed"),
		SampleRate:       conf.Get[int64]("worker.tencent_podcast_tts_sample_rate"),
		EmotionCategory:  conf.Get[string]("worker.tencent_podcast_" + key + "_emotion"),
		EmotionIntensity: conf.Get[int64]("worker.tencent_podcast_" + key + "_emotion_intensity"),
	}
	if overrideVoice != nil && *overrideVoice != 0 {
		profile.VoiceType = *overrideVoice
	}
	if profile.VoiceType == 0 {
		profile.VoiceType = conf.Get[int64]("worker.tencent_tts_voice_type")
	}
	if profile.SampleRate == 0 {
		profile.SampleRate = 24000
	}
	if profile.EmotionIntensity == 0 {
		profile.EmotionIntensity = 100
	}
	return profile
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

func chineseVoiceProfilePrefix(contentProfile string) string {
	switch normalizeContentProfile(contentProfile) {
	case "social_issue", "international":
		return "public"
	default:
		return "daily"
	}
}

func providerConfigForLanguage(language string) tts.Config {
	if isJapaneseLanguage(language) {
		return tts.Config{
			Provider:               "elevenlabs",
			ElevenLabsBaseURL:      conf.Get[string]("worker.elevenlabs_tts_base_url"),
			ElevenLabsAPIKey:       conf.Get[string]("worker.elevenlabs_tts_api_key"),
			ElevenLabsVoiceID:      conf.Get[string]("worker.elevenlabs_tts_voice_id"),
			ElevenLabsModelID:      conf.Get[string]("worker.elevenlabs_tts_model_id"),
			ElevenLabsOutputFormat: conf.Get[string]("worker.elevenlabs_tts_output_format"),
			ElevenLabsEnableLog:    conf.Get[bool]("worker.elevenlabs_tts_enable_logging"),
		}
	}

	return tts.Config{
		Provider:               "tencent",
		TencentRegion:          conf.Get[string]("worker.tencent_tts_region"),
		TencentSecretID:        conf.Get[string]("worker.tencent_tts_secret_id"),
		TencentSecretKey:       conf.Get[string]("worker.tencent_tts_secret_key"),
		TencentVoiceType:       conf.Get[int64]("worker.tencent_tts_voice_type"),
		TencentPrimaryLanguage: conf.Get[int64]("worker.tencent_tts_primary_language"),
		TencentModelType:       conf.Get[int64]("worker.tencent_tts_model_type"),
		TencentCodec:           conf.Get[string]("worker.tencent_tts_codec"),
	}
}

func segmentGapMSForLanguage(language string) int {
	if isJapaneseLanguage(language) {
		return conf.Get[int]("worker.elevenlabs_podcast_segment_gap_ms")
	}
	return conf.Get[int]("worker.tencent_podcast_segment_gap_ms")
}

func sameSpeakerGapMSForLanguage(language string) int {
	if isJapaneseLanguage(language) {
		return conf.Get[int]("worker.elevenlabs_podcast_same_speaker_gap_ms")
	}
	return conf.Get[int]("worker.tencent_podcast_same_speaker_gap_ms")
}

func chineseGapAfterSegment(segments []dto.PodcastSegment, index, defaultGapMs, sameSpeakerGapMs int, defaultGapPath, sameSpeakerGapPath string) (int, string) {
	if index < 0 || index >= len(segments)-1 {
		return 0, ""
	}
	current := strings.TrimSpace(defaultSpeaker(segments[index].Speaker))
	for next := index + 1; next < len(segments); next++ {
		if strings.TrimSpace(segments[next].ZH) == "" {
			continue
		}
		if current != "" && strings.EqualFold(current, strings.TrimSpace(defaultSpeaker(segments[next].Speaker))) {
			if sameSpeakerGapMs > 0 && strings.TrimSpace(sameSpeakerGapPath) != "" {
				return sameSpeakerGapMs, sameSpeakerGapPath
			}
			return 0, ""
		}
		if defaultGapMs > 0 && strings.TrimSpace(defaultGapPath) != "" {
			return defaultGapMs, defaultGapPath
		}
		return 0, ""
	}
	return 0, ""
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

func projectDirFor(projectID string) string {
	return filepath.Join(conf.Get[string]("worker.ffmpeg_work_dir"), "projects", projectID)
}

func podcastEnabledForLanguage(language string) bool {
	if isJapaneseLanguage(language) {
		return conf.Get[bool]("worker.elevenlabs_tts_enabled")
	}
	return conf.Get[bool]("worker.tencent_tts_enabled")
}

func directProjectSourceDir(name string) string {
	return filepath.Join(conf.Get[string]("worker.ffmpeg_work_dir"), "projects", filepath.Base(strings.TrimSpace(name)))
}

func chineseTokenAlignmentLooksUniform(seg dto.PodcastSegment) bool {
	if len(seg.Tokens) == 0 || seg.EndMS <= seg.StartMS {
		return false
	}
	durations := make(map[int]struct{})
	timed := 0
	firstStart := -1
	for _, token := range seg.Tokens {
		if token.EndMS <= token.StartMS {
			continue
		}
		timed++
		if firstStart == -1 {
			firstStart = token.StartMS
		}
		durations[token.EndMS-token.StartMS] = struct{}{}
	}
	if timed == 0 || firstStart != seg.StartMS {
		return false
	}
	return len(durations) <= 2
}

func scriptPathFor(filename string) string {
	return filepath.Join(conf.Get[string]("worker.worker_assets_dir"), "podcast", "scripts", filepath.Base(strings.TrimSpace(filename)))
}

func sanitizeSegmentID(segmentID string) string {
	raw := strings.TrimSpace(segmentID)
	if raw == "" {
		return "segment"
	}
	raw = strings.ReplaceAll(raw, "/", "-")
	raw = strings.ReplaceAll(raw, "\\", "-")
	return raw
}

func int64Ptr(value int64) *int64 {
	return &value
}

func float64Ptr(value float64) *float64 {
	return &value
}

func boolPtr(value bool) *bool {
	return &value
}

func stringPtr(value string) *string {
	if strings.TrimSpace(value) == "" {
		return nil
	}
	return &value
}

func subtitleMatchesToken(subtitleText, tokenChar string) bool {
	subtitleText = strings.TrimSpace(subtitleText)
	tokenChar = strings.TrimSpace(tokenChar)
	if subtitleText == "" || tokenChar == "" {
		return false
	}
	if subtitleText == tokenChar {
		return true
	}
	if len([]rune(subtitleText)) == 1 && len([]rune(tokenChar)) == 1 {
		return subtitleText == tokenChar
	}
	return false
}

func isSilentToken(charText string) bool {
	rs := []rune(strings.TrimSpace(charText))
	if len(rs) != 1 {
		return false
	}
	return isPunctuationRune(rs[0])
}

func isPunctuationRune(r rune) bool {
	return strings.ContainsRune("，。！？；：“”‘’（）《》、…,.!?;:()[]{}\"'", r)
}

func firstPositive(values ...int) int {
	for _, value := range values {
		if value > 0 {
			return value
		}
	}
	return 0
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func alignedStats(script dto.PodcastScript) (int, int, int, int) {
	timedSegments := 0
	totalSegments := len(script.Segments)
	timedTokens := 0
	totalTokens := 0
	for _, seg := range script.Segments {
		if seg.EndMS > seg.StartMS {
			timedSegments++
		}
		if strings.EqualFold(strings.TrimSpace(script.Language), "ja") || strings.EqualFold(strings.TrimSpace(script.Language), "ja-jp") {
			for _, ch := range seg.Chars {
				totalTokens++
				if ch.EndMS > ch.StartMS {
					timedTokens++
				}
			}
			continue
		}
		for _, token := range seg.Tokens {
			totalTokens++
			if token.EndMS > token.StartMS {
				timedTokens++
			}
		}
	}
	return timedSegments, totalSegments, timedTokens, totalTokens
}

func defaultSpeaker(value string) string {
	value = strings.TrimSpace(strings.ToLower(value))
	if value == "" {
		return "male"
	}
	return value
}

func writeJSON(path string, data interface{}) error {
	raw, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
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

func copyFile(src, dst string) error {
	raw, err := os.ReadFile(src)
	if err != nil {
		return err
	}
	return os.WriteFile(dst, raw, 0o644)
}
