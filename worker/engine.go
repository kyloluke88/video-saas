package main

import (
	"encoding/json"
	"errors"
	"fmt"
	amqp "github.com/rabbitmq/amqp091-go"
	"log"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

	"worker/pkg/helpers"
	storageS3 "worker/pkg/storage/s3"
	"worker/service"
)

func handleMessage(ch *amqp.Channel, cfg Config, msg amqp.Delivery, scheduler map[string]taskHandler) error {
	retries := helpers.HeaderRetry(msg.Headers)

	var task VideoTaskMessage
	if err := json.Unmarshal(msg.Body, &task); err != nil {
		_ = publishToDLQ(ch, cfg, msg.Body, retries+1)
		return msg.Ack(false)
	}

	log.Printf("🎬 收到任务 task_id=%s type=%s retries=%d", task.TaskID, task.TaskType, retries)
	if err := processTask(ch, cfg, task, scheduler); err != nil {
		log.Printf("❌ 任务处理失败 task_id=%s: %v", task.TaskID, err)
		if isNonRetryable(err) {
			log.Printf("⛔ 程序终止：检测到不可重试错误，任务不再重试 task_id=%s", task.TaskID)
			if dlqErr := publishToDLQ(ch, cfg, msg.Body, retries); dlqErr != nil {
				_ = msg.Nack(false, true)
				return dlqErr
			}
			return msg.Ack(false)
		}
		if retries >= cfg.MaxRetries {
			if dlqErr := publishToDLQ(ch, cfg, msg.Body, retries+1); dlqErr != nil {
				_ = msg.Nack(false, true)
				return dlqErr
			}
			return msg.Ack(false)
		}
		if retryErr := publishToRetry(ch, cfg, msg.Body, retries+1); retryErr != nil {
			_ = msg.Nack(false, true)
			return retryErr
		}
		return msg.Ack(false)
	}

	return msg.Ack(false)
}

func processTask(ch *amqp.Channel, cfg Config, task VideoTaskMessage, scheduler map[string]taskHandler) error {
	handler, ok := scheduler[task.TaskType]
	if !ok {
		return fmt.Errorf("unsupported task type: %s", task.TaskType)
	}
	return handler(ch, cfg, task)
}

func handleUploadTask(_ *amqp.Channel, cfg Config, task VideoTaskMessage) error {
	return handleProjectUpload(cfg, task)
}

func handlePlan(ch *amqp.Channel, cfg Config, task VideoTaskMessage) error {
	requestPayload, plan, err := parsePlanTaskPayload(task.Payload)
	if err != nil {
		return err
	}

	payload := parseProjectPlanPayload(requestPayload)
	if payload.ProjectID == "" {
		return errors.New("project_id is required")
	}

	if err := ensureProjectsDir(cfg); err != nil {
		return err
	}
	projectDir := projectDirFor(cfg, payload.ProjectID)
	if err := os.MkdirAll(filepath.Join(projectDir, "scenes"), 0o755); err != nil {
		return err
	}
	traceDir := filepath.Join(projectDir, "traces")
	if err := os.MkdirAll(traceDir, 0o755); err != nil {
		return err
	}

	if err := writeJSON(filepath.Join(projectDir, "request_payload.json"), requestPayload); err != nil {
		return err
	}
	if err := writeJSON(filepath.Join(projectDir, "plan.json"), plan); err != nil {
		return err
	}

	for _, scene := range plan.Scenes {
		scenePayload := map[string]interface{}{
			"project_id":          payload.ProjectID,
			"idiom_name_en":       payload.IdiomNameEn,
			"characters":          plan.Characters,
			"props":               plan.Props,
			"scene_elements":      plan.SceneElements,
			"scene_index":         scene.Index,
			"scene_goal":          scene.Goal,
			"objects_ref":         scene.ObjectsRef,
			"composition":         scene.Composition,
			"action":              scene.Action,
			"prompt":              scene.Prompt,
			"duration_sec":        scene.DurationSec,
			"target_duration_sec": scene.DurationSec,
			"image_urls":          payload.ImageURLs,
			"aspect_ratio":        plan.AspectRatio,
			"resolution":          plan.Resolution,
			"narration":           scene.Narration,
			"project_category":    plan.Category,
			"project_platform":    plan.Platform,
			"project_tone":        payload.Tone,
			"visual_bible":        plan.VisualBible,
			"object_registry":     plan.ObjectRegistry,
		}
		if err := publishTask(ch, cfg, "scene.generate.v1", scenePayload); err != nil {
			return err
		}
	}
	log.Printf("🧠 规划任务接收完成 project_id=%s scenes=%d", payload.ProjectID, len(plan.Scenes))
	return nil
}

func handleSceneGenerate(ch *amqp.Channel, cfg Config, task VideoTaskMessage) error {
	projectID := helpers.GetString(task.Payload, "project_id")
	sceneIndex := helpers.GetInt(task.Payload, "scene_index", 0)
	if projectID == "" || sceneIndex <= 0 {
		return errors.New("project_id or scene_index invalid")
	}

	projectDir := projectDirFor(cfg, projectID)
	traceDir := filepath.Join(projectDir, "traces")
	if err := os.MkdirAll(traceDir, 0o755); err != nil {
		return err
	}
	tracePrefix := fmt.Sprintf("scene_%02d", sceneIndex)
	scenePrompt := buildSeedanceScenePrompt(task.Payload)
	if scenePrompt == "" {
		scenePrompt = helpers.GetString(task.Payload, "prompt")
	}
	seedanceRequestPayload := map[string]interface{}{
		"prompt":         scenePrompt,
		"aspect_ratio":   helpers.GetString(task.Payload, "aspect_ratio"),
		"resolution":     helpers.GetString(task.Payload, "resolution"),
		"duration":       helpers.NormalizeDuration(helpers.GetInt(task.Payload, "duration_sec", 8)),
		"generate_audio": true,
		"image_urls":     task.Payload["image_urls"],
	}
	if err := writeJSON(filepath.Join(traceDir, fmt.Sprintf("seedance_compiled_prompt_%s.json", tracePrefix)), map[string]interface{}{
		"project_id":        projectID,
		"scene_index":       sceneIndex,
		"compiled_prompt":   scenePrompt,
		"base_scene_prompt": helpers.GetString(task.Payload, "prompt"),
		"scene_goal":        helpers.GetString(task.Payload, "scene_goal"),
		"objects_ref":       helpers.GetStringSlice(task.Payload, "objects_ref"),
		"composition":       task.Payload["composition"],
		"action":            helpers.GetStringSlice(task.Payload, "action"),
		"visual_bible":      task.Payload["visual_bible"],
		"object_registry":   task.Payload["object_registry"],
		"seedance_request":  seedanceRequestPayload,
	}); err != nil {
		return err
	}
	if cfg.SeedanceDryRunEnable {
		log.Printf("⏭️ Seedance request skipped by dry-run flag project_id=%s scene=%d", projectID, sceneIndex)
		return nil
	}

	videoURL, err := service.RunSeedanceGenerate(toServiceConfig(cfg), seedanceRequestPayload, traceDir, tracePrefix)
	if err != nil {
		return err
	}

	scenesDir := filepath.Join(projectDir, "scenes")
	if err := os.MkdirAll(scenesDir, 0o755); err != nil {
		return err
	}
	rawPath := filepath.Join(scenesDir, fmt.Sprintf("%02d_raw.mp4", sceneIndex))
	if err := helpers.DownloadToFile(videoURL, rawPath, cfg.SeedanceHTTPTimeoutSec); err != nil {
		return err
	}

	normalizedPath := filepath.Join(scenesDir, fmt.Sprintf("%02d_norm.mp4", sceneIndex))
	if cfg.FFmpegPostprocessEnabled {
		if err := service.NormalizeSceneVideo(toServiceConfig(cfg), rawPath, normalizedPath); err != nil {
			return err
		}
	} else {
		normalizedPath = rawPath
	}

	targetDurationSec := helpers.GetInt(task.Payload, "target_duration_sec", 0)
	sceneFinalPath := filepath.Join(scenesDir, fmt.Sprintf("%02d.mp4", sceneIndex))
	if err := service.TrimVideoDuration(toServiceConfig(cfg), normalizedPath, sceneFinalPath, targetDurationSec); err != nil {
		return err
	}

	_ = os.WriteFile(filepath.Join(scenesDir, fmt.Sprintf("%02d.done", sceneIndex)), []byte(time.Now().Format(time.RFC3339)), 0o644)
	log.Printf("🎞️ 场景生成完成 project_id=%s scene=%d file=%s", projectID, sceneIndex, sceneFinalPath)

	return tryTriggerCompose(ch, cfg, projectID, helpers.GetString(task.Payload, "idiom_name_en"))
}

func handleProjectCompose(ch *amqp.Channel, cfg Config, task VideoTaskMessage) error {
	projectID := helpers.GetString(task.Payload, "project_id")
	if projectID == "" {
		return errors.New("project_id missing")
	}
	projectDir := projectDirFor(cfg, projectID)
	planPath := filepath.Join(projectDir, "plan.json")
	var plan ProjectPlanResult
	if err := readJSON(planPath, &plan); err != nil {
		return err
	}

	scenesDir := filepath.Join(projectDir, "scenes")
	sceneFiles := make([]string, 0, len(plan.Scenes))
	for _, scene := range plan.Scenes {
		scenePath := filepath.Join(scenesDir, fmt.Sprintf("%02d.mp4", scene.Index))
		if _, err := os.Stat(scenePath); err != nil {
			return fmt.Errorf("scene file missing: %s", scenePath)
		}
		sceneFiles = append(sceneFiles, scenePath)
	}
	sort.Strings(sceneFiles)

	concatInput := filepath.Join(projectDir, "concat.txt")
	var b strings.Builder
	for _, f := range sceneFiles {
		b.WriteString("file '")
		b.WriteString(strings.ReplaceAll(f, "'", "'\\''"))
		b.WriteString("'\n")
	}
	if err := os.WriteFile(concatInput, []byte(b.String()), 0o644); err != nil {
		return err
	}

	stitched := filepath.Join(projectDir, "stitched.mp4")
	if err := service.RunFFmpeg(toServiceConfig(cfg), "-y", "-f", "concat", "-safe", "0", "-i", concatInput, "-c", "copy", stitched); err != nil {
		return err
	}

	withSubtitles := filepath.Join(projectDir, "with_subtitles.mp4")
	if err := service.BurnSubtitles(toServiceConfig(cfg), stitched, toServiceProjectPlan(plan), withSubtitles); err != nil {
		return err
	}

	finalPath := filepath.Join(projectDir, "final.mp4")
	narrationAudio, narrationErr := service.SynthesizeNarrationAudio(toServiceConfig(cfg), toServiceProjectPlan(plan), projectDir)
	if narrationErr != nil {
		log.Printf("⚠️ narration synth failed project_id=%s err=%v", projectID, narrationErr)
		narrationAudio = ""
	}
	bgmPath := ""
	if cfg.BGMEnable {
		var err error
		bgmPath, err = service.SelectRandomBGM(toServiceConfig(cfg))
		if err != nil {
			return err
		}
		if bgmPath != "" {
			_ = os.WriteFile(filepath.Join(projectDir, "bgm_selected.txt"), []byte(bgmPath+"\n"), 0o644)
			log.Printf("🎵 BGM selected project_id=%s bgm=%s", projectID, bgmPath)
		}
	} else {
		log.Printf("⏭️ BGM mix skipped by BGM_ENABLE=false project_id=%s", projectID)
	}
	if err := service.ComposeFinalVideo(toServiceConfig(cfg), toServiceProjectPlan(plan), withSubtitles, narrationAudio, bgmPath, finalPath); err != nil {
		return err
	}

	log.Printf("🎬 项目合成完成 project_id=%s final=%s", projectID, finalPath)
	return publishTask(ch, cfg, "upload.v1", map[string]interface{}{
		"project_id":    projectID,
		"idiom_name_en": helpers.GetString(task.Payload, "idiom_name_en"),
		"file_path":     finalPath,
	})
}

func handleProjectUpload(cfg Config, task VideoTaskMessage) error {
	projectID := helpers.GetString(task.Payload, "project_id")
	filePath := helpers.GetString(task.Payload, "file_path")
	if projectID == "" || filePath == "" {
		return errors.New("project.upload payload invalid")
	}

	if !cfg.S3Enabled || cfg.S3Bucket == "" {
		log.Printf("📦 S3 未启用，保留本地文件 project_id=%s path=%s", projectID, filePath)
		return nil
	}

	objectKey := fmt.Sprintf("projects/%s/final.mp4", projectID)
	publicURL, err := storageS3.UploadFile(storageS3.Config{
		Endpoint:  cfg.S3Endpoint,
		Region:    cfg.S3Region,
		Bucket:    cfg.S3Bucket,
		AccessKey: cfg.S3AccessKey,
		SecretKey: cfg.S3SecretKey,
		PublicURL: cfg.S3PublicURL,
	}, filePath, objectKey)
	if err != nil {
		return err
	}
	log.Printf("☁️ 上传S3完成 project_id=%s url=%s", projectID, publicURL)
	return nil
}

func tryTriggerCompose(ch *amqp.Channel, cfg Config, projectID, idiomNameEn string) error {
	projectDir := projectDirFor(cfg, projectID)
	var plan ProjectPlanResult
	if err := readJSON(filepath.Join(projectDir, "plan.json"), &plan); err != nil {
		return err
	}

	for _, scene := range plan.Scenes {
		if _, err := os.Stat(filepath.Join(projectDir, "scenes", fmt.Sprintf("%02d.done", scene.Index))); err != nil {
			return nil
		}
	}

	marker := filepath.Join(projectDir, "compose.queued")
	f, err := os.OpenFile(marker, os.O_CREATE|os.O_EXCL, 0o644)
	if err != nil {
		return nil
	}
	_ = f.Close()

	return publishTask(ch, cfg, "compose.v1", map[string]interface{}{
		"project_id":    projectID,
		"idiom_name_en": idiomNameEn,
	})
}

func publishTask(ch *amqp.Channel, cfg Config, taskType string, payload map[string]interface{}) error {
	taskID := fmt.Sprintf("task-%d", time.Now().UnixNano())
	if suffix := taskIDSuffixFromPayload(payload); suffix != "" {
		taskID = fmt.Sprintf("%s-%s", taskID, suffix)
	}
	body, err := json.Marshal(VideoTaskMessage{
		TaskID:    taskID,
		TaskType:  taskType,
		Payload:   payload,
		CreatedAt: time.Now().UTC().Format(time.RFC3339),
	})
	if err != nil {
		return err
	}
	return ch.Publish(cfg.Exchange, cfg.RoutingKey, false, false, amqp.Publishing{
		ContentType:  "application/json",
		DeliveryMode: amqp.Persistent,
		Timestamp:    time.Now().UTC(),
		Body:         body,
	})
}

func publishToRetry(ch *amqp.Channel, cfg Config, body []byte, retries int) error {
	return ch.Publish(cfg.Exchange, cfg.RetryRoutingKey, false, false, amqp.Publishing{
		ContentType:  "application/json",
		DeliveryMode: amqp.Persistent,
		Timestamp:    time.Now().UTC(),
		Body:         body,
		Headers: amqp.Table{
			"x-retry-count": int32(retries),
		},
	})
}

func publishToDLQ(ch *amqp.Channel, cfg Config, body []byte, retries int) error {
	return ch.Publish(cfg.DLX, cfg.DLQRoutingKey, false, false, amqp.Publishing{
		ContentType:  "application/json",
		DeliveryMode: amqp.Persistent,
		Timestamp:    time.Now().UTC(),
		Body:         body,
		Headers: amqp.Table{
			"x-retry-count": int32(retries),
		},
	})
}

func parseProjectPlanPayload(payload map[string]interface{}) ProjectPlanPayload {
	return ProjectPlanPayload{
		ProjectID:         helpers.GetString(payload, "project_id"),
		IdiomName:         helpers.GetString(payload, "idiom_name"),
		IdiomNameEn:       helpers.GetString(payload, "idiom_name_en"),
		Dynasty:           helpers.DefaultString(helpers.GetString(payload, "dynasty"), "泛古代"),
		Platform:          helpers.DefaultString(helpers.GetString(payload, "platform"), "both"),
		Category:          helpers.GetString(payload, "category"),
		NarrationLanguage: helpers.DefaultString(helpers.GetString(payload, "narration_language"), "zh-CN"),
		TargetDurationSec: helpers.GetInt(payload, "target_duration_sec", 90),
		ImageURLs:         helpers.GetStringSlice(payload, "image_urls"),
		Characters:        helpers.GetStringSlice(payload, "characters"),
		Props:             helpers.GetStringSlice(payload, "props"),
		SceneElements:     helpers.GetStringSlice(payload, "scene_elements"),
		Audience:          helpers.GetString(payload, "audience"),
		Tone:              helpers.DefaultString(helpers.GetString(payload, "tone"), "cinematic"),
		AspectRatio:       helpers.DefaultString(helpers.GetString(payload, "aspect_ratio"), "16:9"),
		Resolution:        helpers.DefaultString(helpers.GetString(payload, "resolution"), "720p"),
	}
}

func parsePlanTaskPayload(payload map[string]interface{}) (map[string]interface{}, ProjectPlanResult, error) {
	var requestPayload map[string]interface{}
	if rawRequest, ok := payload["request_payload"]; ok {
		reqMap, ok := rawRequest.(map[string]interface{})
		if !ok {
			return nil, ProjectPlanResult{}, errors.New("plan task request_payload invalid")
		}
		requestPayload = reqMap
	}

	rawPlan, ok := payload["plan"]
	if !ok {
		return nil, ProjectPlanResult{}, errors.New("plan task missing plan")
	}
	planBytes, err := json.Marshal(rawPlan)
	if err != nil {
		return nil, ProjectPlanResult{}, fmt.Errorf("plan task marshal failed: %w", err)
	}
	var plan ProjectPlanResult
	if err := json.Unmarshal(planBytes, &plan); err != nil {
		return nil, ProjectPlanResult{}, fmt.Errorf("plan task parse failed: %w", err)
	}
	if strings.TrimSpace(plan.ProjectID) == "" {
		var wrapped struct {
			Payload struct {
				ProjectID         string   `json:"project_id"`
				Platform          string   `json:"platform"`
				Category          string   `json:"category"`
				NarrationLanguage string   `json:"narration_language"`
				TargetDurationSec int      `json:"target_duration_sec"`
				AspectRatio       string   `json:"aspect_ratio"`
				Resolution        string   `json:"resolution"`
				ImageURLs         []string `json:"image_urls"`
			} `json:"payload"`
			Schema struct {
				Narration      string       `json:"narration"`
				Characters     []string     `json:"characters"`
				Props          []string     `json:"props"`
				SceneElements  []string     `json:"scene_elements"`
				VisualBible    VisualBible  `json:"visual_bible"`
				ObjectRegistry []ObjectSpec `json:"object_registry"`
				Scenes         []ScenePlan  `json:"scenes"`
			} `json:"schema"`
		}
		if err := json.Unmarshal(planBytes, &wrapped); err == nil && strings.TrimSpace(wrapped.Payload.ProjectID) != "" {
			plan = ProjectPlanResult{
				ProjectID:         wrapped.Payload.ProjectID,
				Platform:          wrapped.Payload.Platform,
				Category:          wrapped.Payload.Category,
				NarrationLanguage: wrapped.Payload.NarrationLanguage,
				TargetDurationSec: wrapped.Payload.TargetDurationSec,
				AspectRatio:       wrapped.Payload.AspectRatio,
				Resolution:        wrapped.Payload.Resolution,
				ImageURLs:         wrapped.Payload.ImageURLs,
				Characters:        wrapped.Schema.Characters,
				Props:             wrapped.Schema.Props,
				SceneElements:     wrapped.Schema.SceneElements,
				NarrationFull:     wrapped.Schema.Narration,
				VisualBible:       wrapped.Schema.VisualBible,
				ObjectRegistry:    wrapped.Schema.ObjectRegistry,
				Scenes:            wrapped.Schema.Scenes,
			}
		}
	}
	if strings.TrimSpace(plan.ProjectID) == "" && requestPayload != nil {
		plan.ProjectID = helpers.GetString(requestPayload, "project_id")
	}
	if strings.TrimSpace(plan.NarrationFull) == "" {
		if rawPlanMap, ok := rawPlan.(map[string]interface{}); ok {
			plan.NarrationFull = helpers.GetString(rawPlanMap, "narration")
		}
	}
	if strings.TrimSpace(plan.ProjectID) == "" {
		return nil, ProjectPlanResult{}, errors.New("plan task missing project_id")
	}
	if requestPayload == nil {
		requestPayload = map[string]interface{}{
			"project_id":          plan.ProjectID,
			"platform":            helpers.DefaultString(plan.Platform, "both"),
			"category":            plan.Category,
			"narration_language":  helpers.DefaultString(plan.NarrationLanguage, "zh-CN"),
			"target_duration_sec": plan.TargetDurationSec,
			"image_urls":          plan.ImageURLs,
			"characters":          plan.Characters,
			"props":               plan.Props,
			"scene_elements":      plan.SceneElements,
			"aspect_ratio":        helpers.DefaultString(plan.AspectRatio, "16:9"),
			"resolution":          helpers.DefaultString(plan.Resolution, "720p"),
		}
	}
	return requestPayload, plan, nil
}

func writeJSON(path string, data interface{}) error {
	raw, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, raw, 0o644)
}

func readJSON(path string, out interface{}) error {
	raw, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	return json.Unmarshal(raw, out)
}

func isNonRetryable(err error) bool {
	var permanent nonRetryableError
	if errors.As(err, &permanent) {
		return true
	}
	var svcPermanent service.NonRetryableError
	return errors.As(err, &svcPermanent)
}

func isTTSEnabled(cfg Config) bool {
	switch strings.ToLower(strings.TrimSpace(cfg.TTSProvider)) {
	case "tencent":
		return cfg.TTSTencentSecretID != "" && cfg.TTSTencentSecretKey != ""
	case "", "http":
		return cfg.TTSAPIURL != "" && cfg.TTSAPIKey != ""
	default:
		return false
	}
}

func toServiceConfig(cfg Config) service.Config {
	return service.Config(cfg)
}

func toServiceProjectPlan(plan ProjectPlanResult) service.ProjectPlanResult {
	s := make([]service.ScenePlan, 0, len(plan.Scenes))
	for _, scene := range plan.Scenes {
		s = append(s, service.ScenePlan(scene))
	}
	registry := make([]service.ObjectSpec, 0, len(plan.ObjectRegistry))
	for _, item := range plan.ObjectRegistry {
		registry = append(registry, service.ObjectSpec{
			ID:        item.ID,
			Type:      item.Type,
			Label:     item.Label,
			Immutable: item.Immutable,
			Mutable:   item.Mutable,
		})
	}
	return service.ProjectPlanResult{
		ProjectID:         plan.ProjectID,
		Platform:          plan.Platform,
		Category:          plan.Category,
		NarrationLanguage: plan.NarrationLanguage,
		TargetDurationSec: plan.TargetDurationSec,
		AspectRatio:       plan.AspectRatio,
		Resolution:        plan.Resolution,
		ImageURLs:         plan.ImageURLs,
		Characters:        plan.Characters,
		Props:             plan.Props,
		SceneElements:     plan.SceneElements,
		NarrationFull:     plan.NarrationFull,
		VisualBible:       service.VisualBible(plan.VisualBible),
		ObjectRegistry:    registry,
		Scenes:            s,
		CreatedAt:         plan.CreatedAt,
	}
}

func buildSeedanceScenePrompt(payload map[string]interface{}) string {
	base := strings.TrimSpace(helpers.GetString(payload, "prompt"))
	if base == "" {
		return ""
	}

	styleAnchor, characterAnchor, environmentAnchor, _ := extractVisualAnchors(payload["visual_bible"])
	sceneGoal := strings.TrimSpace(helpers.GetString(payload, "scene_goal"))
	frontCharacters := helpers.GetStringSlice(payload, "characters")
	objectRefs := helpers.GetStringSlice(payload, "objects_ref")
	characterDesign := buildCharacterDesignSection(payload["object_registry"], frontCharacters)
	assetsRequired := buildAssetsRequiredSection(payload["object_registry"], objectRefs)
	composition := buildCompositionSection(payload["composition"])
	action := buildActionSection(payload)
	if action == "" {
		action = base
	}

	parts := make([]string, 0, 10)
	parts = append(parts, fixedStyleSection(styleAnchor))
	if characterAnchor != "" && characterDesign == "" {
		characterDesign = characterAnchor
	} else if characterAnchor != "" {
		characterDesign = characterAnchor + "\n" + characterDesign
	}
	parts = append(parts, "WORLD SETTING (fixed):\n"+helpers.DefaultString(environmentAnchor, ""))
	parts = append(parts, "CHARACTER DESIGN (immutable):\n"+helpers.DefaultString(characterDesign, ""))
	parts = append(parts, "ASSETS REQUIRED IN FRAME:\n"+helpers.DefaultString(assetsRequired, ""))
	parts = append(parts, "COMPOSITION (must follow):\n"+helpers.DefaultString(composition, ""))
	if sceneGoal != "" {
		action = action + "\nScene goal: " + sceneGoal
	}
	parts = append(parts, "ACTION:\n"+action)
	parts = append(parts, fixedContinuitySection())
	parts = append(parts, fixedNegativeSection())
	return strings.Join(parts, "\n")
}

func extractVisualAnchors(v interface{}) (styleAnchor, characterAnchor, environmentAnchor, negativePrompt string) {
	m, ok := v.(map[string]interface{})
	if !ok {
		return "", "", "", ""
	}
	return strings.TrimSpace(helpers.GetString(m, "style_anchor")),
		strings.TrimSpace(helpers.GetString(m, "character_anchor")),
		strings.TrimSpace(helpers.GetString(m, "environment_anchor")),
		strings.TrimSpace(helpers.GetString(m, "negative_prompt"))
}

func buildObjectCards(registryRaw interface{}, refs []string) []string {
	registry, ok := registryRaw.([]interface{})
	if !ok || len(registry) == 0 {
		return nil
	}
	refSet := make(map[string]struct{}, len(refs))
	for _, ref := range refs {
		if ref = strings.TrimSpace(ref); ref != "" {
			refSet[ref] = struct{}{}
		}
	}

	cards := make([]string, 0, len(registry))
	for _, raw := range registry {
		obj, ok := raw.(map[string]interface{})
		if !ok {
			continue
		}
		id := strings.TrimSpace(helpers.GetString(obj, "id"))
		if id == "" {
			continue
		}
		if len(refSet) > 0 {
			if _, found := refSet[id]; !found {
				continue
			}
		}
		typ := strings.TrimSpace(helpers.GetString(obj, "type"))
		immutable := formatKVMap(obj["immutable"])
		mutable := formatKVMap(obj["mutable"])
		card := fmt.Sprintf("- %s (%s): immutable={%s}", id, helpers.DefaultString(typ, "object"), immutable)
		if mutable != "" {
			card += fmt.Sprintf("; mutable={%s}", mutable)
		}
		cards = append(cards, card)
	}
	return cards
}

func buildAssetsRequiredSection(registryRaw interface{}, refs []string) string {
	registry, ok := registryRaw.([]interface{})
	if !ok || len(registry) == 0 {
		return ""
	}
	refSet := make(map[string]struct{}, len(refs))
	for _, ref := range refs {
		if ref = strings.TrimSpace(ref); ref != "" {
			refSet[ref] = struct{}{}
		}
	}

	assets := make([]string, 0, len(registry))
	for _, raw := range registry {
		obj, ok := raw.(map[string]interface{})
		if !ok {
			continue
		}
		id := strings.TrimSpace(helpers.GetString(obj, "id"))
		if id == "" {
			continue
		}
		if len(refSet) > 0 {
			if _, found := refSet[id]; !found {
				continue
			}
		}
		assets = append(assets, id)
	}
	return strings.Join(assets, ", ")
}

func buildCompositionSection(raw interface{}) string {
	comp, ok := raw.(map[string]interface{})
	if !ok || len(comp) == 0 {
		return ""
	}

	lines := make([]string, 0, 6)
	shotType := strings.TrimSpace(helpers.GetString(comp, "shot_type"))
	if shotType != "" {
		lines = append(lines, shotType)
	}
	blocking := helpers.GetStringSlice(comp, "subject_blocking")
	for _, b := range blocking {
		b = strings.TrimSpace(b)
		if b == "" {
			continue
		}
		lines = append(lines, b)
	}
	return strings.Join(lines, "\n")
}

func buildActionSection(payload map[string]interface{}) string {
	lines := helpers.GetStringSlice(payload, "action")
	if len(lines) == 0 {
		return ""
	}
	out := make([]string, 0, len(lines))
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		out = append(out, line)
	}
	return strings.Join(out, "\n")
}

func fixedStyleSection(styleAnchor string) string {
	if strings.TrimSpace(styleAnchor) != "" {
		return "STYLE (fixed across all scenes):\n" + strings.TrimSpace(styleAnchor)
	}
	return "STYLE (fixed across all scenes):\n" +
		"Chinese ink-wash hand-drawn animation, watercolor texture, soft natural golden-hour lighting, poetic negative space, rural agrarian ancient China (generic era), calm cinematic pacing, no exaggerated animation."
}

func fixedContinuitySection() string {
	return "CONTINUITY RULES:\n" +
		"Keep outfit, object appearance, and relative object positions consistent.\n" +
		"Maintain same lighting tone and art style.\n" +
		"No camera shake, no fast cuts."
}

func fixedNegativeSection() string {
	return "NEGATIVE CONSTRAINTS:\n" +
		"modern objects, text overlays, subtitles, logos, fantasy effects, dynasty-specific symbols, exaggerated expressions, blood, violence, extra characters."
}

func formatKVMap(v interface{}) string {
	m, ok := v.(map[string]interface{})
	if !ok || len(m) == 0 {
		return ""
	}
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	parts := make([]string, 0, len(keys))
	for _, k := range keys {
		val := strings.TrimSpace(fmt.Sprint(m[k]))
		if val == "" {
			continue
		}
		parts = append(parts, fmt.Sprintf("%s:%s", k, val))
	}
	return strings.Join(parts, ", ")
}

func buildCharacterDesignSection(registryRaw interface{}, frontCharacters []string) string {
	if len(frontCharacters) == 0 {
		return ""
	}
	registry, ok := registryRaw.([]interface{})
	if !ok || len(registry) == 0 {
		return ""
	}

	lines := make([]string, 0, len(frontCharacters))
	for _, raw := range registry {
		obj, ok := raw.(map[string]interface{})
		if !ok {
			continue
		}
		if strings.ToLower(strings.TrimSpace(helpers.GetString(obj, "type"))) != "character" {
			continue
		}
		id := strings.TrimSpace(helpers.GetString(obj, "id"))
		if id == "" {
			continue
		}
		appearance := extractAppearance(obj["immutable"])
		if appearance == "" {
			continue
		}
		lines = append(lines, fmt.Sprintf("- %s: %s", id, appearance))
	}
	return strings.Join(lines, "\n")
}

func extractAppearance(v interface{}) string {
	m, ok := v.(map[string]interface{})
	if !ok || len(m) == 0 {
		return ""
	}
	if ap, ok := m["appearance"]; ok {
		s := strings.TrimSpace(fmt.Sprint(ap))
		if s != "" {
			return s
		}
	}
	parts := make([]string, 0, len(m))
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		val := strings.TrimSpace(fmt.Sprint(m[k]))
		if val == "" {
			continue
		}
		parts = append(parts, fmt.Sprintf("%s=%s", k, val))
	}
	return strings.Join(parts, ", ")
}

func ensureProjectsDir(cfg Config) error {
	return os.MkdirAll(filepath.Join(cfg.FFmpegWorkDir, "projects"), 0o755)
}

func projectDirFor(cfg Config, projectID string) string {
	return filepath.Join(cfg.FFmpegWorkDir, "projects", projectID)
}

func taskIDSuffixFromPayload(payload map[string]interface{}) string {
	if payload == nil {
		return ""
	}
	raw := strings.TrimSpace(helpers.GetString(payload, "idiom_name_en"))
	if raw == "" {
		return ""
	}
	return toSafeSlug(raw)
}

func toSafeSlug(s string) string {
	s = strings.ToLower(strings.TrimSpace(s))
	if s == "" {
		return ""
	}
	re := regexp.MustCompile(`[^a-z0-9]+`)
	s = re.ReplaceAllString(s, "-")
	s = strings.Trim(s, "-")
	if len(s) > 40 {
		s = strings.Trim(s[:40], "-")
	}
	return s
}
