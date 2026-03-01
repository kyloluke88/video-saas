package deepseek

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"strings"
	"time"

	"api/pkg/config"
)

type Config struct {
	BaseURL        string
	APIKey         string
	Model          string
	HTTPTimeoutSec int
}

// 从控制器传来的数据
type IdiomPlanInput struct {
	ProjectID         string
	IdiomName         string
	IdiomNameEn       string
	Dynasty           string
	Platform          string
	Category          string
	NarrationLanguage string
	TargetDurationSec int
	Audience          string
	Tone              string
	AspectRatio       string
	Resolution        string
	VisualStyle       string
	AnimationStyle    string
	ExpressionStyle   string
	CameraShotSize    string
	CameraAngle       string
	CameraMovement    string
}

// 是plan的一部分
type VisualBible struct {
	StyleAnchor       string `json:"style_anchor,omitempty"`
	CharacterAnchor   string `json:"character_anchor,omitempty"`
	EnvironmentAnchor string `json:"environment_anchor,omitempty"`
	NegativePrompt    string `json:"negative_prompt,omitempty"`
}

// 是plan的一部分
type ObjectSpec struct {
	ID        string                 `json:"id"`
	Type      string                 `json:"type,omitempty"`
	Label     string                 `json:"label,omitempty"`
	Immutable map[string]interface{} `json:"immutable,omitempty"`
	Mutable   map[string]interface{} `json:"mutable,omitempty"`
}

// 是plan的一部分
type ScenePlan struct {
	Index       int                    `json:"index"`
	DurationSec int                    `json:"duration_sec"`
	Goal        string                 `json:"goal,omitempty"`
	ObjectsRef  []string               `json:"objects_ref,omitempty"`
	Composition map[string]interface{} `json:"composition,omitempty"`
	Action      []string               `json:"action,omitempty"`
	Prompt      string                 `json:"prompt"`
	Narration   string                 `json:"narration"`
}

// The main output schema from DeepSeek planner, which can be directly consumed by the video generation service.
type ProjectPlanSchema struct {
	Title          string       `json:"title"`
	Hook           string       `json:"hook"`
	Narration      string       `json:"narration"`
	Characters     []string     `json:"characters"`
	Props          []string     `json:"props"`
	SceneElements  []string     `json:"scene_elements"`
	VisualBible    VisualBible  `json:"visual_bible"`
	ObjectRegistry []ObjectSpec `json:"object_registry"`
	Scenes         []ScenePlan  `json:"scenes"`
}

type deepSeekChatResponse struct {
	Choices []struct {
		Message struct {
			Content string `json:"content"`
		} `json:"message"`
	} `json:"choices"`
}

func LoadConfig() Config {
	return Config{
		BaseURL:        config.Get[string]("deepseek.base_url"),
		APIKey:         config.Get[string]("deepseek.api_key"),
		Model:          config.Get[string]("deepseek.model"),
		HTTPTimeoutSec: config.Get[int]("deepseek.http_timeout_sec"),
	}
}

func BuildIdiomPlan(cfg Config, input IdiomPlanInput) (ProjectPlanSchema, error) {
	if strings.TrimSpace(cfg.APIKey) == "" {
		return ProjectPlanSchema{}, errors.New("deepseek api key is empty")
	}
	prompt := buildIdiomPlannerPrompt(input)

	content, err := callDeepSeekPlan(cfg, prompt)
	if err != nil {
		return ProjectPlanSchema{}, err
	}

	content = stripCodeFence(content)

	var planSchema ProjectPlanSchema
	if err := json.Unmarshal([]byte(content), &planSchema); err != nil {
		fmt.Println("DeepSeek returned (first 200 chars):", string([]rune(content)[:200]))
		return ProjectPlanSchema{}, fmt.Errorf("deepseek json parse failed: %w", err)
	}
	if len(planSchema.Scenes) == 0 {
		return ProjectPlanSchema{}, errors.New("deepseek scenes empty")
	}

	for i := range planSchema.Scenes {
		if planSchema.Scenes[i].Index <= 0 {
			planSchema.Scenes[i].Index = i + 1
		}
		planSchema.Scenes[i].DurationSec = normalizeSeedanceDuration(planSchema.Scenes[i])
	}

	return planSchema, nil
}

func RunDeepSeekTest(cfg Config, prompt string) (string, error) {
	if strings.TrimSpace(cfg.APIKey) == "" {
		return "", errors.New("deepseek api key is empty")
	}
	if strings.TrimSpace(prompt) == "" {
		prompt = `Return strict JSON only: {"ok":true,"provider":"deepseek","msg":"ping"}`
	}
	return callDeepSeekPlan(cfg, prompt)
}

func buildIdiomPlannerPrompt(input IdiomPlanInput) string {

	return fmt.Sprintf(`You are a short-video director and production planner for Chinese idiom storytelling.
Create a structured JSON plan for a %d-second vertical video for platform=%s.

Idiom name (ZH): %s
Narration language: %s
Tone: %s
Global style lock:
- visual_style: %s
- animation_style: %s
- expression_style: %s
Global camera baseline:
- shot_size: %s
- angle: %s
- movement: %s

Era lock (fixed across all scenes): %s

Schema:
{
  "title": "short title",
  "hook": "first-second hook (<=12 words in narration language)",
  "visual_bible": {
    "style_anchor": "global style anchor for all scenes (1-2 sentences)",
    "environment_anchor": "global environment anchor for all scenes (1-2 sentences)",
    "character_anchor": "global character anchor for all scenes (1-2 sentences)",
    "negative_prompt": "comma-separated list of things to avoid globally"
  },
  "object_registry": [
    {
      "id": "snake_case_id",
      "type": "character|prop|environment",
      "label": "short human label in narration language",
      "immutable": {"key":"value"},
      "mutable": {"key":"value"}
    }
  ],
  "narration": "full narration script in narration language",
  "characters": ["extracted character labels from idiom story context"],
  "props": ["extracted prop labels from idiom story context"],
  "scene_elements": ["extracted scene element labels from idiom story context"],
  "scenes": [
    {
      "index": 1,
      "duration_sec": 4,
      "goal": "hook",
      "objects_ref": ["id1","id2"],
      "composition": {
        "shot_type": "e.g., static medium-wide / close-up / over-shoulder",
        "subject_blocking": ["2-3 concise lines describing positions and camera rules"]
      },
      "action": ["3-4 concise action lines in temporal order; end naturally"],
      "prompt": "scene delta prompt only (no global anchors repeated)",
      "narration": "scene narration line(s) in narration language"
    }
  ],
  "cta": "one short CTA in narration language"
}

Hard rules:
- JSON only. No markdown or extra text.
- Seedance duration constraint: each scene.duration_sec must be one of [4, 8, 12] only.
- Choose 4/8/12 based on scene complexity (action amount, blocking complexity, and narrative load):
  light scene -> 4s, medium scene -> 8s, heavy scene -> 12s.
- When designing scene plot (goal/composition/action/narration), prioritize 4s and 8s feasibility:
  4s scene = single clear beat, 1 key action progression, minimal blocking changes, one short narration clause.
  8s scene = two compact beats, 2-3 coherent action progressions, simple blocking continuity, 1-2 short narration clauses.
  12s scene = only if truly necessary for complex turning-point or consequence scenes with higher action/narrative load.
- Total duration should be as close as possible to target=%d while respecting the [4,8,12] rule.
- Platform pacing for %s: strong first 1-2s hook; optimize rhythm and CTA.
- Era lock fixed to "%s" for all scenes.
- Keep style lock across all scenes:
  visual_style="%s", animation_style="%s", expression_style="%s".
- Keep camera baseline across all scenes unless scene goal requires a slight variation:
  shot_size="%s", angle="%s", movement="%s".
- Extract characters/props/scene_elements from idiom story context first; return them in top-level arrays.
- Build object_registry from extracted elements; objects_ref must only reference registry IDs.
- Keep immutable attributes consistent across scenes; only mutable attributes can change.
- Each scene must include non-empty composition.shot_type, 2-3 composition.subject_blocking lines, and 3-4 action lines.
- Each scene.action must include 1-2 explicit audible cues aligned to visible actions
  (e.g., footsteps, wind, rustling grass, wood knock, distant birds).
- Audible cues must be natural diegetic sounds only; no dialogue, no narration, no UI/text sound effects.
- prompt is scene delta only; do not repeat global anchors.
- Narration must be in %s, fit duration, and use short TTS-friendly clauses.
- Cinematic but production-realistic; each scene ends naturally.
- Avoid medical claims and policy-risk language.`,
		input.TargetDurationSec,
		input.Platform,
		input.IdiomName,
		input.NarrationLanguage,
		input.Tone,
		input.VisualStyle,
		input.AnimationStyle,
		input.ExpressionStyle,
		input.CameraShotSize,
		input.CameraAngle,
		input.CameraMovement,
		input.Dynasty,
		input.TargetDurationSec,
		input.Platform,
		input.Dynasty,
		input.VisualStyle,
		input.AnimationStyle,
		input.ExpressionStyle,
		input.CameraShotSize,
		input.CameraAngle,
		input.CameraMovement,
		input.NarrationLanguage,
	)
}

// 共用
func callDeepSeekPlan(cfg Config, plannerPrompt string) (string, error) {

	client := &http.Client{Timeout: time.Duration(cfg.HTTPTimeoutSec) * time.Second}

	body := map[string]interface{}{
		"model": cfg.Model,
		"messages": []map[string]string{
			{"role": "system", "content": "You output strict JSON only."},
			{"role": "user", "content": plannerPrompt},
		},
		"temperature": 0.4,
	}
	raw, _ := json.Marshal(body)
	endpoint := strings.TrimRight(cfg.BaseURL, "/") + "/chat/completions"

	const maxAttempts = 5
	var lastErr error
	for attempt := 1; attempt <= maxAttempts; attempt++ {
		content, err := callDeepSeekPlanOnce(client, endpoint, cfg.APIKey, raw)
		if err == nil {
			return content, nil
		}
		lastErr = err
		if !isRetryableDeepSeekError(err) || attempt == maxAttempts {
			break
		}
		time.Sleep(time.Duration(attempt*2) * time.Second)
	}
	return "", lastErr
}

func callDeepSeekPlanOnce(client *http.Client, endpoint, apiKey string, raw []byte) (string, error) {
	req, err := http.NewRequest(http.MethodPost, endpoint, bytes.NewReader(raw))
	if err != nil {
		return "", err
	}
	req.Header.Set("Authorization", "Bearer "+apiKey)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", "video-saas-backend/1.0")
	req.Header.Set("Connection", "close")
	req.Header.Set("Accept-Encoding", "identity")

	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	respBody, readErr := io.ReadAll(resp.Body)
	if readErr != nil {
		return "", fmt.Errorf("deepseek read response failed: %w status=%d", readErr, resp.StatusCode)
	}
	if len(bytes.TrimSpace(respBody)) == 0 {
		return "", errors.New("deepseek empty response body")
	}
	if resp.StatusCode >= 300 {
		return "", fmt.Errorf("deepseek failed status=%d body=%s", resp.StatusCode, string(respBody))
	}

	var parsed deepSeekChatResponse
	if err := json.Unmarshal(respBody, &parsed); err != nil {
		return "", err
	}
	if len(parsed.Choices) == 0 {
		return "", errors.New("deepseek empty choices")
	}
	return parsed.Choices[0].Message.Content, nil
}

func isRetryableDeepSeekError(err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, io.ErrUnexpectedEOF) {
		return true
	}
	var netErr net.Error
	if errors.As(err, &netErr) && netErr.Timeout() {
		return true
	}

	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "deadline exceeded") ||
		strings.Contains(msg, "timeout") ||
		strings.Contains(msg, "unexpected eof") ||
		strings.Contains(msg, "temporary") ||
		strings.Contains(msg, "empty response body")
}

func stripCodeFence(s string) string {
	trimmed := strings.TrimSpace(s)
	if strings.HasPrefix(trimmed, "```") {
		trimmed = strings.TrimPrefix(trimmed, "```")
		trimmed = strings.TrimSpace(trimmed)
		if idx := strings.Index(trimmed, "\n"); idx >= 0 {
			firstLine := strings.TrimSpace(trimmed[:idx])
			if !strings.HasPrefix(firstLine, "{") && !strings.HasPrefix(firstLine, "[") {
				trimmed = strings.TrimSpace(trimmed[idx+1:])
			}
		}
		if end := strings.LastIndex(trimmed, "```"); end >= 0 {
			trimmed = strings.TrimSpace(trimmed[:end])
		}
	}
	return trimmed
}

func normalizeSeedanceDuration(scene ScenePlan) int {
	if scene.DurationSec == 4 || scene.DurationSec == 8 || scene.DurationSec == 12 {
		return scene.DurationSec
	}

	score := 0
	score += len(scene.Action) * 2
	score += len(scene.ObjectsRef)
	score += compositionBlockingCount(scene.Composition)

	narrLen := len([]rune(strings.TrimSpace(scene.Narration)))
	switch {
	case narrLen >= 45:
		score += 3
	case narrLen >= 20:
		score += 2
	case narrLen > 0:
		score += 1
	}

	goal := strings.ToLower(strings.TrimSpace(scene.Goal))
	if goal == "hook" || goal == "conclusion" {
		score--
	}

	switch {
	case score <= 5:
		return 4
	case score <= 11:
		return 8
	default:
		return 12
	}
}

func compositionBlockingCount(composition map[string]interface{}) int {
	if composition == nil {
		return 0
	}
	raw, ok := composition["subject_blocking"]
	if !ok || raw == nil {
		return 0
	}
	switch v := raw.(type) {
	case []interface{}:
		return len(v)
	case []string:
		return len(v)
	default:
		return 0
	}
}
