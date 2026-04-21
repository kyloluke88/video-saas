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

func generateGoogleSingleAudioOnly(
	ctx context.Context,
	client *googlecloud.Client,
	projectID string,
	language string,
	artifacts audioArtifacts,
	script dto.PodcastScript,
	requestedBlocks map[int]struct{},
	currentTTSMode *int,
) ([]blockSynthesisResult, error) {
	results := make([]blockSynthesisResult, len(script.Blocks))

	for blockIndex, block := range script.Blocks {
		forceRerun := isRequestedBlock(requestedBlocks, blockIndex)
		if !forceRerun {
			reused, ok, err := tryReuseCompletedBlockWithoutMFA(
				podcastTTSTypeGoogle,
				"google",
				projectID,
				language,
				artifacts,
				blockIndex,
				block,
				currentTTSMode,
				false,
				false,
			)
			if err != nil {
				return nil, err
			}
			if ok {
				results[blockIndex] = reused
				continue
			}
		}

		segmentPaths, estimatedBytes, err := synthesizeGoogleSingleBlockSegments(ctx, client, projectID, language, artifacts, blockIndex, block)
		if err != nil {
			return nil, err
		}
		blockExt := "mp3"
		blockAudioPath := unitAudioPath(artifacts.blocksDir, blockIndex, block.BlockID, blockExt)
		if err := concatAudioFiles(ctx, artifacts.projectDir, segmentPaths, blockAudioPath); err != nil {
			return nil, err
		}
		blockDurationMS, err := audioDurationMS(blockAudioPath)
		if err != nil {
			return nil, err
		}
		if err := persistBlockCheckpoint(artifacts.blockStatesDir, blockIndex, block, blockDurationMS, currentTTSMode); err != nil {
			return nil, err
		}
		results[blockIndex] = blockSynthesisResult{
			AudioPath:    blockAudioPath,
			DurationMS:   blockDurationMS,
			AlignedBlock: block,
		}
		log.Printf("✅ podcast tts block done provider=google mode=single block=%03d/%03d turns=%d text_bytes=%d project_id=%s",
			blockIndex+1, len(script.Blocks), len(segmentPaths), estimatedBytes, projectID)
	}
	return results, nil
}

func synthesizeGoogleSingleBlockSegments(
	ctx context.Context,
	client *googlecloud.Client,
	projectID string,
	language string,
	artifacts audioArtifacts,
	blockIndex int,
	block dto.PodcastBlock,
) ([]string, int, error) {
	blockID := strings.TrimSpace(block.BlockID)
	segmentPaths := make([]string, 0, len(block.Segments))
	estimatedBytes := 0

	femaleName, maleName := geminiPromptSpeakerNames(language)
	maleVoiceID, femaleVoiceID := googleVoiceIDs(language)

	for segIndex, seg := range block.Segments {
		text := spokenTextForGoogleSynthesis(language, seg)
		if strings.TrimSpace(text) == "" {
			continue
		}
		estimatedBytes += len([]byte(strings.TrimSpace(text)))

		speaker := normalizeConversationSpeaker(seg.Speaker)
		voiceID := femaleVoiceID
		speakerLabel := femaleName
		speakerDescription := "warm, natural, friendly, grounded"
		if speaker == "male" {
			voiceID = maleVoiceID
			speakerLabel = maleName
			speakerDescription = "calm, steady, thoughtful, grounded"
		}

		request := googlecloud.SynthesizeSingleRequest{
			Prompt:  buildSingleSegmentPrompt(language, speakerLabel, speakerDescription),
			Text:    text,
			VoiceID: voiceID,
		}
		if err := persistGoogleTTSSegmentDebugArtifacts(artifacts.blockStatesDir, blockID, seg.SegmentID, request); err != nil {
			return nil, 0, err
		}
		log.Printf("🎛️ podcast tts segment start provider=google mode=single block=%03d segment=%03d project_id=%s",
			blockIndex+1, segIndex+1, projectID)
		ttsResult, err := client.SynthesizeSingle(ctx, request)
		if err != nil {
			return nil, 0, err
		}
		ext := strings.TrimPrefix(strings.TrimSpace(ttsResult.Ext), ".")
		if ext == "" {
			ext = "wav"
		}
		segmentPath := unitAudioPath(artifacts.segmentsDir, blockIndex, fmt.Sprintf("%s_%s", blockID, seg.SegmentID), ext)
		if err := os.WriteFile(segmentPath, ttsResult.Audio, 0o644); err != nil {
			return nil, 0, err
		}
		segmentPaths = append(segmentPaths, segmentPath)
	}

	if len(segmentPaths) == 0 {
		return nil, 0, fmt.Errorf("google single-speaker synthesis produced no segment audio for block=%s", blockID)
	}
	return segmentPaths, estimatedBytes, nil
}

func buildSingleSegmentPrompt(language string, speakerName, speakerDescription string) string {
	languageLabel := geminiPromptLanguageLabel(language)
	return strings.TrimSpace(strings.Join([]string{
		fmt.Sprintf("Synthesize the following %s single-speaker podcast line.", languageLabel),
		"Output audio only.",
		"Do not read headings, notes, or instructions aloud.",
		"Only speak the dialogue under BLOCK TRANSCRIPT.",
		"",
		"This request is one standalone line from a learning podcast.",
		"Keep the speaker voice stable and do not swap identities.",
		"",
		"# AUDIO PROFILE",
		fmt.Sprintf("%s:", speakerName),
		fmt.Sprintf("A %s podcast host.", languageLabel),
		speakerDescription,
		"",
		"# STUDIO SETTING",
		"Quiet home podcast studio.",
		"Close-mic, intimate distance, warm and dry room sound.",
		"",
		"# DIRECTOR'S NOTES",
		"Style: natural, subtle, realistic, and grounded.",
		"Pacing: slow-medium, relaxed, and easy to follow.",
		"Use a clear pause after each sentence and keep the rhythm unhurried.",
		"Energy: soft and controlled.",
		"Pronunciation: clear standard pronunciation, no strong regional accent.",
		"",
		"# SPEAKER BINDING",
		fmt.Sprintf("%s is always the same voice in this request.", speakerName),
		"Never swap voices.",
		"",
		"# BLOCK TRANSCRIPT",
		"Use only the transcript text supplied in this request.",
	}, "\n"))
}

func persistGoogleTTSSegmentDebugArtifacts(
	blockStatesDir string,
	blockID string,
	segmentID string,
	req googlecloud.SynthesizeSingleRequest,
) error {
	name := sanitizeSegmentID(fmt.Sprintf("%s_%s", blockID, segmentID))
	requestPath := filepath.Join(blockStatesDir, fmt.Sprintf("%s.google_request.json", name))
	body := googlecloud.BuildSingleGenerateContentRequestBody(googlecloud.DefaultTTSModel, req)
	return writeJSON(requestPath, body)
}
