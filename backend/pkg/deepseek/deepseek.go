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
	"api/pkg/logger"
)

type Config struct {
	Enable         bool
	BaseURL        string
	APIKey         string
	Model          string
	HTTPTimeoutSec int
}

type IdiomPlanInput struct {
	ProjectID         string
	IdiomName         string
	IdiomNameEn       string
	Description       string
	Category          string
	NarrationLanguage string
	TargetDurationSec int
	AspectRatio       string
	Resolution        string
}

type ProjectPlanSchema struct {
	Meta           PlanMeta           `json:"meta"`
	VisualBible    PlanVisualBible    `json:"visual_bible"`
	ObjectRegistry PlanObjectRegistry `json:"object_registry"`
	Scenes         []PlanScene        `json:"scenes"`
}

type PlanMeta struct {
	Title                 string `json:"title"`
	TargetTotalSeconds    int    `json:"target_total_seconds"`
	FirstSceneFixedSecond int    `json:"first_scene_fixed_duration"`
}

type PlanVisualBible struct {
	StyleAnchor    string `json:"style_anchor"`
	ColorPalette   string `json:"color_palette"`
	Lighting       string `json:"lighting"`
	EraSetting     string `json:"era_setting"`
	NegativePrompt string `json:"negative_prompt"`
}

type PlanObjectRegistry struct {
	Characters   []PlanObjectSpec `json:"characters"`
	Props        []PlanObjectSpec `json:"props"`
	Environments []PlanObjectSpec `json:"environments"`
}

type PlanObjectSpec struct {
	ID        string                 `json:"id"`
	Immutable map[string]interface{} `json:"immutable"`
}

type PlanScene struct {
	SceneID         int            `json:"scene_id"`
	DurationSeconds int            `json:"duration_seconds"`
	ActionUnit      PlanActionUnit `json:"action_unit"`
	AudioLayer      PlanAudioLayer `json:"audio_layer"`
	Camera          PlanCamera     `json:"camera"`
	Cast            PlanCast       `json:"cast"`
}

type PlanActionUnit struct {
	UnitGoal   string     `json:"unit_goal"`
	StartState string     `json:"start_state"`
	Beats      []PlanBeat `json:"beats"`
	EndState   string     `json:"end_state"`
}

type PlanBeat struct {
	TRange        string `json:"t_range"`
	MicroAction   string `json:"micro_action"`
	VisibleChange string `json:"visible_change"`
}

type PlanAudioLayer struct {
	Dialogue []PlanDialogue `json:"dialogue"`
}

type PlanDialogue struct {
	Speaker string `json:"speaker"`
	TRange  string `json:"t_range"`
	Content string `json:"content"`
}

type PlanCamera struct {
	ShotType string `json:"shot_type"`
	Angle    string `json:"angle"`
	Movement string `json:"movement"`
	Focus    string `json:"focus"`
}

type PlanCast struct {
	Characters  []string `json:"characters"`
	Props       []string `json:"props"`
	Environment string   `json:"environment"`
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
		Enable:         config.Get[bool]("deepseek.enable"),
		BaseURL:        config.Get[string]("deepseek.base_url"),
		APIKey:         config.Get[string]("deepseek.api_key"),
		Model:          config.Get[string]("deepseek.model"),
		HTTPTimeoutSec: config.Get[int]("deepseek.http_timeout_sec"),
	}
}

func BuildIdiomPlan(cfg Config, input IdiomPlanInput) (json.RawMessage, error) {
	prompt := buildIdiomPlannerPrompt(input)

	logger.InfoString("LLM generate", "deepseek_planner_generate", prompt)

	return json.RawMessage{}, nil
	content, err := callDeepSeekPlan(cfg, prompt)
	if err != nil {
		return nil, err
	}

	content = stripCodeFence(content)

	logger.InfoString("LLM response", "deepseek_planner_response", content)
	return json.RawMessage(content), nil
}

func RunDeepSeekTest(cfg Config, prompt string) (string, error) {
	return callDeepSeekPlan(cfg, prompt)
}

func buildIdiomPlannerPrompt(input IdiomPlanInput) string {
	return fmt.Sprintf(`You are an expert storyboard-script designer for an AI video generation model (only supports 4s / 8s clips).
Output JSON ONLY. Do NOT output Markdown. Do NOT output any explanations.

========================
PART 1: 硬性约束（最高优先级，必须严格遵守）
========================

[DURATION RULES 时长规则]

- 每个 scene.duration_seconds 只能是：4、8。
- 所有 scene.duration_seconds 的总和必须 **完全等于** target_total_seconds。
- meta.allowed_clip_durations 必须是 [4,8]。

[ID & REFERENCE RULES 对象与引用规则]

- 所有 ID 必须使用 English snake_case。
- 在 action_unit、audio_layer、camera、cast 中提到的对象必须在 object_registry 中定义，并且通过 id （英文）来引用。
- speaker 如果是 "narrator" 可以不注册, 因为 narrator 只允许作为音频出现。
- 【重要】在 action_unit (包括 unit_goal,start_state, end_state, beats 中的 micro_action, visible_change)、camera.composition、以及 audio_layer(ambience, sound_effects 中的 description 等所有需要描述的地方，当提及任何已在 object_registry 中注册的角色、道具或环境时，必须使用其对应的英文 id, 严禁使用中文名称。 例如，应使用 "tiger" 而非“老虎”，使用 "fox" 而非“狐狸”。

[BEATS TIME COVERAGE RULES 时间覆盖规则]

- 每个 scene.beats 必须从 0.0 开始。
- beats 必须覆盖整个 scene 时长直到 duration_seconds 结束。
- 所有 t_range 必须格式为 "x.x-y.y"，并且 **保留一位小数**。

========================
PART 2: 音频规则 AudioPolicy（必须遵守）
========================

[SAFETY PAD 安全尾部]

- duration=4  → safety_pad=0.8
- duration=8  → safety_pad=0.8

- 每一条 dialogue 的结束时间必须：
  dialogue.t_range.end ≤ duration_seconds - safety_pad

- 禁止在尾部禁区开始新的对白：
  - 4秒场景：dialogue start < 3.3
  - 8秒场景：dialogue start < 7.0

[CHINESE DIALOGUE ESTIMATION 中文对白时长估算]

对白最短时长计算：
 - estimated_sec = 中文汉字 / 3.3 + 0.33, (只统计汉字，不包含标点、空格、字母、数字)

对白分配时间必须满足：
 - (t_end - t_start) ≥ estimated_sec
 - 多条对白之间必须至少间隔 0.10 秒。
 - 如果违反 safety_pad,必须整体向前移动。

[DIALOGUE LENGTH LIMITS 对白长度限制]
- 每条对白 ≤ 10 汉字, 推荐 ≤ 9 汉字,超过 10 必须自动压缩表达。

[TAIL SILENCE 尾部静默]
每个 scene 的最后 safety_pad 秒：
- 禁止对白, 只允许 ambience 或 sound_effects
- action_unit.end_state 必须发生在这个尾部区域内。

========================
PART 3: 尾帧稳定规则 TAIL HOLD STABILITY
========================

[TAIL HOLD RULE]

最后一个 beat 必须是 tail_hold, 其时间必须 精确等于：
(duration_seconds - safety_pad) → duration_seconds

最终 beat 必须：
start = duration_seconds - safety_pad  
end   = duration_seconds

格式：
"x.x-y.y"

其中：
x.x = duration_seconds - safety_pad  
y.y = duration_seconds

- 这个 beat 中 micro_action 必须包含 "hold still" 或 "no new action", 禁止新的剧情动作。
- 只允许微小环境变化，例如：
  - fog drift
  - breathing
  - cloth movement
  - light flicker

[END_STATE GENERATION RULE]

end_state 必须基于以下两个要素生成：
1. 从最后一个 beat 的 visible_change 推导出"变化完成后的状态"
2. 将最后一个 beat 的 emotional_tone 转化为"场景的氛围描述"

格式要求：
- visible_change 描述"什么在变化/正在发生"
- end_state 描述"变化完成后的结果"
- emotional_tone 中的情绪关键词应体现在 end_state 的环境氛围中

作用：
- 保证音频尾部干净
- 保持画面稳定
- 确保叙事连贯
- 情绪氛围统一
- 方便剪辑。

========================
PART 4: CAMERA ENUM CONSTRAINTS（镜头枚举规则）
========================
camera[].shot_type 必须属于：
["close_up","medium","wide","over_shoulder","insert","two_shot","establishing","extreme_close_up"]

camera[].angle 必须属于：
["eye_level","low_angle","high_angle","top_down","overhead"]

camera[].movement 必须属于：
["static","pan","tilt","dolly_in","dolly_out","track","handheld","arc","push_in","pull_back"]

camera[].movement_speed 必须属于：
["slow","medium","fast"]

camera[].focus 必须属于：
["character_face","character_full_body","hands","prop","foreground_object","environment_depth","background_action"]

camera[].transition_to_next 必须属于：
["cut","match_cut","fade","wipe","whip_pan","sound_bridge"]

[BEAT-CAMERA COVERAGE RULE]

每个 beat 在其 t_range 内，必须被至少一个 camera 覆盖，且：
- 如果 beat 的 visible_change 描述面部变化 → 覆盖该 beat 的 camera 必须有 "character_face"
- 如果 beat 的 visible_change 描述手部动作 → 覆盖该 beat 的 camera 必须有 "hands" 或 "prop"
- 如果 beat 跨越多个 camera 分段 → 每个分段都要满足对应的 focus 要求

剧情信息必须写在：
action_unit 或 camera.composition。

========================
PART 5: CAMERA SEGMENT COUNT RULE
========================

[GENERAL STRUCTURE]

每个 camera 元素必须包含：
- t_range
- shot_type
- angle
- movement
- movement_speed
- focus
- composition

[FOR 4s SCENES]

if duration_seconds 为 4：

- camera 只能有 1 个元素
- t_range 必须是：0.0-duration_seconds
- 禁止内部镜头切换。

[FOR 8s SCENES]

if duration_seconds 为 8：
camera 可以最多有2个元素，但必须：
 - 无重叠
 - 无空隙
 - 从 0.0 开始
 - 在 8.0 结束

每个镜头 ≥ 2 秒。
最后 safety_pad 区间禁止镜头变化。
最终 camera 必须覆盖：(duration_seconds - safety_pad) → duration_seconds。

========================
PART 6: 画面比例规则 FRAME & ASPECT RATIO
========================

aspect_ratio: %s

[GENERAL FRAMING RULE]
所有构图必须针对该比例设计。
禁止默认使用 16:9 思维。
[9:16 Vertical]

- 使用前景→中景→背景的纵向层次
- 主体保持在中间 65%%
- 避免顶部 12%% 和底部 22%%

强调 **纵深而不是横向宽度**。

========================
PART 7: 4秒叙事原子规则 4-SECOND SCENE SPECIAL RULES
========================

4秒场景只能表达 一个叙事单元：
- Hook
- Setup
- Key Action
- Reaction
- Reveal
- Transition

- 禁止同时引入角色并解决冲突
- 禁止多个独立动作, 但是可以一个主要动作 + 一个稳定动作
- 禁止复杂动作 + 对白同时发生。

动作规则：
- 1 个清晰动作
- 1 个稳定结果。

========================
PART 8: 创作原则 CREATIVE PRINCIPLES
========================

你正在创作 **视觉叙事**：
用画面讲故事。
要求：
- 每个 scene 推动剧情
- 最终 scene 必须视觉呈现成语寓意
- 动作必须符合现实物理
- 禁止心理独白
- 使用可见动作而不是抽象词
- 空间关系必须稳定
- 时间流动合理。

========================
PART 9: object_registry REQUIREMENTS
========================

[OPTIONAL FIELDS RULE]

characters.immutable 中的以下字段为可选字段，**只有当有具体值时**才包含在输出中：

- age
- gender
- body_type
- height_reference
- skin_tone
- face_shape
- hair_style
- clothing_top
- clothing_bottom
- shoes
- accessories
- distinctive_features
- expression_base

props.immutable 中的以下字段为可选字段，**只有当有具体值时**才包含在输出中：

- type
- material
- color
- size_reference
- surface_detail
- wear_marks
- default_position

environments.immutable 中的以下字段为可选字段，**只有当有具体值时**才包含在输出中：

- layout
- key_landmarks
- spatial_reference
- time_of_day
- weather
- ground_texture
- background_depth

[FIELD INCLUSION RULE]

- 只有当某个属性能够匹配到具体的、有意义的描述时，才将该字段包含在输出中
- 如果无法匹配到合适的值，则**完全省略该字段**，不在 JSON 中显示
- 禁止使用空字符串 "" 或 "无"、"none" 等占位符
- 禁止为了保持字段存在而填入无意义的默认值

========================
PART 10: SCENE 1 HOOK RULES
========================

scene_id=1 必须：

- 在 3 秒内出现强视觉钩子
- 激发好奇
- 避免平淡开场。
- 不一定非要 duration_seconds = 4

========================
PART 11: META CHECK
========================

生成 JSON 后必须保证：

meta.duration_sum_check = true  
meta.audio_policy_check = true  
meta.invalid_ids = []  
meta.violations = []

如果违反任何规则：

必须 **内部修正** 后再输出。
最终 JSON **不能包含违规**。

========================
OUTPUT JSON SCHEMA (MUST MATCH EXACTLY)
========================

{
  "meta": {
    "title": %s,
    "target_total_seconds": %d,
    "allowed_clip_durations": [4,8],
    "duration_sum_check": true,
    "audio_policy_check": true,
    "invalid_ids": [],
    "violations": []
  },
  "visual_bible": {
    "style_anchor": "中文描述",
    "color_palette": "中文描述",
    "lighting": "中文描述",
    "era_setting": "中文描述",
    "negative_prompt": "中文描述"
  },
  "object_registry": {
    "characters": [
      {
        "id": "snake_case_id",
        "immutable": {
			"age": "30",
			"gender": "中文描述",
			"body_type": "中文描述",
			"height_reference": "中文描述",
			"skin_tone": "中文描述",
			"face_shape": "中文描述",
			"hair_style": "中文描述",
			"clothing_top": "中文描述",
			"clothing_bottom": "中文描述",
			"shoes": "中文描述",
			"accessories": "中文描述",
			"distinctive_features": "中文描述",
			"voice": {
				"pitch": "tenor_high",
				"timbre_core": "bright",
				"speed_base": "fast",
				"resonance": "nasal",
				"emotional_base": "cunning", 
				"pitch_variance": "dramatic",   
				"texture": "silky",             
				"articulation": "sharp"
			}
		}
      }
    ],
    "props": [
      {
        "id": "snake_case_id",
        "immutable": {
			"type": "中文描述",
			"material": "中文描述",
			"color": "中文描述",
			"size_reference": "中文描述",
			"surface_detail": "中文描述",
			"wear_marks": "中文描述",
			"default_position": "中文描述"
		}
      }
    ],
    "environments": [
      {
        "id": "snake_case_id",
        "immutable": {
			"layout": "中文描述",
			"key_landmarks": "中文描述",
			"spatial_reference": "中文描述",
			"time_of_day": "中文描述",
			"weather": "中文描述",
			"ground_texture": "中文描述",
			"background_depth": "中文描述"
		}
      }
    ]
  },
  "scenes": [
    {
      "scene_id": 1,
      "duration_seconds": 4,
      "safety_pad": 0.5,
      "action_unit": {
        "unit_goal": "中文短语",
        "start_state": "snake_case_id 中文短语",
        "beats": [
          {
            "t_range": "0.0-2.0",
            "micro_action": "中文短语",
            "visible_change": "中文短语",
            "emotional_tone": "中文短语"
          },
          {
            "t_range": "2.0-4.0",
            "micro_action": "hold still",
            "visible_change": "中文短语",
            "emotional_tone": "中文短语"
          }
        ],
        "end_state": "snake_case_id 中文短语"
      },
      "audio_layer": {
        "dialogue": [
          {
            "speaker": "character_id_or_narrator",
            "t_range": "0.0-1.2",
            "content": "snake_case_id 说了什么"
          }
        ],
        "sound_effects": [
          {
            "t_range": "0.0-0.5",
            "description": "snake_case_id 发出了什么声音"
          }
        ],
        "ambience": "snake_case_id 发出什么响声"
      },
      "camera": [{
	    "t_range": "0.0-4.0",
        "shot_type": "wide",
        "angle": "eye_level",
        "movement": "static",
        "movement_speed": "slow",
        "focus": "environment_depth",
        "composition": "snake_case_id 在画面什么位置，和其他元素的空间关系，构图方式等中文描述"
      }],
      "cast": {
        "characters": ["snake_case_id"],
        "props": ["snake_case_id"],
        "environment": "snake_case_id"
      },
      "transition_to_next": "cut"
    }
  ]
}

========================
INPUTS
========================
- idiom_name: %s
- idiom_name_en: %s
- description: %s
- narration_language: %s
- target_duration_sec: %d

GENERATE the full JSON now.
`, input.IdiomNameEn, input.AspectRatio, input.TargetDurationSec, input.IdiomName, input.IdiomNameEn, strings.TrimSpace(input.Description), input.NarrationLanguage, input.TargetDurationSec)
}

func callDeepSeekPlan(cfg Config, plannerPrompt string) (string, error) {
	client := &http.Client{Timeout: time.Duration(cfg.HTTPTimeoutSec) * time.Second}

	body := map[string]interface{}{
		"model": cfg.Model,
		"messages": []map[string]string{
			{"role": "system", "content": "你是一个成语故事分镜师。你必须严格按照下面的JSON格式输出，不能有任何其他文字。每个字段必须存在，如果某个元素不存在就用空字符串。"},
			{"role": "user", "content": plannerPrompt},
		},
		"temperature": 0.4,
		"stream":      false,
	}

	logger.DebugJSON("deepseek request body", "body-data", body)
	if !cfg.Enable {
		return "", errors.New("deepseek disabled by config: DEEPSEEK_ENABLED=false")
	}
	raw, _ := json.Marshal(body)
	endpoint := strings.TrimRight(cfg.BaseURL, "/") + "/chat/completions"

	const maxAttempts = 1 // 目前先不重试，后续如果需要可以调整这个值
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

	if resp.StatusCode >= 300 {
		var errBody map[string]interface{}
		if err := json.NewDecoder(resp.Body).Decode(&errBody); err == nil {
			rawErrBody, _ := json.Marshal(errBody)
			return "", fmt.Errorf("deepseek failed status=%d body=%s", resp.StatusCode, string(rawErrBody))
		}
		return "", fmt.Errorf("deepseek failed status=%d", resp.StatusCode)
	}

	var parsed deepSeekChatResponse
	if err := json.NewDecoder(resp.Body).Decode(&parsed); err != nil && !errors.Is(err, io.EOF) {
		return "", err
	}
	if len(parsed.Choices) == 0 {
		return "", nil
	}
	return parsed.Choices[0].Message.Content, nil
}

func isRetryableDeepSeekError(err error) bool {
	if err == nil {
		return false
	}
	if err == io.ErrUnexpectedEOF {
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
		strings.Contains(msg, "temporary")
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
