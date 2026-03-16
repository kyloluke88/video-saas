package podcast_audio_service

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"

	"worker/internal/dto"
	conf "worker/pkg/config"
	ffmpegcommon "worker/services/ffmpeg_service/common"
)

type templateProject struct {
	audioPath  string
	script     dto.PodcastScript
	durationMS int
}

func applyStartEndTemplates(projectID, dialoguePath string, script dto.PodcastScript) (dto.PodcastScript, error) {
	language, ok := templateLanguageKey(script.Language)
	if !ok {
		return script, nil
	}

	openTpl, openOK, err := loadTemplateProject(language, "open")
	if err != nil {
		return dto.PodcastScript{}, err
	}
	closeTpl, closeOK, err := loadTemplateProject(language, "close")
	if err != nil {
		return dto.PodcastScript{}, err
	}
	if !openOK && !closeOK {
		return script, nil
	}

	body := script
	body.SyncBlocksFromSegments()
	namespaceScriptIDs(&body, "body")

	audioInputs := make([]string, 0, 3)
	bodyDurationMS, err := audioDurationMS(dialoguePath)
	if err != nil {
		return dto.PodcastScript{}, err
	}
	gapMS := segmentGapMSForLanguage(language)
	gapPath, err := ensureTemplateBoundaryGap(filepath.Dir(dialoguePath), gapMS)
	if err != nil {
		return dto.PodcastScript{}, err
	}

	mergedBlocks := make([]dto.PodcastBlock, 0, len(body.Blocks)+2)
	mergedChapters := make([]dto.PodcastYouTubeChapter, 0, len(body.YouTube.Chapters)+2)
	offsetMS := 0
	if openOK {
		openScript := openTpl.script
		namespaceScriptIDs(&openScript, "open")
		audioInputs = appendConcatPath(audioInputs, openTpl.audioPath, gapMS > 0 && gapPath != "", gapPath)
		shiftScriptTiming(&body, openTpl.durationMS+gapMS)
		offsetMS += openTpl.durationMS
		if gapMS > 0 {
			offsetMS += gapMS
		}
		mergedBlocks = append(mergedBlocks, openScript.Blocks...)
		mergedChapters = append(mergedChapters, openScript.YouTube.Chapters...)
	}

	mergedBlocks = append(mergedBlocks, body.Blocks...)
	mergedChapters = append(mergedChapters, body.YouTube.Chapters...)
	audioInputs = appendConcatPath(audioInputs, dialoguePath, closeOK && gapMS > 0 && gapPath != "", gapPath)

	if closeOK {
		closeScript := closeTpl.script
		namespaceScriptIDs(&closeScript, "close")
		closeOffsetMS := offsetMS + bodyDurationMS
		if gapMS > 0 {
			closeOffsetMS += gapMS
		}
		shiftScriptTiming(&closeScript, closeOffsetMS)
		mergedBlocks = append(mergedBlocks, closeScript.Blocks...)
		mergedChapters = append(mergedChapters, closeScript.YouTube.Chapters...)
		audioInputs = append(audioInputs, closeTpl.audioPath)
	}

	combined := body
	combined.Blocks = mergedBlocks
	combined.YouTube.Chapters = mergedChapters
	combined.RefreshSegmentsFromBlocks()

	mergedAudioPath := filepath.Join(filepath.Dir(dialoguePath), "dialogue_merged.mp3")
	if err := concatAudioFiles(filepath.Dir(dialoguePath), audioInputs, mergedAudioPath); err != nil {
		return dto.PodcastScript{}, err
	}
	if err := os.Rename(mergedAudioPath, dialoguePath); err != nil {
		return dto.PodcastScript{}, err
	}

	log.Printf("🧩 start/end template merged project_id=%s language=%s open=%t close=%t output=%s",
		projectID, language, openOK, closeOK, dialoguePath)
	return combined, nil
}

func templateLanguageKey(language string) (string, bool) {
	normalized := normalizeLanguage(language)
	if isJapaneseLanguage(normalized) {
		return "ja", true
	}
	if strings.HasPrefix(strings.ToLower(strings.TrimSpace(normalized)), "zh") {
		return "zh", true
	}
	return "", false
}

func ensureTemplateBoundaryGap(projectDir string, gapMS int) (string, error) {
	if gapMS <= 0 {
		return "", nil
	}
	path := filepath.Join(projectDir, fmt.Sprintf("template_boundary_gap_%dms.mp3", gapMS))
	if fileExists(path) {
		return path, nil
	}
	if err := createSilenceAudio(path, gapMS); err != nil {
		return "", err
	}
	return path, nil
}

func loadTemplateProject(language, kind string) (templateProject, bool, error) {
	dir, ok := templateProjectDir(language, kind)
	if !ok {
		return templateProject{}, false, nil
	}

	audioPath := filepath.Join(dir, "dialogue.mp3")
	scriptPath := filepath.Join(dir, "script_aligned.json")
	if !fileExists(audioPath) || !fileExists(scriptPath) {
		return templateProject{}, false, nil
	}

	var script dto.PodcastScript
	if err := readJSON(scriptPath, &script); err != nil {
		return templateProject{}, false, fmt.Errorf("read %s template script failed: %w", kind, err)
	}
	normalizeTemplateScriptKind(&script, kind)
	durationMS, err := audioDurationMS(audioPath)
	if err != nil {
		return templateProject{}, false, err
	}

	return templateProject{
		audioPath:  audioPath,
		script:     script,
		durationMS: durationMS,
	}, true, nil
}

func templateProjectDir(language, kind string) (string, bool) {
	base := filepath.Join(conf.Get[string]("worker.worker_assets_dir"), "podcast", "start_end_template")
	candidates := []string{
		filepath.Join(base, language+"_"+kind),
		filepath.Join(base, kind+"_"+language),
		filepath.Join(base, language, kind),
		filepath.Join(base, language+"_channel_"+kind),
		filepath.Join(base, kind+"_channel_"+language),
	}
	for _, candidate := range candidates {
		if info, err := os.Stat(candidate); err == nil && info.IsDir() {
			return candidate, true
		}
	}
	return "", false
}

func namespaceScriptIDs(script *dto.PodcastScript, prefix string) {
	if script == nil {
		return
	}
	prefix = strings.TrimSpace(prefix)
	if prefix == "" {
		return
	}

	for i := range script.YouTube.Chapters {
		if id := strings.TrimSpace(script.YouTube.Chapters[i].ChapterID); id != "" {
			script.YouTube.Chapters[i].ChapterID = prefix + ":" + id
		}
		for j := range script.YouTube.Chapters[i].BlockIDs {
			if id := strings.TrimSpace(script.YouTube.Chapters[i].BlockIDs[j]); id != "" {
				script.YouTube.Chapters[i].BlockIDs[j] = prefix + ":" + id
			}
		}
	}

	for i := range script.Blocks {
		if id := strings.TrimSpace(script.Blocks[i].ChapterID); id != "" {
			script.Blocks[i].ChapterID = prefix + ":" + id
		}
		if id := strings.TrimSpace(script.Blocks[i].TTSBlockID); id != "" {
			script.Blocks[i].TTSBlockID = prefix + ":" + id
		}
		for j := range script.Blocks[i].Segments {
			if id := strings.TrimSpace(script.Blocks[i].Segments[j].SegmentID); id != "" {
				script.Blocks[i].Segments[j].SegmentID = prefix + ":" + id
			}
		}
	}
	script.RefreshSegmentsFromBlocks()
}

func normalizeTemplateScriptKind(script *dto.PodcastScript, kind string) {
	if script == nil {
		return
	}
	if strings.TrimSpace(kind) != "close" {
		return
	}
	for i := range script.Blocks {
		script.Blocks[i].MacroBlock = "channel_cta"
		for j := range script.Blocks[i].Segments {
			script.Blocks[i].Segments[j].Summary = false
		}
	}
	script.RefreshSegmentsFromBlocks()
}

func shiftScriptTiming(script *dto.PodcastScript, offsetMS int) {
	if script == nil || offsetMS == 0 {
		return
	}
	for i := range script.Blocks {
		for j := range script.Blocks[i].Segments {
			shiftSegmentTiming(&script.Blocks[i].Segments[j], offsetMS)
		}
	}
	script.RefreshSegmentsFromBlocks()
}

func shiftSegmentTiming(seg *dto.PodcastSegment, offsetMS int) {
	if seg == nil || offsetMS == 0 {
		return
	}
	if seg.StartMS > 0 || seg.EndMS > 0 {
		seg.StartMS += offsetMS
		seg.EndMS += offsetMS
	}
	for i := range seg.Tokens {
		if seg.Tokens[i].StartMS > 0 || seg.Tokens[i].EndMS > 0 {
			seg.Tokens[i].StartMS += offsetMS
			seg.Tokens[i].EndMS += offsetMS
		}
	}
	for i := range seg.Chars {
		if seg.Chars[i].StartMS > 0 || seg.Chars[i].EndMS > 0 {
			seg.Chars[i].StartMS += offsetMS
			seg.Chars[i].EndMS += offsetMS
		}
	}
}

func audioDurationMS(path string) (int, error) {
	sec, err := ffmpegcommon.AudioDurationSec(path)
	if err != nil {
		return 0, err
	}
	return int(sec * 1000), nil
}
