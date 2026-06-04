package practical_audio_service

import (
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"

	"worker/pkg/googlecloud"
	services "worker/services"
	dto "worker/services/practical/model"
)

type GenerateInput struct {
	ProjectID      string
	Language       string
	ScriptFilename string
	BlockNums      []int
	ChapterNums    []int
}

type GenerateResult struct {
	ScriptPath        string
	ChapterAudioPaths []string
	Script            dto.PracticalScript
}

func Generate(ctx context.Context, input GenerateInput) (GenerateResult, error) {
	if strings.TrimSpace(input.ProjectID) == "" {
		return GenerateResult{}, fmt.Errorf("project_id is required")
	}
	language, err := requirePracticalLanguage(input.Language)
	if err != nil {
		return GenerateResult{}, err
	}
	if strings.TrimSpace(input.ScriptFilename) == "" {
		return GenerateResult{}, fmt.Errorf("script_filename is required")
	}

	projectDir := projectDirFor(input.ProjectID)
	if err := os.MkdirAll(projectDir, 0o755); err != nil {
		return GenerateResult{}, err
	}

	script, err := loadScriptForGeneration(projectDir, language, input.ScriptFilename)
	if err != nil {
		return GenerateResult{}, err
	}
	if err := script.Validate(); err != nil {
		return GenerateResult{}, err
	}
	if err := writeJSON(projectScriptInputPath(projectDir), script); err != nil {
		return GenerateResult{}, err
	}

	requestedBlocks, err := buildRequestedBlockSet(input.BlockNums, len(script.Blocks))
	if err != nil {
		return GenerateResult{}, err
	}
	requestedChapterNums, err := buildRequestedChapterSet(script, input.ChapterNums)
	if err != nil {
		return GenerateResult{}, err
	}
	generateAll := len(requestedBlocks) == 0 && len(requestedChapterNums) == 0

	client, err := newGoogleSpeechClient()
	if err != nil {
		return GenerateResult{}, services.NonRetryableError{Err: fmt.Errorf("google tts client init failed: %w", err)}
	}
	maleVoice, femaleVoice := practicalTTSVoiceIDs(language)
	if strings.TrimSpace(maleVoice) == "" || strings.TrimSpace(femaleVoice) == "" {
		return GenerateResult{}, services.NonRetryableError{Err: fmt.Errorf("google voice ids are required for lang=%s", language)}
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

		if shouldGenerateBlockTopic {
			topicRawAudioPath := blockIntroRawAudioPath(projectDir, block.BlockID, blockIndex+1)
			if err := synthesizeBlockTopicAudio(ctx, client, language, block, narratorVoice, topicRawAudioPath); err != nil {
				return GenerateResult{}, err
			}
			generatedAssetCount++
		}

		if len(chapterIndexes) > 0 {
			speakerVoicesByRole, err := block.SpeakerVoicesByRole()
			if err != nil {
				return GenerateResult{}, fmt.Errorf("block %s: %w", block.BlockID, err)
			}
			speakerNames := practicalSpeakerNamesForTTS(block)
			for _, chapterIndex := range chapterIndexes {
				chapter := block.Chapters[chapterIndex]
				audioPath := chapterRawAudioPath(projectDir, block.BlockID, chapter.ChapterID, blockIndex+1, chapterIndex+1)
				if err := synthesizeChapterAudio(ctx, client, language, block, chapter, speakerVoicesByRole, speakerNames, maleVoice, femaleVoice, audioPath); err != nil {
					return GenerateResult{}, err
				}
				chapterAudioPaths = append(chapterAudioPaths, audioPath)
				generatedAssetCount++
			}
		}
		chapterCursor += len(block.Chapters)
	}

	if generatedAssetCount == 0 {
		return GenerateResult{}, services.NonRetryableError{Err: fmt.Errorf("no practical audio targets selected for generation")}
	}

	log.Printf("📝 practical script cached project_id=%s source=%s path=%s", input.ProjectID, scriptPathFor(input.ScriptFilename), projectScriptInputPath(projectDir))
	return GenerateResult{
		ScriptPath:        projectScriptInputPath(projectDir),
		ChapterAudioPaths: chapterAudioPaths,
		Script:            script,
	}, nil
}

func loadScriptForGeneration(projectDir, language, scriptFilename string) (dto.PracticalScript, error) {
	projectScriptPath := projectScriptInputPath(projectDir)
	if fileExists(projectScriptPath) {
		return loadScriptFromPath(language, projectScriptPath)
	}

	scriptPath := scriptPathFor(scriptFilename)
	script, err := loadScriptFromPath(language, scriptPath)
	if err != nil {
		return dto.PracticalScript{}, err
	}
	if err := writeJSON(projectScriptPath, script); err != nil {
		return dto.PracticalScript{}, err
	}
	return script, nil
}

func loadScriptFromPath(language, scriptPath string) (dto.PracticalScript, error) {
	var script dto.PracticalScript
	if err := readJSON(scriptPath, &script); err != nil {
		if os.IsNotExist(err) {
			return dto.PracticalScript{}, services.NonRetryableError{Err: fmt.Errorf("script file not found: %s", strings.TrimSpace(scriptPath))}
		}
		return dto.PracticalScript{}, err
	}
	if err := validatePracticalScriptLanguage(script.Language, language); err != nil {
		return dto.PracticalScript{}, err
	}
	script.Language = language
	script.Normalize()
	return script, nil
}

func validatePracticalScriptLanguage(scriptLanguage, payloadLanguage string) error {
	scriptLang := strings.ToLower(strings.TrimSpace(scriptLanguage))
	payloadLang := strings.ToLower(strings.TrimSpace(payloadLanguage))
	if _, err := requirePracticalLanguage(scriptLang); err != nil {
		return services.NonRetryableError{Err: fmt.Errorf("script language mismatch: script=%q payload=%q", strings.TrimSpace(scriptLanguage), payloadLanguage)}
	}
	if scriptLang != payloadLang {
		return services.NonRetryableError{Err: fmt.Errorf("script language mismatch: script=%q payload=%q", scriptLang, payloadLanguage)}
	}
	return nil
}

func synthesizeChapterAudio(
	ctx context.Context,
	client *googlecloud.Client,
	language string,
	block dto.PracticalBlock,
	chapter dto.PracticalChapter,
	speakerVoicesByRole map[string]string,
	speakerNames map[string]string,
	maleVoiceID, femaleVoiceID string,
	outputPath string,
) error {
	if client == nil {
		return fmt.Errorf("google tts client is required")
	}
	if err := os.MkdirAll(filepath.Dir(outputPath), 0o755); err != nil {
		return err
	}

	turns := make([]googlecloud.ConversationTurn, 0, len(chapter.Turns))
	for _, turn := range chapter.Turns {
		speech := practicalSpeechText(turn)
		if speech == "" {
			continue
		}
		speakerRole := strings.TrimSpace(turn.SpeakerRole)
		if speakerRole == "" {
			speakerRole = strings.TrimSpace(turn.SpeakerID)
		}
		voice, ok := speakerVoicesByRole[speakerRole]
		if !ok {
			resolvedVoice, resolveErr := block.ResolveTurnVoice(turn)
			if resolveErr != nil {
				return services.NonRetryableError{Err: resolveErr}
			}
			voice = resolvedVoice
		}
		turns = append(turns, googlecloud.ConversationTurn{
			Speaker: voice,
			Text:    speech,
		})
	}
	if len(turns) == 0 {
		return services.NonRetryableError{Err: fmt.Errorf("chapter %s has no speakable turns", chapter.ChapterID)}
	}

	prompt := buildPracticalChapterTTSPrompt(language, block, chapter)
	result, err := client.SynthesizeConversation(ctx, googlecloud.SynthesizeConversationRequest{
		LanguageCode:  language,
		Prompt:        prompt,
		Turns:         turns,
		SpeakerNames:  speakerNames,
		MaleVoiceID:   maleVoiceID,
		FemaleVoiceID: femaleVoiceID,
		SpeakingRate:  practicalSpeakingRate(language),
	})
	if err != nil {
		return err
	}
	if err := os.WriteFile(outputPath, result.Audio, 0o644); err != nil {
		return err
	}
	return nil
}

func synthesizeBlockTopicAudio(
	ctx context.Context,
	client *googlecloud.Client,
	language string,
	block dto.PracticalBlock,
	narratorVoiceID string,
	outputPath string,
) error {
	if client == nil {
		return fmt.Errorf("google tts client is required")
	}
	topic := normalizePracticalNarrationText(block.Topic)
	if topic == "" {
		return services.NonRetryableError{Err: fmt.Errorf("block %s topic is required for narration", block.BlockID)}
	}
	if strings.TrimSpace(narratorVoiceID) == "" {
		return services.NonRetryableError{Err: fmt.Errorf("google narrator voice id is required")}
	}
	if err := os.MkdirAll(filepath.Dir(outputPath), 0o755); err != nil {
		return err
	}

	result, err := client.SynthesizeSingle(ctx, googlecloud.SynthesizeSingleRequest{
		LanguageCode: language,
		Prompt:       buildPracticalBlockTopicPrompt(language, block),
		Text:         topic,
		VoiceID:      narratorVoiceID,
		SpeakingRate: practicalNarratorSpeakingRate(language),
	})
	if err != nil {
		return err
	}
	if err := os.WriteFile(outputPath, result.Audio, 0o644); err != nil {
		return err
	}
	return nil
}

func buildPracticalChapterTTSPrompt(language string, block dto.PracticalBlock, chapter dto.PracticalChapter) string {
	lines := []string{
		"Read this chapter as a natural everyday conversation in the described scene.",
		"Keep the acting subtle but alive.",
		"Use light emotional variation such as greeting, hesitation, curiosity, confirmation, apology, gratitude, or emphasis when the line requires it.",
		"Do not sound flat, over-careful, or like isolated textbook example sentences.",
		"Keep each speaker's voice consistent and do not swap the female and male voices.",
		"Keep every turn in the original order.",
		"Do not add extra words.",
		"Do not insert exaggerated long pauses between turns. Timing pauses are handled by the audio pipeline.",
	}
	if topic := strings.TrimSpace(block.Topic); topic != "" {
		lines = append(lines, "Topic: "+topic)
	}
	if scene := strings.TrimSpace(chapter.Scene); scene != "" {
		lines = append(lines, "Scene: "+scene)
	}
	if scenePrompt := strings.TrimSpace(chapter.ScenePrompt); scenePrompt != "" {
		lines = append(lines, "Visual scene: "+scenePrompt)
	}
	if casting := practicalVoiceCastingNote(block); casting != "" {
		lines = append(lines, casting)
	}
	if placeholders := practicalBlockPromptPlaceholders(block); len(placeholders) > 0 {
		lines = append(lines, "Characters: "+strings.Join(placeholders, ", "))
	}
	if strings.EqualFold(strings.TrimSpace(language), "ja") {
		lines = append(lines,
			"Language: Japanese.",
			"Pronounce every Japanese word completely and clearly.",
			"Do not swallow, clip, or drop the ending of a word or sentence.",
			"Fully pronounce final verb endings and final morae such as ru, u, ku, tsu, and i.",
			"Keep particles and polite endings audible and complete, including desu, masu, and dictionary-form endings.",
		)
	} else {
		lines = append(lines, "Language: Chinese.")
	}
	return strings.Join(lines, " ")
}

func buildPracticalBlockTopicPrompt(language string, block dto.PracticalBlock) string {
	lines := []string{
		"Speak this section title as a short transition narration.",
		"Read only the title text exactly as written, once.",
		"The title may be short. Even if it is short, pronounce every word, character, and ending completely before stopping.",
		"Do not skip, swallow, clip, merge, paraphrase, or add any word.",
		"Use a calm, steady, complete delivery.",
		"Do not rush the final character or final mora.",
	}
	if strings.EqualFold(strings.TrimSpace(language), "ja") {
		lines = append(lines,
			"Language: Japanese.",
			"Pronounce every Japanese word completely and clearly.",
			"Do not swallow, clip, or drop the ending of a word or sentence.",
			"Fully pronounce final verb endings and final morae such as ru, u, ku, tsu, and i.",
			"Keep particles and polite endings audible and complete, including desu, masu, and dictionary-form endings.",
		)
	} else {
		lines = append(lines, "Language: Chinese.")
	}
	return strings.Join(lines, " ")
}

func normalizePracticalNarrationText(value string) string {
	fields := strings.Fields(strings.TrimSpace(value))
	if len(fields) == 0 {
		return ""
	}
	return strings.Join(fields, " ")
}

func practicalBlockPromptPlaceholders(block dto.PracticalBlock) []string {
	if len(block.Speakers) == 0 {
		return nil
	}
	out := make([]string, 0, len(block.Speakers))
	for _, speaker := range block.Speakers {
		name := firstNonEmpty(
			strings.TrimSpace(speaker.SpeakerRole),
			strings.TrimSpace(speaker.Name),
			strings.TrimSpace(speaker.SpeakerID),
		)
		if name == "" {
			continue
		}
		if !strings.HasPrefix(name, "@[") || !strings.HasSuffix(name, "]") {
			name = fmt.Sprintf("@[%s]", strings.Trim(name, "@[] "))
		}
		if name == "@[]" {
			continue
		}
		out = append(out, name)
	}
	return out
}

func practicalSpeakerNamesForTTS(block dto.PracticalBlock) map[string]string {
	out := map[string]string{
		"female": "female speaker",
		"male":   "male speaker",
	}
	assigned := map[string]bool{}
	for _, speaker := range block.Speakers {
		voice := normalizePracticalVoice(speaker.SpeakerID)
		if voice == "" || assigned[voice] {
			continue
		}
		role := strings.TrimSpace(speaker.SpeakerRole)
		name := strings.TrimSpace(speaker.Name)
		label := firstNonEmpty(name, role, voice)
		if label == "" {
			continue
		}
		if role != "" && !strings.EqualFold(label, role) {
			label = role
		}
		out[voice] = fmt.Sprintf("%s %s", voice, label)
		assigned[voice] = true
	}
	return out
}

func practicalVoiceCastingNote(block dto.PracticalBlock) string {
	if len(block.Speakers) == 0 {
		return ""
	}
	rolesByVoice := map[string][]string{
		"female": {},
		"male":   {},
	}
	for _, speaker := range block.Speakers {
		voice := normalizePracticalVoice(speaker.SpeakerID)
		role := strings.TrimSpace(speaker.SpeakerRole)
		if voice == "" || role == "" {
			continue
		}
		rolesByVoice[voice] = append(rolesByVoice[voice], role)
	}
	parts := make([]string, 0, 2)
	if len(rolesByVoice["female"]) > 0 {
		parts = append(parts, "Female voice speaks "+strings.Join(rolesByVoice["female"], ", ")+" turns.")
	}
	if len(rolesByVoice["male"]) > 0 {
		parts = append(parts, "Male voice speaks "+strings.Join(rolesByVoice["male"], ", ")+" turns.")
	}
	return strings.Join(parts, " ")
}

func normalizePracticalVoice(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "female", "f", "woman", "girl", "女":
		return "female"
	case "male", "m", "man", "boy", "男":
		return "male"
	default:
		return ""
	}
}

func buildRequestedBlockSet(values []int, total int) (map[int]struct{}, error) {
	cleaned := compactPositiveInts(values)
	if len(cleaned) == 0 {
		return nil, nil
	}
	out := make(map[int]struct{}, len(cleaned))
	for _, value := range cleaned {
		if value > total {
			return nil, services.NonRetryableError{Err: fmt.Errorf("block_nums contains out-of-range block %d", value)}
		}
		out[value] = struct{}{}
	}
	return out, nil
}

func buildRequestedChapterSet(script dto.PracticalScript, values []int) (map[int]struct{}, error) {
	cleaned := compactPositiveInts(values)
	if len(cleaned) == 0 {
		return nil, nil
	}
	totalChapters := 0
	for _, block := range script.Blocks {
		totalChapters += len(block.Chapters)
	}

	out := make(map[int]struct{}, len(cleaned))
	for _, chapterNum := range cleaned {
		if chapterNum > totalChapters {
			return nil, services.NonRetryableError{Err: fmt.Errorf("chapter_nums contains out-of-range chapter %d", chapterNum)}
		}
		out[chapterNum] = struct{}{}
	}
	if len(out) == 0 {
		return nil, nil
	}
	return out, nil
}

func practicalSelectedChapterIndexes(block dto.PracticalBlock, requested map[int]struct{}, generateAll bool, chapterCursor int) []int {
	if generateAll {
		out := make([]int, 0, len(block.Chapters))
		for chapterIndex := range block.Chapters {
			out = append(out, chapterIndex)
		}
		return out
	}
	if len(requested) == 0 {
		return nil
	}
	out := make([]int, 0, len(block.Chapters))
	for chapterIndex := range block.Chapters {
		if _, ok := requested[chapterCursor+chapterIndex+1]; ok {
			out = append(out, chapterIndex)
		}
	}
	return out
}

func practicalSpeechText(turn dto.PracticalTurn) string {
	text := strings.TrimSpace(turn.SpeechText)
	if text != "" {
		return text
	}
	return strings.TrimSpace(turn.Text)
}
