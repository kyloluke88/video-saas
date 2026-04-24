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

	bgImgFilenames := compactStringSlice(req.BgImgFilenames)
	blockNums := compactPositiveInts(req.BlockNums)
	if runMode == 0 {
		if strings.TrimSpace(req.Lang) == "" {
			response.BadRequest(c, fmt.Errorf("lang is required when run_mode is 0"), "lang is required when run_mode is 0")
			return
		}
		if strings.TrimSpace(req.ContentProfile) == "" {
			response.BadRequest(c, fmt.Errorf("content_profile is required when run_mode is 0"), "content_profile is required when run_mode is 0")
			return
		}
		if strings.TrimSpace(req.ScriptFilename) == "" {
			response.BadRequest(c, fmt.Errorf("script_filename is required when run_mode is 0"), "script_filename is required when run_mode is 0")
			return
		}
		if len(bgImgFilenames) == 0 {
			response.BadRequest(c, fmt.Errorf("bg_img_filenames is required when run_mode is 0"), "bg_img_filenames is required when run_mode is 0")
			return
		}
	}
	podcastSeed := 0
	if runMode == 0 {
		podcastSeed = buildPodcastSeed(projectID)
	} else if req.Seed > 0 {
		podcastSeed = req.Seed
	}

	requestPayload := buildPodcastRequestPayload(req, projectID, runMode, blockNums, bgImgFilenames, podcastSeed)
	trackedPayload := buildTrackedPodcastPayload(runMode, requestPayload)

	ttsType := normalizePodcastTTSType(payloadInt(trackedPayload, "tts_type", req.TTSType))
	requestSpecifyTasks := compactStringSlice(req.SpecifyTasks)
	if runMode == 1 {
		specifyTasks, err := normalizePodcastSpecifyTasks(ttsType, requestSpecifyTasks)
		if err != nil {
			response.BadRequest(c, err, err.Error())
			return
		}
		if len(specifyTasks) == 0 {
			response.BadRequest(c, fmt.Errorf("specify_tasks is required when run_mode is 1"), "specify_tasks is required when run_mode is 1")
			return
		}
		if podcastSpecifiesStage(specifyTasks, podcastStageGenerate) && len(blockNums) == 0 {
			response.BadRequest(c, fmt.Errorf("block_nums is required when specify_tasks includes generate"), "block_nums is required when specify_tasks includes generate")
			return
		}
		trackedPayload["specify_tasks"] = specifyTasks
		trackedPayload["source_project_id"] = strings.TrimSpace(req.ProjectID)
	} else {
		delete(trackedPayload, "specify_tasks")
		delete(trackedPayload, "source_project_id")
	}
	trackedPayload["tts_type"] = ttsType
	trackedPayload["run_mode"] = runMode
	trackedPayload["project_id"] = projectID

	taskType, err := podcastTaskTypeForInitialStage(ttsType, runMode, payloadStringSlice(trackedPayload, "specify_tasks"))
	if err != nil {
		response.BadRequest(c, err, err.Error())
		return
	}
	payload := buildPodcastTaskPayload(trackedPayload)

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

func (ctrl *VideoController) CreatePracticalDialogue(c *gin.Context) {
	var req video.CreatePracticalDialogueRequest
	if !ctrl.BindJSON(c, &req) {
		return
	}

	runMode := normalizePracticalRunMode(req.RunMode)
	projectID, err := resolvePracticalProjectID(req, runMode)
	if err != nil {
		response.BadRequest(c, err, err.Error())
		return
	}

	bgImgFilenames := compactStringSlice(req.BgImgFilenames)
	blockBgImgFilenames := compactStringSlice(req.BlockBgImgFilenames)
	blockNums := compactPositiveInts(req.BlockNums)
	if runMode == 0 {
		if strings.TrimSpace(req.Lang) == "" {
			response.BadRequest(c, fmt.Errorf("lang is required when run_mode is 0"), "lang is required when run_mode is 0")
			return
		}
		if strings.TrimSpace(req.ScriptFilename) == "" {
			response.BadRequest(c, fmt.Errorf("script_filename is required when run_mode is 0"), "script_filename is required when run_mode is 0")
			return
		}
		if len(bgImgFilenames) == 0 {
			response.BadRequest(c, fmt.Errorf("bg_img_filenames is required when run_mode is 0"), "bg_img_filenames is required when run_mode is 0")
			return
		}
	}

	requestPayload := buildPracticalRequestPayload(req, projectID, runMode, blockNums, bgImgFilenames, blockBgImgFilenames)
	trackedPayload := buildTrackedPracticalPayload(runMode, requestPayload)

	requestSpecifyTasks := compactStringSlice(req.SpecifyTasks)
	if runMode == 1 {
		specifyTasks, err := normalizePracticalSpecifyTasks(requestSpecifyTasks)
		if err != nil {
			response.BadRequest(c, err, err.Error())
			return
		}
		if len(specifyTasks) == 0 {
			response.BadRequest(c, fmt.Errorf("specify_tasks is required when run_mode is 1"), "specify_tasks is required when run_mode is 1")
			return
		}
		if practicalSpecifiesStage(specifyTasks, practicalStageGenerate) && len(blockNums) == 0 {
			response.BadRequest(c, fmt.Errorf("block_nums is required when specify_tasks includes generate"), "block_nums is required when specify_tasks includes generate")
			return
		}
		trackedPayload["specify_tasks"] = specifyTasks
		trackedPayload["source_project_id"] = strings.TrimSpace(req.ProjectID)
	} else {
		delete(trackedPayload, "specify_tasks")
		delete(trackedPayload, "source_project_id")
	}
	trackedPayload["run_mode"] = runMode
	trackedPayload["project_id"] = projectID

	taskType, err := practicalTaskTypeForInitialStage(runMode, payloadStringSlice(trackedPayload, "specify_tasks"))
	if err != nil {
		response.BadRequest(c, err, err.Error())
		return
	}
	payload := buildPracticalTaskPayload(trackedPayload)

	trackPracticalProject(projectID, runMode, taskType, trackedPayload)

	taskID, err := queue.PublishVideoTask(taskType, payload)
	if err != nil {
		markPracticalProjectRequestFailed(projectID, taskType, err)
		if errors.Is(err, queue.ErrDisabled) {
			c.AbortWithStatusJSON(http.StatusServiceUnavailable, gin.H{
				"message": "rabbitmq is disabled on this environment",
			})
			return
		}
		response.Abort500(c, "enqueue practical audio task failed: "+err.Error())
		return
	}

	response.JSON(c, gin.H{
		"message":    "practical dialogue accepted",
		"project_id": projectID,
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

func buildPracticalProjectID(lang string) string {
	prefix := normalizePodcastLang(lang) + "_practical"
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

func resolvePodcastProjectID(req video.CreatePodcastDialogueRequest, runMode int) (string, error) {
	if runMode == 1 {
		sourceProjectID := strings.TrimSpace(req.ProjectID)
		if sourceProjectID == "" {
			return "", fmt.Errorf("project_id is required when run_mode is 1")
		}
		return buildPodcastReplayProjectID(sourceProjectID), nil
	}

	lang := normalizePodcastLang(req.Lang)
	return buildPodcastProjectID(lang), nil
}

func resolvePracticalProjectID(req video.CreatePracticalDialogueRequest, runMode int) (string, error) {
	if runMode == 1 {
		sourceProjectID := strings.TrimSpace(req.ProjectID)
		if sourceProjectID == "" {
			return "", fmt.Errorf("project_id is required when run_mode is 1")
		}
		return buildPracticalReplayProjectID(sourceProjectID), nil
	}

	lang := normalizePodcastLang(req.Lang)
	return buildPracticalProjectID(lang), nil
}

func buildPodcastRequestPayload(
	req video.CreatePodcastDialogueRequest,
	projectID string,
	runMode int,
	blockNums []int,
	bgImgFilenames []string,
	podcastSeed int,
) map[string]interface{} {
	payload := map[string]interface{}{
		"content_type": "podcast",
		"project_id":   projectID,
		"run_mode":     runMode,
	}
	if title := strings.TrimSpace(req.Title); title != "" {
		payload["title"] = title
	}
	if lang := strings.TrimSpace(req.Lang); lang != "" {
		payload["lang"] = lang
	}
	if profile := strings.TrimSpace(req.ContentProfile); profile != "" {
		payload["content_profile"] = profile
	}
	if scriptFile := strings.TrimSpace(req.ScriptFilename); scriptFile != "" {
		payload["script_filename"] = scriptFile
	}
	if platform := strings.TrimSpace(req.TargetPlatform); platform != "" {
		payload["target_platform"] = platform
	}
	if aspect := strings.TrimSpace(req.AspectRatio); aspect != "" {
		payload["aspect_ratio"] = aspect
	}
	if resolution := strings.TrimSpace(req.Resolution); resolution != "" {
		payload["resolution"] = resolution
	}
	if req.DesignStyle > 0 {
		payload["design_style"] = req.DesignStyle
	}
	if runMode == 1 {
		if sourceProjectID := strings.TrimSpace(req.ProjectID); sourceProjectID != "" {
			payload["source_project_id"] = sourceProjectID
		}
		if tasks := compactStringSlice(req.SpecifyTasks); len(tasks) > 0 {
			payload["specify_tasks"] = tasks
		}
	}
	if podcastSeed > 0 {
		payload["seed"] = podcastSeed
	}
	if req.TTSType == 1 || req.TTSType == 2 {
		payload["tts_type"] = req.TTSType
	}
	if req.IsMultiple != nil {
		payload["is_multiple"] = *req.IsMultiple
	}
	if len(blockNums) > 0 {
		payload["block_nums"] = blockNums
	}
	if len(bgImgFilenames) > 0 {
		payload["bg_img_filenames"] = bgImgFilenames
	}
	return payload
}

func buildPracticalRequestPayload(
	req video.CreatePracticalDialogueRequest,
	projectID string,
	runMode int,
	blockNums []int,
	bgImgFilenames []string,
	blockBgImgFilenames []string,
) map[string]interface{} {
	payload := map[string]interface{}{
		"content_type": "practical",
		"project_id":   projectID,
		"run_mode":     runMode,
	}
	if runMode == 1 {
		if sourceProjectID := strings.TrimSpace(req.ProjectID); sourceProjectID != "" {
			payload["source_project_id"] = sourceProjectID
		}
		if tasks := compactStringSlice(req.SpecifyTasks); len(tasks) > 0 {
			payload["specify_tasks"] = tasks
		}
	}
	if lang := strings.TrimSpace(req.Lang); lang != "" {
		payload["lang"] = lang
	}
	if scriptFile := strings.TrimSpace(req.ScriptFilename); scriptFile != "" {
		payload["script_filename"] = scriptFile
	}
	if len(bgImgFilenames) > 0 {
		payload["bg_img_filenames"] = bgImgFilenames
	}
	if len(blockBgImgFilenames) > 0 {
		payload["block_bg_img_filenames"] = blockBgImgFilenames
	}
	if len(blockNums) > 0 {
		payload["block_nums"] = blockNums
	}
	if resolution := strings.TrimSpace(req.Resolution); resolution != "" {
		payload["resolution"] = resolution
	} else if runMode == 0 {
		payload["resolution"] = "1080p"
	}
	if req.DesignType > 0 {
		payload["design_type"] = normalizePracticalDesignType(req.DesignType)
	} else if runMode == 0 {
		payload["design_type"] = 1
	}
	return payload
}

func buildPracticalTaskPayload(payload map[string]interface{}) map[string]interface{} {
	out := map[string]interface{}{
		"content_type": "practical",
		"project_id":   strings.TrimSpace(payloadString(payload, "project_id")),
		"run_mode":     normalizePracticalRunMode(payloadInt(payload, "run_mode", 0)),
		"tts_type":     1,
	}
	if sourceProjectID := strings.TrimSpace(payloadString(payload, "source_project_id")); sourceProjectID != "" && payloadInt(payload, "run_mode", 0) == 1 {
		out["source_project_id"] = sourceProjectID
	}
	if tasks := compactStringSlice(payloadStringSlice(payload, "specify_tasks")); len(tasks) > 0 && payloadInt(payload, "run_mode", 0) == 1 {
		out["specify_tasks"] = tasks
	}
	if lang := strings.TrimSpace(payloadString(payload, "lang")); lang != "" {
		out["lang"] = lang
	}
	if scriptFile := strings.TrimSpace(payloadString(payload, "script_filename")); scriptFile != "" {
		out["script_filename"] = scriptFile
	}
	if backgrounds := compactStringSlice(payloadStringSlice(payload, "bg_img_filenames")); len(backgrounds) > 0 {
		out["bg_img_filenames"] = backgrounds
	}
	if blockBackgrounds := compactStringSlice(payloadStringSlice(payload, "block_bg_img_filenames")); len(blockBackgrounds) > 0 {
		out["block_bg_img_filenames"] = blockBackgrounds
	}
	if blockNums := compactPositiveInts(payloadIntSlice(payload, "block_nums")); len(blockNums) > 0 {
		out["block_nums"] = blockNums
	}
	if resolution := strings.TrimSpace(payloadString(payload, "resolution")); resolution != "" {
		out["resolution"] = resolution
	}
	if designType := payloadInt(payload, "design_type", 0); designType > 0 {
		out["design_type"] = normalizePracticalDesignType(designType)
	}
	return out
}

func buildPodcastTaskPayload(payload map[string]interface{}) map[string]interface{} {
	out := map[string]interface{}{
		"content_type": "podcast",
		"project_id":   strings.TrimSpace(payloadString(payload, "project_id")),
		"run_mode":     normalizePodcastRunMode(payloadInt(payload, "run_mode", 0)),
	}
	if sourceProjectID := strings.TrimSpace(payloadString(payload, "source_project_id")); sourceProjectID != "" && payloadInt(payload, "run_mode", 0) == 1 {
		out["source_project_id"] = sourceProjectID
	}
	ttsType := normalizePodcastTTSType(payloadInt(payload, "tts_type", 1))
	out["tts_type"] = ttsType
	if tasks := compactStringSlice(payloadStringSlice(payload, "specify_tasks")); len(tasks) > 0 && payloadInt(payload, "run_mode", 0) == 1 {
		out["specify_tasks"] = tasks
	}
	if title := strings.TrimSpace(payloadString(payload, "title")); title != "" {
		out["title"] = title
	}
	if lang := strings.TrimSpace(payloadString(payload, "lang")); lang != "" {
		out["lang"] = lang
	}
	if profile := strings.TrimSpace(payloadString(payload, "content_profile")); profile != "" {
		out["content_profile"] = profile
	}
	if scriptFile := strings.TrimSpace(payloadString(payload, "script_filename")); scriptFile != "" {
		out["script_filename"] = scriptFile
	}
	if platform := strings.TrimSpace(payloadString(payload, "target_platform")); platform != "" {
		out["target_platform"] = platform
	}
	if aspect := strings.TrimSpace(payloadString(payload, "aspect_ratio")); aspect != "" {
		out["aspect_ratio"] = aspect
	}
	if resolution := strings.TrimSpace(payloadString(payload, "resolution")); resolution != "" {
		out["resolution"] = resolution
	}
	if designStyle := payloadInt(payload, "design_style", 0); designStyle > 0 {
		out["design_style"] = designStyle
	}
	if seed := payloadInt(payload, "seed", 0); seed > 0 {
		out["seed"] = seed
	}
	if isMultiple, ok := payloadIntWithPresence(payload, "is_multiple"); ok {
		out["is_multiple"] = isMultiple
	} else if ttsType == 1 {
		out["is_multiple"] = 1
	}
	if blockNums := compactPositiveInts(payloadIntSlice(payload, "block_nums")); len(blockNums) > 0 {
		out["block_nums"] = blockNums
	}
	if backgrounds := compactStringSlice(payloadStringSlice(payload, "bg_img_filenames")); len(backgrounds) > 0 {
		out["bg_img_filenames"] = backgrounds
	}
	return out
}

func buildPodcastReplayProjectID(sourceProjectID string) string {
	return fmt.Sprintf("%s__rm1__%s", normalizePodcastReplayRootProjectID(sourceProjectID), time.Now().Format("20060102150405"))
}

func buildPracticalReplayProjectID(sourceProjectID string) string {
	return fmt.Sprintf("%s__rm1__%s", normalizePodcastReplayRootProjectID(sourceProjectID), time.Now().Format("20060102150405"))
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

func compactPositiveInts(values []int) []int {
	seen := make(map[int]struct{}, len(values))
	out := make([]int, 0, len(values))
	for _, value := range values {
		if value <= 0 {
			continue
		}
		if _, exists := seen[value]; exists {
			continue
		}
		seen[value] = struct{}{}
		out = append(out, value)
	}
	return out
}

func payloadString(payload map[string]interface{}, key string) string {
	if payload == nil {
		return ""
	}
	value, ok := payload[key]
	if !ok || value == nil {
		return ""
	}
	if text, ok := value.(string); ok {
		return strings.TrimSpace(text)
	}
	text := strings.TrimSpace(fmt.Sprint(value))
	if text == "<nil>" {
		return ""
	}
	return text
}

func payloadInt(payload map[string]interface{}, key string, fallback int) int {
	if value, ok := payloadIntWithPresence(payload, key); ok {
		return value
	}
	return fallback
}

func payloadIntWithPresence(payload map[string]interface{}, key string) (int, bool) {
	if payload == nil {
		return 0, false
	}
	value, ok := payload[key]
	if !ok || value == nil {
		return 0, false
	}
	switch typed := value.(type) {
	case int:
		return typed, true
	case int8:
		return int(typed), true
	case int16:
		return int(typed), true
	case int32:
		return int(typed), true
	case int64:
		return int(typed), true
	case float32:
		return int(typed), true
	case float64:
		return int(typed), true
	case json.Number:
		if num, err := typed.Int64(); err == nil {
			return int(num), true
		}
		if num, err := typed.Float64(); err == nil {
			return int(num), true
		}
		return 0, false
	default:
		text := strings.TrimSpace(fmt.Sprint(typed))
		if text == "" || text == "<nil>" {
			return 0, false
		}
		var parsed int
		if _, err := fmt.Sscanf(text, "%d", &parsed); err == nil {
			return parsed, true
		}
		return 0, false
	}
}

func payloadStringSlice(payload map[string]interface{}, key string) []string {
	if payload == nil {
		return nil
	}
	value, ok := payload[key]
	if !ok || value == nil {
		return nil
	}
	clean := func(raw string) string {
		text := strings.TrimSpace(raw)
		if text == "" || text == "<nil>" {
			return ""
		}
		return text
	}
	switch typed := value.(type) {
	case []string:
		out := make([]string, 0, len(typed))
		for _, item := range typed {
			if text := clean(item); text != "" {
				out = append(out, text)
			}
		}
		return out
	case []interface{}:
		out := make([]string, 0, len(typed))
		for _, item := range typed {
			if text := clean(fmt.Sprint(item)); text != "" {
				out = append(out, text)
			}
		}
		return out
	default:
		if text := clean(fmt.Sprint(typed)); text != "" {
			return []string{text}
		}
		return nil
	}
}

func payloadIntSlice(payload map[string]interface{}, key string) []int {
	if payload == nil {
		return nil
	}
	value, ok := payload[key]
	if !ok || value == nil {
		return nil
	}
	out := make([]int, 0)
	switch typed := value.(type) {
	case []int:
		out = append(out, typed...)
	case []int64:
		for _, item := range typed {
			out = append(out, int(item))
		}
	case []float64:
		for _, item := range typed {
			out = append(out, int(item))
		}
	case []interface{}:
		for _, item := range typed {
			if num, ok := payloadIntFromValue(item); ok {
				out = append(out, num)
			}
		}
	default:
		if num, ok := payloadIntFromValue(typed); ok {
			out = append(out, num)
		}
	}
	return compactPositiveInts(out)
}

func payloadIntFromValue(value interface{}) (int, bool) {
	switch typed := value.(type) {
	case int:
		return typed, true
	case int8:
		return int(typed), true
	case int16:
		return int(typed), true
	case int32:
		return int(typed), true
	case int64:
		return int(typed), true
	case float32:
		return int(typed), true
	case float64:
		return int(typed), true
	case json.Number:
		if num, err := typed.Int64(); err == nil {
			return int(num), true
		}
		if num, err := typed.Float64(); err == nil {
			return int(num), true
		}
		return 0, false
	default:
		text := strings.TrimSpace(fmt.Sprint(typed))
		if text == "" || text == "<nil>" {
			return 0, false
		}
		var parsed int
		if _, err := fmt.Sscanf(text, "%d", &parsed); err == nil {
			return parsed, true
		}
		return 0, false
	}
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
