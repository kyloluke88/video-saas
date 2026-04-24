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
}

type GenerateResult struct {
	ScriptPath      string
	BlockAudioPaths []string
	Script          dto.PracticalScript
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

	blockAudioPaths := make([]string, 0, len(script.Blocks))
	for blockIndex, block := range script.Blocks {
		if len(requestedBlocks) > 0 {
			if _, ok := requestedBlocks[blockIndex+1]; !ok {
				continue
			}
		}

		speakerVoicesByRole, err := block.SpeakerVoicesByRole()
		if err != nil {
			return GenerateResult{}, fmt.Errorf("block %s: %w", block.BlockID, err)
		}
		speakerNames := practicalSpeakerNamesForTTS(block)
		audioPath := blockAudioPath(projectDir, block.BlockID, blockIndex+1)
		if err := synthesizeBlockAudio(ctx, client, language, block, speakerVoicesByRole, speakerNames, maleVoice, femaleVoice, audioPath); err != nil {
			return GenerateResult{}, err
		}
		topicAudioPath := blockIntroAudioPath(projectDir, block.BlockID, blockIndex+1)
		if err := synthesizeBlockTopicAudio(ctx, client, language, block, narratorVoice, topicAudioPath); err != nil {
			return GenerateResult{}, err
		}
		blockAudioPaths = append(blockAudioPaths, audioPath)
	}

	if len(blockAudioPaths) == 0 {
		return GenerateResult{}, services.NonRetryableError{Err: fmt.Errorf("no blocks selected for generation")}
	}

	log.Printf("📝 practical script cached project_id=%s source=%s path=%s", input.ProjectID, scriptPathFor(input.ScriptFilename), projectScriptInputPath(projectDir))
	return GenerateResult{
		ScriptPath:      projectScriptInputPath(projectDir),
		BlockAudioPaths: blockAudioPaths,
		Script:          script,
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

func synthesizeBlockAudio(
	ctx context.Context,
	client *googlecloud.Client,
	language string,
	block dto.PracticalBlock,
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

	turns := make([]googlecloud.ConversationTurn, 0, practicalBlockTurnCount(block))
	for _, turn := range practicalBlockTurns(block) {
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
		return services.NonRetryableError{Err: fmt.Errorf("block %s has no speakable turns", block.BlockID)}
	}

	prompt := buildPracticalTTSPrompt(language, block)
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
	return applyAudioTempoToFile(ctx, outputPath, practicalTurnTempo())
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
	topic := strings.TrimSpace(block.Topic)
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
	return applyAudioTempoToFile(ctx, outputPath, practicalBlockTempo())
}

func buildPracticalTTSPrompt(language string, block dto.PracticalBlock) string {
	lines := []string{
		"Read the dialogue naturally and clearly.",
		"Keep the pace beginner-friendly and easy to understand.",
		"Leave a noticeably longer pause between each turn so beginners can clearly follow the conversation.",
		"Do not rush the next turn; let the pause feel calm and deliberate.",
		"Keep each speaker's voice consistent and do not swap the female and male voices.",
		"Keep every turn in the original order.",
		"Do not add extra words.",
	}
	if topic := strings.TrimSpace(block.Topic); topic != "" {
		lines = append(lines, "Topic: "+topic)
	}
	if casting := practicalVoiceCastingNote(block); casting != "" {
		lines = append(lines, casting)
	}
	if placeholders := practicalBlockPromptPlaceholders(block); len(placeholders) > 0 {
		lines = append(lines, "Characters: "+strings.Join(placeholders, ", "))
	}
	if strings.EqualFold(strings.TrimSpace(language), "ja") {
		lines = append(lines, "Language: Japanese.")
	} else {
		lines = append(lines, "Language: Chinese.")
	}
	return strings.Join(lines, " ")
}

func buildPracticalBlockTopicPrompt(language string, block dto.PracticalBlock) string {
	lines := []string{
		"Speak this section title as a natural transition narration.",
		"Read only the title text exactly as written.",
		"Use normal pronunciation and a natural speaking rhythm.",
		"Do not sound flat, segmented, or overly slow.",
	}
	if prompt := strings.TrimSpace(block.BlockPrompt); prompt != "" {
		lines = append(lines, "Visual scene: "+prompt)
	}
	if strings.EqualFold(strings.TrimSpace(language), "ja") {
		lines = append(lines, "Language: Japanese.")
	} else {
		lines = append(lines, "Language: Chinese.")
	}
	return strings.Join(lines, " ")
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

func practicalBlockTurnCount(block dto.PracticalBlock) int {
	count := 0
	for _, chapter := range block.Chapters {
		count += len(chapter.Turns)
	}
	return count
}

func practicalBlockTurns(block dto.PracticalBlock) []dto.PracticalTurn {
	out := make([]dto.PracticalTurn, 0, practicalBlockTurnCount(block))
	for _, chapter := range block.Chapters {
		out = append(out, chapter.Turns...)
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
