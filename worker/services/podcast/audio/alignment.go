package podcast_audio_service

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"unicode"

	"worker/pkg/mfa"
	ffmpegcommon "worker/services/media/ffmpeg/common"
	dto "worker/services/podcast/model"
)

type blockAligner struct {
	client     *mfa.Client
	workingDir string
}

type segmentSpec struct {
	Index           int
	Text            string
	Runes           []rune
	Normalized      []rune
	NormToOriginal  []int
	GlobalNormStart int
	GlobalNormEnd   int
}

type timedWordMatch struct {
	StartNorm int
	EndNorm   int
	StartMS   int
	EndMS     int
}

type segmentWindow struct {
	StartMS  int
	EndMS    int
	HasMatch bool
}

func newBlockAligner(client *mfa.Client, workingDir string) *blockAligner {
	return &blockAligner{
		client:     client,
		workingDir: workingDir,
	}
}

// AlignBlock keeps TTS and alignment decoupled: Gemini produces the audio, MFA
// produces word intervals, and this layer translates those intervals into the
// token/char timing expected by the subtitle renderer.
func (a *blockAligner) AlignBlock(ctx context.Context, language string, block dto.PodcastBlock, audioPath string, durationMS int) (dto.PodcastBlock, error) {
	if len(block.Segments) == 0 {
		return block, nil
	}
	if a == nil || a.client == nil || !a.client.Enabled() {
		return a.alignBlockHeuristically(language, block, durationMS), nil
	}

	alignmentAudioPath, err := extractAlignmentAudio(ctx, audioPath, a.workingDir)
	if err != nil {
		return dto.PodcastBlock{}, err
	}
	defer os.Remove(alignmentAudioPath)

	words, err := a.client.AlignWords(ctx, mfa.AlignRequest{
		LanguageCode: language,
		AudioPath:    alignmentAudioPath,
		Transcript:   blockTranscript(language, block),
		WorkingDir:   a.workingDir,
	})
	if err == nil && len(words) > 0 {
		aligned, ok := alignBlockWithTimedWords(language, block, words, durationMS)
		if ok {
			return aligned, nil
		}
	}

	segmented, segErr := a.alignBlockBySegments(ctx, language, block, alignmentAudioPath, durationMS)
	if segErr == nil {
		return segmented, nil
	}
	if err != nil {
		return dto.PodcastBlock{}, fmt.Errorf("%w; segment_retry=%v", err, segErr)
	}
	if len(words) == 0 {
		return dto.PodcastBlock{}, fmt.Errorf("mfa block alignment returned no words and segment retry failed: %w", segErr)
	}
	return dto.PodcastBlock{}, fmt.Errorf("mfa block alignment did not map cleanly and segment retry failed: %w", segErr)
}

func extractAlignmentAudio(ctx context.Context, audioPath, workingDir string) (string, error) {
	if strings.TrimSpace(workingDir) == "" {
		workingDir = os.TempDir()
	}
	if err := os.MkdirAll(workingDir, 0o755); err != nil {
		return "", err
	}
	chunkFile, err := os.CreateTemp(workingDir, "align_full_*.wav")
	if err != nil {
		return "", err
	}
	chunkPath := chunkFile.Name()
	if err := chunkFile.Close(); err != nil {
		_ = os.Remove(chunkPath)
		return "", err
	}

	if err := ffmpegcommon.RunFFmpegContext(
		ctx,
		"-y",
		"-i", audioPath,
		"-ac", "1",
		"-ar", "16000",
		"-c:a", "pcm_s16le",
		chunkPath,
	); err != nil {
		_ = os.Remove(chunkPath)
		return "", err
	}
	return chunkPath, nil
}

func extractAlignmentAudioChunk(ctx context.Context, audioPath, workingDir string, startMS, endMS int) (string, error) {
	startMS = maxInt(0, startMS)
	endMS = maxInt(startMS+1, endMS)
	if strings.TrimSpace(workingDir) == "" {
		workingDir = os.TempDir()
	}
	if err := os.MkdirAll(workingDir, 0o755); err != nil {
		return "", err
	}
	chunkFile, err := os.CreateTemp(workingDir, "align_chunk_*.wav")
	if err != nil {
		return "", err
	}
	chunkPath := chunkFile.Name()
	if err := chunkFile.Close(); err != nil {
		_ = os.Remove(chunkPath)
		return "", err
	}

	startSec := fmt.Sprintf("%.3f", float64(startMS)/1000.0)
	endSec := fmt.Sprintf("%.3f", float64(endMS)/1000.0)
	if err := ffmpegcommon.RunFFmpegContext(
		ctx,
		"-y",
		"-i", audioPath,
		"-ss", startSec,
		"-to", endSec,
		"-ac", "1",
		"-ar", "16000",
		"-c:a", "pcm_s16le",
		chunkPath,
	); err != nil {
		_ = os.Remove(chunkPath)
		return "", err
	}
	return chunkPath, nil
}

func blockTranscript(language string, block dto.PodcastBlock) string {
	parts := make([]string, 0, len(block.Segments))
	for _, seg := range block.Segments {
		text := strings.TrimSpace(spokenTextForAlignment(language, seg))
		if text == "" {
			continue
		}
		parts = append(parts, text)
	}
	return strings.Join(parts, "\n")
}

func spokenTextForAlignment(language string, seg dto.PodcastSegment) string {
	if isJapaneseLanguage(language) {
		return japaneseDisplayText(seg)
	}
	return strings.TrimSpace(seg.Text)
}

func (a *blockAligner) alignBlockBySegments(ctx context.Context, language string, block dto.PodcastBlock, audioPath string, blockDurationMS int) (dto.PodcastBlock, error) {
	estimated := a.alignBlockHeuristically(language, block, blockDurationMS)
	aligned := block

	for i, seg := range block.Segments {
		estimatedSeg := estimated.Segments[i]
		chunkStartMS, chunkEndMS := paddedSegmentWindow(i, estimated.Segments, blockDurationMS)
		chunkPath, err := extractAlignmentAudioChunk(ctx, audioPath, a.workingDir, chunkStartMS, chunkEndMS)
		if err != nil {
			return dto.PodcastBlock{}, fmt.Errorf("extract segment chunk %s failed: %w", seg.SegmentID, err)
		}

		words, alignErr := a.client.AlignWords(ctx, mfa.AlignRequest{
			LanguageCode: language,
			AudioPath:    chunkPath,
			Transcript:   spokenTextForAlignment(language, seg),
			WorkingDir:   a.workingDir,
		})
		_ = os.Remove(chunkPath)
		if alignErr != nil {
			return dto.PodcastBlock{}, fmt.Errorf("segment %s mfa align failed: %w", seg.SegmentID, alignErr)
		}
		if len(words) == 0 {
			return dto.PodcastBlock{}, fmt.Errorf("segment %s mfa align returned no words", seg.SegmentID)
		}

		alignedSeg, ok := alignSingleSegmentWithWords(language, seg, words, chunkStartMS, chunkEndMS)
		if !ok {
			return dto.PodcastBlock{}, fmt.Errorf("segment %s mfa words could not be mapped to transcript", seg.SegmentID)
		}
		if alignedSeg.EndMS <= alignedSeg.StartMS {
			alignedSeg.StartMS = estimatedSeg.StartMS
			alignedSeg.EndMS = estimatedSeg.EndMS
		}
		aligned.Segments[i] = alignedSeg
	}
	return aligned, nil
}

func paddedSegmentWindow(index int, segments []dto.PodcastSegment, blockDurationMS int) (int, int) {
	if index < 0 || index >= len(segments) {
		return 0, maxInt(1, blockDurationMS)
	}
	seg := segments[index]
	startMS := seg.StartMS
	endMS := seg.EndMS
	if endMS <= startMS {
		return 0, maxInt(1, blockDurationMS)
	}

	padMS := 180
	if index > 0 {
		prevGap := startMS - segments[index-1].EndMS
		startMS -= minInt(padMS, maxInt(0, prevGap/2))
	}
	if index+1 < len(segments) {
		nextGap := segments[index+1].StartMS - endMS
		endMS += minInt(padMS, maxInt(0, nextGap/2))
	}

	startMS = maxInt(0, startMS)
	endMS = minInt(blockDurationMS, endMS)
	if endMS <= startMS {
		endMS = minInt(blockDurationMS, startMS+1)
	}
	return startMS, endMS
}

func alignBlockWithTimedWords(language string, block dto.PodcastBlock, words []mfa.WordTiming, blockDurationMS int) (dto.PodcastBlock, bool) {
	specs, transcript := buildSegmentSpecs(language, block)
	if len(specs) == 0 || len(transcript) == 0 {
		return block, false
	}

	matches := matchWordsToTranscript(transcript, words)
	if len(matches) == 0 {
		return block, false
	}

	windows := deriveSegmentWindows(specs, matches, blockDurationMS)
	aligned := block
	matchedSegments := 0

	for i, seg := range aligned.Segments {
		window := windows[i]
		if window.EndMS <= window.StartMS {
			window = segmentWindow{StartMS: 0, EndMS: maxInt(1, blockDurationMS/maxInt(len(aligned.Segments), 1))}
		}
		spec := specs[i]
		localMatches := segmentWordMatches(spec, matches)
		if isJapaneseLanguage(language) {
			aligned.Segments[i] = alignJapaneseSegmentWithWords(seg, spec, localMatches, window)
		} else {
			aligned.Segments[i] = alignChineseSegmentWithWords(seg, localMatches, window)
		}
		if window.HasMatch {
			matchedSegments++
		}
	}
	return aligned, matchedSegments > 0
}

func buildSegmentSpecs(language string, block dto.PodcastBlock) ([]segmentSpec, []rune) {
	specs := make([]segmentSpec, 0, len(block.Segments))
	transcript := make([]rune, 0, 2048)
	cursor := 0

	for idx, seg := range block.Segments {
		text := spokenTextForAlignment(language, seg)
		runes := []rune(text)
		normalized, normToOriginal := normalizeTextForAlignment(text)
		specs = append(specs, segmentSpec{
			Index:           idx,
			Text:            text,
			Runes:           runes,
			Normalized:      normalized,
			NormToOriginal:  normToOriginal,
			GlobalNormStart: cursor,
			GlobalNormEnd:   cursor + len(normalized),
		})
		transcript = append(transcript, normalized...)
		cursor += len(normalized)
	}

	return specs, transcript
}

func normalizeTextForAlignment(text string) ([]rune, []int) {
	out := make([]rune, 0, len(text))
	indexes := make([]int, 0, len(text))
	for idx, r := range []rune(text) {
		if !keepAlignmentRune(r) {
			continue
		}
		out = append(out, unicode.ToLower(r))
		indexes = append(indexes, idx)
	}
	return out, indexes
}

func keepAlignmentRune(r rune) bool {
	if unicode.IsSpace(r) {
		return false
	}
	return !isPunctuationRune(r)
}

func matchWordsToTranscript(transcript []rune, words []mfa.WordTiming) []timedWordMatch {
	matches := make([]timedWordMatch, 0, len(words))
	cursor := 0

	for _, word := range words {
		normalized, _ := normalizeTextForAlignment(word.Text)
		if len(normalized) == 0 {
			continue
		}

		start := findSubslice(transcript, normalized, cursor)
		if start < 0 {
			start = findSubslice(transcript, normalized, 0)
		}
		if start < 0 {
			continue
		}
		end := start + len(normalized)
		matches = append(matches, timedWordMatch{
			StartNorm: start,
			EndNorm:   end,
			StartMS:   word.StartMS,
			EndMS:     maxInt(word.EndMS, word.StartMS+1),
		})
		cursor = end
	}
	return matches
}

func findSubslice(haystack, needle []rune, from int) int {
	if len(needle) == 0 || len(haystack) == 0 || from >= len(haystack) {
		return -1
	}
	maxStart := len(haystack) - len(needle)
	for start := maxInt(0, from); start <= maxStart; start++ {
		ok := true
		for i := range needle {
			if haystack[start+i] != needle[i] {
				ok = false
				break
			}
		}
		if ok {
			return start
		}
	}
	return -1
}

func deriveSegmentWindows(specs []segmentSpec, matches []timedWordMatch, blockDurationMS int) []segmentWindow {
	windows := make([]segmentWindow, len(specs))
	for i, spec := range specs {
		start := -1
		end := -1
		for _, match := range matches {
			if match.EndNorm <= spec.GlobalNormStart || match.StartNorm >= spec.GlobalNormEnd {
				continue
			}
			if start == -1 || match.StartMS < start {
				start = match.StartMS
			}
			if end == -1 || match.EndMS > end {
				end = match.EndMS
			}
		}
		if start >= 0 && end > start {
			windows[i] = segmentWindow{StartMS: start, EndMS: end, HasMatch: true}
		}
	}

	for i := 0; i < len(windows); {
		if windows[i].HasMatch {
			i++
			continue
		}
		j := i
		totalWeight := 0
		for j < len(windows) && !windows[j].HasMatch {
			totalWeight += maxInt(1, len(specs[j].Normalized))
			j++
		}

		windowStart := 0
		if i > 0 {
			windowStart = windows[i-1].EndMS
		}
		windowEnd := blockDurationMS
		if j < len(windows) && windows[j].HasMatch {
			windowEnd = windows[j].StartMS
		}
		if windowEnd <= windowStart {
			windowEnd = windowStart + maxInt(j-i, 1)
		}

		cursor := windowStart
		accWeight := 0
		for k := i; k < j; k++ {
			weight := maxInt(1, len(specs[k].Normalized))
			start := cursor
			accWeight += weight
			end := windowStart + (windowEnd-windowStart)*accWeight/maxInt(totalWeight, 1)
			if k == j-1 {
				end = windowEnd
			}
			if end <= start {
				end = start + 1
			}
			windows[k] = segmentWindow{StartMS: start, EndMS: end}
			cursor = end
		}
		i = j
	}
	return windows
}

func segmentWordMatches(spec segmentSpec, matches []timedWordMatch) []timedWordMatch {
	out := make([]timedWordMatch, 0, 8)
	for _, match := range matches {
		if match.EndNorm <= spec.GlobalNormStart || match.StartNorm >= spec.GlobalNormEnd {
			continue
		}

		localStartNorm := maxInt(match.StartNorm, spec.GlobalNormStart) - spec.GlobalNormStart
		localEndNorm := minInt(match.EndNorm, spec.GlobalNormEnd) - spec.GlobalNormStart
		if localStartNorm < 0 || localEndNorm <= localStartNorm || localEndNorm > len(spec.NormToOriginal) {
			continue
		}

		startOriginal := spec.NormToOriginal[localStartNorm]
		endOriginal := spec.NormToOriginal[localEndNorm-1] + 1
		out = append(out, timedWordMatch{
			StartNorm: startOriginal,
			EndNorm:   endOriginal,
			StartMS:   match.StartMS,
			EndMS:     match.EndMS,
		})
	}
	return out
}

func alignSingleSegmentWithWords(language string, seg dto.PodcastSegment, words []mfa.WordTiming, chunkStartMS, chunkEndMS int) (dto.PodcastSegment, bool) {
	spec := buildSingleSegmentSpec(language, seg)
	if len(spec.Normalized) == 0 {
		return seg, false
	}
	matches := matchWordsToSingleSegment(spec, words, chunkStartMS)
	if len(matches) == 0 {
		return seg, false
	}
	window := windowForMatches(matches, chunkStartMS, chunkEndMS)
	if isJapaneseLanguage(language) {
		return alignJapaneseSegmentWithWords(seg, spec, matches, window), true
	}
	return alignChineseSegmentWithWords(seg, matches, window), true
}

func buildSingleSegmentSpec(language string, seg dto.PodcastSegment) segmentSpec {
	text := spokenTextForAlignment(language, seg)
	runes := []rune(text)
	normalized, normToOriginal := normalizeTextForAlignment(text)
	return segmentSpec{
		Text:           text,
		Runes:          runes,
		Normalized:     normalized,
		NormToOriginal: normToOriginal,
	}
}

func matchWordsToSingleSegment(spec segmentSpec, words []mfa.WordTiming, offsetMS int) []timedWordMatch {
	matches := make([]timedWordMatch, 0, len(words))
	cursor := 0

	for _, word := range words {
		normalized, _ := normalizeTextForAlignment(word.Text)
		if len(normalized) == 0 {
			continue
		}

		start := findSubslice(spec.Normalized, normalized, cursor)
		if start < 0 {
			start = findSubslice(spec.Normalized, normalized, 0)
		}
		if start < 0 {
			continue
		}
		end := start + len(normalized)
		if end > len(spec.NormToOriginal) {
			continue
		}

		matches = append(matches, timedWordMatch{
			StartNorm: spec.NormToOriginal[start],
			EndNorm:   spec.NormToOriginal[end-1] + 1,
			StartMS:   offsetMS + word.StartMS,
			EndMS:     offsetMS + maxInt(word.EndMS, word.StartMS+1),
		})
		cursor = end
	}
	return matches
}

func windowForMatches(matches []timedWordMatch, fallbackStartMS, fallbackEndMS int) segmentWindow {
	window := segmentWindow{
		StartMS: fallbackStartMS,
		EndMS:   maxInt(fallbackEndMS, fallbackStartMS+1),
	}
	for i, match := range matches {
		if i == 0 || match.StartMS < window.StartMS {
			window.StartMS = match.StartMS
		}
		if i == 0 || match.EndMS > window.EndMS {
			window.EndMS = match.EndMS
		}
	}
	window.HasMatch = len(matches) > 0 && window.EndMS > window.StartMS
	return window
}

func alignChineseSegmentWithWords(seg dto.PodcastSegment, matches []timedWordMatch, window segmentWindow) dto.PodcastSegment {
	seg.StartMS = window.StartMS
	seg.EndMS = maxInt(window.EndMS, window.StartMS+1)

	tokens := chineseSegmentTokens(seg)
	indexByRune := chineseTokenIndexByRune(seg, tokens)
	for _, match := range matches {
		tokenIndexes := chineseTokenIndexesForRange(indexByRune, match.StartNorm, match.EndNorm)
		assignWindowToChineseTokens(tokens, tokenIndexes, match.StartMS, match.EndMS)
	}
	fillUnalignedChineseTokens(tokens, seg.StartMS, seg.EndMS)
	seg.Tokens = tokens
	return seg
}

func chineseTokenIndexesForRange(indexByRune map[int]int, startRune, endRune int) []int {
	if len(indexByRune) == 0 || endRune <= startRune {
		return nil
	}
	out := make([]int, 0, endRune-startRune)
	seen := make(map[int]struct{}, endRune-startRune)
	for runeIndex := startRune; runeIndex < endRune; runeIndex++ {
		tokenIndex, ok := indexByRune[runeIndex]
		if !ok || tokenIndex < 0 {
			continue
		}
		if _, exists := seen[tokenIndex]; exists {
			continue
		}
		seen[tokenIndex] = struct{}{}
		out = append(out, tokenIndex)
	}
	return out
}

func assignWindowToChineseTokens(tokens []dto.PodcastToken, tokenIndexes []int, startMS, endMS int) {
	if len(tokens) == 0 || len(tokenIndexes) == 0 {
		return
	}
	if endMS <= startMS {
		endMS = startMS + 1
	}

	count := len(tokenIndexes)
	span := maxInt(1, endMS-startMS)
	for i, tokenIndex := range tokenIndexes {
		if tokenIndex < 0 || tokenIndex >= len(tokens) {
			continue
		}
		partStart := startMS + (span*i)/count
		partEnd := startMS + (span*(i+1))/count
		if i == count-1 {
			partEnd = endMS
		}
		if partEnd <= partStart {
			partEnd = partStart + 1
		}

		if tokens[tokenIndex].EndMS > tokens[tokenIndex].StartMS {
			if partStart < tokens[tokenIndex].StartMS {
				tokens[tokenIndex].StartMS = partStart
			}
			if partEnd > tokens[tokenIndex].EndMS {
				tokens[tokenIndex].EndMS = partEnd
			}
			continue
		}
		tokens[tokenIndex].StartMS = partStart
		tokens[tokenIndex].EndMS = partEnd
	}
}

func chineseTokenIndexByRune(seg dto.PodcastSegment, tokens []dto.PodcastToken) map[int]int {
	indexes := make(map[int]int, len(tokens))
	visibleRuneIndexes := make([]int, 0, len([]rune(strings.TrimSpace(seg.Text))))
	for runeIndex, r := range []rune(strings.TrimSpace(seg.Text)) {
		if unicode.IsSpace(r) {
			continue
		}
		visibleRuneIndexes = append(visibleRuneIndexes, runeIndex)
	}

	visibleCursor := 0
	for tokenIndex, token := range tokens {
		for _, r := range []rune(token.Char) {
			if unicode.IsSpace(r) {
				continue
			}
			if visibleCursor >= len(visibleRuneIndexes) {
				return indexes
			}
			indexes[visibleRuneIndexes[visibleCursor]] = tokenIndex
			visibleCursor++
		}
	}
	return indexes
}

func alignJapaneseSegmentWithWords(seg dto.PodcastSegment, spec segmentSpec, matches []timedWordMatch, window segmentWindow) dto.PodcastSegment {
	seg.StartMS = window.StartMS
	seg.EndMS = maxInt(window.EndMS, window.StartMS+1)
	seg.HighlightSpans = buildJapaneseMFAHighlightSpans(spec.Runes, matches)
	spans := buildJapaneseAnnotationSpans(seg)
	if len(spans) == 0 {
		return normalizeJapaneseSegment(seg)
	}

	tokens := make([]dto.PodcastToken, len(seg.Tokens))
	copy(tokens, seg.Tokens)
	for _, item := range spans {
		startMS := 0
		endMS := 0
		for _, match := range matches {
			if match.EndNorm <= item.Span.StartIndex || match.StartNorm >= item.Span.EndIndex+1 {
				continue
			}
			if startMS == 0 || match.StartMS < startMS {
				startMS = match.StartMS
			}
			if match.EndMS > endMS {
				endMS = match.EndMS
			}
		}
		if startMS > 0 && endMS > startMS {
			tokens[item.TokenIndex].StartMS = startMS
			tokens[item.TokenIndex].EndMS = endMS
		}
	}
	seg.Tokens = tokens
	return normalizeJapaneseSegment(seg)
}

func buildJapaneseMFAHighlightSpans(runes []rune, matches []timedWordMatch) []dto.PodcastHighlightSpan {
	if len(runes) == 0 || len(matches) == 0 {
		return nil
	}

	out := make([]dto.PodcastHighlightSpan, 0, len(matches))
	for _, match := range matches {
		start := maxInt(0, match.StartNorm)
		end := minInt(len(runes), match.EndNorm) - 1
		if start >= len(runes) || end < start {
			continue
		}
		if match.EndMS <= match.StartMS {
			continue
		}

		span := dto.PodcastHighlightSpan{
			StartIndex: start,
			EndIndex:   end,
			StartMS:    match.StartMS,
			EndMS:      match.EndMS,
		}

		if len(out) > 0 && span.StartIndex <= out[len(out)-1].EndIndex {
			prev := &out[len(out)-1]
			if span.EndIndex > prev.EndIndex {
				prev.EndIndex = span.EndIndex
			}
			if span.StartMS < prev.StartMS {
				prev.StartMS = span.StartMS
			}
			if span.EndMS > prev.EndMS {
				prev.EndMS = span.EndMS
			}
			continue
		}

		out = append(out, span)
	}

	if len(out) == 0 {
		return nil
	}
	return out
}

func (a *blockAligner) alignBlockHeuristically(language string, block dto.PodcastBlock, blockDurationMS int) dto.PodcastBlock {
	aligned := block
	specs, _ := buildSegmentSpecs(language, block)
	if len(specs) == 0 {
		return aligned
	}

	totalWeight := 0
	for _, spec := range specs {
		totalWeight += maxInt(1, len(spec.Normalized))
	}

	cursor := 0
	for idx, seg := range aligned.Segments {
		weight := maxInt(1, len(specs[idx].Normalized))
		span := blockDurationMS * weight / maxInt(totalWeight, 1)
		if idx == len(aligned.Segments)-1 {
			span = blockDurationMS - cursor
		}
		if span <= 0 {
			span = 1
		}
		seg.StartMS = cursor
		seg.EndMS = cursor + span
		if isJapaneseLanguage(language) {
			seg = normalizeJapaneseSegment(seg)
		} else {
			tokens := chineseSegmentTokens(seg)
			fillUnalignedChineseTokens(tokens, seg.StartMS, seg.EndMS)
			seg.Tokens = tokens
		}
		aligned.Segments[idx] = seg
		cursor += span
	}
	return aligned
}

func chunkWorkingDir(baseDir string) string {
	return filepath.Join(baseDir, "alignment_chunks")
}
