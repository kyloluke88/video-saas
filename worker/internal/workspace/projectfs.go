package workspace

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	conf "worker/pkg/config"
	services "worker/services"
)

func ProjectDir(projectID string) string {
	return filepath.Join(conf.Get[string]("worker.ffmpeg_work_dir"), "projects", strings.TrimSpace(projectID))
}

func RequestPayloadPath(projectID string) string {
	return filepath.Join(ProjectDir(projectID), "request_payload.json")
}

func WriteRequestPayload(projectID string, payload interface{}) error {
	projectDir := ProjectDir(projectID)
	if err := os.MkdirAll(projectDir, 0o755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(payload, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(RequestPayloadPath(projectID), data, 0o644)
}

func LoadRequestPayloadMap(projectID string) (map[string]interface{}, error) {
	raw, err := os.ReadFile(RequestPayloadPath(projectID))
	if err != nil {
		if os.IsNotExist(err) {
			return nil, services.NonRetryableError{Err: fmt.Errorf("project request payload not found for %s: %w", projectID, err)}
		}
		return nil, err
	}

	var payload map[string]interface{}
	if err := json.Unmarshal(raw, &payload); err != nil {
		return nil, fmt.Errorf("project request payload invalid for %s: %w", projectID, err)
	}
	return payload, nil
}
