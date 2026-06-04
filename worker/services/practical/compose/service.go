package practical_compose_service

import (
	"context"
	"fmt"
	"log"
	"path/filepath"
	"strconv"
	"strings"

	services "worker/services"
	ffmpegcommon "worker/services/media/ffmpeg/common"
	dto "worker/services/practical/model"
)

type ComposeInput struct {
	ProjectID  string
	Language   string
	Resolution string
	DesignType int
}

type RenderResult struct {
	FinalVideoPath  string
	SubtitleASSPath string
}

type composeArtifacts struct {
	ProjectDir           string
	Language             string
	Resolution           string
	DesignType           int
	DialoguePath         string
	ScriptPath           string
	FinalVideoPath       string
	BackgroundPaths      []string
	BlockBackgroundPaths []string
	Script               dto.PracticalScript
}

func Render(ctx context.Context, input ComposeInput) (RenderResult, error) {
	artifacts, err := prepareRenderArtifacts(input)
	if err != nil {
		return RenderResult{}, err
	}

	assPath, err := writePracticalASS(artifacts.Script, artifacts.ProjectDir, artifacts.Resolution, artifacts.DesignType)
	if err != nil {
		return RenderResult{}, err
	}
	if strings.TrimSpace(assPath) == "" {
		return RenderResult{}, services.NonRetryableError{Err: fmt.Errorf("practical subtitle ass could not be generated")}
	}

	if err := renderFinalVideo(ctx, artifacts, assPath); err != nil {
		return RenderResult{}, err
	}
	log.Printf("🎬 practical render complete project_id=%s ass=%s final=%s", filepath.Base(artifacts.ProjectDir), assPath, artifacts.FinalVideoPath)
	return RenderResult{
		FinalVideoPath:  artifacts.FinalVideoPath,
		SubtitleASSPath: assPath,
	}, nil
}

func prepareBaseArtifacts(input ComposeInput) (composeArtifacts, error) {
	projectID := strings.TrimSpace(input.ProjectID)
	if projectID == "" {
		return composeArtifacts{}, fmt.Errorf("project_id is required")
	}
	language, err := requirePracticalLanguage(input.Language)
	if err != nil {
		return composeArtifacts{}, err
	}

	projectDir := projectDirFor(projectID)
	artifacts := composeArtifacts{
		ProjectDir:     projectDir,
		Language:       language,
		Resolution:     strings.TrimSpace(input.Resolution),
		DesignType:     normalizePracticalDesignType(input.DesignType),
		DialoguePath:   projectDialoguePath(projectDir),
		ScriptPath:     projectScriptAlignedPath(projectDir),
		FinalVideoPath: projectFinalVideoPath(projectDir),
	}
	if artifacts.Resolution == "" {
		artifacts.Resolution = "1080p"
	}
	return artifacts, nil
}

func prepareRenderArtifacts(input ComposeInput) (composeArtifacts, error) {
	artifacts, err := prepareBaseArtifacts(input)
	if err != nil {
		return composeArtifacts{}, err
	}
	if !fileExists(artifacts.DialoguePath) {
		return composeArtifacts{}, services.NonRetryableError{Err: fmt.Errorf("dialogue audio missing: %s", artifacts.DialoguePath)}
	}
	if !fileExists(artifacts.ScriptPath) {
		return composeArtifacts{}, services.NonRetryableError{Err: fmt.Errorf("aligned script missing: %s", artifacts.ScriptPath)}
	}

	script, err := loadAlignedScript(artifacts.ProjectDir, artifacts.Language)
	if err != nil {
		return composeArtifacts{}, err
	}
	artifacts.Script = script

	chapters := flattenChapters(script)
	if len(chapters) == 0 {
		return composeArtifacts{}, services.NonRetryableError{Err: fmt.Errorf("practical script has no chapters")}
	}

	manifest, err := loadImageManifest(artifacts.ProjectDir)
	if err != nil {
		return composeArtifacts{}, err
	}
	artifacts.BackgroundPaths = make([]string, 0, len(chapters))
	chapterAssets := make(map[string]string, len(manifest.Chapters))
	for _, asset := range manifest.Chapters {
		chapterAssets[strings.TrimSpace(asset.ChapterID)] = strings.TrimSpace(asset.Filename)
	}
	for _, chapter := range chapters {
		filename, ok := chapterAssets[strings.TrimSpace(chapter.Chapter.ChapterID)]
		if !ok || strings.TrimSpace(filename) == "" {
			return composeArtifacts{}, services.NonRetryableError{Err: fmt.Errorf("chapter image missing for %s", strings.TrimSpace(chapter.Chapter.ChapterID))}
		}
		bgPath := projectImageAssetPath(artifacts.ProjectDir, filename)
		if !fileExists(bgPath) {
			return composeArtifacts{}, services.NonRetryableError{Err: fmt.Errorf("background image missing: %s", bgPath)}
		}
		artifacts.BackgroundPaths = append(artifacts.BackgroundPaths, bgPath)
	}

	artifacts.BlockBackgroundPaths = make([]string, 0, len(script.Blocks))
	blockAssets := make(map[string]string, len(manifest.Blocks))
	for _, asset := range manifest.Blocks {
		blockAssets[strings.TrimSpace(asset.BlockID)] = strings.TrimSpace(asset.Filename)
	}
	for _, block := range script.Blocks {
		filename, ok := blockAssets[strings.TrimSpace(block.BlockID)]
		if !ok || strings.TrimSpace(filename) == "" {
			return composeArtifacts{}, services.NonRetryableError{Err: fmt.Errorf("block image missing for %s", strings.TrimSpace(block.BlockID))}
		}
		bgPath := projectImageAssetPath(artifacts.ProjectDir, filename)
		if !fileExists(bgPath) {
			return composeArtifacts{}, services.NonRetryableError{Err: fmt.Errorf("block background image missing: %s", bgPath)}
		}
		artifacts.BlockBackgroundPaths = append(artifacts.BlockBackgroundPaths, bgPath)
	}
	return artifacts, nil
}

func loadImageManifest(projectDir string) (dto.PracticalImageManifest, error) {
	manifestPath := projectImageManifestPath(projectDir)
	if !fileExists(manifestPath) {
		return dto.PracticalImageManifest{}, services.NonRetryableError{Err: fmt.Errorf("image manifest missing: %s", manifestPath)}
	}
	var manifest dto.PracticalImageManifest
	if err := readJSON(manifestPath, &manifest); err != nil {
		return dto.PracticalImageManifest{}, err
	}
	return manifest, nil
}

func renderFinalVideo(ctx context.Context, artifacts composeArtifacts, assPath string) error {
	if strings.TrimSpace(assPath) == "" {
		return fmt.Errorf("subtitle path is required")
	}

	segments := buildPracticalRenderSegments(
		artifacts.Script,
		artifacts.BackgroundPaths,
		artifacts.BlockBackgroundPaths,
		practicalChapterGapMS(),
		practicalBlockGapMS(),
	)
	if len(segments) == 0 {
		return services.NonRetryableError{Err: fmt.Errorf("practical script has no chapters")}
	}

	preset := practicalX264Preset()
	threads := practicalX264Threads()
	timeout := practicalFFmpegTimeout(artifacts.DialoguePath)

	args := []string{"-y"}
	for _, segment := range segments {
		args = append(args, "-loop", "1", "-i", segment.BackgroundPath)
	}
	args = append(args, "-i", artifacts.DialoguePath)

	filterComplex, finalLabel, err := buildPracticalVideoFilter(segments)
	if err != nil {
		return services.NonRetryableError{Err: err}
	}
	subtitleLabel := "[vsub]"
	filterComplex += fmt.Sprintf(";%ssubtitles=%s:fontsdir=%s%s",
		finalLabel,
		escapeFFmpegPath(assPath),
		escapeFFmpegPath(practicalFontsDir()),
		subtitleLabel,
	)
	finalLabel = subtitleLabel
	audioInputIndex := len(segments)
	args = append(args,
		"-filter_complex", filterComplex,
		"-map", finalLabel,
		"-map", fmt.Sprintf("%d:a:0", audioInputIndex),
		"-c:v", "libx264",
		"-preset", preset,
		"-threads", strconv.Itoa(threads),
		"-pix_fmt", "yuv420p",
		"-c:a", "aac",
		"-shortest",
		artifacts.FinalVideoPath,
	)
	return ffmpegcommon.RunFFmpegWithTimeoutContext(ctx, timeout, args...)
}

type practicalRenderSegment struct {
	BackgroundPath string
	DurationSec    float64
}

func buildPracticalVideoFilter(segments []practicalRenderSegment) (string, string, error) {
	if len(segments) == 0 {
		return "", "", fmt.Errorf("practical render filter is empty")
	}

	fps := practicalVideoFPS()
	filterParts := make([]string, 0, len(segments)+1)
	labels := make([]string, 0, len(segments))
	for idx, segment := range segments {
		label := fmt.Sprintf("[v%d]", idx)
		filterParts = append(filterParts, fmt.Sprintf("[%d:v]setsar=1,fps=%d,format=yuv420p,trim=duration=%.3f,setpts=PTS-STARTPTS%s",
			idx, fps, segment.DurationSec, label))
		labels = append(labels, label)
	}

	if len(labels) == 1 {
		return strings.Join(filterParts, ";"), labels[0], nil
	}

	finalLabel := "[vout]"
	filterParts = append(filterParts, fmt.Sprintf("%sconcat=n=%d:v=1:a=0%s", strings.Join(labels, ""), len(labels), finalLabel))
	return strings.Join(filterParts, ";"), finalLabel, nil
}

func buildPracticalRenderSegments(script dto.PracticalScript, chapterBackgroundPaths, blockBackgroundPaths []string, _ int, _ int) []practicalRenderSegment {
	segments := make([]practicalRenderSegment, 0, len(chapterBackgroundPaths)+len(blockBackgroundPaths))
	chapterCursor := 0

	for blockIndex, block := range script.Blocks {
		firstChapterStartMS := practicalFirstChapterStartMS(block)
		if blockIndex < len(blockBackgroundPaths) && block.TopicEndMS > block.TopicStartMS {
			segmentEndMS := maxInt(block.TopicEndMS, firstChapterStartMS)
			if segmentEndMS > block.TopicStartMS {
				segments = append(segments, practicalRenderSegment{
					BackgroundPath: blockBackgroundPaths[blockIndex],
					DurationSec:    float64(segmentEndMS-block.TopicStartMS) / 1000.0,
				})
			}
		}

		for chapterIndex, chapter := range block.Chapters {
			if chapterCursor >= len(chapterBackgroundPaths) {
				return segments
			}
			startMS, endMS := chapterStartEndMS(chapter)
			segmentEndMS := endMS
			if chapterIndex < len(block.Chapters)-1 {
				nextStartMS, _ := chapterStartEndMS(block.Chapters[chapterIndex+1])
				if nextStartMS > startMS {
					segmentEndMS = maxInt(segmentEndMS, nextStartMS)
				}
			} else if blockIndex < len(script.Blocks)-1 {
				nextBlock := script.Blocks[blockIndex+1]
				if nextBlock.TopicStartMS > startMS {
					segmentEndMS = maxInt(segmentEndMS, nextBlock.TopicStartMS)
				}
			}
			segments = append(segments, practicalRenderSegment{
				BackgroundPath: chapterBackgroundPaths[chapterCursor],
				DurationSec:    float64(maxInt(1, segmentEndMS-startMS)) / 1000.0,
			})
			chapterCursor++
		}
	}

	return segments
}

func practicalFirstChapterStartMS(block dto.PracticalBlock) int {
	for _, chapter := range block.Chapters {
		startMS, endMS := chapterStartEndMS(chapter)
		if endMS > startMS {
			return startMS
		}
	}
	return 0
}
