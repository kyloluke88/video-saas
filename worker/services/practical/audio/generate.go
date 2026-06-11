package practical_audio_service

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"worker/pkg/googlecloud"
	services "worker/services"
	dto "worker/services/practical/model"
)

type GenerateInput struct {
	ProjectID      string
	TTSType        int
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
	return generateWithGoogle(ctx, projectDir, input, script, requestedBlocks, requestedChapterNums, generateAll)
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
	voiceAssignments map[string]string,
	outputPath string,
) error {
	if client == nil {
		return fmt.Errorf("google tts client is required")
	}
	if err := os.MkdirAll(filepath.Dir(outputPath), 0o755); err != nil {
		return err
	}

	speakerVoiceConfigs, speakerNames, err := practicalGoogleSpeakerConfigs(block, chapter, voiceAssignments)
	if err != nil {
		return err
	}
	turns := make([]googlecloud.ConversationTurn, 0, len(chapter.Turns))
	for _, turn := range chapter.Turns {
		speech := practicalSpeechText(turn)
		if speech == "" {
			continue
		}
		speakerRole := practicalTurnRole(turn)
		if speakerRole == "" {
			return services.NonRetryableError{Err: fmt.Errorf("turn %s speaker_role is required", strings.TrimSpace(turn.TurnID))}
		}
		turns = append(turns, googlecloud.ConversationTurn{
			Speaker: speakerRole,
			Text:    speech,
		})
	}
	if len(turns) == 0 {
		return services.NonRetryableError{Err: fmt.Errorf("chapter %s has no speakable turns", chapter.ChapterID)}
	}

	prompt := buildPracticalChapterTTSPrompt(language, block, chapter)
	if speakerMapping := practicalSpeakerVoiceMappingPrompt(block, chapter, voiceAssignments); speakerMapping != "" {
		prompt = strings.TrimSpace(prompt + " " + speakerMapping)
	}
	result, err := client.SynthesizeConversation(ctx, googlecloud.SynthesizeConversationRequest{
		LanguageCode:        language,
		Prompt:              prompt,
		Turns:               turns,
		SpeakerNames:        speakerNames,
		SpeakerVoiceConfigs: speakerVoiceConfigs,
		SpeakingRate:        practicalSpeakingRate(language),
	})
	if err != nil {
		return err
	}
	if err := os.WriteFile(outputPath, result.Audio, 0o644); err != nil {
		return err
	}
	return nil
}

func practicalGoogleSpeakerConfigs(
	block dto.PracticalBlock,
	chapter dto.PracticalChapter,
	voiceAssignments map[string]string,
) ([]googlecloud.SpeakerVoiceConfig, map[string]string, error) {
	activeRoles := practicalChapterSpeakerRoles(chapter)
	if len(activeRoles) == 0 {
		return nil, nil, services.NonRetryableError{Err: fmt.Errorf("chapter %s has no speakable turns", strings.TrimSpace(chapter.ChapterID))}
	}

	configs := make([]googlecloud.SpeakerVoiceConfig, 0, 2)
	names := make(map[string]string, 2)
	seenRoles := make(map[string]struct{}, 2)
	appendRole := func(role string) error {
		role = strings.TrimSpace(role)
		if role == "" {
			return nil
		}
		if _, exists := seenRoles[role]; exists {
			return nil
		}
		speaker, ok, err := practicalSpeakerByRole(block, role)
		if err != nil {
			return err
		}
		if !ok {
			return services.NonRetryableError{Err: fmt.Errorf("speaker_role %s is not declared in speakers", role)}
		}
		voiceID := strings.TrimSpace(voiceAssignments[role])
		if voiceID == "" {
			return services.NonRetryableError{Err: fmt.Errorf("speaker_role %s has no voice assignment", role)}
		}
		configs = append(configs, googlecloud.SpeakerVoiceConfig{
			Speaker: role,
			VoiceID: voiceID,
		})
		names[role] = firstNonEmpty(strings.TrimSpace(speaker.Name), role)
		seenRoles[role] = struct{}{}
		return nil
	}

	for _, role := range activeRoles {
		if err := appendRole(role); err != nil {
			return nil, nil, err
		}
	}
	if len(configs) < 2 {
		for _, speaker := range block.Speakers {
			if err := appendRole(strings.TrimSpace(speaker.SpeakerRole)); err != nil {
				return nil, nil, err
			}
			if len(configs) == 2 {
				break
			}
		}
	}
	if len(configs) != 2 {
		return nil, nil, services.NonRetryableError{Err: fmt.Errorf("chapter %s requires exactly 2 google speaker configs", strings.TrimSpace(chapter.ChapterID))}
	}
	return configs, names, nil
}

func practicalChapterSpeakerRoles(chapter dto.PracticalChapter) []string {
	roles := make([]string, 0, 2)
	seen := make(map[string]struct{}, 2)
	for _, turn := range chapter.Turns {
		if practicalSpeechText(turn) == "" {
			continue
		}
		role := practicalTurnRole(turn)
		if role == "" {
			continue
		}
		if _, exists := seen[role]; exists {
			continue
		}
		roles = append(roles, role)
		seen[role] = struct{}{}
	}
	return roles
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
		"This content is for beginners.",
		"Keep the overall speaking pace very slow and easy to follow.",
		"Keep the acting subtle but alive.",
		"Use light emotional variation such as greeting, hesitation, curiosity, confirmation, apology, gratitude, or emphasis when the line requires it.",
		"Do not sound flat, over-careful, or like isolated textbook example sentences.",
		"Keep each speaker's voice consistent and do not swap speaker identities.",
		"Keep every turn in the original order.",
		"Do not add extra words.",
		"Use clearly noticeable, wide pauses between turns so beginners can easily hear the speaker change.",
		"Leave a large, consistent, audible gap after each turn before the next speaker begins.",
		"At every speaker change, insert [extended pause, about 800ms] between the two turns.",
		"Do not rush turn changes.",
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
	if speakerRoles := practicalSpeakerRolesNote(block); speakerRoles != "" {
		lines = append(lines, speakerRoles)
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

func practicalSpeakerRolesNote(block dto.PracticalBlock) string {
	if len(block.Speakers) == 0 {
		return ""
	}
	roles := make([]string, 0, len(block.Speakers))
	for _, speaker := range block.Speakers {
		if role := strings.TrimSpace(speaker.SpeakerRole); role != "" {
			roles = append(roles, role)
		}
	}
	if len(roles) == 0 {
		return ""
	}
	return "Speaker roles in this chapter: " + strings.Join(roles, ", ") + "."
}

func practicalSpeakerVoiceMappingPrompt(
	block dto.PracticalBlock,
	chapter dto.PracticalChapter,
	voiceAssignments map[string]string,
) string {
	activeRoles := practicalChapterSpeakerRoles(chapter)
	if len(activeRoles) == 0 {
		return ""
	}

	lines := make([]string, 0, len(activeRoles)*2+4)
	lines = append(lines, "Speaker mapping for this chapter:")
	for _, role := range activeRoles {
		speaker, ok, err := practicalSpeakerByRole(block, role)
		if err != nil || !ok {
			continue
		}
		gender := normalizePracticalVoice(speaker.SpeakerID)
		if gender == "" {
			gender = "assigned"
		}
		voiceID := strings.TrimSpace(voiceAssignments[role])
		if voiceID == "" {
			continue
		}
		lines = append(lines, fmt.Sprintf("- %s: %s voice %s.", role, gender, voiceID))
	}

	for _, role := range activeRoles {
		speaker, ok, err := practicalSpeakerByRole(block, role)
		if err != nil || !ok {
			continue
		}
		gender := normalizePracticalVoice(speaker.SpeakerID)
		if gender == "" {
			gender = "assigned"
		}
		voiceID := strings.TrimSpace(voiceAssignments[role])
		if voiceID == "" {
			continue
		}
		lines = append(lines, fmt.Sprintf("Every line labeled \"%s:\" must be spoken only with the %s voice %s.", role, gender, voiceID))
	}

	lines = append(lines,
		"Never merge two speakers into one voice.",
		"Never let one speaker read another speaker's line.",
		"Keep the same assigned voice for the entire chapter, even when turns are short.",
	)
	return strings.Join(lines, " ")
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
	return strings.TrimSpace(turn.Text)
}
