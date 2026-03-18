package podcast_audio_service

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"unicode"

	"worker/internal/dto"
	"worker/pkg/mfa"
	ffmpegcommon "worker/services/ffmpeg_service/common"
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

	alignmentAudioPath, err := extractAlignmentAudio(audioPath, a.workingDir)
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
	if err != nil {
		return dto.PodcastBlock{}, err
	}
	if len(words) == 0 {
		return a.alignBlockHeuristically(language, block, durationMS), nil
	}

	aligned, ok := alignBlockWithTimedWords(language, block, words, durationMS)
	if !ok {
		return a.alignBlockHeuristically(language, block, durationMS), nil
	}
	return aligned, nil
}

func extractAlignmentAudio(audioPath, workingDir string) (string, error) {
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

	if err := ffmpegcommon.RunFFmpeg(
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
			aligned.Segments[i] = alignChineseSegmentWithWords(seg, spec, localMatches, window)
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

func alignChineseSegmentWithWords(seg dto.PodcastSegment, spec segmentSpec, matches []timedWordMatch, window segmentWindow) dto.PodcastSegment {
	seg.StartMS = window.StartMS
	seg.EndMS = maxInt(window.EndMS, window.StartMS+1)

	tokens := chineseSegmentTokens(seg)
	indexByRune := chineseTokenIndexByRune(seg, tokens)
	for _, match := range matches {
		for runeIndex := match.StartNorm; runeIndex < match.EndNorm; runeIndex++ {
			tokenIndex, ok := indexByRune[runeIndex]
			if !ok || tokenIndex < 0 || tokenIndex >= len(tokens) {
				continue
			}
			tokens[tokenIndex].StartMS = match.StartMS
			tokens[tokenIndex].EndMS = match.EndMS
		}
	}
	fillUnalignedChineseTokens(tokens, seg.StartMS, seg.EndMS)
	seg.Tokens = tokens
	return seg
}

func chineseTokenIndexByRune(seg dto.PodcastSegment, tokens []dto.PodcastToken) map[int]int {
	indexes := make(map[int]int, len(tokens))
	tokenCursor := 0
	for runeIndex, r := range []rune(strings.TrimSpace(seg.Text)) {
		if unicode.IsSpace(r) {
			continue
		}
		if tokenCursor >= len(tokens) {
			break
		}
		indexes[runeIndex] = tokenCursor
		tokenCursor++
	}
	return indexes
}

func alignJapaneseSegmentWithWords(seg dto.PodcastSegment, spec segmentSpec, matches []timedWordMatch, window segmentWindow) dto.PodcastSegment {
	seg.StartMS = window.StartMS
	seg.EndMS = maxInt(window.EndMS, window.StartMS+1)
	annotationTokens := seg.Tokens
	seg.TokenSpans = buildTokenSpansFromTokens(japaneseDisplayText(seg), annotationTokens)

	tokens := buildJapaneseTokens(seg)
	for _, match := range matches {
		for idx := match.StartNorm; idx < match.EndNorm && idx < len(tokens); idx++ {
			tokens[idx].StartMS = match.StartMS
			tokens[idx].EndMS = match.EndMS
		}
	}
	fillJapaneseTokenTimingGaps(tokens, seg.StartMS, seg.EndMS)
	seg.Tokens = tokens
	return seg
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
			annotationTokens := seg.Tokens
			seg.TokenSpans = buildTokenSpansFromTokens(japaneseDisplayText(seg), annotationTokens)
			seg = assignJapaneseTokenTimes(seg)
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

func fillJapaneseTokenTimingGaps(tokens []dto.PodcastToken, segmentStartMS, segmentEndMS int) {
	if len(tokens) == 0 {
		return
	}
	start := maxInt(0, segmentStartMS)
	end := maxInt(start+1, segmentEndMS)

	for i := 0; i < len(tokens); {
		if tokens[i].EndMS > tokens[i].StartMS {
			i++
			continue
		}
		j := i
		for j < len(tokens) && tokens[j].EndMS <= tokens[j].StartMS {
			j++
		}

		windowStart := start
		if i > 0 && tokens[i-1].EndMS > tokens[i-1].StartMS {
			windowStart = tokens[i-1].EndMS
		}
		windowEnd := end
		if j < len(tokens) && tokens[j].EndMS > tokens[j].StartMS {
			windowEnd = tokens[j].StartMS
		}
		if windowEnd <= windowStart {
			windowEnd = windowStart + (j - i)
		}
		step := maxInt(1, (windowEnd-windowStart)/maxInt(1, j-i))
		cursor := windowStart
		for k := i; k < j; k++ {
			tokens[k].StartMS = cursor
			if k == j-1 {
				tokens[k].EndMS = maxInt(cursor+1, windowEnd)
			} else {
				tokens[k].EndMS = maxInt(cursor+1, cursor+step)
			}
			cursor = tokens[k].EndMS
		}
		i = j
	}
}

func chunkWorkingDir(baseDir string) string {
	return filepath.Join(baseDir, "alignment_chunks")
}
