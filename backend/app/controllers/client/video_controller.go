package client

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
	"time"

	"api/app/requests/client/video"
	"api/pkg/deepseek"
	"api/pkg/logger"
	"api/pkg/queue"
	"api/pkg/response"
	"api/pkg/wanxiang"

	"github.com/gin-gonic/gin"
)

type VideoController struct {
	BaseAPIController
}

// CreateIdiomStory 由 backend 同步调用 DeepSeek 规划，成功后再入队给 worker 执行生成流水线。
func (ctrl *VideoController) CreateIdiomStory(c *gin.Context) {
	var req video.CreateIdiomStoryRequest
	if !ctrl.BindJSON(c, &req) {
		return
	}

	projectID := buildIdiomProjectID(req.IdiomNameEn)
	targetDurationSec := req.TargetDurationSec
	if targetDurationSec == 0 {
		targetDurationSec = 30
	}

	planInput := deepseek.IdiomPlanInput{
		ProjectID:         projectID,
		IdiomName:         req.IdiomName,
		IdiomNameEn:       req.IdiomNameEn,
		Dynasty:           "泛古代",
		Platform:          defaultIfEmpty(req.Platform, "youtube"),
		Category:          "idiom_story", // worker分流标志
		NarrationLanguage: defaultIfEmpty(req.NarrationLanguage, "zh-CN"),
		TargetDurationSec: targetDurationSec,
		Audience:          "亲子",
		Tone:              defaultIfEmpty(req.Tone, "storytelling"),
		AspectRatio:       resolveAspectRatio(req.AspectRatio, req.Platform, "9:16"),
		Resolution:        defaultIfEmpty(req.Resolution, "720p"),
		VisualStyle:       "storybook",
		AnimationStyle:    "subtle",
		ExpressionStyle:   "mild_exaggerated",
		CameraShotSize:    "wide",
		CameraAngle:       "high",
		CameraMovement:    "slow_push",
	}
	plan, err := deepseek.BuildIdiomPlan(deepseek.LoadConfig(), planInput)
	if err != nil {
		c.AbortWithStatusJSON(502, gin.H{
			"message": "deepseek planning failed",
			"error":   err.Error(),
		})
		return
	}

	imageURLs, err := generateReferenceImages(plan)
	if err != nil {
		c.AbortWithStatusJSON(502, gin.H{
			"message": "wanxiang image generation failed",
			"error":   err.Error(),
		})
		return
	}

	requestPayload := map[string]interface{}{
		"project_id":          planInput.ProjectID,
		"platform":            planInput.Platform,
		"category":            planInput.Category,
		"narration_language":  planInput.NarrationLanguage,
		"target_duration_sec": planInput.TargetDurationSec,
		"tone":                planInput.Tone,
		"aspect_ratio":        planInput.AspectRatio,
		"resolution":          planInput.Resolution,
		"image_urls":          imageURLs,
	}

	taskID, err := queue.PublishVideoTask("plan.v1", map[string]interface{}{
		"request_payload": requestPayload,
		"plan":            plan,
	})
	if err != nil {
		response.Abort500(c, "enqueue idiom story plan task failed: "+err.Error())
		return
	}

	response.JSON(c, gin.H{
		"message":    "idiom story accepted",
		"project_id": projectID,
		"task_id":    taskID,
		"task_type":  "plan.v1",
	})
}

// SubmitPlan 前端可直接提交已经生成好的 plan，backend 直接入队给 worker。
// 支持两种入参：
// 1) 直接传 plan JSON；
// 2) 传 {"request_payload": {...}, "plan": {...}, "idiom_name_en":"..."}。
func (ctrl *VideoController) SubmitPlan(c *gin.Context) {
	var req struct {
		RequestPayload map[string]interface{} `json:"request_payload"`
		Plan           json.RawMessage        `json:"plan"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, err, "invalid plan payload")
		return
	}
	if len(req.RequestPayload) == 0 {
		response.BadRequest(c, fmt.Errorf("request_payload empty"), "request_payload is required")
		return
	}
	if len(req.Plan) == 0 {
		response.BadRequest(c, fmt.Errorf("plan empty"), "plan is required")
		return
	}

	requestPayload := req.RequestPayload
	projectID := strings.TrimSpace(anyString(requestPayload["project_id"]))
	if projectID == "" {
		response.BadRequest(c, fmt.Errorf("project_id missing"), "request_payload.project_id is required")
		return
	}
	var schema deepseek.ProjectPlanSchema
	if err := json.Unmarshal(req.Plan, &schema); err != nil {
		response.BadRequest(c, err, "plan parse failed")
		return
	}
	if len(schema.Scenes) == 0 {
		response.BadRequest(c, fmt.Errorf("scenes empty"), "plan.scenes is required")
		return
	}

	taskID, err := queue.PublishVideoTask("plan.v1", map[string]interface{}{
		"request_payload": requestPayload,
		"plan":            schema,
	})
	if err != nil {
		response.Abort500(c, "enqueue plan task failed: "+err.Error())
		return
	}

	response.JSON(c, gin.H{
		"message":    "plan accepted",
		"project_id": projectID,
		"task_id":    taskID,
		"task_type":  "plan.v1",
	})
}

func anyString(v interface{}) string {
	if s, ok := v.(string); ok {
		return s
	}
	return ""
}

func buildIdiomProjectID(idiomNameEn string) string {
	englishName := strings.TrimSpace(idiomNameEn)
	slug := slugForID(englishName)
	return fmt.Sprintf("pro_%d_%s", time.Now().UnixNano(), slug)
}

func defaultIfEmpty(value, fallback string) string {
	if value == "" {
		return fallback
	}
	return value
}

// 如果传了aspect_ratio就用传的；如果没传但指定了平台，就用平台默认的；否则用 fallback。
func resolveAspectRatio(value, platform, fallback string) string {
	if value != "" {
		return value
	}
	switch strings.ToLower(strings.TrimSpace(platform)) {
	case "tiktok":
		return "9:16"
	case "youtube":
		return fallback
	case "both":
		return "9:16"
	default:
		return fallback
	}
}

func slugForID(s string) string {
	s = strings.ToLower(strings.TrimSpace(s))
	if s == "" {
		return "video"
	}
	re := regexp.MustCompile(`[^a-z0-9]+`)
	s = re.ReplaceAllString(s, "-")
	s = strings.Trim(s, "-")
	if s == "" {
		return "video"
	}
	if len(s) > 40 {
		s = s[:40]
	}
	return s
}

func generateReferenceImages(plan deepseek.ProjectPlanSchema) ([]string, error) {
	cfg := wanxiang.LoadConfig()
	if !cfg.Enabled {
		return nil, nil
	}

	charPrompt := buildCharacterImagePrompt(plan)
	worldPrompt := buildWorldImagePrompt(plan)
	negative := strings.TrimSpace(plan.VisualBible.NegativePrompt)

	out := make([]string, 0, 2)
	if charPrompt != "" {
		res, err := wanxiang.Generate(cfg, wanxiang.GenerateRequest{
			Prompt:         charPrompt,
			NegativePrompt: negative,
		})
		if err != nil {
			return nil, err
		}
		if len(res.ImageURLs) > 0 {
			out = append(out, res.ImageURLs[0])
		}
	}

	if worldPrompt != "" {
		res, err := wanxiang.Generate(cfg, wanxiang.GenerateRequest{
			Prompt:         worldPrompt,
			NegativePrompt: negative,
		})
		if err != nil {
			return nil, err
		}
		if len(res.ImageURLs) > 0 {
			out = append(out, res.ImageURLs[0])
		}
	}

	logger.DebugJSON("wanxiang", "image_urls", out)
	return out, nil
}

func buildCharacterImagePrompt(plan deepseek.ProjectPlanSchema) string {
	if len(plan.Characters) == 0 {
		return ""
	}
	style := strings.TrimSpace(plan.VisualBible.StyleAnchor)
	characterAnchor := strings.TrimSpace(plan.VisualBible.CharacterAnchor)
	return fmt.Sprintf(
		"Character concept art for Chinese idiom short-video. Characters: %s. Style: %s. Character anchor: %s. Clean background, full body, clear costume and silhouette, high readability.",
		strings.Join(plan.Characters, ", "),
		style,
		characterAnchor,
	)
}

func buildWorldImagePrompt(plan deepseek.ProjectPlanSchema) string {
	if len(plan.SceneElements) == 0 && len(plan.Props) == 0 {
		return ""
	}
	style := strings.TrimSpace(plan.VisualBible.StyleAnchor)
	environmentAnchor := strings.TrimSpace(plan.VisualBible.EnvironmentAnchor)
	return fmt.Sprintf(
		"Environment and props concept art for Chinese idiom short-video. Scene elements: %s. Props: %s. Style: %s. Environment anchor: %s. Wide composition, no characters, clear prop details.",
		strings.Join(plan.SceneElements, ", "),
		strings.Join(plan.Props, ", "),
		style,
		environmentAnchor,
	)
}
