package practical_audio_service

import (
	"fmt"
	"strings"

	conf "worker/pkg/config"
	services "worker/services"
	dto "worker/services/practical/model"
)

const (
	practicalTTSTypeGoogle = 1
	practicalHeroRole      = "hero"
)

func normalizePracticalTTSType(value int) int {
	return practicalTTSTypeGoogle
}

func practicalTurnRole(turn dto.PracticalTurn) string {
	return firstNonEmpty(strings.TrimSpace(turn.SpeakerRole), strings.TrimSpace(turn.SpeakerID))
}

func practicalSpeakerByRole(block dto.PracticalBlock, role string) (dto.PracticalSpeaker, bool, error) {
	speakersByRole, err := block.SpeakersByRole()
	if err != nil {
		return dto.PracticalSpeaker{}, false, err
	}
	speaker, ok := speakersByRole[strings.TrimSpace(role)]
	return speaker, ok, nil
}

func practicalGoogleVoiceAssignments(projectDir, language string, block dto.PracticalBlock) (map[string]string, error) {
	return resolvePracticalSpeakerVoiceAssignments(projectDir, block, practicalGoogleHeroVoiceIDs(language), func(speaker dto.PracticalSpeaker) string {
		return strings.TrimSpace(speaker.GoogleVoiceID)
	}, practicalGoogleVoicePools())
}

func practicalAssignedVoiceIDForTurn(assignments map[string]string, turn dto.PracticalTurn) (string, error) {
	role := practicalTurnRole(turn)
	if role == "" {
		return "", services.NonRetryableError{Err: fmt.Errorf("turn %s speaker_role is required", strings.TrimSpace(turn.TurnID))}
	}
	voiceID := strings.TrimSpace(assignments[role])
	if voiceID == "" {
		return "", services.NonRetryableError{Err: fmt.Errorf("turn %s speaker_role %s is not declared in speakers", strings.TrimSpace(turn.TurnID), role)}
	}
	return voiceID, nil
}

func practicalGoogleHeroVoiceIDs(language string) map[string]string {
	maleVoiceID, femaleVoiceID := practicalTTSVoiceIDs(language)
	return map[string]string{
		"female": strings.TrimSpace(femaleVoiceID),
		"male":   strings.TrimSpace(maleVoiceID),
	}
}

func practicalGoogleVoicePools() map[string][]string {
	return map[string][]string{
		"female": splitVoicePool(conf.Get[string]("worker.google_tts_practical_female_voice_ids")),
		"male":   splitVoicePool(conf.Get[string]("worker.google_tts_practical_male_voice_ids")),
	}
}

func splitVoicePool(raw string) []string {
	if strings.TrimSpace(raw) == "" {
		return nil
	}
	parts := strings.Split(raw, ",")
	out := make([]string, 0, len(parts))
	seen := make(map[string]struct{}, len(parts))
	for _, part := range parts {
		value := strings.TrimSpace(part)
		if value == "" {
			continue
		}
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		out = append(out, value)
	}
	return out
}
