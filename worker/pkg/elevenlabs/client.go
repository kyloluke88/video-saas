package elevenlabs

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"
	"syscall"
	"time"

	services "worker/services"
)

var dialogueRequestRetryDelays = []time.Duration{
	3 * time.Second,
	10 * time.Second,
	30 * time.Second,
}

type Config struct {
	BaseURL            string
	DialoguePath       string
	APIKey             string
	ModelID            string
	OutputFormat       string
	HTTPTimeoutSeconds int
}

type DialogueInput struct {
	Text    string
	VoiceID string
}

type SynthesizeDialogueWithTimestampsRequest struct {
	Inputs       []DialogueInput
	ModelID      string
	OutputFormat string
	Prompt       string
	LanguageCode string
	Seed         int
	Speed        float64
}

type CharacterAlignment struct {
	Characters                 []string  `json:"characters"`
	CharacterStartTimesSeconds []float64 `json:"character_start_times_seconds"`
	CharacterEndTimesSeconds   []float64 `json:"character_end_times_seconds"`
}

type VoiceSegment struct {
	DialogueInputIndex  int     `json:"dialogue_input_index"`
	StartTimeSeconds    float64 `json:"start_time_seconds"`
	EndTimeSeconds      float64 `json:"end_time_seconds"`
	CharacterStartIndex int     `json:"character_start_index"`
	CharacterEndIndex   int     `json:"character_end_index"`
}

type DialogueAudioResult struct {
	Audio         []byte
	Ext           string
	Alignment     CharacterAlignment
	VoiceSegments []VoiceSegment
	RawResponse   []byte
}

type Client struct {
	cfg        Config
	httpClient *http.Client
}

func New(cfg Config) (*Client, error) {
	cfg.BaseURL = strings.TrimRight(strings.TrimSpace(cfg.BaseURL), "/")
	if cfg.BaseURL == "" {
		cfg.BaseURL = "https://api.elevenlabs.io"
	}
	cfg.DialoguePath = strings.TrimSpace(cfg.DialoguePath)
	if cfg.DialoguePath == "" {
		cfg.DialoguePath = "/v1/text-to-dialogue/with-timestamps"
	}
	if !strings.HasPrefix(cfg.DialoguePath, "/") {
		cfg.DialoguePath = "/" + cfg.DialoguePath
	}
	cfg.APIKey = strings.TrimSpace(cfg.APIKey)
	if cfg.APIKey == "" {
		return nil, errors.New("elevenlabs api key is required")
	}
	cfg.ModelID = strings.TrimSpace(cfg.ModelID)
	if cfg.ModelID == "" {
		cfg.ModelID = "eleven_v3"
	}
	cfg.OutputFormat = strings.TrimSpace(strings.ToLower(cfg.OutputFormat))
	if cfg.OutputFormat == "" {
		cfg.OutputFormat = "mp3_44100_128"
	}
	if cfg.HTTPTimeoutSeconds <= 0 {
		cfg.HTTPTimeoutSeconds = 180
	}

	return &Client{
		cfg: cfg,
		httpClient: &http.Client{
			Timeout: time.Duration(cfg.HTTPTimeoutSeconds) * time.Second,
		},
	}, nil
}

func (c *Client) SynthesizeDialogueWithTimestamps(ctx context.Context, req SynthesizeDialogueWithTimestampsRequest) (DialogueAudioResult, error) {
	if len(req.Inputs) == 0 {
		return DialogueAudioResult{}, errors.New("elevenlabs dialogue inputs are required")
	}

	modelID := strings.TrimSpace(req.ModelID)
	if modelID == "" {
		modelID = c.cfg.ModelID
	}
	outputFormat := strings.TrimSpace(strings.ToLower(req.OutputFormat))
	if outputFormat == "" {
		outputFormat = c.cfg.OutputFormat
	}

	inputs := make([]map[string]string, 0, len(req.Inputs))
	for _, input := range req.Inputs {
		text := strings.TrimSpace(input.Text)
		if text == "" {
			continue
		}
		voiceID := strings.TrimSpace(input.VoiceID)
		if voiceID == "" {
			return DialogueAudioResult{}, errors.New("elevenlabs dialogue voice_id is required for each non-empty input")
		}
		inputs = append(inputs, map[string]string{
			"text":     text,
			"voice_id": voiceID,
		})
	}
	if len(inputs) == 0 {
		return DialogueAudioResult{}, errors.New("elevenlabs dialogue inputs are empty")
	}

	body := map[string]any{
		"inputs":   inputs,
		"model_id": modelID,
	}
	if prompt := strings.TrimSpace(req.Prompt); prompt != "" {
		body["prompt"] = prompt
	}
	if languageCode := strings.TrimSpace(req.LanguageCode); languageCode != "" {
		body["language_code"] = languageCode
	}
	if req.Seed > 0 {
		body["seed"] = req.Seed
	}
	if req.Speed > 0 {
		body["settings"] = map[string]any{
			"speed": req.Speed,
		}
	}

	var resp struct {
		AudioBase64   string             `json:"audio_base64"`
		Alignment     CharacterAlignment `json:"alignment"`
		VoiceSegments []VoiceSegment     `json:"voice_segments"`
	}
	requestURL := c.cfg.BaseURL + c.cfg.DialoguePath
	if outputFormat != "" {
		formattedURL, err := withQueryParam(requestURL, "output_format", outputFormat)
		if err != nil {
			return DialogueAudioResult{}, err
		}
		requestURL = formattedURL
	}

	raw, err := c.doJSONWithRetry(ctx, http.MethodPost, requestURL, body, &resp)
	if err != nil {
		return DialogueAudioResult{}, err
	}
	if strings.TrimSpace(resp.AudioBase64) == "" {
		return DialogueAudioResult{}, errors.New("elevenlabs dialogue returned empty audio_base64")
	}

	audio, err := base64.StdEncoding.DecodeString(resp.AudioBase64)
	if err != nil {
		return DialogueAudioResult{}, err
	}
	return DialogueAudioResult{
		Audio:         audio,
		Ext:           audioExtForOutputFormat(outputFormat),
		Alignment:     resp.Alignment,
		VoiceSegments: resp.VoiceSegments,
		RawResponse:   raw,
	}, nil
}

func (c *Client) doJSON(ctx context.Context, method, requestURL string, body any, out any) ([]byte, error) {
	payload, err := json.Marshal(body)
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, method, requestURL, strings.NewReader(string(payload)))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("xi-api-key", c.cfg.APIKey)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		if isRetryableNetworkError(err) {
			return nil, retryableElevenLabsError{err: err}
		}
		return nil, err
	}
	defer resp.Body.Close()

	raw, _ := io.ReadAll(resp.Body)
	if resp.StatusCode >= 300 {
		err := fmt.Errorf("elevenlabs api failed status=%d body=%s", resp.StatusCode, strings.TrimSpace(string(raw)))
		if isNonRetryableStatus(resp.StatusCode) {
			return raw, services.NonRetryableError{Err: err}
		}
		if isRetryableStatus(resp.StatusCode) {
			return raw, retryableElevenLabsError{err: err}
		}
		return raw, err
	}
	if out != nil {
		if err := json.Unmarshal(raw, out); err != nil {
			return raw, err
		}
	}
	return raw, nil
}

func (c *Client) doJSONWithRetry(ctx context.Context, method, requestURL string, body any, out any) ([]byte, error) {
	attempts := len(dialogueRequestRetryDelays) + 1
	var lastRaw []byte
	var lastErr error
	for attempt := 1; attempt <= attempts; attempt++ {
		lastRaw, lastErr = c.doJSON(ctx, method, requestURL, body, out)
		if lastErr == nil {
			return lastRaw, nil
		}
		if !isRetryableElevenLabsRequestError(lastErr) || attempt == attempts {
			return lastRaw, lastErr
		}
		delay := dialogueRequestRetryDelays[attempt-1]
		timer := time.NewTimer(delay)
		select {
		case <-ctx.Done():
			timer.Stop()
			return lastRaw, ctx.Err()
		case <-timer.C:
		}
	}
	return lastRaw, lastErr
}

type retryableElevenLabsError struct {
	err error
}

func (e retryableElevenLabsError) Error() string {
	if e.err == nil {
		return "retryable elevenlabs error"
	}
	return e.err.Error()
}

func (e retryableElevenLabsError) Unwrap() error {
	return e.err
}

func isRetryableElevenLabsRequestError(err error) bool {
	if err == nil {
		return false
	}
	var retryable retryableElevenLabsError
	if errors.As(err, &retryable) {
		return true
	}
	if isRetryableNetworkError(err) {
		return true
	}
	return false
}

func isRetryableNetworkError(err error) bool {
	if err == nil {
		return false
	}
	var netErr net.Error
	if errors.As(err, &netErr) {
		return true
	}
	return errors.Is(err, syscall.ECONNRESET) || errors.Is(err, syscall.EPIPE)
}

func isRetryableStatus(status int) bool {
	return status == http.StatusTooManyRequests || status == http.StatusRequestTimeout || (status >= 500 && status <= 599)
}

func isNonRetryableStatus(status int) bool {
	switch status {
	case http.StatusBadRequest,
		http.StatusUnauthorized,
		http.StatusForbidden,
		http.StatusNotFound,
		http.StatusMethodNotAllowed,
		http.StatusUnprocessableEntity:
		return true
	default:
		return false
	}
}

func audioExtForOutputFormat(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	switch {
	case strings.HasPrefix(value, "mp3"):
		return "mp3"
	case strings.HasPrefix(value, "wav"), strings.HasPrefix(value, "pcm"), strings.HasPrefix(value, "linear16"):
		return "wav"
	case strings.HasPrefix(value, "ogg"), strings.HasPrefix(value, "opus"):
		return "ogg"
	default:
		return "mp3"
	}
}

func withQueryParam(rawURL, key, value string) (string, error) {
	parsed, err := url.Parse(strings.TrimSpace(rawURL))
	if err != nil {
		return "", err
	}
	query := parsed.Query()
	query.Set(strings.TrimSpace(key), strings.TrimSpace(value))
	parsed.RawQuery = query.Encode()
	return parsed.String(), nil
}
