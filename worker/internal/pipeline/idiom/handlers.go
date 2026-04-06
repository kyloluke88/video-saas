package idiom

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"worker/internal/dto"
	"worker/internal/pipeline"
	conf "worker/pkg/config"
	"worker/pkg/helpers"
	storageS3 "worker/pkg/storage/s3"
	ffmpegservice "worker/services/ffmpeg_service"
	idiompromptservice "worker/services/idiom_prompt_service"
	seedancegenerateservice "worker/services/seedance_generate_service"

	amqp "github.com/rabbitmq/amqp091-go"
)

func HandleUploadTask(ch *amqp.Channel, task dto.VideoTaskMessage) error {
	return handleProjectUpload(ch, task)
}

func HandlePlan(ch *amqp.Channel, task dto.VideoTaskMessage) error {
	requestPayload, _ := task.Payload["request_payload"].(map[string]interface{})
	if requestPayload == nil {
		requestPayload = map[string]interface{}{}
	}
	planBytes, err := json.Marshal(task.Payload["plan"])
	if err != nil {
		return err
	}
	var plan dto.ProjectPlanResult
	if err := json.Unmarshal(planBytes, &plan); err != nil {
		return err
	}

	projectID := strings.TrimSpace(helpers.GetString(requestPayload, "project_id"))
	idiomNameEn := strings.TrimSpace(helpers.GetString(requestPayload, "idiom_name_en"))
	aspectRatio := helpers.DefaultString(strings.TrimSpace(helpers.GetString(requestPayload, "aspect_ratio")), "16:9")
	resolution := helpers.DefaultString(strings.TrimSpace(helpers.GetString(requestPayload, "resolution")), "720p")

	if err := ensureProjectsDir(); err != nil {
		return err
	}
	projectDir := projectDirFor(projectID)
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

	visualBible := fmt.Sprintf("Style: %s, Color: %s, Lighting: %s, Era: %s, Negative: %s",
		plan.VisualBible.StyleAnchor,
		plan.VisualBible.ColorPalette,
		plan.VisualBible.Lighting,
		plan.VisualBible.EraSetting,
		plan.VisualBible.NegativePrompt,
	)
	characterCatalog, propsCatalog, environmentCatalog := idiompromptservice.BuildSceneObjectCatalog(plan.ObjectRegistry)

	for _, scene := range plan.Scenes {
		scenePrompt := idiompromptservice.BuildScenePrompt(scene, characterCatalog, propsCatalog, environmentCatalog, visualBible)
		sceneIndex := scene.SceneID

		if err := pipeline.PublishTask(ch, "scene.generate.v1", map[string]interface{}{
			"project_id":          projectID,
			"idiom_name_en":       idiomNameEn,
			"scene_index":         sceneIndex,
			"target_duration_sec": scene.DurationSeconds,
			"aspect_ratio":        aspectRatio,
			"resolution":          resolution,
			"scene_prompt":        scenePrompt,
		}); err != nil {
			return err
		}
	}
	log.Printf("🧠 规划任务接收完成 project_id=%s scenes=%d", projectID, len(plan.Scenes))
	return nil
}

func HandleSceneGenerate(ch *amqp.Channel, task dto.VideoTaskMessage) error {
	projectID := helpers.GetString(task.Payload, "project_id")
	sceneIndex := helpers.GetInt(task.Payload, "scene_index", 0)

	projectDir := projectDirFor(projectID)
	traceDir := filepath.Join(projectDir, "traces")
	if err := os.MkdirAll(traceDir, 0o755); err != nil {
		return err
	}
	tracePrefix := fmt.Sprintf("scene_%02d", sceneIndex)
	scenePrompt := strings.TrimSpace(helpers.GetString(task.Payload, "scene_prompt"))

	videoURL, err := seedancegenerateservice.RunSeedanceGenerate(seedancegenerateservice.SeedanceGenerateInput{
		Prompt:      scenePrompt,
		AspectRatio: helpers.DefaultString(helpers.GetString(task.Payload, "aspect_ratio"), "16:9"),
		Resolution:  helpers.DefaultString(helpers.GetString(task.Payload, "resolution"), "720p"),
		DurationSec: helpers.GetInt(task.Payload, "target_duration_sec", 8),
	}, traceDir, tracePrefix)
	if err != nil {
		return err
	}
	if strings.TrimSpace(videoURL) == "" {
		return nil
	}

	scenesDir := filepath.Join(projectDir, "scenes")
	if err := os.MkdirAll(scenesDir, 0o755); err != nil {
		return err
	}
	rawPath := filepath.Join(scenesDir, fmt.Sprintf("%02d_raw.mp4", sceneIndex))
	if err := helpers.DownloadToFile(videoURL, rawPath, conf.Get[int]("worker.seedance_http_timeout_sec")); err != nil {
		return err
	}

	normalizedPath := filepath.Join(scenesDir, fmt.Sprintf("%02d_norm.mp4", sceneIndex))
	if err := ffmpegservice.NormalizeSceneVideo(rawPath, normalizedPath); err != nil {
		return err
	}

	targetDurationSec := helpers.GetInt(task.Payload, "target_duration_sec", 0)
	sceneFinalPath := filepath.Join(scenesDir, fmt.Sprintf("%02d.mp4", sceneIndex))
	if err := ffmpegservice.TrimVideoDuration(normalizedPath, sceneFinalPath, targetDurationSec); err != nil {
		return err
	}

	_ = os.WriteFile(filepath.Join(scenesDir, fmt.Sprintf("%02d.done", sceneIndex)), []byte(time.Now().Format(time.RFC3339)), 0o644)
	log.Printf("🎞️ 场景生成完成 project_id=%s scene=%d file=%s", projectID, sceneIndex, sceneFinalPath)

	return tryTriggerCompose(ch, projectID, helpers.GetString(task.Payload, "idiom_name_en"))
}

func HandleProjectCompose(ch *amqp.Channel, task dto.VideoTaskMessage) error {
	projectID := helpers.GetString(task.Payload, "project_id")
	projectDir := projectDirFor(projectID)
	planPath := filepath.Join(projectDir, "plan.json")
	var plan dto.ProjectPlanResult
	if err := readJSON(planPath, &plan); err != nil {
		return err
	}

	scenesDir := filepath.Join(projectDir, "scenes")
	sceneFiles := make([]string, 0, len(plan.Scenes))
	for _, scene := range plan.Scenes {
		sceneIndex := scene.SceneID
		if sceneIndex <= 0 {
			sceneIndex = 1
		}
		scenePath := filepath.Join(scenesDir, fmt.Sprintf("%02d.mp4", sceneIndex))
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
	if err := ffmpegservice.RunFFmpeg("-y", "-f", "concat", "-safe", "0", "-i", concatInput, "-c", "copy", stitched); err != nil {
		return err
	}

	withSubtitles := filepath.Join(projectDir, "with_subtitles.mp4")
	servicePlan := toServiceProjectPlan(plan)
	if err := ffmpegservice.BurnSubtitles(stitched, servicePlan, withSubtitles); err != nil {
		return err
	}

	finalPath := filepath.Join(projectDir, "final.mp4")
	bgmPath := ""
	if conf.Get[bool]("worker.bgm_enabled") {
		var err error
		bgmPath, err = ffmpegservice.SelectRandomBGM()
		if err != nil {
			return err
		}
		if bgmPath != "" {
			_ = os.WriteFile(filepath.Join(projectDir, "bgm_selected.txt"), []byte(bgmPath+"\n"), 0o644)
			log.Printf("🎵 BGM selected project_id=%s bgm=%s", projectID, bgmPath)
		}
	} else {
		log.Printf("⏭️ BGM mix skipped by BGM_ENABLED=false project_id=%s", projectID)
	}
	if err := ffmpegservice.ComposeFinalVideo(servicePlan, withSubtitles, "", bgmPath, finalPath); err != nil {
		return err
	}

	log.Printf("🎬 项目合成完成 project_id=%s final=%s", projectID, finalPath)
	return pipeline.PublishTask(ch, "upload.v1", map[string]interface{}{
		"project_id":    projectID,
		"idiom_name_en": helpers.GetString(task.Payload, "idiom_name_en"),
		"file_path":     finalPath,
		"content_type":  "idiom",
	})
}

func handleProjectUpload(ch *amqp.Channel, task dto.VideoTaskMessage) error {
	projectID := helpers.GetString(task.Payload, "project_id")
	filePath := helpers.GetString(task.Payload, "file_path")
	contentType := strings.TrimSpace(helpers.GetString(task.Payload, "content_type"))
	videoURL := ""

	if !conf.Get[bool]("worker.s3_enabled") || strings.TrimSpace(conf.Get[string]("worker.s3_bucket")) == "" {
		log.Printf("📦 S3 未启用，保留本地文件 project_id=%s path=%s", projectID, filePath)
	} else {
		objectKey := fmt.Sprintf("projects/%s/final.mp4", projectID)
		publicURL, err := storageS3.UploadFile(storageS3.Config{
			Endpoint:  conf.Get[string]("worker.s3_endpoint"),
			Region:    conf.Get[string]("worker.s3_region"),
			Bucket:    conf.Get[string]("worker.s3_bucket"),
			AccessKey: conf.Get[string]("worker.s3_access_key"),
			SecretKey: conf.Get[string]("worker.s3_secret_key"),
			PublicURL: conf.Get[string]("worker.s3_public_url"),
		}, filePath, objectKey)
		if err != nil {
			return err
		}
		videoURL = publicURL
		log.Printf("☁️ 上传S3完成 project_id=%s url=%s", projectID, publicURL)
	}

	if contentType == "podcast" {
		if err := pipeline.UpdatePodcastProjectUpload(projectID, videoURL, "", ""); err != nil {
			log.Printf("⚠️ update podcast upload status failed project_id=%s err=%v", projectID, err)
		}
		return pipeline.PublishTask(ch, "podcast.page.persist.v1", map[string]interface{}{
			"project_id":   projectID,
			"video_url":    videoURL,
			"content_type": "podcast",
		})
	}

	return nil
}

func tryTriggerCompose(ch *amqp.Channel, projectID, idiomNameEn string) error {
	projectDir := projectDirFor(projectID)
	var plan dto.ProjectPlanResult
	if err := readJSON(filepath.Join(projectDir, "plan.json"), &plan); err != nil {
		return err
	}

	for _, scene := range plan.Scenes {
		sceneIndex := scene.SceneID
		if _, err := os.Stat(filepath.Join(projectDir, "scenes", fmt.Sprintf("%02d.done", sceneIndex))); err != nil {
			return nil
		}
	}

	marker := filepath.Join(projectDir, "compose.queued")
	f, err := os.OpenFile(marker, os.O_CREATE|os.O_EXCL, 0o644)
	if err != nil {
		return nil
	}
	_ = f.Close()

	return pipeline.PublishTask(ch, "compose.v1", map[string]interface{}{
		"project_id":    projectID,
		"idiom_name_en": idiomNameEn,
	})
}
