package idiom

import (
	"encoding/json"
	"os"
	"path/filepath"

	conf "worker/pkg/config"
)

func writeJSON(path string, data interface{}) error {
	raw, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, raw, 0o644)
}

func readJSON(path string, out interface{}) error {
	raw, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	return json.Unmarshal(raw, out)
}

func ensureProjectsDir() error {
	return os.MkdirAll(filepath.Join(conf.Get[string]("worker.ffmpeg_work_dir"), "projects"), 0o755)
}

func projectDirFor(projectID string) string {
	return filepath.Join(conf.Get[string]("worker.ffmpeg_work_dir"), "projects", projectID)
}
