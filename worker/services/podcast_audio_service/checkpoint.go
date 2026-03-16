package podcast_audio_service

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"

	"worker/internal/dto"
)

type audioArtifacts struct {
	projectDir       string
	dialoguePath     string
	alignedPath      string
	segmentsDir      string
	dialogueClipsDir string
	ttsResponsesDir  string
	silencePath      string
	shortSilencePath string
}

func prepareAudioArtifacts(projectDir string) (audioArtifacts, error) {
	artifacts := audioArtifacts{
		projectDir:       projectDir,
		dialoguePath:     filepath.Join(projectDir, "dialogue.mp3"),
		alignedPath:      filepath.Join(projectDir, "script_aligned.json"),
		segmentsDir:      filepath.Join(projectDir, "segments"),
		dialogueClipsDir: filepath.Join(projectDir, "dialogue_clips"),
		ttsResponsesDir:  filepath.Join(projectDir, "tts_responses"),
		silencePath:      filepath.Join(projectDir, "segment_gap.mp3"),
		shortSilencePath: filepath.Join(projectDir, "segment_gap_same_speaker.mp3"),
	}
	if err := os.MkdirAll(artifacts.segmentsDir, 0o755); err != nil {
		return audioArtifacts{}, err
	}
	if err := os.MkdirAll(artifacts.dialogueClipsDir, 0o755); err != nil {
		return audioArtifacts{}, err
	}
	if err := os.MkdirAll(artifacts.ttsResponsesDir, 0o755); err != nil {
		return audioArtifacts{}, err
	}
	return artifacts, nil
}

func loadResumableScript(base dto.PodcastScript, alignedPath string) dto.PodcastScript {
	checkpoint, ok, err := readCheckpointScript(alignedPath)
	if err != nil {
		log.Printf("⚠️ checkpoint ignored path=%s err=%v", alignedPath, err)
		return base
	}
	if !ok {
		return base
	}
	if !scriptCheckpointCompatible(base, checkpoint) {
		log.Printf("⚠️ checkpoint incompatible path=%s; falling back to script input", alignedPath)
		return base
	}
	return mergeScriptPublishingMetadata(base, checkpoint)
}

func readCheckpointScript(path string) (dto.PodcastScript, bool, error) {
	if _, err := os.Stat(path); err != nil {
		if os.IsNotExist(err) {
			return dto.PodcastScript{}, false, nil
		}
		return dto.PodcastScript{}, false, err
	}
	var script dto.PodcastScript
	if err := readJSON(path, &script); err != nil {
		return dto.PodcastScript{}, false, err
	}
	return script, true, nil
}

func scriptCheckpointCompatible(base, candidate dto.PodcastScript) bool {
	if normalizeLanguage(base.Language) != normalizeLanguage(candidate.Language) {
		return false
	}
	if len(base.Segments) != len(candidate.Segments) {
		return false
	}
	for i := range base.Segments {
		if strings.TrimSpace(base.Segments[i].SegmentID) != strings.TrimSpace(candidate.Segments[i].SegmentID) {
			return false
		}
	}
	if len(base.Blocks) != len(candidate.Blocks) {
		return false
	}
	for i := range base.Blocks {
		if strings.TrimSpace(base.Blocks[i].TTSBlockID) != strings.TrimSpace(candidate.Blocks[i].TTSBlockID) {
			return false
		}
		if len(base.Blocks[i].Segments) != len(candidate.Blocks[i].Segments) {
			return false
		}
	}
	return true
}

func persistAlignedCheckpoint(path string, script dto.PodcastScript) error {
	checkpoint := script
	checkpoint.SyncBlocksFromSegments()
	return writeJSON(path, checkpoint)
}

func finalizeAlignedScript(projectID, alignedPath, dialoguePath string, script dto.PodcastScript, contentProfile string) (dto.PodcastScript, error) {
	finalScript := script
	finalScript.SyncBlocksFromSegments()
	mergedScript, err := applyStartEndTemplates(projectID, dialoguePath, finalScript)
	if err != nil {
		return dto.PodcastScript{}, err
	}
	finalScript = mergedScript
	finalScript.RenumberStructureIDs()
	if err := writeJSON(alignedPath, finalScript); err != nil {
		return dto.PodcastScript{}, err
	}
	if err := exportYouTubePublishFiles(filepath.Dir(alignedPath), finalScript); err != nil {
		return dto.PodcastScript{}, err
	}
	if err := exportConversationMinimalFile(filepath.Dir(alignedPath), projectID, contentProfile, finalScript); err != nil {
		return dto.PodcastScript{}, err
	}
	timedSegments, totalSegments, timedTokens, totalTokens := alignedStats(finalScript)
	log.Printf("🧭 script aligned ready project_id=%s path=%s segments_timed=%d/%d tokens_timed=%d/%d",
		projectID, alignedPath, timedSegments, totalSegments, timedTokens, totalTokens)
	return finalScript, nil
}

func unitAudioPath(dir string, index int, unitID, ext string) string {
	ext = strings.TrimPrefix(strings.TrimSpace(ext), ".")
	if ext == "" {
		ext = "mp3"
	}
	return filepath.Join(dir, fmt.Sprintf("%03d_%s.%s", index+1, sanitizeSegmentID(unitID), ext))
}

func existingUnitAudioPath(dir string, index int, unitID, preferredExt string) (string, bool) {
	preferred := unitAudioPath(dir, index, unitID, preferredExt)
	if fileExists(preferred) {
		return preferred, true
	}
	pattern := filepath.Join(dir, fmt.Sprintf("%03d_%s.*", index+1, sanitizeSegmentID(unitID)))
	matches, err := filepath.Glob(pattern)
	if err != nil || len(matches) == 0 {
		return "", false
	}
	return matches[0], true
}

func segmentCheckpointComplete(language string, seg dto.PodcastSegment, audioPath string) bool {
	if !fileExists(audioPath) || seg.EndMS <= seg.StartMS {
		return false
	}
	if isJapaneseLanguage(language) {
		for _, ch := range seg.Chars {
			if ch.EndMS > ch.StartMS {
				return true
			}
		}
		return len(seg.Chars) > 0
	}
	matched, total := chineseAlignmentStats(seg)
	if total == 0 || matched != total {
		return false
	}
	if chineseSegmentHasCollapsedTiming(seg) {
		return false
	}
	for _, token := range seg.Tokens {
		if token.EndMS > token.StartMS {
			return true
		}
	}
	return len(seg.Tokens) > 0
}

func blockCheckpointComplete(block dto.PodcastBlock, audioPath string) bool {
	if !fileExists(audioPath) || len(block.Segments) == 0 {
		return false
	}
	for _, seg := range block.Segments {
		if seg.EndMS <= seg.StartMS {
			return false
		}
	}
	return true
}

func blockEndMS(block dto.PodcastBlock) int {
	end := 0
	for _, seg := range block.Segments {
		if seg.EndMS > end {
			end = seg.EndMS
		}
	}
	return end
}

func blockTimedSegments(block dto.PodcastBlock) int {
	count := 0
	for _, seg := range block.Segments {
		if seg.EndMS > seg.StartMS {
			count++
		}
	}
	return count
}

func appendConcatPath(paths []string, audioPath string, includeGap bool, silencePath string) []string {
	paths = append(paths, audioPath)
	if includeGap && strings.TrimSpace(silencePath) != "" {
		paths = append(paths, silencePath)
	}
	return paths
}

func fileExists(path string) bool {
	if strings.TrimSpace(path) == "" {
		return false
	}
	info, err := os.Stat(path)
	if err != nil {
		return false
	}
	return !info.IsDir()
}

func chineseSegmentHasCollapsedTiming(seg dto.PodcastSegment) bool {
	windows := make(map[string]struct{})
	meaningful := 0
	for _, token := range seg.Tokens {
		charText := strings.TrimSpace(token.Char)
		if charText == "" || isSilentToken(charText) {
			continue
		}
		if token.EndMS <= token.StartMS {
			continue
		}
		meaningful++
		key := fmt.Sprintf("%d-%d", token.StartMS, token.EndMS)
		windows[key] = struct{}{}
	}
	return meaningful > 1 && len(windows) <= 1
}
