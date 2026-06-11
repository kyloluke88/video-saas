package podcast_audio_service

import (
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"

	"worker/pkg/googlecloud"
	dto "worker/services/podcast/model"
)

func GenerateGoogleAudio(ctx context.Context, input GenerateInput) error {
	if strings.TrimSpace(input.ProjectID) == "" {
		return fmt.Errorf("project_id is required")
	}
	language, err := requirePodcastLanguage(input.Language)
	if err != nil {
		return err
	}
	if strings.TrimSpace(input.ScriptFilename) == "" {
		return fmt.Errorf("script_filename is required")
	}

	projectDir := projectDirFor(input.ProjectID)
	if err := os.MkdirAll(projectDir, 0o755); err != nil {
		return err
	}
	artifacts, err := prepareAudioArtifacts(projectDir)
	if err != nil {
		return err
	}
	if err := invalidateAlignedOutputs(artifacts); err != nil {
		return err
	}

	script, err := loadScriptForGeneration(projectDir, language, input.ScriptFilename)
	if err != nil {
		return err
	}
	requestedBlocks, err := buildRequestedBlockSet(input.BlockNums, len(script.Blocks))
	if err != nil {
		return err
	}

	client, err := newGoogleSpeechClient()
	if err != nil {
		return err
	}
	results, err := generateGoogleAudioOnly(ctx, client, input.ProjectID, language, artifacts, script, requestedBlocks, input.IsMultiple == 0)
	if err != nil {
		return err
	}
	provisionalScript, err := buildProvisionalAlignedScript(language, script, results, artifacts.blockGapPath, blockGapMS())
	if err != nil {
		return err
	}
	if err := writeJSON(artifacts.alignedPath, provisionalScript); err != nil {
		return err
	}
	return nil
}

func AlignGoogle(ctx context.Context, input AlignInput) (GenerateResult, error) {
	if strings.TrimSpace(input.ProjectID) == "" {
		return GenerateResult{}, fmt.Errorf("project_id is required")
	}
	language, err := requirePodcastLanguage(input.Language)
	if err != nil {
		return GenerateResult{}, err
	}

	projectDir := projectDirFor(input.ProjectID)
	if err := os.MkdirAll(projectDir, 0o755); err != nil {
		return GenerateResult{}, err
	}
	artifacts, err := prepareAudioArtifacts(projectDir)
	if err != nil {
		return GenerateResult{}, err
	}

	script, err := loadCachedScriptForAlignment(projectDir, language)
	if err != nil {
		return GenerateResult{}, err
	}
	requestedBlocks, err := buildRequestedBlockSet(input.BlockNums, len(script.Blocks))
	if err != nil {
		return GenerateResult{}, err
	}

	blockGapMS := blockGapMS()
	if blockGapMS > 0 {
		if err := createSilenceAudio(ctx, artifacts.blockGapPath, blockGapMS); err != nil {
			return GenerateResult{}, err
		}
	}

	aligner := newBlockAligner(newMFAClient(), chunkWorkingDir(projectDir))
	results, err := alignGoogleAudioOnly(ctx, aligner, input.ProjectID, language, artifacts, script, requestedBlocks)
	if err != nil {
		return GenerateResult{}, err
	}
	applyBlockResults(&script, results)

	finalScript, concatPaths, _, err := assembleDialogue(script, results, artifacts.blockGapPath, blockGapMS)
	if err != nil {
		return GenerateResult{}, err
	}
	if err := concatAudioFiles(ctx, projectDir, concatPaths, artifacts.dialoguePath); err != nil {
		return GenerateResult{}, err
	}

	alignedScript, err := finalizeAlignedScript(input.ProjectID, artifacts.alignedPath, artifacts.dialoguePath, finalScript)
	if err != nil {
		return GenerateResult{}, err
	}
	if err := cleanupGoogleTTSDebugArtifacts(projectDir); err != nil {
		log.Printf("⚠️ podcast google tts debug cleanup warning project_id=%s err=%v", input.ProjectID, err)
	}
	return GenerateResult{
		DialogueAudioPath: artifacts.dialoguePath,
		AlignedScriptPath: artifacts.alignedPath,
		Script:            alignedScript,
	}, nil
}

func generateGoogleAudioOnly(
	ctx context.Context,
	client *googlecloud.Client,
	projectID string,
	language string,
	artifacts audioArtifacts,
	script dto.PodcastScript,
	requestedBlocks map[int]struct{},
	singleSpeaker bool,
) ([]blockSynthesisResult, error) {
	results := make([]blockSynthesisResult, len(script.Blocks))
	if singleSpeaker {
		return generateGoogleSingleAudioOnly(ctx, client, projectID, language, artifacts, script, requestedBlocks)
	}

	for blockIndex, block := range script.Blocks {
		forceRerun := isRequestedBlock(requestedBlocks, blockIndex)
		if !forceRerun {
			reused, ok, err := tryReuseGeneratedGoogleBlock(projectID, artifacts, blockIndex, block, "multi")
			if err != nil {
				return nil, err
			}
			if ok {
				results[blockIndex] = reused
				continue
			}
		}

		request := buildConversationRequest(language, block)
		if err := persistGoogleTTSDebugArtifacts(artifacts.ttsDebugDir, block.BlockID, request); err != nil {
			return nil, err
		}
		estimatedBytes := estimateConversationBytes(request)
		log.Printf("🎛️ podcast tts block start provider=google block=%03d/%03d project_id=%s",
			blockIndex+1,
			len(script.Blocks),
			projectID,
		)
		ttsResult, err := client.SynthesizeConversation(ctx, request)
		if err != nil {
			return nil, err
		}
		blockExt := strings.TrimPrefix(strings.TrimSpace(ttsResult.Ext), ".")
		if blockExt == "" {
			blockExt = "mp3"
		}
		if err := removeRawBlockArtifacts(artifacts, blockIndex, block.BlockID); err != nil {
			return nil, err
		}
		if err := removeAlignedBlockArtifacts(artifacts, blockIndex, block.BlockID); err != nil {
			return nil, err
		}
		blockAudioPath := unitAudioPath(artifacts.rawBlocksDir, blockIndex, block.BlockID, blockExt)
		if err := os.WriteFile(blockAudioPath, ttsResult.Audio, 0o644); err != nil {
			return nil, err
		}
		blockDurationMS, err := audioDurationMS(blockAudioPath)
		if err != nil {
			return nil, err
		}
		results[blockIndex] = blockSynthesisResult{
			AudioPath:    blockAudioPath,
			DurationMS:   blockDurationMS,
			AlignedBlock: block,
		}
		log.Printf("✅ podcast tts block done provider=google block=%03d/%03d turns=%d text_bytes=%d project_id=%s",
			blockIndex+1, len(script.Blocks), len(request.Turns), estimatedBytes, projectID)
	}
	return results, nil
}

func alignGoogleAudioOnly(
	ctx context.Context,
	aligner *blockAligner,
	projectID string,
	language string,
	artifacts audioArtifacts,
	script dto.PodcastScript,
	requestedBlocks map[int]struct{},
) ([]blockSynthesisResult, error) {
	results := make([]blockSynthesisResult, len(script.Blocks))
	tempo := podcastTempo(language)

	for blockIndex, block := range script.Blocks {
		forceRerun := isRequestedBlock(requestedBlocks, blockIndex)
		if !forceRerun {
			reused, ok, err := tryReuseAlignedBlock("google", projectID, language, artifacts, blockIndex, block, tempo, true)
			if err != nil {
				return nil, err
			}
			if ok {
				results[blockIndex] = reused
				continue
			}
		}

		aligned, ok, err := alignGoogleBlockFromRaw(ctx, aligner, projectID, language, artifacts, blockIndex, block, tempo, true)
		if err != nil {
			return nil, err
		}
		if !ok {
			return nil, fmt.Errorf("google alignment requires existing raw block audio: block=%s block_index=%d", strings.TrimSpace(block.BlockID), blockIndex+1)
		}
		results[blockIndex] = aligned
		log.Printf("✅ podcast align block done provider=google block=%03d/%03d block_id=%s duration_ms=%d project_id=%s",
			blockIndex+1, len(script.Blocks), block.BlockID, aligned.DurationMS, projectID)
	}
	return results, nil
}

func loadCachedScriptForAlignment(projectDir, language string) (dto.PodcastScript, error) {
	scriptPath := projectScriptAlignedPath(projectDir)
	script, err := loadScriptFromPath(language, scriptPath)
	if err != nil {
		return dto.PodcastScript{}, err
	}
	return restoreStableBlockIDsFromArtifacts(projectDir, script), nil
}

func invalidateAlignedOutputs(artifacts audioArtifacts) error {
	paths := []string{
		artifacts.dialoguePath,
		artifacts.alignedPath,
		filepath.Join(artifacts.projectDir, "script_partial.json"),
	}
	for _, path := range paths {
		if strings.TrimSpace(path) == "" {
			continue
		}
		if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
			return err
		}
	}
	return nil
}
