package practical_audio_service

import (
	"context"
	"fmt"
	"log"

	"worker/pkg/x/fsx"
	services "worker/services"
	dto "worker/services/practical/model"
)

func generateWithGoogle(
	ctx context.Context,
	projectDir string,
	input GenerateInput,
	script dto.PracticalScript,
	requestedBlocks map[int]struct{},
	requestedChapterNums map[int]struct{},
	generateAll bool,
) (GenerateResult, error) {
	client, err := newGoogleSpeechClient()
	if err != nil {
		return GenerateResult{}, services.NonRetryableError{Err: fmt.Errorf("google tts client init failed: %w", err)}
	}
	narratorVoice := practicalNarratorVoiceID()
	if narratorVoice == "" {
		return GenerateResult{}, services.NonRetryableError{Err: fmt.Errorf("google narrator voice id is required")}
	}

	chapterAudioPaths := make([]string, 0, len(script.Blocks)*2)
	generatedAssetCount := 0
	chapterCursor := 0
	for blockIndex, block := range script.Blocks {
		shouldGenerateBlockTopic := generateAll
		if !shouldGenerateBlockTopic {
			_, shouldGenerateBlockTopic = requestedBlocks[blockIndex+1]
		}

		chapterIndexes := practicalSelectedChapterIndexes(block, requestedChapterNums, generateAll, chapterCursor)
		if !shouldGenerateBlockTopic && len(chapterIndexes) == 0 {
			chapterCursor += len(block.Chapters)
			continue
		}

		voiceAssignments := map[string]string(nil)
		if len(chapterIndexes) > 0 {
			voiceAssignments, err = practicalGoogleVoiceAssignments(projectDir, input.Language, block)
			if err != nil {
				return GenerateResult{}, err
			}
		}

		if shouldGenerateBlockTopic {
			topicRawAudioPath := blockIntroRawAudioPath(projectDir, block.BlockID, blockIndex+1)
			if err := synthesizeBlockTopicAudio(ctx, client, input.Language, block, narratorVoice, topicRawAudioPath); err != nil {
				return GenerateResult{}, err
			}
			if err := fsx.CopyFile(topicRawAudioPath, blockIntroAudioPath(projectDir, block.BlockID, blockIndex+1)); err != nil {
				return GenerateResult{}, err
			}
			generatedAssetCount++
		}

		for _, chapterIndex := range chapterIndexes {
			chapter := block.Chapters[chapterIndex]
			rawAudioPath := chapterRawAudioPath(projectDir, block.BlockID, chapter.ChapterID, blockIndex+1, chapterIndex+1)
			if err := synthesizeChapterAudio(ctx, client, input.Language, block, chapter, voiceAssignments, rawAudioPath); err != nil {
				return GenerateResult{}, err
			}
			chapterAudioPaths = append(chapterAudioPaths, rawAudioPath)
			generatedAssetCount++
		}
		chapterCursor += len(block.Chapters)
	}

	if generatedAssetCount == 0 {
		return GenerateResult{}, services.NonRetryableError{Err: fmt.Errorf("no practical audio targets selected for generation")}
	}

	log.Printf("📝 practical google audio generated project_id=%s source=%s path=%s", input.ProjectID, scriptPathFor(input.ScriptFilename), projectScriptInputPath(projectDir))
	return GenerateResult{
		ScriptPath:        projectScriptInputPath(projectDir),
		ChapterAudioPaths: chapterAudioPaths,
		Script:            script,
	}, nil
}
