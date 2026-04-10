package jsonx

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
)

func WriteRawJSON(dir, filename string, raw []byte) error {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	path := filepath.Join(dir, filename)
	trimmed := bytes.TrimSpace(raw)
	if len(trimmed) == 0 {
		return os.WriteFile(path, []byte("{}\n"), 0o644)
	}

	var parsed interface{}
	if err := json.Unmarshal(trimmed, &parsed); err == nil {
		pretty, err := json.MarshalIndent(parsed, "", "  ")
		if err != nil {
			return err
		}
		pretty = append(pretty, '\n')
		return os.WriteFile(path, pretty, 0o644)
	}

	wrapped := map[string]string{"raw_text": string(trimmed)}
	pretty, err := json.MarshalIndent(wrapped, "", "  ")
	if err != nil {
		return err
	}
	pretty = append(pretty, '\n')
	return os.WriteFile(path, pretty, 0o644)
}
