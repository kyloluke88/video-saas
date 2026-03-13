package podcast_audio_service

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"worker/internal/dto"
)

func exportYouTubePublishFiles(projectDir string, script dto.PodcastScript) error {
	if isEmptyYouTubeMetadata(script.YouTube) {
		return nil
	}

	if err := writeJSON(filepath.Join(projectDir, "youtube_publish.json"), script.YouTube); err != nil {
		return err
	}

	content := buildYouTubePublishText(script)
	if strings.TrimSpace(content) == "" {
		return nil
	}
	return os.WriteFile(filepath.Join(projectDir, "youtube_publish.txt"), []byte(content), 0o644)
}

func buildYouTubePublishText(script dto.PodcastScript) string {
	var lines []string

	if title := strings.TrimSpace(script.YouTube.PublishTitle); title != "" {
		lines = append(lines, "Title:")
		lines = append(lines, title, "")
	}

	chapterLines := buildYouTubeChapterLines(script)
	if len(chapterLines) > 0 {
		lines = append(lines, "Timestamps:")
		lines = append(lines, chapterLines...)
		lines = append(lines, "")
	}

	learn := compactNonEmpty(script.YouTube.InThisEpisodeYouWillLearn)
	if len(learn) > 0 {
		lines = append(lines, "In this episode, you will learn:")
		for _, item := range learn {
			lines = append(lines, "- "+item)
		}
		lines = append(lines, "")
	}

	desc := compactNonEmpty(script.YouTube.DescriptionIntro)
	if len(desc) > 0 {
		lines = append(lines, "Description:")
		lines = append(lines, desc...)
		lines = append(lines, "")
	}

	return strings.TrimSpace(strings.Join(lines, "\n")) + "\n"
}

func buildYouTubeChapterLines(script dto.PodcastScript) []string {
	blockStartMS := make(map[string]int, len(script.Blocks))
	chapterStartMS := make(map[string]int, len(script.Blocks))
	for _, block := range script.Blocks {
		startMS, ok := podcastBlockStartMS(block)
		if !ok {
			continue
		}
		if blockID := strings.TrimSpace(block.TTSBlockID); blockID != "" {
			blockStartMS[blockID] = startMS
		}
		if chapterID := strings.TrimSpace(block.ChapterID); chapterID != "" {
			if current, exists := chapterStartMS[chapterID]; !exists || startMS < current {
				chapterStartMS[chapterID] = startMS
			}
		}
	}

	type chapterLine struct {
		startMS int
		title   string
	}
	items := make([]chapterLine, 0, len(script.YouTube.Chapters))
	for _, chapter := range script.YouTube.Chapters {
		startMS, ok := chapterStartMSForPublish(chapter, blockStartMS, chapterStartMS)
		if !ok {
			continue
		}
		title := strings.TrimSpace(chapter.TitleEN)
		if title == "" {
			title = strings.TrimSpace(chapter.TitleJA)
		}
		if title == "" {
			title = strings.TrimSpace(chapter.TitleZH)
		}
		if title == "" {
			continue
		}
		items = append(items, chapterLine{startMS: startMS, title: title})
	}
	sort.SliceStable(items, func(i, j int) bool {
		if items[i].startMS == items[j].startMS {
			return items[i].title < items[j].title
		}
		return items[i].startMS < items[j].startMS
	})

	lines := make([]string, 0, len(items))
	seen := make(map[string]struct{}, len(items))
	for _, item := range items {
		line := fmt.Sprintf("%s - %s", formatTimestampMMSS(item.startMS), item.title)
		if _, exists := seen[line]; exists {
			continue
		}
		seen[line] = struct{}{}
		lines = append(lines, line)
	}
	return lines
}

func chapterStartMSForPublish(chapter dto.PodcastYouTubeChapter, blockStartMS, chapterStartMS map[string]int) (int, bool) {
	startMS := 0
	found := false
	for _, blockID := range chapter.BlockIDs {
		blockID = strings.TrimSpace(blockID)
		if blockID == "" {
			continue
		}
		value, ok := blockStartMS[blockID]
		if !ok {
			continue
		}
		if !found || value < startMS {
			startMS = value
			found = true
		}
	}
	if found {
		return startMS, true
	}
	if chapterID := strings.TrimSpace(chapter.ChapterID); chapterID != "" {
		value, ok := chapterStartMS[chapterID]
		if ok {
			return value, true
		}
	}
	return 0, false
}

func podcastBlockStartMS(block dto.PodcastBlock) (int, bool) {
	for _, seg := range block.Segments {
		if seg.EndMS > seg.StartMS {
			return seg.StartMS, true
		}
	}
	return 0, false
}

func formatTimestampMMSS(ms int) string {
	if ms < 0 {
		ms = 0
	}
	totalSeconds := ms / 1000
	minutes := totalSeconds / 60
	seconds := totalSeconds % 60
	return fmt.Sprintf("%02d:%02d", minutes, seconds)
}

func compactNonEmpty(values []string) []string {
	out := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		out = append(out, value)
	}
	return out
}

func isEmptyYouTubeMetadata(meta dto.PodcastYouTube) bool {
	return strings.TrimSpace(meta.PublishTitle) == "" &&
		len(compactNonEmpty(meta.InThisEpisodeYouWillLearn)) == 0 &&
		len(compactNonEmpty(meta.DescriptionIntro)) == 0 &&
		len(meta.Chapters) == 0
}

func mergeScriptPublishingMetadata(base, timed dto.PodcastScript) dto.PodcastScript {
	if strings.TrimSpace(timed.Language) == "" {
		timed.Language = base.Language
	}
	if strings.TrimSpace(timed.AudienceLanguage) == "" {
		timed.AudienceLanguage = base.AudienceLanguage
	}
	if strings.TrimSpace(timed.DifficultyLevel) == "" {
		timed.DifficultyLevel = base.DifficultyLevel
	}
	if timed.TargetDurationMinutes == 0 {
		timed.TargetDurationMinutes = base.TargetDurationMinutes
	}
	if strings.TrimSpace(timed.Title) == "" {
		timed.Title = base.Title
	}
	timed.YouTube = base.YouTube

	if len(base.Blocks) == 0 || len(timed.Blocks) == 0 {
		timed.RefreshSegmentsFromBlocks()
		return timed
	}

	baseByID := make(map[string]dto.PodcastBlock, len(base.Blocks))
	for _, block := range base.Blocks {
		if blockID := strings.TrimSpace(block.TTSBlockID); blockID != "" {
			baseByID[blockID] = block
		}
	}
	for i := range timed.Blocks {
		blockID := strings.TrimSpace(timed.Blocks[i].TTSBlockID)
		source, ok := baseByID[blockID]
		if !ok {
			continue
		}
		if strings.TrimSpace(timed.Blocks[i].MacroBlock) == "" {
			timed.Blocks[i].MacroBlock = source.MacroBlock
		}
		if strings.TrimSpace(timed.Blocks[i].ChapterID) == "" {
			timed.Blocks[i].ChapterID = source.ChapterID
		}
		if strings.TrimSpace(timed.Blocks[i].Purpose) == "" {
			timed.Blocks[i].Purpose = source.Purpose
		}
	}
	timed.RefreshSegmentsFromBlocks()
	return timed
}
