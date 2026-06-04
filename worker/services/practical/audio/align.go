package practical_audio_service

import (
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"unicode"

	"worker/pkg/mfa"
	"worker/pkg/x/fsx"
	services "worker/services"
	ffmpegcommon "worker/services/media/ffmpeg/common"
	dto "worker/services/practical/model"
)

type AlignInput struct {
	ProjectID   string
	Language    string
	BlockNums   []int
	ChapterNums []int
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

type practicalTurnAudioSlice struct {
	TurnIndex int
	StartMS   int
	EndMS     int
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
	requestedBlocks, err := buildRequestedBlockSet(input.BlockNums, len(script.Blocks))
	if err != nil {
		return AlignResult{}, err
	}
	requestedChapterNums, err := buildRequestedChapterSet(script, input.ChapterNums)
	if err != nil {
		return AlignResult{}, err
	}
	fullAlign := len(requestedBlocks) == 0 && len(requestedChapterNums) == 0

	reusableLocalScript := dto.PracticalScript{}
	if !fullAlign {
		reusableLocalScript, err = loadReusableLocalAlignedScript(projectDir, language)
		if err != nil {
			return AlignResult{}, err
		}
	}

	alignedScript, err := alignScriptTimings(ctx, projectDir, script, requestedBlocks, requestedChapterNums, fullAlign, reusableLocalScript)
	if err != nil {
		return AlignResult{}, err
	}
	dialoguePath, topicDurations, err := buildDialogueAudio(ctx, projectDir, alignedScript)
	if err != nil {
		return AlignResult{}, err
	}
	timelineScript := applyPracticalTimelineGaps(
		alignedScript,
		topicDurations,
		practicalChapterGapMS(),
		practicalBlockGapMS(),
		practicalChapterTransitionLeadMS(),
		practicalBlockTransitionLeadMS(),
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

func loadReusableLocalAlignedScript(projectDir, language string) (dto.PracticalScript, error) {
	alignedScriptPath := projectScriptAlignedPath(projectDir)
	if !fileExists(alignedScriptPath) {
		return dto.PracticalScript{}, services.NonRetryableError{
			Err: fmt.Errorf("partial practical align requires existing aligned script: %s", alignedScriptPath),
		}
	}
	script, err := loadScriptFromPath(language, alignedScriptPath)
	if err != nil {
		return dto.PracticalScript{}, err
	}
	return localizePracticalScriptTimings(script), nil
}

func alignScriptTimings(
	ctx context.Context,
	projectDir string,
	script dto.PracticalScript,
	requestedBlocks map[int]struct{},
	requestedChapterNums map[int]struct{},
	fullAlign bool,
	reusableLocalScript dto.PracticalScript,
) (dto.PracticalScript, error) {
	aligned := script
	var err error
	if !fullAlign {
		aligned, err = mergeReusableLocalChapterTimings(script, reusableLocalScript)
		if err != nil {
			return dto.PracticalScript{}, err
		}
	}
	client := newMFAClient()
	tempo := practicalTempo()
	turnGapMS := practicalTurnGapMS()
	if turnGapMS > 0 && (fullAlign || len(requestedChapterNums) > 0) {
		if err := createSilenceAudio(ctx, projectTurnGapPath(projectDir), turnGapMS); err != nil {
			return dto.PracticalScript{}, err
		}
	}

	chapterCursor := 0
	for blockIndex := range aligned.Blocks {
		block := &aligned.Blocks[blockIndex]
		finalTopicAudioPath := blockIntroAudioPath(projectDir, block.BlockID, blockIndex+1)
		if fullAlign || practicalSelectionContains(requestedBlocks, blockIndex+1) {
			rawTopicAudioPath := blockIntroRawAudioPath(projectDir, block.BlockID, blockIndex+1)
			if !fileExists(rawTopicAudioPath) {
				return dto.PracticalScript{}, services.NonRetryableError{
					Err: fmt.Errorf("block topic raw audio missing for block %s: %s", strings.TrimSpace(block.BlockID), rawTopicAudioPath),
				}
			}
			if err := renderTempoAdjustedAudio(ctx, rawTopicAudioPath, finalTopicAudioPath, tempo); err != nil {
				return dto.PracticalScript{}, err
			}
		} else if !fileExists(finalTopicAudioPath) {
			return dto.PracticalScript{}, services.NonRetryableError{
				Err: fmt.Errorf("block topic audio missing for block %s: %s", strings.TrimSpace(block.BlockID), finalTopicAudioPath),
			}
		}

		for chapterIndex := range block.Chapters {
			chapter := block.Chapters[chapterIndex]
			globalChapterNum := chapterCursor + chapterIndex + 1
			finalAudioPath := chapterAudioPath(projectDir, block.BlockID, chapter.ChapterID, blockIndex+1, chapterIndex+1)
			if !fullAlign && !practicalSelectionContains(requestedChapterNums, globalChapterNum) {
				if !fileExists(finalAudioPath) {
					return dto.PracticalScript{}, services.NonRetryableError{
						Err: fmt.Errorf("chapter audio missing for block %s chapter %s: %s", strings.TrimSpace(block.BlockID), strings.TrimSpace(chapter.ChapterID), finalAudioPath),
					}
				}
				if !practicalChapterHasLocalTiming(block.Chapters[chapterIndex]) {
					return dto.PracticalScript{}, services.NonRetryableError{
						Err: fmt.Errorf("chapter timing cache missing for block %s chapter %s", strings.TrimSpace(block.BlockID), strings.TrimSpace(chapter.ChapterID)),
					}
				}
				continue
			}

			rawAudioPath := chapterRawAudioPath(projectDir, block.BlockID, chapter.ChapterID, blockIndex+1, chapterIndex+1)
			if !fileExists(rawAudioPath) {
				return dto.PracticalScript{}, services.NonRetryableError{
					Err: fmt.Errorf("chapter audio missing for block %s chapter %s: %s", strings.TrimSpace(block.BlockID), strings.TrimSpace(chapter.ChapterID), rawAudioPath),
				}
			}
			tempoAudioPath := chapterTempoAudioPath(projectDir, block.BlockID, chapter.ChapterID, blockIndex+1, chapterIndex+1)
			if err := renderTempoAdjustedAudio(ctx, rawAudioPath, tempoAudioPath, tempo); err != nil {
				return dto.PracticalScript{}, err
			}

			durationMS, err := audioDurationMS(ctx, tempoAudioPath)
			if err != nil {
				return dto.PracticalScript{}, err
			}

			alignedChapter, err := alignChapterTimings(ctx, client, aligned.Language, projectDir, chapter, tempoAudioPath, durationMS)
			if err != nil {
				return dto.PracticalScript{}, err
			}

			finalChapter, err := materializeChapterAudioWithTurnGaps(ctx, projectDir, tempoAudioPath, finalAudioPath, alignedChapter, durationMS)
			if err != nil {
				return dto.PracticalScript{}, err
			}
			block.Chapters[chapterIndex] = finalChapter
		}
		chapterCursor += len(block.Chapters)
	}

	setPracticalLocalHierarchyTimings(&aligned)
	return aligned, nil
}

func alignChapterTimings(
	ctx context.Context,
	client *mfa.Client,
	language, workingDir string,
	chapter dto.PracticalChapter,
	audioPath string,
	durationMS int,
) (dto.PracticalChapter, error) {
	specs, transcript := buildChapterTimingSpecs(language, chapter)
	if len(specs) == 0 {
		return chapter, nil
	}

	if client == nil || !client.Enabled() {
		return dto.PracticalChapter{}, services.NonRetryableError{
			Err: fmt.Errorf("MFA alignment is required for practical chapter %s but MFA is not enabled", strings.TrimSpace(chapter.ChapterID)),
		}
	}

	words, err := client.AlignWords(ctx, mfa.AlignRequest{
		LanguageCode: language,
		AudioPath:    audioPath,
		Transcript:   transcript,
		WorkingDir:   workingDir,
	})
	chapterID := strings.TrimSpace(chapter.ChapterID)
	if err != nil {
		return dto.PracticalChapter{}, services.NonRetryableError{
			Err: fmt.Errorf("MFA chapter alignment failed for practical chapter %s: %w", chapterID, err),
		}
	}
	if len(words) == 0 {
		return dto.PracticalChapter{}, services.NonRetryableError{
			Err: fmt.Errorf("MFA returned no word timings for practical chapter %s", chapterID),
		}
	}
	aligned, ok := mapWordsToChapterTimings(chapter, specs, words, durationMS)
	if !ok {
		return dto.PracticalChapter{}, services.NonRetryableError{
			Err: fmt.Errorf("MFA produced incomplete turn alignment for practical chapter %s", chapterID),
		}
	}
	return aligned, nil
}

func buildChapterTimingSpecs(language string, chapter dto.PracticalChapter) ([]practicalTurnTimingSpec, string) {
	specs := make([]practicalTurnTimingSpec, 0, len(chapter.Turns))
	transcriptParts := make([]string, 0, len(chapter.Turns))
	cursor := 0
	for _, turn := range chapter.Turns {
		speechText := practicalSpeechText(turn)
		units := practicalAlignmentUnits(language, speechText)
		if len(units) == 0 {
			units = []string{strings.TrimSpace(speechText)}
		}
		normalized := normalizePracticalTextForAlignment(speechText)
		specs = append(specs, practicalTurnTimingSpec{
			UnitCount:       len(units),
			Units:           units,
			Normalized:      normalized,
			GlobalNormStart: cursor,
			GlobalNormEnd:   cursor + len(normalized),
		})
		transcriptParts = append(transcriptParts, practicalTranscriptTextForMFA(language, speechText, units))
		cursor += len(normalized)
	}
	return specs, strings.Join(transcriptParts, "\n")
}

func practicalTranscriptTextForMFA(language, text string, units []string) string {
	trimmed := strings.TrimSpace(text)
	if trimmed == "" {
		return ""
	}
	if strings.EqualFold(strings.TrimSpace(language), "ja") || strings.EqualFold(strings.TrimSpace(language), "ja-jp") {
		return trimmed
	}
	if len(units) == 0 {
		return trimmed
	}
	return strings.Join(units, " ")
}

func mapWordsToChapterTimings(chapter dto.PracticalChapter, specs []practicalTurnTimingSpec, words []mfa.WordTiming, durationMS int) (dto.PracticalChapter, bool) {
	transcript := buildPracticalTranscript(specs)
	if len(transcript) == 0 || len(words) == 0 {
		return dto.PracticalChapter{}, false
	}

	matches := matchPracticalWordsToTranscript(transcript, words)
	if len(matches) == 0 {
		return dto.PracticalChapter{}, false
	}

	windows := derivePracticalTurnWindows(specs, matches, durationMS)

	aligned := chapter
	matchedTurns := 0
	for turnIndex := range aligned.Turns {
		if turnIndex >= len(windows) {
			break
		}
		window := windows[turnIndex]
		if window.EndMS <= window.StartMS {
			window.EndMS = maxInt(window.StartMS+1, durationMS/maxInt(len(specs), 1))
		}
		aligned.Turns[turnIndex].StartMS = window.StartMS
		aligned.Turns[turnIndex].EndMS = window.EndMS
		if window.HasMatch {
			matchedTurns++
		}
	}
	return aligned, matchedTurns == len(specs)
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

func derivePracticalTurnWindows(specs []practicalTurnTimingSpec, matches []practicalTimedWordMatch, totalDurationMS int) []practicalTurnWindow {
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
		windowEnd := totalDurationMS
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
			startMS, endMS, hasRange := practicalChapterLocalRange(*chapter)
			if !hasRange {
				chapter.StartMS = 0
				chapter.EndMS = 0
				continue
			}
			chapter.StartMS = startMS
			chapter.EndMS = endMS
			if !blockFirstSet {
				block.StartMS = startMS
				blockFirstSet = true
			}
			block.EndMS = endMS
		}
	}
}

func localizePracticalScriptTimings(script dto.PracticalScript) dto.PracticalScript {
	localized := script
	for blockIndex := range localized.Blocks {
		block := &localized.Blocks[blockIndex]
		block.TopicStartMS = 0
		block.TopicEndMS = 0
		block.StartMS = 0
		block.EndMS = 0
		for chapterIndex := range block.Chapters {
			chapterLeadMS := 0
			if chapterIndex > 0 {
				chapterLeadMS = maxInt(0, practicalChapterTransitionLeadMS())
			}
			block.Chapters[chapterIndex] = localizePracticalChapterTimings(block.Chapters[chapterIndex], chapterLeadMS)
		}
	}
	setPracticalLocalHierarchyTimings(&localized)
	return localized
}

func localizePracticalChapterTimings(chapter dto.PracticalChapter, chapterLeadMS int) dto.PracticalChapter {
	localized := chapter
	baseStart := maxInt(0, chapter.StartMS)
	shiftBase := baseStart + maxInt(0, chapterLeadMS)
	localized.StartMS = 0
	if chapter.EndMS > shiftBase {
		localized.EndMS = chapter.EndMS - shiftBase
	} else {
		localized.EndMS = 0
	}
	for turnIndex := range localized.Turns {
		turn := &localized.Turns[turnIndex]
		if turn.StartMS > shiftBase {
			turn.StartMS -= shiftBase
		} else if turn.StartMS > 0 {
			turn.StartMS = 0
		}
		if turn.EndMS > shiftBase {
			turn.EndMS -= shiftBase
		} else {
			turn.EndMS = 0
		}
	}
	return localized
}

func mergeReusableLocalChapterTimings(script, reusable dto.PracticalScript) (dto.PracticalScript, error) {
	if len(script.Blocks) != len(reusable.Blocks) {
		return dto.PracticalScript{}, services.NonRetryableError{
			Err: fmt.Errorf("partial practical align requires matching block structure"),
		}
	}
	merged := script
	for blockIndex := range merged.Blocks {
		block := &merged.Blocks[blockIndex]
		reusableBlock := reusable.Blocks[blockIndex]
		if strings.TrimSpace(block.BlockID) != strings.TrimSpace(reusableBlock.BlockID) {
			return dto.PracticalScript{}, services.NonRetryableError{
				Err: fmt.Errorf("partial practical align block mismatch at index %d", blockIndex+1),
			}
		}
		if len(block.Chapters) != len(reusableBlock.Chapters) {
			return dto.PracticalScript{}, services.NonRetryableError{
				Err: fmt.Errorf("partial practical align requires matching chapter structure for block %s", strings.TrimSpace(block.BlockID)),
			}
		}
		for chapterIndex := range block.Chapters {
			mergedChapter, err := mergeReusableLocalChapterTiming(block.Chapters[chapterIndex], reusableBlock.Chapters[chapterIndex])
			if err != nil {
				return dto.PracticalScript{}, err
			}
			block.Chapters[chapterIndex] = mergedChapter
		}
	}
	return merged, nil
}

func mergeReusableLocalChapterTiming(chapter, reusable dto.PracticalChapter) (dto.PracticalChapter, error) {
	if strings.TrimSpace(chapter.ChapterID) != strings.TrimSpace(reusable.ChapterID) {
		return dto.PracticalChapter{}, services.NonRetryableError{
			Err: fmt.Errorf("partial practical align chapter mismatch for %s", strings.TrimSpace(chapter.ChapterID)),
		}
	}
	if len(chapter.Turns) != len(reusable.Turns) {
		return dto.PracticalChapter{}, services.NonRetryableError{
			Err: fmt.Errorf("partial practical align requires matching turn structure for chapter %s", strings.TrimSpace(chapter.ChapterID)),
		}
	}
	merged := chapter
	merged.StartMS = reusable.StartMS
	merged.EndMS = reusable.EndMS
	for turnIndex := range merged.Turns {
		if strings.TrimSpace(merged.Turns[turnIndex].TurnID) != strings.TrimSpace(reusable.Turns[turnIndex].TurnID) {
			return dto.PracticalChapter{}, services.NonRetryableError{
				Err: fmt.Errorf("partial practical align turn mismatch for chapter %s", strings.TrimSpace(chapter.ChapterID)),
			}
		}
		merged.Turns[turnIndex].StartMS = reusable.Turns[turnIndex].StartMS
		merged.Turns[turnIndex].EndMS = reusable.Turns[turnIndex].EndMS
	}
	return merged, nil
}

func practicalSelectionContains(values map[int]struct{}, target int) bool {
	if len(values) == 0 {
		return false
	}
	_, ok := values[target]
	return ok
}

func practicalChapterHasLocalTiming(chapter dto.PracticalChapter) bool {
	_, _, hasRange := practicalChapterLocalRange(chapter)
	return hasRange
}

func materializeChapterAudioWithTurnGaps(
	ctx context.Context,
	projectDir, rawAudioPath, outputPath string,
	chapter dto.PracticalChapter,
	rawDurationMS int,
) (dto.PracticalChapter, error) {
	if err := os.MkdirAll(filepath.Dir(outputPath), 0o755); err != nil {
		return dto.PracticalChapter{}, err
	}

	turnGapMS := practicalTurnGapMS()
	if turnGapMS <= 0 {
		if err := fsx.CopyFile(rawAudioPath, outputPath); err != nil {
			return dto.PracticalChapter{}, err
		}
		chapter.StartMS = 0
		chapter.EndMS = maxInt(0, rawDurationMS)
		return chapter, nil
	}

	shiftedChapter, slices := applyPracticalTurnGapToChapterTimings(chapter, turnGapMS, rawDurationMS)
	if len(slices) == 0 {
		if err := fsx.CopyFile(rawAudioPath, outputPath); err != nil {
			return dto.PracticalChapter{}, err
		}
		return shiftedChapter, nil
	}

	turnGapPath := projectTurnGapPath(projectDir)
	if !fileExists(turnGapPath) {
		return dto.PracticalChapter{}, services.NonRetryableError{Err: fmt.Errorf("turn gap audio missing: %s", turnGapPath)}
	}

	files := make([]string, 0, len(slices)*2)
	chunkPaths := make([]string, 0, len(slices))
	defer func() {
		for _, path := range chunkPaths {
			_ = os.Remove(path)
		}
	}()

	for sliceIndex, slice := range slices {
		if sliceIndex > 0 {
			files = append(files, turnGapPath)
		}
		chunkFile, err := os.CreateTemp(projectDir, fmt.Sprintf("turn_%02d_*.wav", slice.TurnIndex+1))
		if err != nil {
			return dto.PracticalChapter{}, err
		}
		chunkPath := chunkFile.Name()
		if err := chunkFile.Close(); err != nil {
			_ = os.Remove(chunkPath)
			return dto.PracticalChapter{}, err
		}
		if err := extractAudioChunk(ctx, rawAudioPath, chunkPath, slice.StartMS, slice.EndMS); err != nil {
			_ = os.Remove(chunkPath)
			return dto.PracticalChapter{}, err
		}
		chunkPaths = append(chunkPaths, chunkPath)
		files = append(files, chunkPath)
	}

	if err := concatAudioFiles(ctx, projectDir, files, outputPath); err != nil {
		return dto.PracticalChapter{}, err
	}
	return shiftedChapter, nil
}

func applyPracticalTurnGapToChapterTimings(chapter dto.PracticalChapter, turnGapMS, rawDurationMS int) (dto.PracticalChapter, []practicalTurnAudioSlice) {
	shifted := chapter
	shifted.Turns = append([]dto.PracticalTurn(nil), chapter.Turns...)
	slices := make([]practicalTurnAudioSlice, 0, len(chapter.Turns))
	validTurnIndexes := make([]int, 0, len(chapter.Turns))
	for turnIndex, turn := range chapter.Turns {
		shifted.Turns[turnIndex].StartMS = 0
		shifted.Turns[turnIndex].EndMS = 0
		if turn.EndMS > turn.StartMS {
			validTurnIndexes = append(validTurnIndexes, turnIndex)
		}
	}
	if len(validTurnIndexes) == 0 {
		shifted.StartMS = 0
		shifted.EndMS = maxInt(0, rawDurationMS)
		return shifted, nil
	}

	cumulativeGap := 0
	segmentStartMS := 0
	for position, turnIndex := range validTurnIndexes {
		turn := chapter.Turns[turnIndex]
		shifted.Turns[turnIndex].StartMS = turn.StartMS + cumulativeGap
		shifted.Turns[turnIndex].EndMS = turn.EndMS + cumulativeGap
		segmentEndMS := turn.EndMS
		if position == len(validTurnIndexes)-1 {
			segmentEndMS = maxInt(segmentEndMS, rawDurationMS)
		}
		slices = append(slices, practicalTurnAudioSlice{
			TurnIndex: turnIndex,
			StartMS:   segmentStartMS,
			EndMS:     segmentEndMS,
		})
		segmentStartMS = turn.EndMS
		if position < len(validTurnIndexes)-1 {
			cumulativeGap += maxInt(0, turnGapMS)
		}
	}

	shifted.StartMS = 0
	shifted.EndMS = maxInt(maxInt(0, rawDurationMS), segmentStartMS) + maxInt(0, turnGapMS)*(len(validTurnIndexes)-1)
	return shifted, slices
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
	topicDurations := make([]int, len(script.Blocks))

	chapterGapPath := projectChapterGapPath(projectDir)
	blockGapPath := projectBlockGapPath(projectDir)
	chapterTransitionLeadPath := projectChapterTransitionLeadPath(projectDir)
	blockTransitionLeadPath := projectBlockTransitionLeadPath(projectDir)
	for blockIndex, block := range script.Blocks {
		introPath := blockIntroAudioPath(projectDir, block.BlockID, blockIndex+1)
		if !fileExists(introPath) {
			return "", nil, services.NonRetryableError{Err: fmt.Errorf("block topic audio missing: %s", introPath)}
		}
		topicDurationMS, err := audioDurationMS(ctx, introPath)
		if err != nil {
			return "", nil, err
		}
		topicDurations[blockIndex] = topicDurationMS
		files = append(files, introPath)

		for chapterIndex, chapter := range block.Chapters {
			_, _, hasTurns := practicalChapterLocalRange(chapter)
			if !hasTurns {
				continue
			}

			chapterPath := chapterAudioPath(projectDir, block.BlockID, chapter.ChapterID, blockIndex+1, chapterIndex+1)
			if !fileExists(chapterPath) {
				return "", nil, services.NonRetryableError{
					Err: fmt.Errorf("chapter audio missing for block %s chapter %s: %s", strings.TrimSpace(block.BlockID), strings.TrimSpace(chapter.ChapterID), chapterPath),
				}
			}
			if chapterIndex == 0 {
				if blockTransitionLeadMS > 0 {
					if !fileExists(blockTransitionLeadPath) {
						return "", nil, services.NonRetryableError{Err: fmt.Errorf("block transition lead audio missing: %s", blockTransitionLeadPath)}
					}
					files = append(files, blockTransitionLeadPath)
				}
			} else if chapterTransitionLeadMS > 0 {
				if !fileExists(chapterTransitionLeadPath) {
					return "", nil, services.NonRetryableError{Err: fmt.Errorf("chapter transition lead audio missing: %s", chapterTransitionLeadPath)}
				}
				files = append(files, chapterTransitionLeadPath)
			}
			files = append(files, chapterPath)

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
	return outputPath, topicDurations, nil
}

func applyPracticalTimelineGaps(script dto.PracticalScript, topicDurations []int, chapterGapMS, blockGapMS, chapterTransitionLeadMS, blockTransitionLeadMS int) dto.PracticalScript {
	shifted := script
	cumulativeOffset := 0

	for blockIndex := range shifted.Blocks {
		block := &shifted.Blocks[blockIndex]
		block.TopicStartMS = 0
		block.TopicEndMS = 0
		block.StartMS = 0
		block.EndMS = 0
		blockFirstSet := false
		topicDurationMS := 0
		if blockIndex < len(topicDurations) {
			topicDurationMS = maxInt(0, topicDurations[blockIndex])
		}
		if topicDurationMS > 0 {
			block.TopicStartMS = cumulativeOffset
			block.TopicEndMS = cumulativeOffset + topicDurationMS
			cumulativeOffset = block.TopicEndMS
			block.StartMS = block.TopicStartMS
			blockFirstSet = true
		}

		for chapterIndex := range block.Chapters {
			chapter := &block.Chapters[chapterIndex]
			localStart, localEnd, hasRange := practicalChapterLocalRange(*chapter)
			if !hasRange {
				chapter.StartMS = 0
				chapter.EndMS = 0
				continue
			}

			chapterLeadMS := maxInt(0, chapterTransitionLeadMS)
			chapterStartMS := cumulativeOffset
			if chapterIndex == 0 {
				chapterLeadMS = 0
				chapterStartMS += maxInt(0, blockTransitionLeadMS)
			}

			chapter.StartMS = chapterStartMS
			shift := chapterStartMS + chapterLeadMS - localStart
			shiftPracticalChapterTimings(chapter, shift)
			chapter.EndMS = chapterStartMS + chapterLeadMS + maxInt(0, localEnd-localStart)
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
	if chapter.EndMS > chapter.StartMS {
		return chapter.StartMS, chapter.EndMS, true
	}
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
