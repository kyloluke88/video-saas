package googlecloud

import (
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
)

const googleTTSSampleRateHz = 24000
const googleTTSChannels = 1
const googleTTSBitsPerSample = 16
const DefaultTTSModel = "gemini-2.5-pro-preview-tts"

func BuildConversationGenerateContentRequestBody(model string, req SynthesizeConversationRequest) map[string]any {
	return buildGenerateContentRequestBody(model, buildConversationContents(req), buildConversationSpeechConfig(req))
}

func BuildSingleGenerateContentRequestBody(model string, req SynthesizeSingleRequest) map[string]any {
	return buildGenerateContentRequestBody(model, buildSingleContents(req), buildSingleSpeechConfig(req))
}

func buildGenerateContentRequestBody(model, contents string, speechConfig map[string]any) map[string]any {
	body := map[string]any{
		"model": strings.TrimSpace(model),
		"contents": []map[string]any{
			{
				"role": "user",
				"parts": []map[string]string{
					{"text": strings.TrimSpace(contents)},
				},
			},
		},
		"generationConfig": map[string]any{
			"responseModalities": []string{"AUDIO"},
			"speechConfig":       speechConfig,
		},
	}
	return body
}

func buildConversationContents(req SynthesizeConversationRequest) string {
	prompt := strings.TrimSpace(req.Prompt)
	transcript := buildConversationTranscript(req)
	switch {
	case prompt != "" && transcript != "":
		return prompt + "\n\n" + transcript
	case prompt != "":
		return prompt
	default:
		return transcript
	}
}

func buildConversationTranscript(req SynthesizeConversationRequest) string {
	parts := make([]string, 0, len(req.Turns))
	for _, turn := range req.Turns {
		text := strings.TrimSpace(turn.Text)
		if text == "" {
			continue
		}
		name := speakerDisplayName(req, turn.Speaker)
		parts = append(parts, fmt.Sprintf("%s: %s", name, text))
	}
	return strings.Join(parts, "\n")
}

func speakerDisplayName(req SynthesizeConversationRequest, speaker string) string {
	key := normalizeSpeaker(speaker)
	if req.SpeakerNames != nil {
		if name := strings.TrimSpace(req.SpeakerNames[key]); name != "" {
			return name
		}
	}
	return key
}

func buildConversationSpeechConfig(req SynthesizeConversationRequest) map[string]any {
	return map[string]any{
		"multiSpeakerVoiceConfig": map[string]any{
			"speakerVoiceConfigs": []map[string]any{
				{
					"speaker": speakerDisplayName(req, "male"),
					"voiceConfig": map[string]any{
						"prebuiltVoiceConfig": map[string]any{
							"voiceName": strings.TrimSpace(req.MaleVoiceID),
						},
					},
				},
				{
					"speaker": speakerDisplayName(req, "female"),
					"voiceConfig": map[string]any{
						"prebuiltVoiceConfig": map[string]any{
							"voiceName": strings.TrimSpace(req.FemaleVoiceID),
						},
					},
				},
			},
		},
	}
}

func buildSingleContents(req SynthesizeSingleRequest) string {
	prompt := strings.TrimSpace(req.Prompt)
	text := strings.TrimSpace(req.Text)
	switch {
	case prompt != "" && text != "":
		return prompt + "\n\n" + text
	case prompt != "":
		return prompt
	default:
		return text
	}
}

func buildSingleSpeechConfig(req SynthesizeSingleRequest) map[string]any {
	return map[string]any{
		"voiceConfig": map[string]any{
			"prebuiltVoiceConfig": map[string]any{
				"voiceName": strings.TrimSpace(req.VoiceID),
			},
		},
	}
}

func buildGenerateContentURL(urlPattern, model string) string {
	urlPattern = strings.TrimSpace(urlPattern)
	if strings.Contains(urlPattern, "%s") {
		return fmt.Sprintf(urlPattern, strings.TrimSpace(model))
	}
	return urlPattern
}

func decodeInlineAudioFromResponse(raw []byte) ([]byte, error) {
	var parsed struct {
		Candidates []struct {
			Content struct {
				Parts []struct {
					InlineData *struct {
						Data string `json:"data"`
					} `json:"inlineData"`
				} `json:"parts"`
			} `json:"content"`
		} `json:"candidates"`
	}
	if err := json.Unmarshal(raw, &parsed); err != nil {
		return nil, err
	}
	for _, candidate := range parsed.Candidates {
		for _, part := range candidate.Content.Parts {
			if part.InlineData == nil || strings.TrimSpace(part.InlineData.Data) == "" {
				continue
			}
			return wavFromPCMBase64(part.InlineData.Data)
		}
	}
	return nil, errors.New("google tts returned no inline audio data")
}

func wavFromPCMBase64(data string) ([]byte, error) {
	pcm, err := decodeBase64(data)
	if err != nil {
		return nil, err
	}
	return pcmToWAV(pcm, googleTTSSampleRateHz, googleTTSChannels, googleTTSBitsPerSample), nil
}

func decodeBase64(data string) ([]byte, error) {
	return base64.StdEncoding.DecodeString(strings.TrimSpace(data))
}

func pcmToWAV(pcm []byte, sampleRate, channels, bitsPerSample int) []byte {
	if sampleRate <= 0 {
		sampleRate = googleTTSSampleRateHz
	}
	if channels <= 0 {
		channels = googleTTSChannels
	}
	if bitsPerSample <= 0 {
		bitsPerSample = googleTTSBitsPerSample
	}

	byteRate := sampleRate * channels * bitsPerSample / 8
	blockAlign := channels * bitsPerSample / 8
	dataSize := len(pcm)
	riffSize := 36 + dataSize

	out := make([]byte, 44+dataSize)
	copy(out[0:4], []byte("RIFF"))
	binary.LittleEndian.PutUint32(out[4:8], uint32(riffSize))
	copy(out[8:12], []byte("WAVE"))
	copy(out[12:16], []byte("fmt "))
	binary.LittleEndian.PutUint32(out[16:20], 16)
	binary.LittleEndian.PutUint16(out[20:22], 1)
	binary.LittleEndian.PutUint16(out[22:24], uint16(channels))
	binary.LittleEndian.PutUint32(out[24:28], uint32(sampleRate))
	binary.LittleEndian.PutUint32(out[28:32], uint32(byteRate))
	binary.LittleEndian.PutUint16(out[32:34], uint16(blockAlign))
	binary.LittleEndian.PutUint16(out[34:36], uint16(bitsPerSample))
	copy(out[36:40], []byte("data"))
	binary.LittleEndian.PutUint32(out[40:44], uint32(dataSize))
	copy(out[44:], pcm)
	return out
}
