package practical_compose_service

import (
	"context"
	"fmt"
	"log"
	"path/filepath"
	"strings"

	services "worker/services"
	ffmpegcommon "worker/services/media/ffmpeg/common"
	dto "worker/services/practical/model"
)

type ComposeInput struct {
	ProjectID           string
	Language            string
	BgImgFilenames      []string
	BlockBgImgFilenames []string
	Resolution          string
	DesignType          int
}

type RenderResult struct {
	BaseVideoPath string
}

type FinalizeResult struct {
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
	BaseVideoPath        string
	FinalVideoPath       string
	SubtitleASSPath      string
	BackgroundPaths      []string
	BlockBackgroundPaths []string
	Script               dto.PracticalScript
}

func Render(ctx context.Context, input ComposeInput) (RenderResult, error) {
	artifacts, err := prepareRenderArtifacts(input)
	if err != nil {
		return RenderResult{}, err
	}
	if err := renderBaseVideo(ctx, artifacts); err != nil {
		return RenderResult{}, err
	}
	return RenderResult{BaseVideoPath: artifacts.BaseVideoPath}, nil
}

func Finalize(ctx context.Context, input ComposeInput) (FinalizeResult, error) {
	artifacts, err := prepareFinalizeArtifacts(input)
	if err != nil {
		return FinalizeResult{}, err
	}

	assPath, err := writePracticalASS(artifacts.Script, artifacts.ProjectDir, artifacts.Resolution, artifacts.DesignType)
	if err != nil {
		return FinalizeResult{}, err
	}
	if strings.TrimSpace(assPath) == "" {
		return FinalizeResult{}, services.NonRetryableError{Err: fmt.Errorf("practical subtitle ass could not be generated")}
	}

	if err := burnPracticalSubtitles(ctx, artifacts, assPath); err != nil {
		return FinalizeResult{}, err
	}
	log.Printf("🎬 practical finalization complete project_id=%s ass=%s final=%s", filepath.Base(artifacts.ProjectDir), assPath, artifacts.FinalVideoPath)
	return FinalizeResult{
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
		ProjectDir:      projectDir,
		Language:        language,
		Resolution:      strings.TrimSpace(input.Resolution),
		DesignType:      normalizePracticalDesignType(input.DesignType),
		DialoguePath:    projectDialoguePath(projectDir),
		ScriptPath:      projectScriptAlignedPath(projectDir),
		BaseVideoPath:   projectBaseVideoPath(projectDir),
		FinalVideoPath:  projectFinalVideoPath(projectDir),
		SubtitleASSPath: projectSubtitleASSPath(projectDir),
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

	backgroundNames := compactBackgroundNames(input.BgImgFilenames)
	if len(backgroundNames) < len(chapters) {
		return composeArtifacts{}, services.NonRetryableError{Err: fmt.Errorf("bg_img_filenames count %d is less than chapter count %d", len(backgroundNames), len(chapters))}
	}
	artifacts.BackgroundPaths = make([]string, 0, len(chapters))
	for idx := range chapters {
		bgPath := practicalBackgroundImagePath(backgroundNames[idx])
		if !fileExists(bgPath) {
			return composeArtifacts{}, services.NonRetryableError{Err: fmt.Errorf("background image missing: %s", bgPath)}
		}
		artifacts.BackgroundPaths = append(artifacts.BackgroundPaths, bgPath)
	}

	blockBackgroundNames := compactBackgroundNames(input.BlockBgImgFilenames)
	if len(blockBackgroundNames) > 0 && len(blockBackgroundNames) < len(script.Blocks) {
		return composeArtifacts{}, services.NonRetryableError{Err: fmt.Errorf("block_bg_img_filenames count %d is less than block count %d", len(blockBackgroundNames), len(script.Blocks))}
	}
	artifacts.BlockBackgroundPaths = make([]string, 0, len(script.Blocks))
	if len(blockBackgroundNames) > 0 {
		for idx := range script.Blocks {
			bgPath := practicalBackgroundImagePath(blockBackgroundNames[idx])
			if !fileExists(bgPath) {
				return composeArtifacts{}, services.NonRetryableError{Err: fmt.Errorf("block background image missing: %s", bgPath)}
			}
			artifacts.BlockBackgroundPaths = append(artifacts.BlockBackgroundPaths, bgPath)
		}
	} else {
		chapterCursor := 0
		for _, block := range script.Blocks {
			if chapterCursor >= len(artifacts.BackgroundPaths) {
				return composeArtifacts{}, services.NonRetryableError{Err: fmt.Errorf("block background fallback failed: missing chapter background")}
			}
			artifacts.BlockBackgroundPaths = append(artifacts.BlockBackgroundPaths, artifacts.BackgroundPaths[chapterCursor])
			chapterCursor += len(block.Chapters)
		}
	}
	return artifacts, nil
}

func prepareFinalizeArtifacts(input ComposeInput) (composeArtifacts, error) {
	artifacts, err := prepareBaseArtifacts(input)
	if err != nil {
		return composeArtifacts{}, err
	}
	if !fileExists(artifacts.BaseVideoPath) {
		return composeArtifacts{}, services.NonRetryableError{Err: fmt.Errorf("practical base video missing: %s", artifacts.BaseVideoPath)}
	}
	if !fileExists(artifacts.ScriptPath) {
		return composeArtifacts{}, services.NonRetryableError{Err: fmt.Errorf("aligned script missing: %s", artifacts.ScriptPath)}
	}

	script, err := loadAlignedScript(artifacts.ProjectDir, artifacts.Language)
	if err != nil {
		return composeArtifacts{}, err
	}
	artifacts.Script = script
	return artifacts, nil
}

func renderBaseVideo(ctx context.Context, artifacts composeArtifacts) error {
	segments := buildPracticalRenderSegments(
		artifacts.Script,
		artifacts.BackgroundPaths,
		artifacts.BlockBackgroundPaths,
		practicalChapterGapMS(),
		practicalBlockGapMS(),
		practicalChapterTransitionLeadMS(),
		practicalBlockTransitionLeadMS(),
	)
	if len(segments) == 0 {
		return services.NonRetryableError{Err: fmt.Errorf("practical script has no chapters")}
	}

	fadeSec := 0.8
	if fadeSec <= 0 {
		fadeSec = 0.8
	}
	scale := ffmpegcommon.ResolutionToScale(artifacts.Resolution)
	preset := practicalX264Preset()
	timeout := practicalFFmpegTimeout()

	args := []string{"-y"}
	for _, segment := range segments {
		args = append(args, "-loop", "1", "-i", segment.BackgroundPath)
	}
	args = append(args, "-i", artifacts.DialoguePath)

	filterParts := make([]string, 0, len(segments)*2)
	offsets := make([]float64, 0, len(segments))
	cumulative := 0.0
	for idx, segment := range segments {
		segmentDuration := segment.DurationSec
		if idx < len(segments)-1 {
			segmentDuration += fadeSec
		}
		filterParts = append(filterParts, fmt.Sprintf("[%d:v]scale=%s:force_original_aspect_ratio=increase,crop=%s,setsar=1,fps=30,format=yuv420p,trim=duration=%.3f,setpts=PTS-STARTPTS[v%d]",
			idx, scale, scale, segmentDuration, idx))
		offsets = append(offsets, cumulative)
		cumulative += segment.DurationSec
	}

	finalLabel := "[v0]"
	if len(segments) > 1 {
		for idx := 1; idx < len(segments); idx++ {
			prev := finalLabel
			next := fmt.Sprintf("[v%d]", idx)
			out := fmt.Sprintf("[x%d]", idx)
			filterParts = append(filterParts, fmt.Sprintf("%s%sxfade=transition=fade:duration=%.3f:offset=%.3f%s", prev, next, fadeSec, offsets[idx], out))
			finalLabel = out
		}
	}

	filterComplex := strings.Join(filterParts, ";")
	audioInputIndex := len(artifacts.BackgroundPaths)
	audioInputIndex = len(segments)
	if strings.TrimSpace(filterComplex) == "" {
		return services.NonRetryableError{Err: fmt.Errorf("practical render filter is empty")}
	}
	args = append(args,
		"-filter_complex", filterComplex,
		"-map", finalLabel,
		"-map", fmt.Sprintf("%d:a:0", audioInputIndex),
		"-c:v", "libx264",
		"-preset", preset,
		"-pix_fmt", "yuv420p",
		"-c:a", "aac",
		"-shortest",
		artifacts.BaseVideoPath,
	)
	return ffmpegcommon.RunFFmpegWithTimeoutContext(ctx, timeout, args...)
}

type practicalRenderSegment struct {
	BackgroundPath string
	DurationSec    float64
}

func buildPracticalRenderSegments(script dto.PracticalScript, chapterBackgroundPaths, blockBackgroundPaths []string, chapterGapMS, blockGapMS, chapterTransitionLeadMS, blockTransitionLeadMS int) []practicalRenderSegment {
	segments := make([]practicalRenderSegment, 0, len(chapterBackgroundPaths)+len(blockBackgroundPaths))
	chapterCursor := 0
	chapterGapSec := float64(maxInt(0, chapterGapMS)) / 1000.0
	blockGapSec := float64(maxInt(0, blockGapMS)) / 1000.0
	chapterLeadSec := float64(maxInt(0, chapterTransitionLeadMS)) / 1000.0
	blockLeadSec := float64(maxInt(0, blockTransitionLeadMS)) / 1000.0

	for blockIndex, block := range script.Blocks {
		introDurationSec := float64(maxInt(0, block.TopicEndMS-block.TopicStartMS)) / 1000.0
		if introDurationSec > 0 {
			if blockIndex < len(blockBackgroundPaths) {
				segmentDuration := introDurationSec + blockLeadSec
				if blockIndex > 0 {
					segmentDuration += blockGapSec
				}
				segments = append(segments, practicalRenderSegment{
					BackgroundPath: blockBackgroundPaths[blockIndex],
					DurationSec:    segmentDuration,
				})
			}
		}

		for chapterIndex, chapter := range block.Chapters {
			if chapterCursor >= len(chapterBackgroundPaths) {
				return segments
			}
			startMS, endMS := chapterStartEndMS(chapter)
			durationSec := float64(maxInt(1, endMS-startMS))/1000.0 + chapterLeadSec
			if chapterIndex < len(block.Chapters)-1 {
				durationSec += chapterGapSec
			}
			segments = append(segments, practicalRenderSegment{
				BackgroundPath: chapterBackgroundPaths[chapterCursor],
				DurationSec:    durationSec,
			})
			chapterCursor++
		}
	}

	return segments
}

func burnPracticalSubtitles(ctx context.Context, artifacts composeArtifacts, assPath string) error {
	if strings.TrimSpace(assPath) == "" {
		return fmt.Errorf("subtitle path is required")
	}
	filter := fmt.Sprintf("subtitles=%s:fontsdir=%s", escapeFFmpegPath(assPath), escapeFFmpegPath(practicalFontsDir()))
	return ffmpegcommon.RunFFmpegWithTimeoutContext(ctx, practicalFFmpegTimeout(),
		"-y",
		"-i", artifacts.BaseVideoPath,
		"-vf", filter,
		"-c:v", "libx264",
		"-preset", practicalX264Preset(),
		"-pix_fmt", "yuv420p",
		"-c:a", "copy",
		artifacts.FinalVideoPath,
	)
}
