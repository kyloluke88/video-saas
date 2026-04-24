package practical_audio_service

import (
	"context"
	"fmt"
	"log"
	"os"
	"strings"
	"unicode"

	"worker/pkg/mfa"
	services "worker/services"
	ffmpegcommon "worker/services/media/ffmpeg/common"
	dto "worker/services/practical/model"
)

type AlignInput struct {
	ProjectID string
	Language  string
}

type AlignResult struct {
	DialogueAudioPath string
	ScriptPath        string
	Script            dto.PracticalScript
}

type practicalTurnTimingSpec struct {
	UnitCount       int
	Units           []string
	Normalized      []rune
	GlobalNormStart int
	GlobalNormEnd   int
}

type practicalTimedWordMatch struct {
	StartNorm int
	EndNorm   int
	StartMS   int
	EndMS     int
}

type practicalTurnWindow struct {
	StartMS  int
	EndMS    int
	HasMatch bool
}

func Align(ctx context.Context, input AlignInput) (AlignResult, error) {
	if strings.TrimSpace(input.ProjectID) == "" {
		return AlignResult{}, fmt.Errorf("project_id is required")
	}
	language, err := requirePracticalLanguage(input.Language)
	if err != nil {
		return AlignResult{}, err
	}

	projectDir := projectDirFor(input.ProjectID)
	script, err := loadScriptForAlignment(projectDir, language)
	if err != nil {
		return AlignResult{}, err
	}
	if err := script.Validate(); err != nil {
		return AlignResult{}, err
	}

	alignedScript, err := alignScriptTimings(ctx, projectDir, script)
	if err != nil {
		return AlignResult{}, err
	}
	dialoguePath, introDurations, err := buildDialogueAudio(ctx, projectDir, alignedScript)
	if err != nil {
		return AlignResult{}, err
	}
	timelineScript := applyPracticalTimelineGaps(
		alignedScript,
		introDurations,
		practicalChapterGapMS(),
		practicalBlockGapMS(),
		practicalChapterTransitionLeadMS(),
	)
	if err := writeJSON(projectScriptAlignedPath(projectDir), timelineScript); err != nil {
		return AlignResult{}, err
	}

	log.Printf("🎯 practical alignment complete project_id=%s script=%s dialogue=%s", input.ProjectID, projectScriptAlignedPath(projectDir), dialoguePath)
	return AlignResult{
		DialogueAudioPath: dialoguePath,
		ScriptPath:        projectScriptAlignedPath(projectDir),
		Script:            timelineScript,
	}, nil
}

func loadScriptForAlignment(projectDir, language string) (dto.PracticalScript, error) {
	if fileExists(projectScriptInputPath(projectDir)) {
		return loadScriptFromPath(language, projectScriptInputPath(projectDir))
	}
	if fileExists(projectScriptAlignedPath(projectDir)) {
		return loadScriptFromPath(language, projectScriptAlignedPath(projectDir))
	}
	return dto.PracticalScript{}, services.NonRetryableError{Err: fmt.Errorf("practical script input not found in project dir %s", projectDir)}
}

func alignScriptTimings(ctx context.Context, projectDir string, script dto.PracticalScript) (dto.PracticalScript, error) {
	aligned := script
	client := newMFAClient()

	for blockIndex, block := range aligned.Blocks {
		audioPath := blockAudioPath(projectDir, block.BlockID, blockIndex+1)
		if !fileExists(audioPath) {
			return dto.PracticalScript{}, services.NonRetryableError{Err: fmt.Errorf("block audio missing: %s", audioPath)}
		}
		durationMS, err := audioDurationMS(ctx, audioPath)
		if err != nil {
			return dto.PracticalScript{}, err
		}
		alignedBlock, err := alignBlockTimings(ctx, client, aligned.Language, projectDir, block, audioPath, durationMS)
		if err != nil {
			return dto.PracticalScript{}, err
		}
		aligned.Blocks[blockIndex] = alignedBlock
	}

	setPracticalLocalHierarchyTimings(&aligned)
	return aligned, nil
}

func alignBlockTimings(ctx context.Context, client *mfa.Client, language, workingDir string, block dto.PracticalBlock, audioPath string, durationMS int) (dto.PracticalBlock, error) {
	specs, transcript := buildBlockTimingSpecs(language, block)
	if len(specs) == 0 {
		return block, nil
	}

	if client == nil || !client.Enabled() {
		return fillBlockTimingsHeuristically(block, specs, durationMS), nil
	}

	words, err := client.AlignWords(ctx, mfa.AlignRequest{
		LanguageCode: language,
		AudioPath:    audioPath,
		Transcript:   transcript,
		WorkingDir:   workingDir,
	})
	if err != nil || len(words) == 0 {
		return fillBlockTimingsHeuristically(block, specs, durationMS), nil
	}
	aligned, ok := mapWordsToBlockTimings(block, specs, words, durationMS)
	if !ok {
		return fillBlockTimingsHeuristically(block, specs, durationMS), nil
	}
	return aligned, nil
}

func buildBlockTimingSpecs(language string, block dto.PracticalBlock) ([]practicalTurnTimingSpec, string) {
	specs := make([]practicalTurnTimingSpec, 0, practicalBlockTurnCount(block))
	transcript := make([]rune, 0, 2048)
	transcriptParts := make([]string, 0, practicalBlockTurnCount(block))
	cursor := 0
	for _, chapter := range block.Chapters {
		for _, turn := range chapter.Turns {
			units := practicalAlignmentUnits(language, practicalSpeechText(turn))
			if len(units) == 0 {
				units = []string{strings.TrimSpace(practicalSpeechText(turn))}
			}
			normalized := normalizePracticalTextForAlignment(practicalSpeechText(turn))
			specs = append(specs, practicalTurnTimingSpec{
				UnitCount:       len(units),
				Units:           units,
				Normalized:      normalized,
				GlobalNormStart: cursor,
				GlobalNormEnd:   cursor + len(normalized),
			})
			transcript = append(transcript, normalized...)
			transcriptParts = append(transcriptParts, strings.Join(units, " "))
			cursor += len(normalized)
		}
	}
	return specs, strings.Join(transcriptParts, "\n")
}

func mapWordsToBlockTimings(block dto.PracticalBlock, specs []practicalTurnTimingSpec, words []mfa.WordTiming, durationMS int) (dto.PracticalBlock, bool) {
	transcript := buildPracticalTranscript(specs)
	if len(transcript) == 0 || len(words) == 0 {
		return dto.PracticalBlock{}, false
	}

	matches := matchPracticalWordsToTranscript(transcript, words)
	if len(matches) == 0 {
		return dto.PracticalBlock{}, false
	}

	windows := derivePracticalTurnWindows(specs, matches, durationMS)

	aligned := block
	turnCounter := 0
	matchedTurns := 0
	for chapterIndex := range aligned.Chapters {
		chapter := &aligned.Chapters[chapterIndex]
		for turnIndex := range chapter.Turns {
			window := windows[turnCounter]
			if window.EndMS <= window.StartMS {
				window.EndMS = maxInt(window.StartMS+1, durationMS/maxInt(len(specs), 1))
			}
			chapter.Turns[turnIndex].StartMS = window.StartMS
			chapter.Turns[turnIndex].EndMS = window.EndMS
			if window.HasMatch {
				matchedTurns++
			}
			turnCounter++
		}
	}
	return aligned, matchedTurns > 0
}

func fillBlockTimingsHeuristically(block dto.PracticalBlock, specs []practicalTurnTimingSpec, durationMS int) dto.PracticalBlock {
	if durationMS <= 0 {
		durationMS = 1000
	}
	totalUnits := 0
	for _, spec := range specs {
		totalUnits += maxInt(1, spec.UnitCount)
	}
	if totalUnits <= 0 {
		totalUnits = len(specs)
	}
	if totalUnits <= 0 {
		return block
	}

	aligned := block
	cursor := 0
	turnCounter := 0
	for chapterIndex := range aligned.Chapters {
		chapter := &aligned.Chapters[chapterIndex]
		for turnIndex := range chapter.Turns {
			spec := specs[turnCounter]
			share := maxInt(1, durationMS*maxInt(1, spec.UnitCount)/totalUnits)
			chapter.Turns[turnIndex].StartMS = cursor
			chapter.Turns[turnIndex].EndMS = minInt(durationMS, cursor+share)
			if chapter.Turns[turnIndex].EndMS <= chapter.Turns[turnIndex].StartMS {
				chapter.Turns[turnIndex].EndMS = chapter.Turns[turnIndex].StartMS + 1
			}
			cursor = chapter.Turns[turnIndex].EndMS
			turnCounter++
		}
	}
	return aligned
}

func buildPracticalTranscript(specs []practicalTurnTimingSpec) []rune {
	out := make([]rune, 0, 2048)
	for _, spec := range specs {
		out = append(out, spec.Normalized...)
	}
	return out
}

func normalizePracticalTextForAlignment(text string) []rune {
	out := make([]rune, 0, len(text))
	for _, r := range []rune(strings.TrimSpace(text)) {
		if !keepPracticalAlignmentRune(r) {
			continue
		}
		out = append(out, unicode.ToLower(r))
	}
	return out
}

func keepPracticalAlignmentRune(r rune) bool {
	if unicode.IsSpace(r) {
		return false
	}
	return !isPracticalPunctuationRune(r)
}

func matchPracticalWordsToTranscript(transcript []rune, words []mfa.WordTiming) []practicalTimedWordMatch {
	matches := make([]practicalTimedWordMatch, 0, len(words))
	cursor := 0

	for _, word := range words {
		normalized := normalizePracticalTextForAlignment(word.Text)
		if len(normalized) == 0 {
			continue
		}

		start := findPracticalSubslice(transcript, normalized, cursor)
		if start < 0 {
			start = findPracticalSubslice(transcript, normalized, 0)
		}
		if start < 0 {
			continue
		}
		end := start + len(normalized)
		matches = append(matches, practicalTimedWordMatch{
			StartNorm: start,
			EndNorm:   end,
			StartMS:   word.StartMS,
			EndMS:     maxInt(word.EndMS, word.StartMS+1),
		})
		cursor = end
	}
	return matches
}

func findPracticalSubslice(haystack, needle []rune, from int) int {
	if len(needle) == 0 || len(haystack) == 0 || from >= len(haystack) {
		return -1
	}
	maxStart := len(haystack) - len(needle)
	for start := maxInt(0, from); start <= maxStart; start++ {
		ok := true
		for idx := range needle {
			if haystack[start+idx] != needle[idx] {
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

func derivePracticalTurnWindows(specs []practicalTurnTimingSpec, matches []practicalTimedWordMatch, blockDurationMS int) []practicalTurnWindow {
	windows := make([]practicalTurnWindow, len(specs))
	for index, spec := range specs {
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
			windows[index] = practicalTurnWindow{StartMS: start, EndMS: end, HasMatch: true}
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
			windows[k] = practicalTurnWindow{StartMS: start, EndMS: end}
			cursor = end
		}
		i = j
	}
	return windows
}

func setPracticalLocalHierarchyTimings(script *dto.PracticalScript) {
	if script == nil {
		return
	}

	for blockIndex := range script.Blocks {
		block := &script.Blocks[blockIndex]
		block.StartMS = 0
		block.EndMS = 0
		blockFirstSet := false
		for chapterIndex := range block.Chapters {
			chapter := &block.Chapters[chapterIndex]
			chapter.StartMS = 0
			chapter.EndMS = 0
			chapterFirstSet := false
			for turnIndex := range chapter.Turns {
				turn := chapter.Turns[turnIndex]
				if turn.EndMS <= turn.StartMS {
					continue
				}
				if !blockFirstSet {
					block.StartMS = turn.StartMS
					blockFirstSet = true
				}
				if !chapterFirstSet {
					chapter.StartMS = turn.StartMS
					chapterFirstSet = true
				}
				chapter.EndMS = turn.EndMS
				block.EndMS = turn.EndMS
			}
		}
	}
}

func practicalBlockLocalRange(block dto.PracticalBlock) (int, int, bool) {
	start := 0
	end := 0
	firstSet := false
	for _, chapter := range block.Chapters {
		for _, turn := range chapter.Turns {
			if turn.EndMS <= turn.StartMS {
				continue
			}
			if !firstSet || turn.StartMS < start {
				start = turn.StartMS
			}
			if !firstSet || turn.EndMS > end {
				end = turn.EndMS
			}
			firstSet = true
		}
	}
	return start, end, firstSet
}

func shiftPracticalBlockTimings(block *dto.PracticalBlock, shift int) {
	if block == nil || shift == 0 {
		return
	}
	for chapterIndex := range block.Chapters {
		chapter := &block.Chapters[chapterIndex]
		for turnIndex := range chapter.Turns {
			turn := &chapter.Turns[turnIndex]
			if turn.StartMS > 0 {
				turn.StartMS += shift
			} else if shift > 0 {
				turn.StartMS = shift
			}
			if turn.EndMS > 0 {
				turn.EndMS += shift
			}
		}
	}
}

func buildDialogueAudio(ctx context.Context, projectDir string, script dto.PracticalScript) (string, []int, error) {
	chapterGapMS := practicalChapterGapMS()
	blockGapMS := practicalBlockGapMS()
	chapterTransitionLeadMS := practicalChapterTransitionLeadMS()
	blockTransitionLeadMS := practicalBlockTransitionLeadMS()
	if chapterGapMS > 0 {
		if err := createSilenceAudio(ctx, projectChapterGapPath(projectDir), chapterGapMS); err != nil {
			return "", nil, err
		}
	}
	if blockGapMS > 0 {
		if err := createSilenceAudio(ctx, projectBlockGapPath(projectDir), blockGapMS); err != nil {
			return "", nil, err
		}
	}
	if chapterTransitionLeadMS > 0 {
		if err := createSilenceAudio(ctx, projectChapterTransitionLeadPath(projectDir), chapterTransitionLeadMS); err != nil {
			return "", nil, err
		}
	}
	if blockTransitionLeadMS > 0 {
		if err := createSilenceAudio(ctx, projectBlockTransitionLeadPath(projectDir), blockTransitionLeadMS); err != nil {
			return "", nil, err
		}
	}

	files := make([]string, 0, len(script.Blocks)*4)
	chunkPaths := make([]string, 0, len(script.Blocks)*4)
	introDurations := make([]int, len(script.Blocks))
	defer func() {
		for _, path := range chunkPaths {
			_ = os.Remove(path)
		}
	}()

	chapterGapPath := projectChapterGapPath(projectDir)
	blockGapPath := projectBlockGapPath(projectDir)
	chapterTransitionLeadPath := projectChapterTransitionLeadPath(projectDir)
	blockTransitionLeadPath := projectBlockTransitionLeadPath(projectDir)
	for blockIndex, block := range script.Blocks {
		introPath := blockIntroAudioPath(projectDir, block.BlockID, blockIndex+1)
		if !fileExists(introPath) {
			return "", nil, services.NonRetryableError{Err: fmt.Errorf("block topic audio missing: %s", introPath)}
		}
		introDurationMS, err := audioDurationMS(ctx, introPath)
		if err != nil {
			return "", nil, err
		}
		introSegmentDurationMS := introDurationMS
		if blockIndex > 0 && blockTransitionLeadMS > 0 {
			if !fileExists(blockTransitionLeadPath) {
				return "", nil, services.NonRetryableError{Err: fmt.Errorf("block transition lead audio missing: %s", blockTransitionLeadPath)}
			}
			files = append(files, blockTransitionLeadPath)
			introSegmentDurationMS += blockTransitionLeadMS
		}
		introDurations[blockIndex] = introSegmentDurationMS
		files = append(files, introPath)

		audioPath := blockAudioPath(projectDir, block.BlockID, blockIndex+1)
		if !fileExists(audioPath) {
			return "", nil, services.NonRetryableError{Err: fmt.Errorf("block audio missing: %s", audioPath)}
		}

		for chapterIndex, chapter := range block.Chapters {
			startMS, endMS, hasTurns := practicalChapterLocalRange(chapter)
			if !hasTurns {
				continue
			}

			chunkFile, err := os.CreateTemp(projectDir, fmt.Sprintf("chapter_%02d_%02d_*.wav", blockIndex+1, chapterIndex+1))
			if err != nil {
				return "", nil, err
			}
			chunkPath := chunkFile.Name()
			if err := chunkFile.Close(); err != nil {
				_ = os.Remove(chunkPath)
				return "", nil, err
			}
			if err := extractAudioChunk(ctx, audioPath, chunkPath, startMS, endMS); err != nil {
				_ = os.Remove(chunkPath)
				return "", nil, err
			}
			chunkPaths = append(chunkPaths, chunkPath)
			if chapterTransitionLeadMS > 0 {
				if !fileExists(chapterTransitionLeadPath) {
					return "", nil, services.NonRetryableError{Err: fmt.Errorf("chapter transition lead audio missing: %s", chapterTransitionLeadPath)}
				}
				files = append(files, chapterTransitionLeadPath)
			}
			files = append(files, chunkPath)

			if chapterGapMS > 0 && chapterIndex < len(block.Chapters)-1 {
				if !fileExists(chapterGapPath) {
					return "", nil, services.NonRetryableError{Err: fmt.Errorf("chapter gap audio missing: %s", chapterGapPath)}
				}
				files = append(files, chapterGapPath)
			}
		}
		if blockGapMS > 0 && blockIndex < len(script.Blocks)-1 {
			if !fileExists(blockGapPath) {
				return "", nil, services.NonRetryableError{Err: fmt.Errorf("block gap audio missing: %s", blockGapPath)}
			}
			files = append(files, blockGapPath)
		}
	}
	outputPath := projectDialoguePath(projectDir)
	if err := concatAudioFiles(ctx, projectDir, files, outputPath); err != nil {
		return "", nil, err
	}
	return outputPath, introDurations, nil
}

func applyPracticalTimelineGaps(script dto.PracticalScript, introDurations []int, chapterGapMS, blockGapMS, chapterTransitionLeadMS int) dto.PracticalScript {
	shifted := script
	cumulativeOffset := 0

	for blockIndex := range shifted.Blocks {
		block := &shifted.Blocks[blockIndex]
		block.TopicStartMS = 0
		block.TopicEndMS = 0
		block.StartMS = 0
		block.EndMS = 0
		blockFirstSet := false
		introDurationMS := 0
		if blockIndex < len(introDurations) {
			introDurationMS = maxInt(0, introDurations[blockIndex])
		}
		if introDurationMS > 0 {
			block.TopicStartMS = cumulativeOffset
			block.TopicEndMS = cumulativeOffset + introDurationMS
			cumulativeOffset = block.TopicEndMS
			block.StartMS = block.TopicStartMS
			blockFirstSet = true
		}

		for chapterIndex := range block.Chapters {
			chapter := &block.Chapters[chapterIndex]
			localStart, _, hasTurns := practicalChapterLocalRange(*chapter)
			if !hasTurns {
				chapter.StartMS = 0
				chapter.EndMS = 0
				continue
			}

			chapter.StartMS = cumulativeOffset
			shift := cumulativeOffset + maxInt(0, chapterTransitionLeadMS) - localStart
			shiftPracticalChapterTimings(chapter, shift)
			_, chapter.EndMS, _ = practicalChapterLocalRange(*chapter)
			if !blockFirstSet {
				block.StartMS = chapter.StartMS
				blockFirstSet = true
			}
			block.EndMS = chapter.EndMS
			cumulativeOffset = chapter.EndMS
			if chapterIndex < len(block.Chapters)-1 && chapterGapMS > 0 {
				cumulativeOffset += chapterGapMS
			}
		}

		if blockIndex < len(shifted.Blocks)-1 && blockGapMS > 0 {
			cumulativeOffset += blockGapMS
		}
	}

	return shifted
}

func practicalChapterLocalRange(chapter dto.PracticalChapter) (int, int, bool) {
	start := 0
	end := 0
	firstSet := false
	for _, turn := range chapter.Turns {
		if turn.EndMS <= turn.StartMS {
			continue
		}
		if !firstSet || turn.StartMS < start {
			start = turn.StartMS
		}
		if !firstSet || turn.EndMS > end {
			end = turn.EndMS
		}
		firstSet = true
	}
	return start, end, firstSet
}

func shiftPracticalChapterTimings(chapter *dto.PracticalChapter, shift int) {
	if chapter == nil || shift == 0 {
		return
	}
	for turnIndex := range chapter.Turns {
		turn := &chapter.Turns[turnIndex]
		if turn.StartMS > 0 {
			turn.StartMS += shift
		} else if shift > 0 {
			turn.StartMS = shift
		}
		if turn.EndMS > 0 {
			turn.EndMS += shift
		}
	}
}

func audioDurationMS(ctx context.Context, path string) (int, error) {
	sec, err := ffmpegcommon.AudioDurationSecContext(ctx, path)
	if err != nil {
		return 0, err
	}
	return int(sec * 1000.0), nil
}
