package practical_audio_service

import (
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
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
	TTSType     int
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

type practicalSilenceInterval struct {
	StartMS int
	EndMS   int
}

type practicalPreparedChapterAlignment struct {
	AlignedChapter  dto.PracticalChapter
	SourceAudioPath string
	DurationMS      int
}

type practicalAlignmentStrategy struct {
	MaterializeBlockTopic func(ctx context.Context, rawTopicAudioPath, finalTopicAudioPath string) error
	PrepareChapter        func(ctx context.Context, projectDir, language string, block dto.PracticalBlock, blockIndex int, chapter dto.PracticalChapter, chapterIndex int) (practicalPreparedChapterAlignment, error)
}

const (
	practicalTurnBoundarySilenceMinMS   = 80
	practicalTurnBoundaryLookAheadMS    = 450
	practicalTurnBoundarySilenceNoiseDB = "-35dB"
)

var (
	practicalSilenceStartPattern = regexp.MustCompile(`silence_start:\s*([0-9.]+)`)
	practicalSilenceEndPattern   = regexp.MustCompile(`silence_end:\s*([0-9.]+)`)
)

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
	log.Printf("🎯 practical align start project_id=%s mode=%s blocks=%d selected_blocks=%d selected_chapters=%d",
		input.ProjectID,
		map[bool]string{true: "full", false: "partial"}[fullAlign],
		len(script.Blocks),
		len(requestedBlocks),
		len(requestedChapterNums),
	)

	reusableLocalScript := dto.PracticalScript{}
	if !fullAlign {
		var reusableFound bool
		reusableLocalScript, reusableFound, err = loadReusableLocalAlignedScript(projectDir, language)
		if err != nil {
			return AlignResult{}, err
		}
		if !reusableFound {
			if practicalCanRecoverFullAlignFromLocalAudio(projectDir, script) {
				fullAlign = true
				log.Printf("⚠️ practical partial align missing aligned cache, falling back to full align recovery project_id=%s", input.ProjectID)
			} else {
				return AlignResult{}, services.NonRetryableError{
					Err: fmt.Errorf("partial practical align requires existing aligned script: %s", projectScriptAlignedPath(projectDir)),
				}
			}
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

func loadReusableLocalAlignedScript(projectDir, language string) (dto.PracticalScript, bool, error) {
	alignedScriptPath := projectScriptAlignedPath(projectDir)
	if !fileExists(alignedScriptPath) {
		return dto.PracticalScript{}, false, nil
	}
	script, err := loadScriptFromPath(language, alignedScriptPath)
	if err != nil {
		return dto.PracticalScript{}, false, err
	}
	return localizePracticalScriptTimings(script), true, nil
}

func practicalCanRecoverFullAlignFromLocalAudio(projectDir string, script dto.PracticalScript) bool {
	for blockIndex, block := range script.Blocks {
		if !fileExists(blockIntroRawAudioPath(projectDir, block.BlockID, blockIndex+1)) {
			return false
		}
		for chapterIndex, chapter := range block.Chapters {
			if !fileExists(chapterRawAudioPath(projectDir, block.BlockID, chapter.ChapterID, blockIndex+1, chapterIndex+1)) {
				return false
			}
		}
	}
	return true
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
	strategy := newPracticalAlignmentStrategy()

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
			if err := strategy.MaterializeBlockTopic(ctx, rawTopicAudioPath, finalTopicAudioPath); err != nil {
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
			log.Printf("🧩 practical align chapter start project_id=%s block=%03d chapter=%03d block_id=%s chapter_id=%s",
				filepath.Base(projectDir),
				blockIndex+1,
				globalChapterNum,
				strings.TrimSpace(block.BlockID),
				strings.TrimSpace(chapter.ChapterID),
			)
			preparedChapter, err := strategy.PrepareChapter(ctx, projectDir, aligned.Language, *block, blockIndex+1, chapter, chapterIndex+1)
			if err != nil {
				return dto.PracticalScript{}, err
			}

			finalChapter, err := materializeAlignedChapterAudio(ctx, preparedChapter.SourceAudioPath, finalAudioPath, preparedChapter.AlignedChapter, preparedChapter.DurationMS)
			if err != nil {
				return dto.PracticalScript{}, err
			}
			block.Chapters[chapterIndex] = finalChapter
			log.Printf("✅ practical align chapter done project_id=%s block=%03d chapter=%03d chapter_id=%s duration_ms=%d turns=%d",
				filepath.Base(projectDir),
				blockIndex+1,
				globalChapterNum,
				strings.TrimSpace(chapter.ChapterID),
				preparedChapter.DurationMS,
				len(finalChapter.Turns),
			)
		}
		chapterCursor += len(block.Chapters)
	}

	setPracticalLocalHierarchyTimings(&aligned)
	return aligned, nil
}

func newPracticalAlignmentStrategy() practicalAlignmentStrategy {
	client := newMFAClient()
	tempo := practicalTempo()
	return practicalAlignmentStrategy{
		MaterializeBlockTopic: func(ctx context.Context, rawTopicAudioPath, finalTopicAudioPath string) error {
			return renderTempoAdjustedAudio(ctx, rawTopicAudioPath, finalTopicAudioPath, tempo)
		},
		PrepareChapter: func(ctx context.Context, projectDir, language string, block dto.PracticalBlock, blockIndex int, chapter dto.PracticalChapter, chapterIndex int) (practicalPreparedChapterAlignment, error) {
			rawAudioPath := chapterRawAudioPath(projectDir, block.BlockID, chapter.ChapterID, blockIndex, chapterIndex)
			tempoAudioPath := chapterTempoAudioPath(projectDir, block.BlockID, chapter.ChapterID, blockIndex, chapterIndex)
			if err := renderTempoAdjustedAudio(ctx, rawAudioPath, tempoAudioPath, tempo); err != nil {
				return practicalPreparedChapterAlignment{}, err
			}

			durationMS, err := audioDurationMS(ctx, tempoAudioPath)
			if err != nil {
				return practicalPreparedChapterAlignment{}, err
			}

			alignedChapter, err := alignChapterTimings(ctx, client, language, projectDir, chapter, tempoAudioPath, durationMS)
			if err != nil {
				return practicalPreparedChapterAlignment{}, err
			}
			silenceIntervals, err := detectPracticalSilenceIntervals(ctx, tempoAudioPath, durationMS)
			if err != nil {
				return practicalPreparedChapterAlignment{}, err
			}
			alignedChapter = stabilizePracticalChapterTimings(alignedChapter, silenceIntervals, durationMS)
			return practicalPreparedChapterAlignment{
				AlignedChapter:  alignedChapter,
				SourceAudioPath: tempoAudioPath,
				DurationMS:      durationMS,
			}, nil
		},
	}
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

func materializeAlignedChapterAudio(
	_ context.Context,
	sourceAudioPath, outputPath string,
	chapter dto.PracticalChapter,
	audioDurationMS int,
) (dto.PracticalChapter, error) {
	if err := os.MkdirAll(filepath.Dir(outputPath), 0o755); err != nil {
		return dto.PracticalChapter{}, err
	}
	if err := fsx.CopyFile(sourceAudioPath, outputPath); err != nil {
		return dto.PracticalChapter{}, err
	}
	finalChapter := chapter
	finalChapter.StartMS = 0
	finalChapter.EndMS = maxInt(0, audioDurationMS)
	return finalChapter, nil
}

func stabilizePracticalChapterTimings(chapter dto.PracticalChapter, intervals []practicalSilenceInterval, audioDurationMS int) dto.PracticalChapter {
	stabilized := chapter
	if audioDurationMS < 0 {
		audioDurationMS = 0
	}
	stabilized.StartMS = 0
	stabilized.EndMS = audioDurationMS
	if len(stabilized.Turns) == 0 {
		return stabilized
	}
	for turnIndex := range stabilized.Turns {
		turn := &stabilized.Turns[turnIndex]
		if turn.EndMS <= turn.StartMS {
			continue
		}
		nextTurnStartMS := audioDurationMS
		for nextIndex := turnIndex + 1; nextIndex < len(stabilized.Turns); nextIndex++ {
			nextTurn := stabilized.Turns[nextIndex]
			if nextTurn.EndMS <= nextTurn.StartMS {
				continue
			}
			nextTurnStartMS = nextTurn.StartMS
			break
		}
		turn.EndMS = practicalTurnEndAtNextSilence(turn.EndMS, nextTurnStartMS, intervals)
		if turn.EndMS > audioDurationMS {
			turn.EndMS = audioDurationMS
		}
	}
	return stabilized
}

func practicalTurnEndAtNextSilence(boundaryMS, nextTurnStartMS int, intervals []practicalSilenceInterval) int {
	if len(intervals) == 0 {
		return boundaryMS
	}
	lookAheadLimit := boundaryMS + practicalTurnBoundaryLookAheadMS
	if nextTurnStartMS > boundaryMS {
		lookAheadLimit = minInt(lookAheadLimit, nextTurnStartMS)
	}
	for _, interval := range intervals {
		if interval.EndMS <= boundaryMS {
			continue
		}
		if interval.StartMS > lookAheadLimit {
			break
		}
		if interval.StartMS <= boundaryMS {
			return boundaryMS
		}
		return interval.StartMS
	}
	return boundaryMS
}

func detectPracticalSilenceIntervals(ctx context.Context, sourcePath string, rawDurationMS int) ([]practicalSilenceInterval, error) {
	if strings.TrimSpace(sourcePath) == "" {
		return nil, nil
	}
	output, err := ffmpegcommon.RunFFmpegOutputContext(
		ctx,
		"-hide_banner",
		"-i", sourcePath,
		"-af", fmt.Sprintf("silencedetect=noise=%s:d=%.3f", practicalTurnBoundarySilenceNoiseDB, float64(practicalTurnBoundarySilenceMinMS)/1000.0),
		"-f", "null",
		"-",
	)
	if err != nil {
		return nil, err
	}
	return parsePracticalSilenceIntervals(output, rawDurationMS), nil
}

func parsePracticalSilenceIntervals(output string, rawDurationMS int) []practicalSilenceInterval {
	intervals := make([]practicalSilenceInterval, 0, 16)
	pendingStartMS := -1
	for _, line := range strings.Split(output, "\n") {
		if match := practicalSilenceStartPattern.FindStringSubmatch(line); len(match) == 2 {
			if startMS, ok := parsePracticalSilenceMS(match[1]); ok {
				pendingStartMS = startMS
			}
			continue
		}
		if match := practicalSilenceEndPattern.FindStringSubmatch(line); len(match) == 2 {
			endMS, ok := parsePracticalSilenceMS(match[1])
			if !ok {
				continue
			}
			startMS := 0
			if pendingStartMS >= 0 {
				startMS = pendingStartMS
			}
			pendingStartMS = -1
			if endMS > startMS {
				intervals = append(intervals, practicalSilenceInterval{StartMS: startMS, EndMS: endMS})
			}
		}
	}
	if pendingStartMS >= 0 && rawDurationMS > pendingStartMS {
		intervals = append(intervals, practicalSilenceInterval{StartMS: pendingStartMS, EndMS: rawDurationMS})
	}
	return intervals
}

func parsePracticalSilenceMS(value string) (int, bool) {
	seconds, err := strconv.ParseFloat(strings.TrimSpace(value), 64)
	if err != nil {
		return 0, false
	}
	return int(seconds * 1000.0), true
}

func buildDialogueAudio(ctx context.Context, projectDir string, script dto.PracticalScript) (string, []int, error) {
	chapterGapMS := practicalChapterGapMS()
	blockGapMS := practicalBlockGapMS()
	chapterTransitionLeadMS := practicalChapterTransitionLeadMS()
	blockTransitionLeadMS := practicalBlockTransitionLeadMS()
	referenceAudioPath := practicalDialogueReferenceAudioPath(projectDir, script)
	if chapterGapMS > 0 {
		if err := createPracticalDialogueGapAudio(ctx, projectChapterGapPath(projectDir), chapterGapMS, referenceAudioPath); err != nil {
			return "", nil, err
		}
	}
	if blockGapMS > 0 {
		if err := createPracticalDialogueGapAudio(ctx, projectBlockGapPath(projectDir), blockGapMS, referenceAudioPath); err != nil {
			return "", nil, err
		}
	}
	if chapterTransitionLeadMS > 0 {
		if err := createPracticalDialogueGapAudio(ctx, projectChapterTransitionLeadPath(projectDir), chapterTransitionLeadMS, referenceAudioPath); err != nil {
			return "", nil, err
		}
	}
	if blockTransitionLeadMS > 0 {
		if err := createPracticalDialogueGapAudio(ctx, projectBlockTransitionLeadPath(projectDir), blockTransitionLeadMS, referenceAudioPath); err != nil {
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

func createPracticalDialogueGapAudio(ctx context.Context, outputPath string, durationMS int, referenceAudioPath string) error {
	if strings.TrimSpace(referenceAudioPath) == "" {
		return createSilenceAudio(ctx, outputPath, durationMS)
	}
	return createSilenceAudioLike(ctx, outputPath, durationMS, referenceAudioPath)
}

func practicalDialogueReferenceAudioPath(projectDir string, script dto.PracticalScript) string {
	for blockIndex, block := range script.Blocks {
		introPath := blockIntroAudioPath(projectDir, block.BlockID, blockIndex+1)
		if fileExists(introPath) {
			return introPath
		}
		for chapterIndex, chapter := range block.Chapters {
			chapterPath := chapterAudioPath(projectDir, block.BlockID, chapter.ChapterID, blockIndex+1, chapterIndex+1)
			if fileExists(chapterPath) {
				return chapterPath
			}
		}
	}
	return ""
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
