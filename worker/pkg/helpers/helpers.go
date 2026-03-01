package helpers

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	amqp "github.com/rabbitmq/amqp091-go"
)

func HeaderRetry(headers amqp.Table) int {
	if headers == nil {
		return 0
	}
	if v, ok := headers["x-retry-count"]; ok {
		switch t := v.(type) {
		case int8:
			return int(t)
		case int16:
			return int(t)
		case int32:
			return int(t)
		case int64:
			return int(t)
		case int:
			return t
		}
	}
	return 0
}

func GetString(m map[string]interface{}, key string) string {
	v, ok := m[key]
	if !ok {
		return ""
	}
	return fmt.Sprintf("%v", v)
}

func GetInt(m map[string]interface{}, key string, fallback int) int {
	v, ok := m[key]
	if !ok {
		return fallback
	}
	switch t := v.(type) {
	case int:
		return t
	case int32:
		return int(t)
	case int64:
		return int(t)
	case float64:
		return int(t)
	case string:
		n, err := strconv.Atoi(t)
		if err == nil {
			return n
		}
	}
	return fallback
}

func GetBool(m map[string]interface{}, key string, fallback bool) bool {
	v, ok := m[key]
	if !ok {
		return fallback
	}
	switch t := v.(type) {
	case bool:
		return t
	case string:
		return strings.EqualFold(t, "true") || t == "1"
	default:
		return fallback
	}
}

func GetStringSlice(m map[string]interface{}, key string) []string {
	v, ok := m[key]
	if !ok {
		return nil
	}
	switch t := v.(type) {
	case []string:
		return t
	case []interface{}:
		out := make([]string, 0, len(t))
		for _, item := range t {
			out = append(out, fmt.Sprintf("%v", item))
		}
		return out
	default:
		return nil
	}
}

func DefaultString(value, fallback string) string {
	if value == "" {
		return fallback
	}
	return value
}

func StripCodeFence(s string) string {
	s = strings.TrimSpace(s)
	s = strings.TrimPrefix(s, "```json")
	s = strings.TrimPrefix(s, "```")
	s = strings.TrimSuffix(s, "```")
	return strings.TrimSpace(s)
}

func JoinURL(base, path string) string {
	base = strings.TrimRight(base, "/")
	path = strings.TrimLeft(path, "/")
	return base + "/" + path
}

func NormalizeDuration(sec int) string {
	switch {
	case sec <= 4:
		return "4"
	case sec <= 8:
		return "8"
	default:
		return "12"
	}
}

func Min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func Max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

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

func DownloadToFile(fileURL, targetPath string, timeoutSec int) error {
	client := &http.Client{Timeout: time.Duration(timeoutSec) * time.Second}
	resp, err := client.Get(fileURL)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 300 {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("download failed status=%d body=%s", resp.StatusCode, string(body))
	}

	f, err := os.Create(targetPath)
	if err != nil {
		return err
	}
	defer f.Close()
	_, err = io.Copy(f, resp.Body)
	return err
}
