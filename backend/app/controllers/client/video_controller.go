package client

import (
	"encoding/json"
	"errors"
	"fmt"
	"hash/fnv"
	"net/http"
	"regexp"
	"sort"
	"strings"
	"time"

	"api/app/models/content"
	"api/app/requests/client/video"
	appconfig "api/pkg/config"
	"api/pkg/database"
	"api/pkg/deepseek"
	"api/pkg/logger"
	"api/pkg/queue"
	"api/pkg/response"
	"api/pkg/wanxiang"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
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

var podcastReplayProjectPattern = regexp.MustCompile(`^(.*)__rm\d+__\d{14}$`)

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
	trackIdiomProject(projectID, "plan.v1", requestPayload)

	taskID, err := queue.PublishVideoTask("plan.v1", map[string]interface{}{
		"request_payload": requestPayload,
		"plan":            plan,
	})
	if err != nil {
		markProjectRequestFailed(projectID, "plan.v1", err)
		if errors.Is(err, queue.ErrDisabled) {
			c.AbortWithStatusJSON(http.StatusServiceUnavailable, gin.H{
				"message": "rabbitmq is disabled on this environment",
			})
			return
		}
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
	trackIdiomProject(projectID, "plan.v1", requestPayload)

	taskID, err := queue.PublishVideoTask("plan.v1", map[string]interface{}{
		"request_payload": requestPayload,
		"plan":            req.Plan,
	})
	if err != nil {
		markProjectRequestFailed(projectID, "plan.v1", err)
		if errors.Is(err, queue.ErrDisabled) {
			c.AbortWithStatusJSON(http.StatusServiceUnavailable, gin.H{
				"message": "rabbitmq is disabled on this environment",
			})
			return
		}
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

func (ctrl *VideoController) CancelProject(c *gin.Context) {
	var req video.CancelProjectRequest
	if !ctrl.BindJSON(c, &req) {
		return
	}

	projectID := strings.TrimSpace(req.ProjectID)
	if projectID == "" {
		response.BadRequest(c, fmt.Errorf("project_id is required"), "project_id is required")
		return
	}
	if database.DB == nil {
		response.Abort500(c, "database is not initialized")
		return
	}

	project, err := content.FindProjectByProjectID(projectID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			c.AbortWithStatusJSON(http.StatusNotFound, gin.H{
				"message":    "project not found",
				"project_id": projectID,
			})
			return
		}
		response.Abort500(c, "query project failed: "+err.Error())
		return
	}

	switch project.Status {
	case content.ProjectStatusFinished, content.ProjectStatusError:
		c.AbortWithStatusJSON(http.StatusConflict, gin.H{
			"message":     "project is already terminal",
			"project_id":  projectID,
			"status":      project.Status,
			"status_name": content.ProjectStatusName(project.Status),
		})
		return
	case content.ProjectStatusCancelling:
		response.JSON(c, gin.H{
			"message":     "project cancellation already requested",
			"project_id":  projectID,
			"status":      project.Status,
			"status_name": content.ProjectStatusName(project.Status),
		})
		return
	case content.ProjectStatusCancelled:
		response.JSON(c, gin.H{
			"message":     "project already cancelled",
			"project_id":  projectID,
			"status":      project.Status,
			"status_name": content.ProjectStatusName(project.Status),
		})
		return
	}

	now := time.Now().UTC()
	if err := content.UpdateProjectByProjectID(projectID, map[string]interface{}{
		"status":               content.ProjectStatusCancelling,
		"terminated_task_type": project.CurrentTaskType,
		"cancel_requested_at":  &now,
		"cancel_source":        content.ProjectCancelSourceManualAPI,
		"updated_at":           now,
	}); err != nil {
		response.Abort500(c, "cancel project failed: "+err.Error())
		return
	}

	response.JSON(c, gin.H{
		"message":     "project cancellation requested",
		"project_id":  projectID,
		"status":      content.ProjectStatusCancelling,
		"status_name": content.ProjectStatusName(content.ProjectStatusCancelling),
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
	blockNums := mergePodcastBlockNums(req.BlockNums, req.BlockNum)
	podcastSeed := 0
	if runMode == 0 {
		podcastSeed = buildPodcastSeed(projectID)
	}
	payload := buildPodcastTaskPayload(req, projectID, runMode, blockNums, bgImgFilenames, podcastSeed)

	taskType := podcastTaskTypeForRunMode(runMode)

	trackPodcastProject(projectID, runMode, taskType, payload)

	taskID, err := queue.PublishVideoTask(taskType, payload)
	if err != nil {
		markPodcastProjectRequestFailed(projectID, taskType, err)
		if errors.Is(err, queue.ErrDisabled) {
			c.AbortWithStatusJSON(http.StatusServiceUnavailable, gin.H{
				"message": "rabbitmq is disabled on this environment",
			})
			return
		}
		response.Abort500(c, "enqueue podcast audio task failed: "+err.Error())
		return
	}

	response.JSON(c, gin.H{
		"message":    "podcast dialogue accepted",
		"project_id": projectID,
		"seed":       podcastSeed,
		"task_id":    taskID,
		"task_type":  taskType,
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

func buildPodcastProjectID(lang string) string {
	prefix := normalizePodcastLang(lang) + "_podcast"
	return fmt.Sprintf("%s_%s", prefix, time.Now().Format("20060102150405"))
}

func buildPodcastSeed(projectID string) int {
	h := fnv.New32a()
	_, _ = h.Write([]byte(strings.TrimSpace(projectID)))
	seed := int(h.Sum32() & 0x7fffffff)
	if seed > 0 {
		return seed
	}
	return 1
}

func normalizePodcastRunMode(value int) int {
	switch value {
	case 1, 2, 3, 4:
		return value
	default:
		return 0
	}
}

func normalizeOnlyCurrentStep(value int) int {
	if value == 1 {
		return 1
	}
	return 0
}

func podcastTaskTypeForRunMode(runMode int) string {
	switch runMode {
	case 2:
		return "podcast.compose.render.v1"
	case 3:
		return "podcast.page.persist.v1"
	case 4:
		return "podcast.audio.align.v1"
	default:
		return "podcast.audio.generate.v1"
	}
}

func resolvePodcastProjectID(req video.CreatePodcastDialogueRequest, runMode int) (string, error) {
	if runMode == 1 || runMode == 2 || runMode == 3 || runMode == 4 {
		sourceProjectID := strings.TrimSpace(req.ProjectID)
		if sourceProjectID == "" {
			return "", fmt.Errorf("project_id is required when run_mode is 1, 2, 3 or 4")
		}
		return buildPodcastReplayProjectID(sourceProjectID, runMode), nil
	}

	lang := normalizePodcastLang(req.Lang)
	return buildPodcastProjectID(lang), nil
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
	case 3:
		if strings.TrimSpace(req.ProjectID) == "" {
			return fmt.Errorf("project_id is required when run_mode is 3")
		}
		return nil
	case 4:
		if strings.TrimSpace(req.ProjectID) == "" {
			return fmt.Errorf("project_id is required when run_mode is 4")
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

func buildPodcastTaskPayload(
	req video.CreatePodcastDialogueRequest,
	projectID string,
	runMode int,
	blockNums []int,
	bgImgFilenames []string,
	podcastSeed int,
) map[string]interface{} {
	payload := map[string]interface{}{
		"content_type":      "podcast",
		"project_id":        projectID,
		"run_mode":          runMode,
		"only_current_step": normalizeOnlyCurrentStep(req.OnlyCurrentStep),
		"title":             strings.TrimSpace(req.Title),
		"lang":              strings.TrimSpace(req.Lang),
		"content_profile":   strings.TrimSpace(req.ContentProfile),
		"script_filename":   strings.TrimSpace(req.ScriptFilename),
		"target_platform":   strings.TrimSpace(req.TargetPlatform),
		"aspect_ratio":      strings.TrimSpace(req.AspectRatio),
		"resolution":        strings.TrimSpace(req.Resolution),
		"design_style":      req.DesignStyle,
	}
	if runMode == 1 || runMode == 2 || runMode == 3 || runMode == 4 {
		if sourceProjectID := strings.TrimSpace(req.ProjectID); sourceProjectID != "" {
			payload["source_project_id"] = sourceProjectID
		}
	}
	if podcastSeed > 0 {
		payload["seed"] = podcastSeed
	}
	if req.TTSType == 1 || req.TTSType == 2 {
		payload["tts_type"] = req.TTSType
	}
	if len(blockNums) > 0 {
		payload["block_nums"] = blockNums
	}
	if len(bgImgFilenames) > 0 {
		payload["bg_img_filenames"] = bgImgFilenames
	}
	return payload
}

func buildPodcastReplayProjectID(sourceProjectID string, runMode int) string {
	return fmt.Sprintf("%s__rm%d__%s", normalizePodcastReplayRootProjectID(sourceProjectID), runMode, time.Now().Format("20060102150405"))
}

func normalizePodcastReplayRootProjectID(projectID string) string {
	projectID = strings.TrimSpace(projectID)
	if projectID == "" {
		return ""
	}
	matches := podcastReplayProjectPattern.FindStringSubmatch(projectID)
	if len(matches) == 2 && strings.TrimSpace(matches[1]) != "" {
		return strings.TrimSpace(matches[1])
	}
	return projectID
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

func mergePodcastBlockNums(groups ...[]int) []int {
	seen := make(map[int]struct{})
	out := make([]int, 0)
	for _, group := range groups {
		for _, value := range group {
			if value <= 0 {
				continue
			}
			if _, exists := seen[value]; exists {
				continue
			}
			seen[value] = struct{}{}
			out = append(out, value)
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
