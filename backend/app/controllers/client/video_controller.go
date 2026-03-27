package client

import (
	"encoding/json"
	"fmt"
	"regexp"
	"sort"
	"strings"
	"time"

	"api/app/requests/client/video"
	appconfig "api/pkg/config"
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

type referencePromptTask struct {
	Label    string
	Kind     string
	ObjectID string
	Prompt   string
}

type referenceImage struct {
	Label    string `json:"label"`
	Kind     string `json:"kind"`
	ObjectID string `json:"object_id,omitempty"`
	TaskID   string `json:"task_id,omitempty"`
	ImageURL string `json:"image_url,omitempty"`
}

// CreateIdiomStory 由 backend 同步调用 DeepSeek 规划，成功后再入队给 worker 执行生成流水线。
func (ctrl *VideoController) CreateIdiomStory(c *gin.Context) {
	var req video.CreateIdiomStoryRequest
	if !ctrl.BindJSON(c, &req) {
		return
	}

	projectID := buildProjectID(req.IdiomNameEn)
	targetDurationSec := req.TargetDurationSec
	if targetDurationSec == 0 {
		targetDurationSec = 30
	}

	planInput := deepseek.IdiomPlanInput{
		ProjectID:         projectID,
		IdiomName:         req.IdiomName,
		IdiomNameEn:       req.IdiomNameEn,
		Description:       req.Description,
		Category:          "idiom_story", // worker分流标志
		NarrationLanguage: defaultIfEmpty(req.NarrationLanguage, "zh-CN"),
		TargetDurationSec: targetDurationSec,
		AspectRatio:       defaultIfEmpty(req.AspectRatio, "16:9"),
		Resolution:        defaultIfEmpty(req.Resolution, defaultPodcastResolution()),
	}
	plan, err := deepseek.BuildIdiomPlan(deepseek.LoadConfig(), planInput)

	if err != nil {
		c.AbortWithStatusJSON(502, gin.H{
			"message": "deepseek planning failed",
			"error":   err.Error(),
		})
		return
	}
	requestPayload := map[string]interface{}{
		"project_id":          planInput.ProjectID,
		"idiom_name":          planInput.IdiomName,
		"idiom_name_en":       planInput.IdiomNameEn,
		"category":            planInput.Category,
		"narration_language":  planInput.NarrationLanguage,
		"target_duration_sec": planInput.TargetDurationSec,
		"aspect_ratio":        planInput.AspectRatio,
		"resolution":          planInput.Resolution,
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

	taskID, err := queue.PublishVideoTask("plan.v1", map[string]interface{}{
		"request_payload": requestPayload,
		"plan":            req.Plan,
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

func (ctrl *VideoController) CreatePodcastDialogue(c *gin.Context) {
	var req video.CreatePodcastDialogueRequest
	if !ctrl.BindJSON(c, &req) {
		return
	}

	runMode := normalizePodcastRunMode(req.RunMode)
	projectID, err := resolvePodcastProjectID(req, runMode)
	if err != nil {
		response.BadRequest(c, err, err.Error())
		return
	}
	if err := validatePodcastCreateRequest(req, runMode); err != nil {
		response.BadRequest(c, err, err.Error())
		return
	}

	bgImgFilenames := compactStringSlice(req.BgImgFilenames)
	payload := map[string]interface{}{
		"project_id":      projectID,
		"lang":            strings.TrimSpace(req.Lang),
		"content_profile": strings.TrimSpace(req.ContentProfile),
		"run_mode":        runMode,
		"title":           strings.TrimSpace(req.Title),
		"script_filename": strings.TrimSpace(req.ScriptFilename),
		"target_platform": defaultIfEmpty(strings.TrimSpace(req.TargetPlatform), "youtube"),
		"aspect_ratio":    defaultIfEmpty(strings.TrimSpace(req.AspectRatio), "16:9"),
		"resolution":      defaultIfEmpty(strings.TrimSpace(req.Resolution), defaultPodcastResolution()),
		"design_style":    defaultInt(req.DesignStyle, 1),
	}
	if len(bgImgFilenames) > 0 {
		payload["bg_img_filenames"] = bgImgFilenames
	}

	taskID, err := queue.PublishVideoTask("podcast.audio.generate.v1", payload)
	if err != nil {
		response.Abort500(c, "enqueue podcast audio task failed: "+err.Error())
		return
	}

	response.JSON(c, gin.H{
		"message":    "podcast dialogue accepted",
		"project_id": projectID,
		"task_id":    taskID,
		"task_type":  "podcast.audio.generate.v1",
	})
}

func anyString(v interface{}) string {
	if s, ok := v.(string); ok {
		return s
	}
	return ""
}

func buildProjectID(seed string) string {
	englishName := strings.TrimSpace(seed)
	slug := slugForID(englishName)
	return fmt.Sprintf("pro_%d_%s", time.Now().UnixNano(), slug)
}

func buildPodcastProjectID(lang, seed string) string {
	prefix := normalizePodcastLang(lang) + "_podcast"
	slug := slugForID(strings.TrimSpace(seed))
	return fmt.Sprintf("%s_%s_%s", prefix, time.Now().Format("20060102150405"), slug)
}

func normalizePodcastRunMode(value int) int {
	switch value {
	case 1, 2:
		return value
	default:
		return 0
	}
}

func resolvePodcastProjectID(req video.CreatePodcastDialogueRequest, runMode int) (string, error) {
	if runMode == 1 || runMode == 2 {
		projectID := strings.TrimSpace(req.ProjectID)
		if projectID == "" {
			return "", fmt.Errorf("project_id is required when run_mode is 1 or 2")
		}
		return projectID, nil
	}

	lang := normalizePodcastLang(req.Lang)
	projectSeed := req.Title
	if strings.TrimSpace(projectSeed) == "" {
		projectSeed = req.ScriptFilename
	}
	return buildPodcastProjectID(lang, projectSeed), nil
}

func validatePodcastCreateRequest(req video.CreatePodcastDialogueRequest, runMode int) error {
	switch runMode {
	case 1:
		if strings.TrimSpace(req.ProjectID) == "" {
			return fmt.Errorf("project_id is required when run_mode is 1")
		}
		return nil
	case 2:
		if strings.TrimSpace(req.ProjectID) == "" {
			return fmt.Errorf("project_id is required when run_mode is 2")
		}
		return nil
	default:
		if strings.TrimSpace(req.Lang) == "" {
			return fmt.Errorf("lang is required when run_mode is 0")
		}
		if strings.TrimSpace(req.ContentProfile) == "" {
			return fmt.Errorf("content_profile is required when run_mode is 0")
		}
		if strings.TrimSpace(req.ScriptFilename) == "" {
			return fmt.Errorf("script_filename is required when run_mode is 0")
		}
		if !hasPodcastBackgroundInput(req.BgImgFilenames) {
			return fmt.Errorf("bg_img_filenames is required when run_mode is 0")
		}
		return nil
	}
}

func compactStringSlice(values []string) []string {
	out := make([]string, 0, len(values))
	for _, value := range values {
		if trimmed := strings.TrimSpace(value); trimmed != "" {
			out = append(out, trimmed)
		}
	}
	return out
}

func hasPodcastBackgroundInput(many []string) bool {
	return len(compactStringSlice(many)) > 0
}

func normalizePodcastLang(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "ja":
		return "ja"
	default:
		return "zh"
	}
}

func defaultIfEmpty(value, fallback string) string {
	if value == "" {
		return fallback
	}
	return value
}

func defaultInt(value, fallback int) int {
	if value == 0 {
		return fallback
	}
	return value
}

func defaultPodcastResolution() string {
	mode := strings.ToLower(strings.TrimSpace(fmt.Sprint(appconfig.Env("PODCAST_MODE", "debug"))))
	if mode == "production" {
		return "1080p"
	}
	return "480p"
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

func generateReferenceImages(plan deepseek.ProjectPlanSchema) ([]string, []referenceImage, error) {
	cfg := wanxiang.LoadConfig()
	if !cfg.Enabled {
		return nil, nil, nil
	}
	// One image per prompt task to keep prompt->image mapping stable.
	cfg.NumImages = 1

	tasks := buildCharacterImagePrompt(plan)
	tasks = append(tasks, buildWorldImagePrompt(plan)...)
	negative := strings.TrimSpace(plan.VisualBible.NegativePrompt)
	out := make([]string, 0, len(tasks))
	refs := make([]referenceImage, 0, len(tasks))
	for _, task := range tasks {
		if strings.TrimSpace(task.Prompt) == "" {
			continue
		}
		logger.InfoString("wanxiang", task.Label+"_prompt", task.Prompt)
		res, err := wanxiang.Generate(cfg, wanxiang.GenerateRequest{
			Prompt:         task.Prompt,
			NegativePrompt: negative,
		})
		if err != nil {
			return nil, nil, err
		}
		url := ""
		if len(res.ImageURLs) > 0 && strings.TrimSpace(res.ImageURLs[0]) != "" {
			url = strings.TrimSpace(res.ImageURLs[0])
			out = append(out, url)
		}
		refs = append(refs, referenceImage{
			Label:    task.Label,
			Kind:     task.Kind,
			ObjectID: task.ObjectID,
			TaskID:   strings.TrimSpace(res.TaskID),
			ImageURL: url,
		})
		logger.DebugJSON("wanxiang", "image_urls", out)
	}
	logger.DebugJSON("wanxiang", "reference_images", refs)
	return out, refs, nil
}

func buildCharacterImagePrompt(plan deepseek.ProjectPlanSchema) []referencePromptTask {
	vb := visualBiblePrompt(plan)

	out := make([]referencePromptTask, 0, len(plan.ObjectRegistry.Characters))
	for _, character := range plan.ObjectRegistry.Characters {
		id := strings.TrimSpace(character.ID)
		if id == "" {
			continue
		}
		detail := immutableSummary(character.Immutable)
		prompt := fmt.Sprintf(
			"Character concept art for Chinese idiom short-video. Character ID: %s. Immutable traits: %s. %s. Single character only, full body, clear silhouette, neutral clean background, no text, no watermark.",
			id,
			detail,
			vb,
		)
		out = append(out, referencePromptTask{
			Label:    "character:" + id,
			Kind:     "character",
			ObjectID: id,
			Prompt:   prompt,
		})
	}
	return out
}

func buildWorldImagePrompt(plan deepseek.ProjectPlanSchema) []referencePromptTask {
	vb := visualBiblePrompt(plan)

	out := make([]referencePromptTask, 0, 2)

	// propsSummary := objectRegistrySummary(plan.ObjectRegistry.Props)
	// if propsSummary != "" {
	// 	out = append(out, referencePromptTask{
	// 		Label:  "props:all",
	// 		Kind:   "props",
	// 		Prompt: fmt.Sprintf("Props concept art for Chinese idiom short-video. Props: %s. %s. No characters, neutral display background, clear material and shape details, no text, no watermark.", propsSummary, vb),
	// 	})
	// }

	envSummary := objectRegistrySummary(plan.ObjectRegistry.Environments)
	if envSummary != "" {
		out = append(out, referencePromptTask{
			Label:  "environment:all",
			Kind:   "environment",
			Prompt: fmt.Sprintf("Environment concept art for Chinese idiom short-video. Environments: %s. %s. No characters, wide composition, clear layout and depth, no text, no watermark.", envSummary, vb),
		})
	}

	return out
}

func plannerObjectIDs(items []deepseek.PlanObjectSpec) []string {
	out := make([]string, 0, len(items))
	for _, item := range items {
		id := strings.TrimSpace(item.ID)
		if id != "" {
			out = append(out, id)
		}
	}
	return out
}

func immutableSummary(immutable map[string]interface{}) string {
	if len(immutable) == 0 {
		return ""
	}
	keys := make([]string, 0, len(immutable))
	for k := range immutable {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	parts := make([]string, 0, len(keys))
	for _, key := range keys {
		value := strings.TrimSpace(anyToString(immutable[key]))
		if value == "" {
			continue
		}
		parts = append(parts, fmt.Sprintf("%s=%s", key, value))
	}
	return strings.Join(parts, "; ")
}

func objectRegistrySummary(items []deepseek.PlanObjectSpec) string {
	if len(items) == 0 {
		return ""
	}
	parts := make([]string, 0, len(items))
	for _, item := range items {
		id := strings.TrimSpace(item.ID)
		detail := immutableSummary(item.Immutable)
		switch {
		case id != "" && detail != "":
			parts = append(parts, fmt.Sprintf("%s(%s)", id, detail))
		case id != "":
			parts = append(parts, id)
		case detail != "":
			parts = append(parts, detail)
		}
	}
	return strings.Join(parts, " | ")
}

func anyToString(v interface{}) string {
	switch val := v.(type) {
	case string:
		return val
	case []interface{}:
		parts := make([]string, 0, len(val))
		for _, item := range val {
			s := strings.TrimSpace(anyToString(item))
			if s != "" {
				parts = append(parts, s)
			}
		}
		return strings.Join(parts, ", ")
	default:
		return fmt.Sprintf("%v", v)
	}
}

func visualBiblePrompt(plan deepseek.ProjectPlanSchema) string {
	style := strings.TrimSpace(plan.VisualBible.StyleAnchor)
	color := strings.TrimSpace(plan.VisualBible.ColorPalette)
	lighting := strings.TrimSpace(plan.VisualBible.Lighting)
	era := strings.TrimSpace(plan.VisualBible.EraSetting)
	negative := strings.TrimSpace(plan.VisualBible.NegativePrompt)
	return fmt.Sprintf(
		"Visual bible: style_anchor=%s; color_palette=%s; lighting=%s; era_setting=%s; negative_prompt=%s",
		style,
		color,
		lighting,
		era,
		negative,
	)
}
