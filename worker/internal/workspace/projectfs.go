package workspace

import (
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	conf "worker/pkg/config"
	"worker/pkg/x/fsx"
	services "worker/services"
)

var replayProjectPattern = regexp.MustCompile(`^(.*)__rm\d+__\d{14}$`)

func ProjectDir(projectID string) string {
	return filepath.Join(conf.Get[string]("worker.ffmpeg_work_dir"), "projects", strings.TrimSpace(projectID))
}

func RequestPayloadPath(projectID string) string {
	return filepath.Join(ProjectDir(projectID), "request_payload.json")
}

func ReplaySourceProjectID(projectID string) (string, error) {
	projectID = strings.TrimSpace(projectID)
	if projectID == "" {
		return "", services.NonRetryableError{Err: fmt.Errorf("project_id is required")}
	}
	matches := replayProjectPattern.FindStringSubmatch(projectID)
	if len(matches) != 2 || strings.TrimSpace(matches[1]) == "" {
		return "", services.NonRetryableError{Err: fmt.Errorf("project_id is not a replay project id: %s", projectID)}
	}
	return strings.TrimSpace(matches[1]), nil
}

func EnsureReplayProjectDir(sourceProjectID, targetProjectID string) error {
	sourceProjectID = strings.TrimSpace(sourceProjectID)
	targetProjectID = strings.TrimSpace(targetProjectID)
	if sourceProjectID == "" {
		return services.NonRetryableError{Err: fmt.Errorf("source project id is required")}
	}
	if targetProjectID == "" {
		return services.NonRetryableError{Err: fmt.Errorf("project_id is required")}
	}

	targetDir := ProjectDir(targetProjectID)
	if _, err := os.Stat(targetDir); err == nil {
		return nil
	} else if !os.IsNotExist(err) {
		return err
	}

	sourceDir := ProjectDir(sourceProjectID)
	if _, err := os.Stat(sourceDir); err != nil {
		if os.IsNotExist(err) {
			return services.NonRetryableError{Err: fmt.Errorf("source project dir not found for %s: %w", sourceProjectID, err)}
		}
		return err
	}
	return copyReplayProjectDir(sourceDir, targetDir)
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

func copyReplayProjectDir(src, dst string) error {
	return filepath.WalkDir(src, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}
		if isReplayTransientPath(rel, d) {
			if d.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}

		targetPath := filepath.Join(dst, rel)
		if d.IsDir() {
			return os.MkdirAll(targetPath, 0o755)
		}
		return fsx.CopyFile(path, targetPath)
	})
}

func isReplayTransientPath(rel string, d fs.DirEntry) bool {
	if strings.TrimSpace(rel) == "." {
		return false
	}
	if !d.IsDir() {
		return false
	}
	name := strings.TrimSpace(filepath.Base(rel))
	return strings.HasPrefix(name, "mfa_align_")
}
