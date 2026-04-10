package idiom

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"worker/internal/app/task"
	conf "worker/pkg/config"
	"worker/pkg/x/httpx"
	"worker/pkg/x/mapx"
	dto "worker/services/idiom/model"
	idiompromptservice "worker/services/idiom/prompt"
	ffmpegservice "worker/services/media/ffmpeg"
	seedancegenerateservice "worker/services/media/seedance"

	amqp "github.com/rabbitmq/amqp091-go"
)

func HandlePlan(_ context.Context, ch *amqp.Channel, msg task.VideoTaskMessage) error {
	requestPayload, _ := msg.Payload["request_payload"].(map[string]interface{})
	if requestPayload == nil {
		requestPayload = map[string]interface{}{}
	}
	planBytes, err := json.Marshal(msg.Payload["plan"])
	if err != nil {
		return err
	}
	var plan dto.ProjectPlanResult
	if err := json.Unmarshal(planBytes, &plan); err != nil {
		return err
	}

	projectID := strings.TrimSpace(mapx.GetString(requestPayload, "project_id"))
	idiomNameEn := strings.TrimSpace(mapx.GetString(requestPayload, "idiom_name_en"))
	aspectRatio := defaultString(strings.TrimSpace(mapx.GetString(requestPayload, "aspect_ratio")), "16:9")
	resolution := defaultString(strings.TrimSpace(mapx.GetString(requestPayload, "resolution")), "720p")

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

		if err := task.PublishTask(ch, "scene.generate.v1", map[string]interface{}{
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

func HandleSceneGenerate(ctx context.Context, ch *amqp.Channel, msg task.VideoTaskMessage) error {
	projectID := mapx.GetString(msg.Payload, "project_id")
	sceneIndex := mapx.GetInt(msg.Payload, "scene_index", 0)

	projectDir := projectDirFor(projectID)
	traceDir := filepath.Join(projectDir, "traces")
	if err := os.MkdirAll(traceDir, 0o755); err != nil {
		return err
	}
	tracePrefix := fmt.Sprintf("scene_%02d", sceneIndex)
	scenePrompt := strings.TrimSpace(mapx.GetString(msg.Payload, "scene_prompt"))

	videoURL, err := seedancegenerateservice.RunSeedanceGenerate(ctx, seedancegenerateservice.SeedanceGenerateInput{
		Prompt:      scenePrompt,
		AspectRatio: defaultString(mapx.GetString(msg.Payload, "aspect_ratio"), "16:9"),
		Resolution:  defaultString(mapx.GetString(msg.Payload, "resolution"), "720p"),
		DurationSec: mapx.GetInt(msg.Payload, "target_duration_sec", 8),
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
	if err := httpx.DownloadToFileWithContext(ctx, videoURL, rawPath, conf.Get[int]("worker.seedance_http_timeout_sec")); err != nil {
		return err
	}

	normalizedPath := filepath.Join(scenesDir, fmt.Sprintf("%02d_norm.mp4", sceneIndex))
	if err := ffmpegservice.NormalizeSceneVideoContext(ctx, rawPath, normalizedPath); err != nil {
		return err
	}

	targetDurationSec := mapx.GetInt(msg.Payload, "target_duration_sec", 0)
	sceneFinalPath := filepath.Join(scenesDir, fmt.Sprintf("%02d.mp4", sceneIndex))
	if err := ffmpegservice.TrimVideoDurationContext(ctx, normalizedPath, sceneFinalPath, targetDurationSec); err != nil {
		return err
	}

	_ = os.WriteFile(filepath.Join(scenesDir, fmt.Sprintf("%02d.done", sceneIndex)), []byte(time.Now().Format(time.RFC3339)), 0o644)
	log.Printf("🎞️ 场景生成完成 project_id=%s scene=%d file=%s", projectID, sceneIndex, sceneFinalPath)

	return tryTriggerCompose(ch, projectID, mapx.GetString(msg.Payload, "idiom_name_en"))
}

func HandleProjectCompose(ctx context.Context, ch *amqp.Channel, msg task.VideoTaskMessage) error {
	projectID := mapx.GetString(msg.Payload, "project_id")
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
	if err := ffmpegservice.RunFFmpegContext(ctx, "-y", "-f", "concat", "-safe", "0", "-i", concatInput, "-c", "copy", stitched); err != nil {
		return err
	}

	withSubtitles := filepath.Join(projectDir, "with_subtitles.mp4")
	servicePlan := toServiceProjectPlan(plan)
	if err := ffmpegservice.BurnSubtitlesContext(ctx, stitched, servicePlan, withSubtitles); err != nil {
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
	if err := ffmpegservice.ComposeFinalVideoContext(ctx, servicePlan, withSubtitles, "", bgmPath, finalPath); err != nil {
		return err
	}

	log.Printf("🎬 项目合成完成 project_id=%s final=%s", projectID, finalPath)
	return task.PublishTask(ch, "upload.v1", map[string]interface{}{
		"project_id":    projectID,
		"idiom_name_en": mapx.GetString(msg.Payload, "idiom_name_en"),
		"file_path":     finalPath,
		"content_type":  "idiom",
	})
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

	return task.PublishTask(ch, "compose.v1", map[string]interface{}{
		"project_id":    projectID,
		"idiom_name_en": idiomNameEn,
	})
}

func defaultString(value, fallback string) string {
	if strings.TrimSpace(value) == "" {
		return fallback
	}
	return value
}
