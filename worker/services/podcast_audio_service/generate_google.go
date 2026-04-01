package podcast_audio_service

import (
	"context"
	"log"
	"os"
	"path/filepath"
	"strings"

	"worker/internal/dto"
	"worker/pkg/googlecloud"
)

func synthesizeWithGoogle(
	ctx context.Context,
	client *googlecloud.Client,
	aligner *blockAligner,
	language string,
	projectDir string,
	artifacts audioArtifacts,
	script dto.PodcastScript,
	blockGapMS int,
	requestedBlocks map[int]struct{},
) ([]blockSynthesisResult, error) {
	results := make([]blockSynthesisResult, len(script.Blocks))
	if err := validateGoogleBlocks(language, script.Blocks); err != nil {
		return nil, err
	}

	log.Printf("🎛️ podcast tts mode provider=google request_mode=per_block blocks=%d selected_blocks=%d",
		len(script.Blocks), len(requestedBlocks))
	for blockIndex, block := range script.Blocks {
		forceRerun := isRequestedBlock(requestedBlocks, blockIndex)
		if !forceRerun && hasRequestedBlocks(requestedBlocks) {
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
		if !forceRerun {
			reused, ok, err := tryReuseCachedBlock(ctx, aligner, language, artifacts, blockIndex, block)
			if err != nil {
				return nil, err
			}
			if ok {
				results[blockIndex] = reused
				script.Blocks[blockIndex] = reused.AlignedBlock
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
		alignedBlock, err := aligner.AlignBlock(ctx, language, block, blockAudioPath, blockDurationMS)
		if err != nil {
			return nil, err
		}
		if err := persistBlockCheckpoint(artifacts.blockStatesDir, blockIndex, alignedBlock, blockDurationMS); err != nil {
			return nil, err
		}
		results[blockIndex] = blockSynthesisResult{
			AudioPath:    blockAudioPath,
			DurationMS:   blockDurationMS,
			AlignedBlock: alignedBlock,
		}
		script.Blocks[blockIndex] = alignedBlock

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
		log.Printf("✅ podcast tts block done provider=google block=%03d/%03d block_id=%s audio=%s duration_ms=%d",
			blockIndex+1, len(script.Blocks), block.BlockID, blockAudioPath, blockDurationMS)
	}
	return results, nil
}
