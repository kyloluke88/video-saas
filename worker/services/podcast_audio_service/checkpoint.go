package podcast_audio_service

import (
	"log"
	"os"
	"path/filepath"
	"strings"

	"worker/internal/dto"
)

type audioArtifacts struct {
	projectDir      string
	dialoguePath    string
	alignedPath     string
	blocksDir       string
	blockStatesDir  string
	blockGapPath    string
	templateGapPath string
}

func prepareAudioArtifacts(projectDir string) (audioArtifacts, error) {
	artifacts := audioArtifacts{
		projectDir:      projectDir,
		dialoguePath:    filepath.Join(projectDir, "dialogue.mp3"),
		alignedPath:     filepath.Join(projectDir, "script_aligned.json"),
		blocksDir:       filepath.Join(projectDir, "blocks"),
		blockStatesDir:  filepath.Join(projectDir, "block_states"),
		blockGapPath:    filepath.Join(projectDir, "block_gap.mp3"),
		templateGapPath: filepath.Join(projectDir, "template_gap.mp3"),
	}
	if err := os.MkdirAll(artifacts.blocksDir, 0o755); err != nil {
		return audioArtifacts{}, err
	}
	if err := os.MkdirAll(artifacts.blockStatesDir, 0o755); err != nil {
		return audioArtifacts{}, err
	}
	if err := os.MkdirAll(chunkWorkingDir(projectDir), 0o755); err != nil {
		return audioArtifacts{}, err
	}
	return artifacts, nil
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

func appendConcatPath(paths []string, audioPath string, includeGap bool, gapPath string) []string {
	paths = append(paths, audioPath)
	if includeGap && strings.TrimSpace(gapPath) != "" {
		paths = append(paths, gapPath)
	}
	return paths
}
