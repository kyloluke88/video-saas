package podcast_audio_service

import (
	"fmt"
	"path/filepath"

	"worker/pkg/googlecloud"
)

func persistGoogleTTSDebugArtifacts(
	blockStatesDir string,
	blockID string,
	req googlecloud.SynthesizeConversationRequest,
) error {
	name := sanitizeSegmentID(blockID)
	requestPath := filepath.Join(blockStatesDir, fmt.Sprintf("%s.google_request.json", name))

	body := googlecloud.BuildConversationGenerateContentRequestBody(googlecloud.DefaultTTSModel, req)
	return writeJSON(requestPath, body)
}
