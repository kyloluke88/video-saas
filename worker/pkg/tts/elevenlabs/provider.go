package elevenlabs

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
	"worker/pkg/tts/shared"
	services "worker/services"
)

type Provider struct {
	baseURL      string
	apiKey       string
	voiceID      string
	modelID      string
	outputFormat string
	enableLog    bool
}

type ttsResponse struct {
	AudioBase64         string    `json:"audio_base64"`
	Alignment           alignment `json:"alignment"`
	NormalizedAlignment alignment `json:"normalized_alignment"`
}

type DialogueInput struct {
	Text    string `json:"text"`
	VoiceID string `json:"voice_id"`
}

type DialogueResult struct {
	Audio               []byte
	Ext                 string
	Alignment           []shared.Subtitle
	NormalizedAlignment []shared.Subtitle
	VoiceSegments       []DialogueVoiceSegment
}

type DialogueVoiceSegment struct {
	VoiceID             string
	StartTime           int
	EndTime             int
	CharacterStartIndex int
	CharacterEndIndex   int
	DialogueInputIndex  int
}

type dialogueResponse struct {
	AudioBase64   string                 `json:"audio_base64"`
	Alignment     alignment              `json:"alignment"`
	Normalized    alignment              `json:"normalized_alignment"`
	VoiceSegments []dialogueVoiceSegment `json:"voice_segments"`
}

type dialogueVoiceSegment struct {
	VoiceID             string          `json:"voice_id"`
	StartTimeSeconds    json.RawMessage `json:"start_time_seconds"`
	EndTimeSeconds      json.RawMessage `json:"end_time_seconds"`
	CharacterStartIndex int             `json:"character_start_index"`
	CharacterEndIndex   int             `json:"character_end_index"`
	DialogueInputIndex  int             `json:"dialogue_input_index"`
}

type alignment struct {
	Characters    []string  `json:"characters"`
	StartTimesSec []float64 `json:"character_start_times_seconds"`
	EndTimesSec   []float64 `json:"character_end_times_seconds"`
}

func New(cfg shared.Config) (*Provider, error) {
	if strings.TrimSpace(cfg.ElevenLabsAPIKey) == "" {
		return nil, errors.New("elevenlabs api key is required")
	}
	baseURL := strings.TrimSpace(cfg.ElevenLabsBaseURL)
	if baseURL == "" {
		baseURL = "https://api.elevenlabs.io"
	}
	modelID := strings.TrimSpace(cfg.ElevenLabsModelID)
	if modelID == "" {
		modelID = "eleven_multilingual_v2"
	}
	outputFormat := strings.TrimSpace(cfg.ElevenLabsOutputFormat)
	if outputFormat == "" {
		outputFormat = "mp3_44100_128"
	}
	return &Provider{
		baseURL:      strings.TrimRight(baseURL, "/"),
		apiKey:       cfg.ElevenLabsAPIKey,
		voiceID:      strings.TrimSpace(cfg.ElevenLabsVoiceID),
		modelID:      modelID,
		outputFormat: outputFormat,
		enableLog:    cfg.ElevenLabsEnableLog,
	}, nil
}

func (p *Provider) Synthesize(ctx context.Context, req shared.Request) (shared.Result, error) {
	voiceID := p.voiceID
	if req.VoiceID != nil && strings.TrimSpace(*req.VoiceID) != "" {
		voiceID = strings.TrimSpace(*req.VoiceID)
	}
	if voiceID == "" {
		return shared.Result{}, errors.New("elevenlabs voice id is required")
	}

	body := map[string]any{
		"text":     req.Text,
		"model_id": p.modelID,
	}
	voiceSettings := buildVoiceSettings(req)
	if len(voiceSettings) > 0 {
		body["voice_settings"] = voiceSettings
	}
	if language := normalizeLanguageCode(req.Language); language != "" {
		body["language_code"] = language
		if language == "ja" {
			body["apply_text_normalization"] = "auto"
			body["apply_language_text_normalization"] = true
		}
	}
	raw, err := json.Marshal(body)
	if err != nil {
		return shared.Result{}, err
	}

	endpoint := fmt.Sprintf("%s/v1/text-to-speech/%s/with-timestamps", p.baseURL, url.PathEscape(voiceID))
	values := url.Values{}
	if strings.TrimSpace(p.outputFormat) != "" {
		values.Set("output_format", p.outputFormat)
	}
	values.Set("enable_logging", boolString(p.enableLog))
	if encoded := values.Encode(); encoded != "" {
		endpoint += "?" + encoded
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(raw))
	if err != nil {
		return shared.Result{}, err
	}
	httpReq.Header.Set("xi-api-key", p.apiKey)
	httpReq.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: 120 * time.Second}
	resp, err := client.Do(httpReq)
	if err != nil {
		return shared.Result{}, err
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)
	if resp.StatusCode >= 300 {
		err := fmt.Errorf("elevenlabs tts failed status=%d body=%s", resp.StatusCode, string(respBody))
		if isNonRetryableAPIError(resp.StatusCode, respBody) {
			return shared.Result{}, services.NonRetryableError{Err: err}
		}
		return shared.Result{}, err
	}

	var parsed ttsResponse
	if err := json.Unmarshal(respBody, &parsed); err != nil {
		return shared.Result{}, err
	}
	if strings.TrimSpace(parsed.AudioBase64) == "" {
		return shared.Result{}, nil
	}

	audioBytes, err := base64.StdEncoding.DecodeString(parsed.AudioBase64)
	if err != nil {
		return shared.Result{}, err
	}

	return shared.Result{
		Audio:       audioBytes,
		Ext:         outputExtForFormat(p.outputFormat),
		Subtitles:   convertAlignment(parsed.Alignment, parsed.NormalizedAlignment),
		RawResponse: respBody,
	}, nil
}

func (p *Provider) SynthesizeDialogue(ctx context.Context, inputs []DialogueInput, language string, stability *float64) (DialogueResult, error) {
	if len(inputs) == 0 {
		return DialogueResult{}, errors.New("elevenlabs dialogue inputs are required")
	}
	body := map[string]any{
		"inputs":   inputs,
		"model_id": dialogueModelID(p.modelID),
	}
	if stability != nil {
		body["settings"] = map[string]any{
			"stability": *stability,
		}
	}
	if normalized := normalizeLanguageCode(language); normalized != "" {
		body["language_code"] = normalized
		if normalized == "ja" {
			body["apply_text_normalization"] = "auto"
		}
	}
	raw, err := json.Marshal(body)
	if err != nil {
		return DialogueResult{}, err
	}

	endpoint := fmt.Sprintf("%s/v1/text-to-dialogue/with-timestamps", p.baseURL)
	values := url.Values{}
	if strings.TrimSpace(p.outputFormat) != "" {
		values.Set("output_format", p.outputFormat)
	}
	values.Set("enable_logging", boolString(p.enableLog))
	if encoded := values.Encode(); encoded != "" {
		endpoint += "?" + encoded
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(raw))
	if err != nil {
		return DialogueResult{}, err
	}
	httpReq.Header.Set("xi-api-key", p.apiKey)
	httpReq.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: 180 * time.Second}
	resp, err := client.Do(httpReq)
	if err != nil {
		return DialogueResult{}, err
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)
	if resp.StatusCode >= 300 {
		err := fmt.Errorf("elevenlabs dialogue failed status=%d body=%s", resp.StatusCode, string(respBody))
		if isNonRetryableAPIError(resp.StatusCode, respBody) {
			return DialogueResult{}, services.NonRetryableError{Err: err}
		}
		return DialogueResult{}, err
	}

	var parsed dialogueResponse
	if err := json.Unmarshal(respBody, &parsed); err != nil {
		return DialogueResult{}, err
	}
	if err := validateDialogueResponse(parsed, len(inputs)); err != nil {
		return DialogueResult{}, err
	}
	if strings.TrimSpace(parsed.AudioBase64) == "" {
		return DialogueResult{}, nil
	}

	audioBytes, err := base64.StdEncoding.DecodeString(parsed.AudioBase64)
	if err != nil {
		return DialogueResult{}, err
	}

	return DialogueResult{
		Audio:               audioBytes,
		Ext:                 outputExtForFormat(p.outputFormat),
		Alignment:           convertAlignment(parsed.Alignment, parsed.Normalized),
		NormalizedAlignment: convertAlignment(parsed.Normalized, parsed.Alignment),
		VoiceSegments:       convertVoiceSegments(parsed.VoiceSegments),
	}, nil
}

func validateDialogueResponse(resp dialogueResponse, inputCount int) error {
	if inputCount <= 0 {
		return errors.New("dialogue input count must be positive")
	}
	if err := validateAlignment("alignment", resp.Alignment); err != nil {
		return err
	}
	if err := validateAlignment("normalized_alignment", resp.Normalized); err != nil {
		return err
	}
	charCount := len(resp.Normalized.Characters)
	if charCount == 0 {
		charCount = len(resp.Alignment.Characters)
	}
	for i, seg := range resp.VoiceSegments {
		if seg.DialogueInputIndex < 0 || seg.DialogueInputIndex >= inputCount {
			return fmt.Errorf("dialogue response voice_segments[%d] invalid dialogue_input_index=%d", i, seg.DialogueInputIndex)
		}
		if seg.CharacterStartIndex < 0 || seg.CharacterEndIndex < seg.CharacterStartIndex {
			return fmt.Errorf("dialogue response voice_segments[%d] invalid character range %d-%d", i, seg.CharacterStartIndex, seg.CharacterEndIndex)
		}
		if charCount > 0 && seg.CharacterEndIndex > charCount {
			return fmt.Errorf("dialogue response voice_segments[%d] character_end_index=%d exceeds alignment chars=%d", i, seg.CharacterEndIndex, charCount)
		}
	}
	return nil
}

func validateAlignment(label string, item alignment) error {
	if len(item.Characters) == 0 {
		return nil
	}
	if len(item.Characters) != len(item.StartTimesSec) || len(item.Characters) != len(item.EndTimesSec) {
		return fmt.Errorf("dialogue response %s length mismatch chars=%d starts=%d ends=%d", label, len(item.Characters), len(item.StartTimesSec), len(item.EndTimesSec))
	}
	return nil
}

func convertAlignment(primary, fallback alignment) []shared.Subtitle {
	source := primary
	if len(source.Characters) == 0 {
		source = fallback
	}
	if len(source.Characters) == 0 {
		return nil
	}
	limit := minInt(len(source.Characters), len(source.StartTimesSec), len(source.EndTimesSec))
	out := make([]shared.Subtitle, 0, limit)
	for i := 0; i < limit; i++ {
		out = append(out, shared.Subtitle{
			Text:      source.Characters[i],
			BeginTime: int(source.StartTimesSec[i] * 1000),
			EndTime:   int(source.EndTimesSec[i] * 1000),
		})
	}
	return out
}

func normalizeLanguageCode(language string) string {
	switch strings.ToLower(strings.TrimSpace(language)) {
	case "ja", "ja-jp":
		return "ja"
	case "zh", "zh-cn":
		return "zh"
	case "en", "en-us":
		return "en"
	default:
		return strings.TrimSpace(language)
	}
}

func outputExtForFormat(outputFormat string) string {
	switch {
	case strings.HasPrefix(strings.ToLower(strings.TrimSpace(outputFormat)), "pcm_"):
		return "wav"
	default:
		return "mp3"
	}
}

func dialogueModelID(modelID string) string {
	if strings.TrimSpace(modelID) == "" || strings.TrimSpace(modelID) == "eleven_multilingual_v2" {
		return "eleven_v3"
	}
	return modelID
}

func boolString(v bool) string {
	if v {
		return "true"
	}
	return "false"
}

func buildVoiceSettings(req shared.Request) map[string]any {
	voiceSettings := map[string]any{}
	if req.Speed != nil && *req.Speed > 0 {
		voiceSettings["speed"] = *req.Speed
	}
	if req.Stability != nil {
		voiceSettings["stability"] = *req.Stability
	}
	if req.SimilarityBoost != nil {
		voiceSettings["similarity_boost"] = *req.SimilarityBoost
	}
	if req.Style != nil {
		voiceSettings["style"] = *req.Style
	}
	if req.UseSpeakerBoost != nil {
		voiceSettings["use_speaker_boost"] = *req.UseSpeakerBoost
	}
	return voiceSettings
}

func isNonRetryableAPIError(statusCode int, respBody []byte) bool {
	if statusCode == http.StatusPaymentRequired {
		return true
	}
	body := strings.ToLower(strings.TrimSpace(string(respBody)))
	return strings.Contains(body, "\"code\":\"paid_plan_required\"") ||
		strings.Contains(body, "\"type\":\"payment_required\"")
}

func minInt(values ...int) int {
	if len(values) == 0 {
		return 0
	}
	min := values[0]
	for _, value := range values[1:] {
		if value < min {
			min = value
		}
	}
	return min
}

func convertVoiceSegments(items []dialogueVoiceSegment) []DialogueVoiceSegment {
	if len(items) == 0 {
		return nil
	}
	out := make([]DialogueVoiceSegment, 0, len(items))
	for _, item := range items {
		out = append(out, DialogueVoiceSegment{
			VoiceID:             strings.TrimSpace(item.VoiceID),
			StartTime:           parseDialogueTimeMS(item.StartTimeSeconds),
			EndTime:             parseDialogueTimeMS(item.EndTimeSeconds),
			CharacterStartIndex: item.CharacterStartIndex,
			CharacterEndIndex:   item.CharacterEndIndex,
			DialogueInputIndex:  item.DialogueInputIndex,
		})
	}
	return out
}

func parseDialogueTimeMS(raw json.RawMessage) int {
	if len(raw) == 0 {
		return 0
	}
	var seconds float64
	if err := json.Unmarshal(raw, &seconds); err == nil {
		return int(seconds * 1000)
	}
	var millis int
	if err := json.Unmarshal(raw, &millis); err == nil {
		return millis
	}
	var text string
	if err := json.Unmarshal(raw, &text); err == nil {
		text = strings.TrimSpace(text)
		if text == "" {
			return 0
		}
		if v, err := time.ParseDuration(text + "s"); err == nil {
			return int(v / time.Millisecond)
		}
	}
	return 0
}
