package podcast_audio_service

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"

	"worker/pkg/elevenlabs"
)

func persistElevenTTSDebugArtifacts(
	blockStatesDir string,
	blockID string,
	request elevenlabs.SynthesizeDialogueWithTimestampsRequest,
	rawResponse []byte,
	audioBytes int,
) error {
	name := sanitizeSegmentID(blockID)
	requestPath := filepath.Join(blockStatesDir, fmt.Sprintf("%s.eleven_request.json", name))
	responsePath := filepath.Join(blockStatesDir, fmt.Sprintf("%s.eleven_response.json", name))

	requestDebug := map[string]interface{}{
		"inputs":        request.Inputs,
		"model_id":      strings.TrimSpace(request.ModelID),
		"output_format": strings.TrimSpace(request.OutputFormat),
		"prompt":        strings.TrimSpace(request.Prompt),
		"language_code": strings.TrimSpace(request.LanguageCode),
		"seed":          request.Seed,
		"speed":         request.Speed,
	}
	if err := writeJSON(requestPath, requestDebug); err != nil {
		return err
	}

	responseDebug := sanitizeElevenRawResponse(rawResponse, audioBytes)
	if err := writeJSON(responsePath, responseDebug); err != nil {
		return err
	}
	return nil
}

func sanitizeElevenRawResponse(raw []byte, audioBytes int) map[string]interface{} {
	out := map[string]interface{}{
		"raw_size_bytes": len(raw),
		"audio_base64": map[string]interface{}{
			"omitted":       true,
			"decoded_bytes": audioBytes,
		},
	}
	if len(raw) == 0 {
		return out
	}

	var payload map[string]interface{}
	if err := json.Unmarshal(raw, &payload); err != nil {
		out["parse_error"] = err.Error()
		return out
	}

	if audioBase64, ok := payload["audio_base64"].(string); ok {
		out["audio_base64"] = map[string]interface{}{
			"omitted":       true,
			"encoded_chars": len(audioBase64),
			"decoded_bytes": audioBytes,
		}
	}
	delete(payload, "audio_base64")
	payload["raw_size_bytes"] = len(raw)
	payload["audio_base64"] = out["audio_base64"]
	return payload
}
