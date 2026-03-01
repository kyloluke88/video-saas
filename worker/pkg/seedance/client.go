package seedance

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"worker/pkg/helpers"
)

func Generate(cfg Config, req GenerateRequest) (GenerateResult, error) {
	seedanceTaskID, createRespRaw, err := submitGenerate(cfg, req)
	if err != nil {
		return GenerateResult{CreateResponseBody: createRespRaw}, err
	}

	videoURL, statusRespRaw, err := pollResult(cfg, seedanceTaskID)
	return GenerateResult{
		VideoURL:           videoURL,
		CreateResponseBody: createRespRaw,
		StatusResponseBody: statusRespRaw,
	}, err
}

func submitGenerate(cfg Config, req GenerateRequest) (string, []byte, error) {
	body, err := json.Marshal(req)
	if err != nil {
		return "", nil, err
	}

	client := &http.Client{Timeout: time.Duration(cfg.HTTPTimeoutSec) * time.Second}
	endpoint := helpers.JoinURL(cfg.BaseURL, cfg.GeneratePath)
	httpReq, err := http.NewRequest(http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		return "", nil, err
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+cfg.APIKey)

	resp, err := client.Do(httpReq)
	if err != nil {
		return "", nil, err
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)
	if resp.StatusCode >= 300 {
		err := fmt.Errorf("seedance create failed status=%d body=%s", resp.StatusCode, string(respBody))
		if resp.StatusCode >= 400 && resp.StatusCode < 500 && resp.StatusCode != http.StatusTooManyRequests {
			return "", respBody, NonRetryableError{Err: err}
		}
		return "", respBody, err
	}

	var parsed generateResponse
	if err := json.Unmarshal(respBody, &parsed); err != nil {
		return "", respBody, fmt.Errorf("seedance create parse failed: %w body=%s", err, string(respBody))
	}
	if parsed.Data.TaskID == "" {
		return "", respBody, fmt.Errorf("seedance task_id missing body=%s", string(respBody))
	}
	return parsed.Data.TaskID, respBody, nil
}

func pollResult(cfg Config, seedanceTaskID string) (string, []byte, error) {
	client := &http.Client{Timeout: time.Duration(cfg.HTTPTimeoutSec) * time.Second}
	var lastStatusBody []byte
	for i := 0; i < cfg.MaxPollAttempts; i++ {
		statusResp, statusBody, err := getStatus(client, cfg, seedanceTaskID)
		if err != nil {
			return "", lastStatusBody, err
		}
		lastStatusBody = statusBody

		status := strings.ToUpper(statusResp.Data.Status)
		switch status {
		case "SUCCESS", "SUCCEEDED", "COMPLETED":
			videoURL := extractVideoURL(statusResp)
			if videoURL == "" {
				return "", lastStatusBody, errors.New("seedance success but video url missing")
			}
			return videoURL, lastStatusBody, nil
		case "FAILED", "ERROR", "CANCELED", "CANCELLED":
			if statusResp.Data.Error != "" {
				return "", lastStatusBody, fmt.Errorf("seedance failed: %s", statusResp.Data.Error)
			}
			return "", lastStatusBody, fmt.Errorf("seedance failed status=%s", statusResp.Data.Status)
		}
		time.Sleep(time.Duration(cfg.PollIntervalSec) * time.Second)
	}
	return "", lastStatusBody, fmt.Errorf("seedance poll timeout task_id=%s", seedanceTaskID)
}

func getStatus(client *http.Client, cfg Config, seedanceTaskID string) (*statusResponse, []byte, error) {
	endpoint := helpers.JoinURL(cfg.BaseURL, cfg.StatusPath)
	u, err := url.Parse(endpoint)
	if err != nil {
		return nil, nil, err
	}
	q := u.Query()
	q.Set("task_id", seedanceTaskID)
	u.RawQuery = q.Encode()

	httpReq, err := http.NewRequest(http.MethodGet, u.String(), nil)
	if err != nil {
		return nil, nil, err
	}
	httpReq.Header.Set("Authorization", "Bearer "+cfg.APIKey)

	resp, err := client.Do(httpReq)
	if err != nil {
		return nil, nil, err
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)
	if resp.StatusCode >= 300 {
		err := fmt.Errorf("seedance status failed status=%d body=%s", resp.StatusCode, string(respBody))
		if resp.StatusCode >= 400 && resp.StatusCode < 500 && resp.StatusCode != http.StatusTooManyRequests {
			return nil, respBody, NonRetryableError{Err: err}
		}
		return nil, respBody, err
	}

	var parsed statusResponse
	if err := json.Unmarshal(respBody, &parsed); err != nil {
		return nil, respBody, fmt.Errorf("seedance status parse failed: %w body=%s", err, string(respBody))
	}
	return &parsed, respBody, nil
}

func extractVideoURL(resp *statusResponse) string {
	if resp == nil {
		return ""
	}
	if resp.Data.VideoURL != "" {
		return resp.Data.VideoURL
	}
	if len(resp.Data.Response) > 0 {
		return resp.Data.Response[0]
	}
	if len(resp.Data.Output) > 0 {
		return resp.Data.Output[0].URL
	}
	return ""
}
