package practical_image_service

import (
	"context"
	"fmt"
	"log"
	"sort"
	"strings"

	services "worker/services"
	dto "worker/services/practical/model"
)

type GenerateInput struct {
	ProjectID   string
	Language    string
	Resolution  string
	AspectRatio string
	BlockNums   []int
}

type GenerateResult struct {
	PlanPath     string
	ManifestPath string
	Manifest     dto.PracticalImageManifest
}

func Generate(ctx context.Context, input GenerateInput) (GenerateResult, error) {
	if strings.TrimSpace(input.ProjectID) == "" {
		return GenerateResult{}, fmt.Errorf("project_id is required")
	}

	projectDir := projectDirFor(input.ProjectID)
	script, err := loadAlignedScript(projectDir, input.Language)
	if err != nil {
		return GenerateResult{}, services.NonRetryableError{Err: err}
	}

	selectedBlocks := selectBlocks(script, input.BlockNums)
	if len(selectedBlocks) == 0 {
		return GenerateResult{}, services.NonRetryableError{Err: fmt.Errorf("no practical blocks selected for image generation")}
	}

	return generateFromLocalAssets(
		ctx,
		projectDir,
		script,
		selectedBlocks,
		normalizeResolution(input.Resolution),
		normalizeAspectRatio(input.AspectRatio),
	)
}

func generateFromLocalAssets(ctx context.Context, projectDir string, script dto.PracticalScript, blocks []dto.PracticalBlock, resolution, aspectRatio string) (GenerateResult, error) {
	plan := buildStaticImagePlan(blocks, resolution, aspectRatio)
	normalizeLocalPlanAssetFilenames(&plan)
	if err := writeJSON(projectImagePlanPath(projectDir), plan); err != nil {
		return GenerateResult{}, err
	}

	manifest := loadExistingManifest(projectDir)
	manifest.Version = "v1"
	manifest.Provider = "local-assets"
	manifest.Model = "static"
	manifest.Resolution = resolution
	manifest.AspectRatio = aspectRatio

	items := make([]dto.PracticalImagePlanItem, 0, len(plan.Blocks)+len(plan.Chapters))
	items = append(items, plan.Blocks...)
	items = append(items, plan.Chapters...)
	for _, item := range items {
		sourcePath, resolvedFilename, ok := resolvePracticalStaticImageAsset(item.Filename)
		if !ok {
			sourcePath = practicalStaticImageAssetPath(item.Filename)
			return GenerateResult{}, services.NonRetryableError{Err: fmt.Errorf("local practical image missing: %s", sourcePath)}
		}
		item.Filename = resolvedFilename

		targetPath := projectImageAbsolutePath(projectDir, item.Filename)
		if err := normalizeImageAsset(ctx, sourcePath, targetPath, resolution); err != nil {
			return GenerateResult{}, err
		}

		asset := dto.PracticalImageAsset{
			AssetKey:     item.AssetKey,
			AssetType:    item.AssetType,
			BlockID:      item.BlockID,
			ChapterID:    item.ChapterID,
			Filename:     item.Filename,
			SourcePrompt: item.SourcePrompt,
			Prompt:       item.Prompt,
		}
		if err := persistImageAsset(projectDir, script, &manifest, asset); err != nil {
			return GenerateResult{}, err
		}
	}

	sortManifest(&manifest, script)
	if err := writeJSON(projectImageManifestPath(projectDir), manifest); err != nil {
		return GenerateResult{}, err
	}
	log.Printf("🖼️ practical image asset-normalize mode project_dir=%s manifest=%s", projectDir, projectImageManifestPath(projectDir))
	return GenerateResult{
		PlanPath:     projectImagePlanPath(projectDir),
		ManifestPath: projectImageManifestPath(projectDir),
		Manifest:     manifest,
	}, nil
}

func persistImageAsset(projectDir string, script dto.PracticalScript, manifest *dto.PracticalImageManifest, asset dto.PracticalImageAsset) error {
	switch asset.AssetType {
	case "block":
		upsertBlockAsset(manifest, asset)
	case "chapter":
		upsertChapterAsset(manifest, asset)
	}
	sortManifest(manifest, script)
	return writeJSON(projectImageManifestPath(projectDir), manifest)
}

func buildStaticImagePlan(blocks []dto.PracticalBlock, resolution, aspectRatio string) dto.PracticalImagePlan {
	plan := dto.PracticalImagePlan{
		Version:     "v1",
		Provider:    "local-assets",
		Model:       "static",
		Resolution:  resolution,
		AspectRatio: aspectRatio,
		Blocks:      make([]dto.PracticalImagePlanItem, 0, len(blocks)),
	}
	for _, block := range blocks {
		plan.Blocks = append(plan.Blocks, dto.PracticalImagePlanItem{
			AssetKey:     sanitizePracticalID(block.BlockID),
			AssetType:    "block",
			BlockID:      block.BlockID,
			SourcePrompt: strings.TrimSpace(block.BlockPrompt),
			Prompt:       strings.TrimSpace(block.BlockPrompt),
			Filename:     projectBlockImageRelativePath(block.BlockID, "png"),
		})
		for _, chapter := range block.Chapters {
			plan.Chapters = append(plan.Chapters, dto.PracticalImagePlanItem{
				AssetKey:     sanitizePracticalID(chapter.ChapterID),
				AssetType:    "chapter",
				BlockID:      block.BlockID,
				ChapterID:    chapter.ChapterID,
				SourcePrompt: strings.TrimSpace(chapter.ScenePrompt),
				Prompt:       strings.TrimSpace(chapter.ScenePrompt),
				Filename:     projectChapterImageRelativePath(chapter.ChapterID, "png"),
			})
		}
	}
	return plan
}

func loadExistingManifest(projectDir string) dto.PracticalImageManifest {
	var manifest dto.PracticalImageManifest
	if err := readJSON(projectImageManifestPath(projectDir), &manifest); err != nil {
		return dto.PracticalImageManifest{}
	}
	return manifest
}

func upsertBlockAsset(manifest *dto.PracticalImageManifest, asset dto.PracticalImageAsset) {
	for idx := range manifest.Blocks {
		if strings.TrimSpace(manifest.Blocks[idx].BlockID) == strings.TrimSpace(asset.BlockID) {
			manifest.Blocks[idx] = asset
			return
		}
	}
	manifest.Blocks = append(manifest.Blocks, asset)
}

func upsertChapterAsset(manifest *dto.PracticalImageManifest, asset dto.PracticalImageAsset) {
	for idx := range manifest.Chapters {
		if strings.TrimSpace(manifest.Chapters[idx].ChapterID) == strings.TrimSpace(asset.ChapterID) {
			manifest.Chapters[idx] = asset
			return
		}
	}
	manifest.Chapters = append(manifest.Chapters, asset)
}

func sortManifest(manifest *dto.PracticalImageManifest, script dto.PracticalScript) {
	blockOrder := make(map[string]int, len(script.Blocks))
	chapterOrder := make(map[string]int, len(flattenChapters(script)))
	for idx, block := range script.Blocks {
		blockOrder[strings.TrimSpace(block.BlockID)] = idx
	}
	chapterIdx := 0
	for _, block := range script.Blocks {
		for _, chapter := range block.Chapters {
			chapterOrder[strings.TrimSpace(chapter.ChapterID)] = chapterIdx
			chapterIdx++
		}
	}
	sort.Slice(manifest.Blocks, func(i, j int) bool {
		return blockOrder[strings.TrimSpace(manifest.Blocks[i].BlockID)] < blockOrder[strings.TrimSpace(manifest.Blocks[j].BlockID)]
	})
	sort.Slice(manifest.Chapters, func(i, j int) bool {
		return chapterOrder[strings.TrimSpace(manifest.Chapters[i].ChapterID)] < chapterOrder[strings.TrimSpace(manifest.Chapters[j].ChapterID)]
	})
}

func selectBlocks(script dto.PracticalScript, blockNums []int) []dto.PracticalBlock {
	selected := compactPositiveInts(blockNums)
	if len(selected) == 0 {
		return append([]dto.PracticalBlock(nil), script.Blocks...)
	}
	allowed := make(map[int]struct{}, len(selected))
	for _, value := range selected {
		allowed[value] = struct{}{}
	}
	out := make([]dto.PracticalBlock, 0, len(selected))
	for idx, block := range script.Blocks {
		if _, ok := allowed[idx+1]; ok {
			out = append(out, block)
		}
	}
	return out
}

func normalizeLocalPlanAssetFilenames(plan *dto.PracticalImagePlan) {
	if plan == nil {
		return
	}
	for idx := range plan.Blocks {
		if _, resolvedFilename, ok := resolvePracticalStaticImageAsset(plan.Blocks[idx].Filename); ok {
			plan.Blocks[idx].Filename = resolvedFilename
		}
	}
	for idx := range plan.Chapters {
		if _, resolvedFilename, ok := resolvePracticalStaticImageAsset(plan.Chapters[idx].Filename); ok {
			plan.Chapters[idx].Filename = resolvedFilename
		}
	}
}
