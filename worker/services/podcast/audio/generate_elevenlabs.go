package podcast_audio_service

import (
	"context"
	"fmt"
	"hash/fnv"
	"log"
	"math"
	"os"
	"path/filepath"
	"strings"
	"unicode"

	conf "worker/pkg/config"
	"worker/pkg/elevenlabs"
	services "worker/services"
	dto "worker/services/podcast/model"
	podcastspeaker "worker/services/podcast/speaker"
)

type elevenLabsInputTimingRange struct {
	HasChars  bool
	StartChar int
	EndChar   int // exclusive
	HasTime   bool
	StartMS   int
	EndMS     int
}

type timedRune struct {
	StartMS int
	EndMS   int
	Matched bool
}

func synthesizeWithElevenLabs(
	ctx context.Context,
	client *elevenlabs.Client,
	projectID string,
	language string,
	projectDir string,
	artifacts audioArtifacts,
	script dto.PodcastScript,
	blockGapMS int,
	configSeed int,
	requestedBlocks map[int]struct{},
) ([]blockSynthesisResult, error) {
	results := make([]blockSynthesisResult, len(script.Blocks))
	projectSeed := elevenLabsProjectSeed(projectID, configSeed)
	log.Printf("🎛️ podcast tts mode provider=elevenlabs request_mode=per_block blocks=%d selected_blocks=%d", len(script.Blocks), len(requestedBlocks))

	for blockIndex, block := range script.Blocks {
		forceRerun := isRequestedBlock(requestedBlocks, blockIndex)
		if !forceRerun {
			reused, ok, err := tryReuseCachedBlockWithoutMFA(language, artifacts, blockIndex, block)
			if err != nil {
				return nil, err
			}
			if ok {
				results[blockIndex] = reused
				script.Blocks[blockIndex] = reused.AlignedBlock
				continue
			}
		}

		inputs, inputAlignmentTexts, segmentIndexes, err := buildElevenLabsDialogueInputs(language, block)
		if err != nil {
			return nil, err
		}
		log.Printf("🎛️ podcast tts block start provider=elevenlabs block=%03d/%03d block_id=%s turns=%d force_rerun=%t",
			blockIndex+1, len(script.Blocks), strings.TrimSpace(block.BlockID), len(inputs), forceRerun)

		ttsResult, err := client.SynthesizeDialogueWithTimestamps(ctx, elevenlabs.SynthesizeDialogueWithTimestampsRequest{
			Inputs:       inputs,
			Prompt:       buildElevenLabsDialoguePrompt(language),
			LanguageCode: elevenLabsLanguageCode(language),
			Speed:        elevenLabsSpeakingSpeed(),
			Seed:         projectSeed,
		})
		if err != nil {
			return nil, err
		}

		blockExt := strings.TrimPrefix(strings.TrimSpace(ttsResult.Ext), ".")
		if blockExt == "" {
			blockExt = "mp3"
		}
		blockAudioPath := unitAudioPath(artifacts.blocksDir, blockIndex, block.BlockID, blockExt)
		if err := os.WriteFile(blockAudioPath, ttsResult.Audio, 0o644); err != nil {
			return nil, err
		}
		blockDurationMS, err := audioDurationMS(blockAudioPath)
		if err != nil {
			return nil, err
		}

		alignedBlock := alignBlockWithElevenLabsTimestamps(
			language,
			block,
			segmentIndexes,
			inputs,
			inputAlignmentTexts,
			ttsResult.Alignment,
			ttsResult.VoiceSegments,
			blockDurationMS,
		)
		if err := persistBlockCheckpoint(artifacts.blockStatesDir, blockIndex, alignedBlock, blockDurationMS); err != nil {
			return nil, err
		}

		results[blockIndex] = blockSynthesisResult{
			AudioPath:    blockAudioPath,
			DurationMS:   blockDurationMS,
			AlignedBlock: alignedBlock,
		}
		script.Blocks[blockIndex] = alignedBlock

		partialScript, _, _, err := assembleDialogue(
			dto.PodcastScript{
				Language: script.Language,
				Title:    script.Title,
				YouTube:  script.YouTube,
				Blocks:   append([]dto.PodcastBlock(nil), script.Blocks[:blockIndex+1]...),
			},
			results[:blockIndex+1],
			artifacts.blockGapPath,
			blockGapMS,
		)
		if err == nil {
			_ = writeJSON(filepath.Join(projectDir, "script_partial.json"), partialScript)
		}
		log.Printf("✅ podcast tts block done provider=elevenlabs block=%03d/%03d block_id=%s audio=%s duration_ms=%d",
			blockIndex+1, len(script.Blocks), strings.TrimSpace(block.BlockID), blockAudioPath, blockDurationMS)
	}
	return results, nil
}

func buildElevenLabsDialogueInputs(language string, block dto.PodcastBlock) ([]elevenlabs.DialogueInput, []string, []int, error) {
	inputs := make([]elevenlabs.DialogueInput, 0, len(block.Segments))
	alignmentTexts := make([]string, 0, len(block.Segments))
	segmentIndexes := make([]int, 0, len(block.Segments))

	for segIndex, seg := range block.Segments {
		speechText := strings.TrimSpace(spokenTextForElevenSynthesis(language, seg))
		if speechText == "" {
			continue
		}
		voiceID, err := elevenLabsVoiceIDForSpeaker(seg.Speaker)
		if err != nil {
			return nil, nil, nil, err
		}
		alignmentText := strings.TrimSpace(spokenTextForGoogleSynthesis(language, seg))
		if alignmentText == "" {
			alignmentText = strings.TrimSpace(stripElevenEmotionTags(speechText))
		}
		inputs = append(inputs, elevenlabs.DialogueInput{
			Text:    speechText,
			VoiceID: voiceID,
		})
		alignmentTexts = append(alignmentTexts, alignmentText)
		segmentIndexes = append(segmentIndexes, segIndex)
	}
	if len(inputs) == 0 {
		return nil, nil, nil, services.NonRetryableError{
			Err: fmt.Errorf("block %s has no non-empty dialogue segments for elevenlabs", strings.TrimSpace(block.BlockID)),
		}
	}
	return inputs, alignmentTexts, segmentIndexes, nil
}

func elevenLabsVoiceIDForSpeaker(speaker string) (string, error) {
	role := normalizeSpeakerRole(speaker)
	switch role {
	case "female":
		if voiceID := strings.TrimSpace(conf.Get[string]("worker.elevenlabs_tts_female_voice_id")); voiceID != "" {
			return voiceID, nil
		}
		return "", services.NonRetryableError{Err: fmt.Errorf("elevenlabs female voice id is required")}
	default:
		if voiceID := strings.TrimSpace(conf.Get[string]("worker.elevenlabs_tts_male_voice_id")); voiceID != "" {
			return voiceID, nil
		}
		return "", services.NonRetryableError{Err: fmt.Errorf("elevenlabs male voice id is required")}
	}
}

func normalizeSpeakerRole(value string) string {
	return podcastspeaker.NormalizeRole(value)
}

func elevenLabsProjectSeed(projectID string, configSeed int) int {
	if configSeed > 0 {
		return configSeed
	}
	h := fnv.New32a()
	_, _ = h.Write([]byte(strings.TrimSpace(projectID)))
	seed := int(h.Sum32() & 0x7fffffff)
	if seed > 0 {
		return seed
	}
	return 1
}

func buildElevenLabsDialoguePrompt(language string) string {
	languageLabel := "Mandarin Chinese"
	if isJapaneseLanguage(language) {
		languageLabel = "Japanese"
	}
	base := strings.TrimSpace(fmt.Sprintf(
		"Generate a natural two-speaker %s learning podcast dialogue. Interpret inline square-bracket emotion/action tags as performance instructions and never read tag words aloud. Prefer common tags such as [soft laugh], [laughs], [amused], [sigh], [surprised], [whispers], [calmly], [gently], [confidently], [relieved], [quizzically], [indecisive], and [elated], but treat these examples as non-exhaustive: other short, clear English square-bracket emotion/action tags are also allowed. The speakers are longtime close friends in a relaxed everyday conversation with subtle humor and occasional self-deprecating banter. Keep voices stable and recognizable while allowing natural, human emotional shading. Keep pacing clear for learners but not robotic. Avoid theatrical overacting, exaggerated cartoon voices, and extreme emotional swings.",
		languageLabel,
	))

	appendParts := []string{strings.TrimSpace(conf.Get[string]("worker.elevenlabs_tts_prompt_append"))}
	if isJapaneseLanguage(language) {
		appendParts = append(appendParts, strings.TrimSpace(conf.Get[string]("worker.elevenlabs_tts_ja_prompt_append")))
	} else {
		appendParts = append(appendParts, strings.TrimSpace(conf.Get[string]("worker.elevenlabs_tts_zh_prompt_append")))
	}

	prompt := base
	maxBytes := elevenLabsPromptMaxBytes()
	for _, part := range appendParts {
		if part == "" {
			continue
		}
		next := strings.TrimSpace(prompt + " " + part)
		if maxBytes > 0 && len([]byte(next)) > maxBytes {
			log.Printf("⚠️ elevenlabs tts prompt append truncated lang=%s max_bytes=%d", language, maxBytes)
			break
		}
		prompt = next
	}
	if maxBytes > 0 && len([]byte(prompt)) > maxBytes {
		return truncateUTF8ByBytes(prompt, maxBytes)
	}
	return prompt
}

func elevenLabsPromptMaxBytes() int {
	return firstPositive(conf.Get[int]("worker.elevenlabs_tts_prompt_max_bytes"), 1200)
}

func elevenLabsSpeakingSpeed() float64 {
	speed := conf.Get[float64]("worker.elevenlabs_tts_speed")
	if speed <= 0 {
		return 1.0
	}
	return speed
}

func elevenLabsLanguageCode(language string) string {
	switch strings.ToLower(strings.TrimSpace(language)) {
	case "zh", "zh-cn":
		return "zh"
	case "ja", "ja-jp":
		return "ja"
	default:
		return strings.TrimSpace(language)
	}
}

func tryReuseCachedBlockWithoutMFA(
	language string,
	artifacts audioArtifacts,
	index int,
	block dto.PodcastBlock,
) (blockSynthesisResult, bool, error) {
	return tryReuseCompletedBlockWithoutMFA(
		podcastTTSTypeElevenLabs,
		"elevenlabs",
		language,
		artifacts,
		index,
		block,
	)
}

func alignBlockWithElevenLabsTimestamps(
	language string,
	block dto.PodcastBlock,
	segmentIndexes []int,
	inputs []elevenlabs.DialogueInput,
	inputAlignmentTexts []string,
	alignment elevenlabs.CharacterAlignment,
	voiceSegments []elevenlabs.VoiceSegment,
	durationMS int,
) dto.PodcastBlock {
	aligned := clonePodcastBlock(block)
	heuristic := (&blockAligner{}).alignBlockHeuristically(language, clonePodcastBlock(block), durationMS)

	charCount := minInt(
		len(alignment.Characters),
		len(alignment.CharacterStartTimesSeconds),
		len(alignment.CharacterEndTimesSeconds),
	)
	ranges := buildElevenLabsInputTimingRanges(len(inputs), charCount, voiceSegments)
	fillMissingInputCharRangesByLength(ranges, inputAlignmentTexts, charCount)
	fillMissingInputTimeRangesFromHeuristic(ranges, heuristic, segmentIndexes)

	for inputIndex, segIndex := range segmentIndexes {
		if segIndex < 0 || segIndex >= len(aligned.Segments) {
			continue
		}
		seg := aligned.Segments[segIndex]
		fallbackSeg := heuristic.Segments[segIndex]
		rangeInfo := ranges[inputIndex]

		textForAlignment := strings.TrimSpace(inputs[inputIndex].Text)
		if inputIndex >= 0 && inputIndex < len(inputAlignmentTexts) && strings.TrimSpace(inputAlignmentTexts[inputIndex]) != "" {
			textForAlignment = strings.TrimSpace(inputAlignmentTexts[inputIndex])
		}
		textRunes := []rune(textForAlignment)
		runes := make([]timedRune, len(textRunes))
		_ = assignRuneTimingsFromAlignment(runes, textRunes, alignment, rangeInfo, charCount)

		startMS, endMS, ok := timingWindowFromRunes(runes)
		if !ok && rangeInfo.HasTime {
			startMS = rangeInfo.StartMS
			endMS = rangeInfo.EndMS
			ok = endMS > startMS
		}
		if !ok {
			startMS = fallbackSeg.StartMS
			endMS = fallbackSeg.EndMS
		}
		if endMS <= startMS {
			endMS = startMS + 1
		}
		seg.StartMS = maxInt(0, startMS)
		seg.EndMS = maxInt(seg.StartMS+1, endMS)

		if isJapaneseLanguage(language) {
			seg = applyJapaneseRuneTimings(seg, runes)
		} else {
			seg = applyChineseRuneTimings(seg, runes)
		}
		clampNestedSegmentTimings(&seg)
		aligned.Segments[segIndex] = seg
	}

	for idx := range aligned.Segments {
		seg := aligned.Segments[idx]
		if seg.EndMS <= seg.StartMS {
			seg.StartMS = heuristic.Segments[idx].StartMS
			seg.EndMS = heuristic.Segments[idx].EndMS
		}
		if isJapaneseLanguage(language) {
			seg = normalizeJapaneseSegment(seg)
		} else {
			tokens := chineseSegmentTokens(seg)
			fillUnalignedChineseTokens(tokens, seg.StartMS, seg.EndMS)
			seg.Tokens = tokens
		}
		clampNestedSegmentTimings(&seg)
		aligned.Segments[idx] = seg
	}

	enforceMonotonicTimeline(&aligned, durationMS)
	return aligned
}

func buildElevenLabsInputTimingRanges(inputCount int, charCount int, voiceSegments []elevenlabs.VoiceSegment) []elevenLabsInputTimingRange {
	ranges := make([]elevenLabsInputTimingRange, inputCount)
	for i := range ranges {
		ranges[i].StartChar = maxInt(charCount, 0)
	}

	for _, item := range voiceSegments {
		idx := item.DialogueInputIndex
		if idx < 0 || idx >= inputCount {
			continue
		}
		startChar, endChar, hasChar := normalizeCharRange(item.CharacterStartIndex, item.CharacterEndIndex, charCount)
		if hasChar {
			if !ranges[idx].HasChars || startChar < ranges[idx].StartChar {
				ranges[idx].StartChar = startChar
			}
			if !ranges[idx].HasChars || endChar > ranges[idx].EndChar {
				ranges[idx].EndChar = endChar
			}
			ranges[idx].HasChars = true
		}

		startMS := secondsToMS(item.StartTimeSeconds)
		endMS := maxInt(startMS+1, secondsToMS(item.EndTimeSeconds))
		if endMS <= startMS {
			continue
		}
		if !ranges[idx].HasTime || startMS < ranges[idx].StartMS {
			ranges[idx].StartMS = startMS
		}
		if !ranges[idx].HasTime || endMS > ranges[idx].EndMS {
			ranges[idx].EndMS = endMS
		}
		ranges[idx].HasTime = true
	}
	return ranges
}

func fillMissingInputCharRangesByLength(ranges []elevenLabsInputTimingRange, texts []string, charCount int) {
	if len(ranges) == 0 || charCount <= 0 {
		return
	}
	cursor := 0
	for i := range ranges {
		if ranges[i].HasChars {
			cursor = maxInt(cursor, ranges[i].EndChar)
			continue
		}
		size := 0
		if i >= 0 && i < len(texts) {
			size = len([]rune(strings.TrimSpace(texts[i])))
		}
		if size <= 0 {
			continue
		}
		start := minInt(maxInt(cursor, 0), charCount)
		end := minInt(charCount, start+size)
		if end <= start {
			continue
		}
		ranges[i].HasChars = true
		ranges[i].StartChar = start
		ranges[i].EndChar = end
		cursor = end
	}
}

func fillMissingInputTimeRangesFromHeuristic(
	ranges []elevenLabsInputTimingRange,
	heuristic dto.PodcastBlock,
	segmentIndexes []int,
) {
	for i := range ranges {
		if ranges[i].HasTime {
			continue
		}
		if i < 0 || i >= len(segmentIndexes) {
			continue
		}
		segIndex := segmentIndexes[i]
		if segIndex < 0 || segIndex >= len(heuristic.Segments) {
			continue
		}
		seg := heuristic.Segments[segIndex]
		if seg.EndMS <= seg.StartMS {
			continue
		}
		ranges[i].HasTime = true
		ranges[i].StartMS = seg.StartMS
		ranges[i].EndMS = seg.EndMS
	}
}

func assignRuneTimingsFromAlignment(
	out []timedRune,
	textRunes []rune,
	alignment elevenlabs.CharacterAlignment,
	inputRange elevenLabsInputTimingRange,
	charCount int,
) int {
	if len(out) == 0 || len(textRunes) == 0 || charCount <= 0 || !inputRange.HasChars {
		return 0
	}

	start := minInt(maxInt(inputRange.StartChar, 0), charCount)
	end := minInt(maxInt(inputRange.EndChar, 0), charCount)
	if end <= start {
		return 0
	}
	if len(alignment.Characters) < end || len(alignment.CharacterStartTimesSeconds) < end || len(alignment.CharacterEndTimesSeconds) < end {
		return 0
	}

	sliceChars := alignment.Characters[start:end]
	if len(sliceChars) == 0 {
		return 0
	}

	matched := 0
	if len(sliceChars) == len(textRunes) {
		for i := range textRunes {
			startMS, endMS, ok := alignmentTimingAt(alignment, start+i)
			if !ok {
				continue
			}
			out[i] = timedRune{StartMS: startMS, EndMS: endMS, Matched: true}
			matched++
		}
		return matched
	}

	cursor := 0
	for runeIndex, r := range textRunes {
		found := -1
		for i := cursor; i < len(sliceChars); i++ {
			if runeEqualForAlignment(r, sliceChars[i]) {
				found = i
				break
			}
		}
		if found < 0 {
			continue
		}
		startMS, endMS, ok := alignmentTimingAt(alignment, start+found)
		if !ok {
			cursor = found + 1
			continue
		}
		out[runeIndex] = timedRune{StartMS: startMS, EndMS: endMS, Matched: true}
		matched++
		cursor = found + 1
	}
	return matched
}

func alignmentTimingAt(alignment elevenlabs.CharacterAlignment, index int) (int, int, bool) {
	if index < 0 || index >= len(alignment.CharacterStartTimesSeconds) || index >= len(alignment.CharacterEndTimesSeconds) {
		return 0, 0, false
	}
	startMS := secondsToMS(alignment.CharacterStartTimesSeconds[index])
	endMS := maxInt(startMS+1, secondsToMS(alignment.CharacterEndTimesSeconds[index]))
	if endMS <= startMS {
		return 0, 0, false
	}
	return startMS, endMS, true
}

func timingWindowFromRunes(runes []timedRune) (int, int, bool) {
	startMS := 0
	endMS := 0
	matched := 0
	for _, item := range runes {
		if !item.Matched || item.EndMS <= item.StartMS {
			continue
		}
		if matched == 0 || item.StartMS < startMS {
			startMS = item.StartMS
		}
		if matched == 0 || item.EndMS > endMS {
			endMS = item.EndMS
		}
		matched++
	}
	return startMS, endMS, matched > 0 && endMS > startMS
}

func applyChineseRuneTimings(seg dto.PodcastSegment, runes []timedRune) dto.PodcastSegment {
	tokens := chineseSegmentTokens(seg)
	if len(tokens) == 0 {
		return seg
	}
	indexByRune := chineseTokenIndexByRune(seg, tokens)
	for runeIndex, timed := range runes {
		if !timed.Matched || timed.EndMS <= timed.StartMS {
			continue
		}
		tokenIndex, ok := indexByRune[runeIndex]
		if !ok || tokenIndex < 0 || tokenIndex >= len(tokens) {
			continue
		}
		if tokens[tokenIndex].StartMS == 0 || timed.StartMS < tokens[tokenIndex].StartMS {
			tokens[tokenIndex].StartMS = timed.StartMS
		}
		if timed.EndMS > tokens[tokenIndex].EndMS {
			tokens[tokenIndex].EndMS = timed.EndMS
		}
	}
	fillUnalignedChineseTokens(tokens, seg.StartMS, seg.EndMS)
	seg.Tokens = tokens
	return seg
}

func applyJapaneseRuneTimings(seg dto.PodcastSegment, runes []timedRune) dto.PodcastSegment {
	// For Japanese lines with annotation tokens, token timing is the more stable
	// highlight source. Rune-wide highlight spans can drift after pagination
	// because hidden earlier characters still consume timeline. Keep rune-based
	// spans only for kana-only lines that have no tokens.
	if len(seg.Tokens) == 0 {
		seg.HighlightSpans = buildJapaneseHighlightSpansFromRunes([]rune(japaneseDisplayText(seg)), runes)
	} else {
		seg.HighlightSpans = nil
	}

	if len(seg.Tokens) > 0 {
		tokens := make([]dto.PodcastToken, len(seg.Tokens))
		copy(tokens, seg.Tokens)
		refs := dto.BuildJapaneseTokenSpanRefs(japaneseDisplayText(seg), tokens)
		for _, ref := range refs {
			startMS := 0
			endMS := 0
			for idx := ref.Span.StartIndex; idx <= ref.Span.EndIndex && idx < len(runes); idx++ {
				timed := runes[idx]
				if !timed.Matched || timed.EndMS <= timed.StartMS {
					continue
				}
				if startMS == 0 || timed.StartMS < startMS {
					startMS = timed.StartMS
				}
				if timed.EndMS > endMS {
					endMS = timed.EndMS
				}
			}
			if startMS > 0 && endMS > startMS && ref.TokenIndex >= 0 && ref.TokenIndex < len(tokens) {
				tokens[ref.TokenIndex].StartMS = startMS
				tokens[ref.TokenIndex].EndMS = endMS
			}
		}
		seg.Tokens = tokens
	}
	return normalizeJapaneseSegment(seg)
}

func buildJapaneseHighlightSpansFromRunes(textRunes []rune, runes []timedRune) []dto.PodcastHighlightSpan {
	limit := minInt(len(textRunes), len(runes))
	if limit <= 0 {
		return nil
	}
	spans := make([]dto.PodcastHighlightSpan, 0, limit)
	for idx := 0; idx < limit; idx++ {
		timed := runes[idx]
		if !timed.Matched || timed.EndMS <= timed.StartMS {
			continue
		}
		span := dto.PodcastHighlightSpan{
			StartIndex: idx,
			EndIndex:   idx,
			StartMS:    timed.StartMS,
			EndMS:      timed.EndMS,
		}
		if len(spans) > 0 {
			prev := &spans[len(spans)-1]
			if span.StartIndex == prev.EndIndex+1 && span.StartMS <= prev.EndMS+140 {
				prev.EndIndex = span.EndIndex
				prev.EndMS = maxInt(prev.EndMS, span.EndMS)
				continue
			}
		}
		spans = append(spans, span)
	}
	if len(spans) == 0 {
		return nil
	}
	return spans
}

func enforceMonotonicTimeline(block *dto.PodcastBlock, durationMS int) {
	if block == nil {
		return
	}
	cursor := 0
	for i := range block.Segments {
		seg := block.Segments[i]
		if seg.StartMS < cursor {
			seg.StartMS = cursor
		}
		if seg.EndMS <= seg.StartMS {
			seg.EndMS = seg.StartMS + 1
		}
		if durationMS > 0 && seg.EndMS > durationMS {
			seg.EndMS = durationMS
		}
		if seg.EndMS <= seg.StartMS {
			seg.StartMS = maxInt(0, seg.EndMS-1)
			seg.EndMS = seg.StartMS + 1
		}
		clampNestedSegmentTimings(&seg)
		block.Segments[i] = seg
		cursor = seg.EndMS
	}
}

func clampNestedSegmentTimings(seg *dto.PodcastSegment) {
	if seg == nil || seg.EndMS <= seg.StartMS {
		return
	}
	for i := range seg.Tokens {
		token := seg.Tokens[i]
		if token.StartMS < seg.StartMS {
			token.StartMS = seg.StartMS
		}
		if token.EndMS > seg.EndMS {
			token.EndMS = seg.EndMS
		}
		if token.EndMS <= token.StartMS {
			token.EndMS = minInt(seg.EndMS, token.StartMS+1)
		}
		if token.EndMS <= token.StartMS {
			token.StartMS = maxInt(seg.StartMS, token.EndMS-1)
			token.EndMS = maxInt(token.StartMS+1, token.EndMS)
		}
		seg.Tokens[i] = token
	}
	for i := range seg.HighlightSpans {
		span := seg.HighlightSpans[i]
		if span.StartMS < seg.StartMS {
			span.StartMS = seg.StartMS
		}
		if span.EndMS > seg.EndMS {
			span.EndMS = seg.EndMS
		}
		if span.EndMS <= span.StartMS {
			span.EndMS = minInt(seg.EndMS, span.StartMS+1)
		}
		if span.EndMS <= span.StartMS {
			span.StartMS = maxInt(seg.StartMS, span.EndMS-1)
			span.EndMS = maxInt(span.StartMS+1, span.EndMS)
		}
		seg.HighlightSpans[i] = span
	}
}

func normalizeCharRange(start, end, charCount int) (int, int, bool) {
	if charCount <= 0 {
		return 0, 0, false
	}
	if start < 0 {
		start = 0
	}
	if end < 0 {
		return 0, 0, false
	}
	if end <= start {
		// Some APIs return inclusive end index; normalize to exclusive.
		end = end + 1
	}
	start = minInt(maxInt(start, 0), charCount)
	end = minInt(maxInt(end, 0), charCount)
	if end <= start {
		return 0, 0, false
	}
	return start, end, true
}

func secondsToMS(value float64) int {
	if value <= 0 {
		return 0
	}
	return int(math.Round(value * 1000.0))
}

func runeEqualForAlignment(target rune, candidate string) bool {
	candidateRunes := []rune(candidate)
	if len(candidateRunes) == 0 {
		return false
	}
	return normalizeAlignmentComparableRune(target) == normalizeAlignmentComparableRune(candidateRunes[0])
}

func normalizeAlignmentComparableRune(value rune) rune {
	switch value {
	case '’', '‘', '＇':
		return '\''
	case '”', '“', '＂':
		return '"'
	case '　':
		return ' '
	}
	return unicode.ToLower(value)
}
