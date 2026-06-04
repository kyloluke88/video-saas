package practical_image_service

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"

	dto "worker/services/practical/model"
)

var supportedPracticalImageExtensions = []string{".png", ".jpeg", ".jpg", ".webp"}

func projectDirFor(projectID string) string {
	return filepath.Join(practicalOutputsRoot(), "projects", strings.TrimSpace(projectID))
}

func projectScriptAlignedPath(projectDir string) string {
	return filepath.Join(projectDir, "script_aligned.json")
}

func projectImagePlanPath(projectDir string) string {
	return filepath.Join(projectDir, "image_plan.json")
}

func projectImageManifestPath(projectDir string) string {
	return filepath.Join(projectDir, "image_manifest.json")
}

func projectBlockImageRelativePath(blockID, format string) string {
	return filepath.Join("images", "blocks", sanitizePracticalID(blockID)+"."+normalizeImageExtension(format))
}

func projectChapterImageRelativePath(chapterID, format string) string {
	return filepath.Join("images", "chapters", sanitizePracticalID(chapterID)+"."+normalizeImageExtension(format))
}

func projectImageAbsolutePath(projectDir, relativePath string) string {
	return filepath.Join(projectDir, filepath.Clean(strings.TrimSpace(relativePath)))
}

func practicalStaticImageAssetPath(relativePath string) string {
	if resolved, _, ok := resolvePracticalStaticImageAsset(relativePath); ok {
		return resolved
	}

	trimmed := filepath.ToSlash(filepath.Clean(strings.TrimSpace(relativePath)))
	trimmed = strings.TrimPrefix(trimmed, "./")
	candidates := []string{
		filepath.Join(practicalReferenceAssetDir(), filepath.FromSlash(strings.TrimPrefix(trimmed, "images/"))),
		filepath.Join(practicalReferenceAssetDir(), filepath.FromSlash(trimmed)),
	}
	for _, candidate := range candidates {
		if info, err := os.Stat(candidate); err == nil && !info.IsDir() {
			return candidate
		}
	}
	return candidates[0]
}

func resolvePracticalStaticImageAsset(relativePath string) (string, string, bool) {
	trimmed := filepath.ToSlash(filepath.Clean(strings.TrimSpace(relativePath)))
	trimmed = strings.TrimPrefix(trimmed, "./")
	if trimmed == "" {
		return "", "", false
	}

	lookupPaths := []string{
		strings.TrimPrefix(trimmed, "images/"),
		trimmed,
	}
	for _, lookup := range lookupPaths {
		candidate := filepath.Join(practicalReferenceAssetDir(), filepath.FromSlash(lookup))
		if info, err := os.Stat(candidate); err == nil && !info.IsDir() {
			return candidate, trimmed, true
		}
	}

	currentExt := strings.ToLower(filepath.Ext(trimmed))
	stem := strings.TrimSuffix(trimmed, filepath.Ext(trimmed))
	for _, ext := range supportedPracticalImageExtensions {
		if ext == currentExt {
			continue
		}
		relativeWithExt := stem + ext
		for _, lookup := range []string{
			strings.TrimPrefix(relativeWithExt, "images/"),
			relativeWithExt,
		} {
			candidate := filepath.Join(practicalReferenceAssetDir(), filepath.FromSlash(lookup))
			if info, err := os.Stat(candidate); err == nil && !info.IsDir() {
				return candidate, relativeWithExt, true
			}
		}
	}

	return "", trimmed, false
}

func practicalReferenceAssetDir() string {
	candidates := []string{
		filepath.Join(practicalAssetsRoot(), "practical", "bg-images"),
		filepath.Join(practicalAssetsRoot(), "practicle", "bg-images"),
	}
	for _, candidate := range candidates {
		if info, err := os.Stat(candidate); err == nil && info.IsDir() {
			return candidate
		}
	}
	return candidates[0]
}

func sanitizePracticalID(value string) string {
	raw := strings.TrimSpace(value)
	if raw == "" {
		return ""
	}
	raw = strings.ReplaceAll(raw, "/", "_")
	raw = strings.ReplaceAll(raw, "\\", "_")
	raw = strings.ReplaceAll(raw, " ", "_")
	return raw
}

func normalizeImageExtension(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "jpg":
		return "jpg"
	case "jpeg":
		return "jpeg"
	case "webp":
		return "webp"
	default:
		return "png"
	}
}

func normalizeResolution(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "720p":
		return "720p"
	case "1080p":
		return "1080p"
	default:
		return "1080p"
	}
}

func normalizeAspectRatio(value string) string {
	if strings.TrimSpace(value) == "" {
		return "16:9"
	}
	return strings.TrimSpace(value)
}

func fileExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && !info.IsDir()
}

func copyFile(srcPath, dstPath string) error {
	src, err := os.Open(srcPath)
	if err != nil {
		return err
	}
	defer src.Close()

	if err := os.MkdirAll(filepath.Dir(dstPath), 0o755); err != nil {
		return err
	}
	dst, err := os.Create(dstPath)
	if err != nil {
		return err
	}

	if _, err := io.Copy(dst, src); err != nil {
		_ = dst.Close()
		return err
	}
	return dst.Close()
}

func readJSON(path string, out interface{}) error {
	raw, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	return json.Unmarshal(raw, out)
}

func writeJSON(path string, data interface{}) error {
	raw, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return os.WriteFile(path, raw, 0o644)
}

func loadAlignedScript(projectDir, language string) (dto.PracticalScript, error) {
	path := projectScriptAlignedPath(projectDir)
	if !fileExists(path) {
		return dto.PracticalScript{}, fmt.Errorf("aligned script missing: %s", path)
	}
	var script dto.PracticalScript
	if err := readJSON(path, &script); err != nil {
		return dto.PracticalScript{}, err
	}
	script.Language = strings.ToLower(strings.TrimSpace(script.Language))
	if strings.TrimSpace(script.Language) == "" {
		script.Language = strings.ToLower(strings.TrimSpace(language))
	}
	return script, script.Validate()
}

func flattenChapters(script dto.PracticalScript) []dto.PracticalChapter {
	chapters := make([]dto.PracticalChapter, 0, len(script.Blocks))
	for _, block := range script.Blocks {
		chapters = append(chapters, block.Chapters...)
	}
	return chapters
}

func compactPositiveInts(values []int) []int {
	seen := make(map[int]struct{}, len(values))
	out := make([]int, 0, len(values))
	for _, value := range values {
		if value <= 0 {
			continue
		}
		if _, exists := seen[value]; exists {
			continue
		}
		seen[value] = struct{}{}
		out = append(out, value)
	}
	sort.Ints(out)
	return out
}

func practicalAssetsRoot() string {
	candidates := []string{
		"assets",
		filepath.Join("worker", "assets"),
		"/app/assets",
	}
	for _, candidate := range candidates {
		if info, err := os.Stat(candidate); err == nil && info.IsDir() {
			return practicalAbsolutePath(candidate)
		}
	}
	return practicalAbsolutePath(candidates[0])
}

func practicalOutputsRoot() string {
	candidates := []string{
		"outputs",
		filepath.Join("worker", "outputs"),
		"/app/outputs",
	}
	for _, candidate := range candidates {
		if info, err := os.Stat(candidate); err == nil && info.IsDir() {
			return practicalAbsolutePath(candidate)
		}
	}
	return practicalAbsolutePath(candidates[0])
}

func practicalAbsolutePath(path string) string {
	if filepath.IsAbs(strings.TrimSpace(path)) {
		return strings.TrimSpace(path)
	}
	abs, err := filepath.Abs(path)
	if err != nil {
		return path
	}
	return abs
}
