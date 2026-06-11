package practical_audio_service

import (
	"fmt"
	"strings"

	services "worker/services"
	dto "worker/services/practical/model"
)

type practicalVoiceAssignmentFile struct {
	Version int                          `json:"version"`
	Google  map[string]map[string]string `json:"google,omitempty"`
}

func loadPracticalVoiceAssignmentFile(projectDir string) (practicalVoiceAssignmentFile, error) {
	assignments := practicalVoiceAssignmentFile{
		Version: 1,
		Google:  make(map[string]map[string]string),
	}
	if strings.TrimSpace(projectDir) == "" {
		return assignments, nil
	}
	path := projectSpeakerVoiceMapPath(projectDir)
	if !fileExists(path) {
		return assignments, nil
	}
	if err := readJSON(path, &assignments); err != nil {
		return practicalVoiceAssignmentFile{}, err
	}
	if assignments.Version <= 0 {
		assignments.Version = 1
	}
	if assignments.Google == nil {
		assignments.Google = make(map[string]map[string]string)
	}
	return assignments, nil
}

func (f *practicalVoiceAssignmentFile) save(projectDir string) error {
	if strings.TrimSpace(projectDir) == "" {
		return nil
	}
	if f.Version <= 0 {
		f.Version = 1
	}
	return writeJSON(projectSpeakerVoiceMapPath(projectDir), f)
}

func (f *practicalVoiceAssignmentFile) blockAssignments(blockID string) map[string]string {
	target := strings.TrimSpace(blockID)
	if target == "" {
		return nil
	}
	if f.Google == nil {
		f.Google = make(map[string]map[string]string)
	}
	if f.Google[target] == nil {
		f.Google[target] = make(map[string]string)
	}
	return f.Google[target]
}

func resolvePracticalSpeakerVoiceAssignments(
	projectDir string,
	block dto.PracticalBlock,
	heroVoices map[string]string,
	explicitVoice func(dto.PracticalSpeaker) string,
	pools map[string][]string,
) (map[string]string, error) {
	if explicitVoice == nil {
		explicitVoice = func(dto.PracticalSpeaker) string { return "" }
	}
	if _, err := block.SpeakersByRole(); err != nil {
		return nil, services.NonRetryableError{Err: err}
	}

	state, err := loadPracticalVoiceAssignmentFile(projectDir)
	if err != nil {
		return nil, err
	}
	stored := state.blockAssignments(block.BlockID)
	final := make(map[string]string, len(block.Speakers))
	usedVoices := make(map[string]struct{}, len(block.Speakers))

	for _, speaker := range block.Speakers {
		role := strings.TrimSpace(speaker.SpeakerRole)
		channel := normalizePracticalVoice(speaker.SpeakerID)
		if channel == "" {
			return nil, services.NonRetryableError{Err: fmt.Errorf("speaker %s requires speaker_id male/female", role)}
		}

		voiceID, err := resolvePracticalVoiceForSpeaker(role, channel, speaker, stored, usedVoices, heroVoices, explicitVoice, pools)
		if err != nil {
			return nil, err
		}
		final[role] = voiceID
		usedVoices[voiceID] = struct{}{}
	}

	if stored != nil {
		for key := range stored {
			delete(stored, key)
		}
		for role, voiceID := range final {
			stored[role] = voiceID
		}
	}
	if err := state.save(projectDir); err != nil {
		return nil, err
	}
	return final, nil
}

func resolvePracticalVoiceForSpeaker(
	role string,
	channel string,
	speaker dto.PracticalSpeaker,
	stored map[string]string,
	usedVoices map[string]struct{},
	heroVoices map[string]string,
	explicitVoice func(dto.PracticalSpeaker) string,
	pools map[string][]string,
) (string, error) {
	if strings.EqualFold(strings.TrimSpace(role), practicalHeroRole) {
		voiceID := strings.TrimSpace(heroVoices[channel])
		if voiceID == "" {
			return "", services.NonRetryableError{Err: fmt.Errorf("google hero %s voice id is required", channel)}
		}
		return voiceID, nil
	}

	if voiceID := strings.TrimSpace(explicitVoice(speaker)); voiceID != "" {
		return voiceID, nil
	}

	if stored != nil {
		if voiceID := strings.TrimSpace(stored[role]); voiceID != "" {
			if _, exists := usedVoices[voiceID]; !exists {
				return voiceID, nil
			}
		}
	}

	pool := pools[channel]
	if len(pool) == 0 {
		return "", services.NonRetryableError{Err: fmt.Errorf("google %s voice pool is empty for speaker %s", channel, role)}
	}
	for _, voiceID := range pool {
		voiceID = strings.TrimSpace(voiceID)
		if voiceID == "" {
			continue
		}
		if _, exists := usedVoices[voiceID]; exists {
			continue
		}
		return voiceID, nil
	}
	return "", services.NonRetryableError{Err: fmt.Errorf("google %s voice pool exhausted for speaker %s", channel, strings.TrimSpace(role))}
}
