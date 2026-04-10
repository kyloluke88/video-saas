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
	if _, err := generateGoogleAudioOnly(ctx, client, language, artifacts, script, requestedBlocks); err != nil {
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
	results, err := alignGoogleAudioOnly(ctx, aligner, language, projectDir, artifacts, script, blockGapMS, requestedBlocks)
	if err != nil {
		return GenerateResult{}, err
	}
	for i := range results {
		if i >= len(script.Blocks) {
			break
		}
		script.Blocks[i] = results[i].AlignedBlock
	}

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
	return GenerateResult{
		DialogueAudioPath: artifacts.dialoguePath,
		AlignedScriptPath: artifacts.alignedPath,
		Script:            alignedScript,
	}, nil
}

func generateGoogleAudioOnly(
	ctx context.Context,
	client *googlecloud.Client,
	language string,
	artifacts audioArtifacts,
	script dto.PodcastScript,
	requestedBlocks map[int]struct{},
) ([]blockSynthesisResult, error) {
	results := make([]blockSynthesisResult, len(script.Blocks))
	if err := validateGoogleBlocks(language, script.Blocks); err != nil {
		return nil, err
	}

	log.Printf("🎛️ podcast tts mode provider=google stage=audio_generate blocks=%d selected_blocks=%d",
		len(script.Blocks), len(requestedBlocks))
	for blockIndex, block := range script.Blocks {
		forceRerun := isRequestedBlock(requestedBlocks, blockIndex)
		if !forceRerun {
			reused, ok, err := tryReuseCompletedBlockWithoutMFA(
				podcastTTSTypeGoogle,
				"google",
				language,
				artifacts,
				blockIndex,
				block,
			)
			if err != nil {
				return nil, err
			}
			if ok {
				results[blockIndex] = reused
				continue
			}
		}

		request := buildConversationRequest(language, block)
		estimatedBytes := estimateConversationBytes(request)
		log.Printf("🎛️ podcast tts block start provider=google block=%03d/%03d block_id=%s turns=%d text_bytes=%d force_rerun=%t",
			blockIndex+1,
			len(script.Blocks),
			block.BlockID,
			len(request.Turns),
			estimatedBytes,
			forceRerun,
		)
		ttsResult, err := client.SynthesizeConversation(ctx, request)
		if err != nil {
			return nil, err
		}
		blockExt := strings.TrimPrefix(strings.TrimSpace(ttsResult.Ext), ".")
		if blockExt == "" {
			blockExt = "mp3"
		}
		blockAudioPath := unitAudioPath(artifacts.blocksDir, blockIndex, block.BlockID, blockExt)
		if err := os.WriteFile(blockAudioPath, ttsResult.Audio, 0o644); err != nil {
			return nil, err
		}
		blockDurationMS, err := audioDurationMS(blockAudioPath)
		if err != nil {
			return nil, err
		}
		if err := persistBlockCheckpoint(artifacts.blockStatesDir, blockIndex, block, blockDurationMS); err != nil {
			return nil, err
		}
		results[blockIndex] = blockSynthesisResult{
			AudioPath:    blockAudioPath,
			DurationMS:   blockDurationMS,
			AlignedBlock: block,
		}
		log.Printf("✅ podcast tts block done provider=google stage=audio_generate block=%03d/%03d block_id=%s audio=%s duration_ms=%d",
			blockIndex+1, len(script.Blocks), block.BlockID, blockAudioPath, blockDurationMS)
	}
	return results, nil
}

func alignGoogleAudioOnly(
	ctx context.Context,
	aligner *blockAligner,
	language string,
	projectDir string,
	artifacts audioArtifacts,
	script dto.PodcastScript,
	blockGapMS int,
	requestedBlocks map[int]struct{},
) ([]blockSynthesisResult, error) {
	results := make([]blockSynthesisResult, len(script.Blocks))
	log.Printf("🧭 podcast align mode provider=google stage=audio_align blocks=%d selected_blocks=%d",
		len(script.Blocks), len(requestedBlocks))

	for blockIndex, block := range script.Blocks {
		forceRerun := isRequestedBlock(requestedBlocks, blockIndex)
		if !forceRerun {
			reused, ok, err := tryReuseCompletedBlockWithoutMFA(
				podcastTTSTypeGoogle,
				"google",
				language,
				artifacts,
				blockIndex,
				block,
			)
			if err != nil {
				return nil, err
			}
			if ok {
				results[blockIndex] = reused
				script.Blocks[blockIndex] = reused.AlignedBlock
				continue
			}
		}

		aligned, ok, err := tryReuseCachedBlock(ctx, aligner, language, artifacts, blockIndex, block)
		if err != nil {
			return nil, err
		}
		if !ok {
			return nil, fmt.Errorf("google alignment requires existing block audio: block=%s block_index=%d", strings.TrimSpace(block.BlockID), blockIndex+1)
		}
		results[blockIndex] = aligned
		script.Blocks[blockIndex] = aligned.AlignedBlock

		partialScript, _, _, err := assembleDialogue(
			dto.PodcastScript{
				Language: script.Language,
				Title:    script.Title,
				YouTube:  script.YouTube,
				Blocks:   append([]dto.PodcastBlock(nil), script.Blocks[:blockIndex+1]...),
			},
			results[:blockIndex+1],
			artifacts.blockGapPath,
			blockGapMS,
		)
		if err == nil {
			_ = writeJSON(filepath.Join(projectDir, "script_partial.json"), partialScript)
		}
		log.Printf("✅ podcast align block done provider=google block=%03d/%03d block_id=%s audio=%s duration_ms=%d",
			blockIndex+1, len(script.Blocks), block.BlockID, aligned.AudioPath, aligned.DurationMS)
	}
	return results, nil
}

func loadCachedScriptForAlignment(projectDir, language string) (dto.PodcastScript, error) {
	scriptPath := projectScriptInputPath(projectDir)
	return loadScriptFromPath(language, scriptPath)
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
