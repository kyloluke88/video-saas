package wanxiang

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"api/pkg/config"
)

type Config struct {
	Enabled          bool
	BaseURL          string
	APIKey           string
	Model            string
	CreatePath       string
	TaskPathTemplate string
	Size             string
	PromptExtend     bool
	NumImages        int
	HTTPTimeoutSec   int
	PollIntervalSec  int
	MaxPollAttempts  int
}

type GenerateRequest struct {
	Prompt         string
	NegativePrompt string
}

type GenerateResult struct {
	TaskID    string
	ImageURLs []string
}

func LoadConfig() Config {
	return Config{
		Enabled:          config.Get[bool]("wanxiang.enabled"),
		BaseURL:          config.Get[string]("wanxiang.base_url"),
		APIKey:           config.Get[string]("wanxiang.api_key"),
		Model:            config.Get[string]("wanxiang.model"),
		CreatePath:       config.Get[string]("wanxiang.create_path"),
		TaskPathTemplate: config.Get[string]("wanxiang.task_path_template"),
		Size:             config.Get[string]("wanxiang.size"),
		PromptExtend:     config.Get[bool]("wanxiang.prompt_extend"),
		NumImages:        config.Get[int]("wanxiang.num_images"),
		HTTPTimeoutSec:   config.Get[int]("wanxiang.http_timeout_sec"),
		PollIntervalSec:  config.Get[int]("wanxiang.poll_interval_sec"),
		MaxPollAttempts:  config.Get[int]("wanxiang.max_poll_attempts"),
	}
}

func Generate(cfg Config, req GenerateRequest) (GenerateResult, error) {
	if !cfg.Enabled {
		return GenerateResult{}, nil
	}
	if strings.TrimSpace(cfg.APIKey) == "" {
		return GenerateResult{}, errors.New("wanxiang api key is empty")
	}
	if strings.TrimSpace(req.Prompt) == "" {
		return GenerateResult{}, errors.New("wanxiang prompt is empty")
	}

	imageURLs, taskID, err := createTask(cfg, req)
	if err != nil {
		return GenerateResult{}, err
	}
	if len(imageURLs) == 0 && strings.TrimSpace(taskID) != "" {
		imageURLs, err = pollTaskResult(cfg, taskID)
		if err != nil {
			return GenerateResult{}, err
		}
	}
	return GenerateResult{TaskID: taskID, ImageURLs: imageURLs}, nil
}

func createTask(cfg Config, req GenerateRequest) ([]string, string, error) {
	timeoutSec := cfg.HTTPTimeoutSec
	if timeoutSec <= 0 {
		timeoutSec = 120
	}
	client := &http.Client{Timeout: time.Duration(timeoutSec) * time.Second}

	numImages := cfg.NumImages
	if numImages <= 0 {
		numImages = 1
	}

	body := map[string]interface{}{
		"model": cfg.Model,
		"input": map[string]interface{}{
			"messages": []map[string]interface{}{
				{
					"role": "user",
					"content": []map[string]string{
						{"text": req.Prompt},
					},
				},
			},
		},
		"parameters": map[string]interface{}{
			"prompt_extend": cfg.PromptExtend,
			"watermark":     false,
			"n":             numImages,
			"size":          cfg.Size,
		},
	}
	body["parameters"].(map[string]interface{})["negative_prompt"] = strings.TrimSpace(req.NegativePrompt)

	raw, _ := json.Marshal(body)
	endpoint := strings.TrimRight(cfg.BaseURL, "/") + cfg.CreatePath
	httpReq, err := http.NewRequest(http.MethodPost, endpoint, bytes.NewReader(raw))
	if err != nil {
		return nil, "", err
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+cfg.APIKey)

	resp, err := client.Do(httpReq)
	if err != nil {
		return nil, "", err
	}
	defer resp.Body.Close()

	respRaw, _ := io.ReadAll(resp.Body)
	if resp.StatusCode >= 300 {
		return nil, "", fmt.Errorf("wanxiang create failed status=%d body=%s", resp.StatusCode, string(respRaw))
	}
	imageURLs, taskID, err := parseCreateResponse(respRaw)
	if err != nil {
		return nil, "", err
	}
	if len(imageURLs) == 0 && strings.TrimSpace(taskID) == "" {
		return nil, "", fmt.Errorf("wanxiang create no image and no task_id body=%s", string(respRaw))
	}
	return imageURLs, taskID, nil
}

func pollTaskResult(cfg Config, taskID string) ([]string, error) {
	timeoutSec := cfg.HTTPTimeoutSec
	if timeoutSec <= 0 {
		timeoutSec = 120
	}
	pollSec := cfg.PollIntervalSec
	if pollSec <= 0 {
		pollSec = 5
	}
	maxAttempts := cfg.MaxPollAttempts
	if maxAttempts <= 0 {
		maxAttempts = 60
	}

	client := &http.Client{Timeout: time.Duration(timeoutSec) * time.Second}
	endpointTpl := cfg.TaskPathTemplate
	if strings.TrimSpace(endpointTpl) == "" {
		endpointTpl = "/api/v1/tasks/%s"
	}
	endpoint := strings.TrimRight(cfg.BaseURL, "/") + fmt.Sprintf(endpointTpl, taskID)

	for i := 0; i < maxAttempts; i++ {
		httpReq, err := http.NewRequest(http.MethodGet, endpoint, nil)
		if err != nil {
			return nil, err
		}
		httpReq.Header.Set("Authorization", "Bearer "+cfg.APIKey)

		resp, err := client.Do(httpReq)
		if err != nil {
			return nil, err
		}
		respRaw, _ := io.ReadAll(resp.Body)
		resp.Body.Close()

		if resp.StatusCode >= 300 {
			return nil, fmt.Errorf("wanxiang status failed status=%d body=%s", resp.StatusCode, string(respRaw))
		}

		status, urls, err := parseTaskStatus(respRaw)
		if err != nil {
			return nil, err
		}
		switch status {
		case "SUCCEEDED", "SUCCESS":
			if len(urls) == 0 {
				return nil, errors.New("wanxiang task succeeded but image urls empty")
			}
			return urls, nil
		case "FAILED":
			return nil, fmt.Errorf("wanxiang task failed task_id=%s body=%s", taskID, string(respRaw))
		}

		time.Sleep(time.Duration(pollSec) * time.Second)
	}

	return nil, fmt.Errorf("wanxiang poll timeout task_id=%s", taskID)
}

func parseTaskStatus(raw []byte) (string, []string, error) {
	var parsed struct {
		Output struct {
			TaskStatus string `json:"task_status"`
			Results    []struct {
				URL string `json:"url"`
			} `json:"results"`
		} `json:"output"`
	}
	if err := json.Unmarshal(raw, &parsed); err != nil {
		return "", nil, fmt.Errorf("wanxiang status parse failed: %w body=%s", err, string(raw))
	}

	status := strings.ToUpper(strings.TrimSpace(parsed.Output.TaskStatus))
	urls := make([]string, 0, len(parsed.Output.Results))
	for _, item := range parsed.Output.Results {
		u := strings.TrimSpace(item.URL)
		if u != "" {
			urls = append(urls, u)
		}
	}
	return status, urls, nil
}

func parseCreateResponse(raw []byte) ([]string, string, error) {
	var root map[string]interface{}
	if err := json.Unmarshal(raw, &root); err != nil {
		return nil, "", fmt.Errorf("wanxiang create parse failed: %w body=%s", err, string(raw))
	}

	output, _ := root["output"].(map[string]interface{})
	if output == nil {
		return nil, "", nil
	}

	taskID := strings.TrimSpace(asString(output["task_id"]))
	urls := extractImageURLs(output)
	return urls, taskID, nil
}

func extractImageURLs(output map[string]interface{}) []string {
	out := make([]string, 0, 4)
	addURL := func(raw string) {
		u := strings.TrimSpace(raw)
		if u == "" || u == "<nil>" || strings.EqualFold(u, "null") {
			return
		}
		out = append(out, u)
	}

	if results, ok := output["results"].([]interface{}); ok {
		for _, item := range results {
			if m, ok := item.(map[string]interface{}); ok {
				addURL(asString(m["url"]))
			}
		}
	}

	if choices, ok := output["choices"].([]interface{}); ok {
		for _, choice := range choices {
			m, ok := choice.(map[string]interface{})
			if !ok {
				continue
			}
			message, _ := m["message"].(map[string]interface{})
			if message == nil {
				continue
			}
			content, _ := message["content"].([]interface{})
			for _, c := range content {
				cm, ok := c.(map[string]interface{})
				if !ok {
					continue
				}
				addURL(asString(cm["url"]))
				addURL(asString(cm["image"]))
			}
		}
	}

	return out
}

func asString(v interface{}) string {
	switch t := v.(type) {
	case nil:
		return ""
	case string:
		return t
	default:
		s := strings.TrimSpace(fmt.Sprintf("%v", v))
		if s == "" || s == "<nil>" || strings.EqualFold(s, "null") {
			return ""
		}
		return s
	}
}
