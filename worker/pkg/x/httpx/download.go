package httpx

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"
)

func JoinURL(baseURL, path string) string {
	baseURL = strings.TrimRight(baseURL, "/")
	path = strings.TrimLeft(path, "/")
	return baseURL + "/" + path
}

func DownloadToFile(fileURL, targetPath string, timeoutSec int) error {
	return DownloadToFileWithContext(context.Background(), fileURL, targetPath, timeoutSec)
}

func DownloadToFileWithContext(ctx context.Context, fileURL, targetPath string, timeoutSec int) error {
	if ctx == nil {
		ctx = context.Background()
	}
	client := &http.Client{Timeout: time.Duration(timeoutSec) * time.Second}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, fileURL, nil)
	if err != nil {
		return err
	}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 300 {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("download failed status=%d body=%s", resp.StatusCode, string(body))
	}

	file, err := os.Create(targetPath)
	if err != nil {
		return err
	}
	defer file.Close()

	_, err = io.Copy(file, resp.Body)
	return err
}

func ParseURL(raw string) (*url.URL, error) {
	return url.Parse(raw)
}
