package upload

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"

	"worker/internal/app/task"
	podcastreplay "worker/internal/app/workflow/podcast/replay"
	"worker/internal/persistence"
	conf "worker/pkg/config"
	storageS3 "worker/pkg/storage/s3"
	"worker/pkg/x/mapx"

	amqp "github.com/rabbitmq/amqp091-go"
)

type payload struct {
	ProjectID       string   `json:"project_id"`
	SourceProjectID string   `json:"source_project_id,omitempty"`
	RunMode         int      `json:"run_mode,omitempty"`
	TTSType         int      `json:"tts_type,omitempty"`
	SpecifyTasks    []string `json:"specify_tasks,omitempty"`
	FilePath        string   `json:"file_path,omitempty"`
	ContentType     string   `json:"content_type"`
}

type downloadAsset struct {
	Label  string `json:"label"`
	Format string `json:"format"`
	URL    string `json:"url,omitempty"`
	Ready  bool   `json:"ready"`
}

func HandleUploadTask(ctx context.Context, ch *amqp.Channel, task task.VideoTaskMessage) error {
	payload, err := decodePayload(task.Payload)
	if err != nil {
		return err
	}
	switch strings.ToLower(strings.TrimSpace(payload.ContentType)) {
	case "podcast":
		return handlePodcastUpload(ctx, payload)
	default:
		return handleSingleFileUpload(ctx, payload)
	}
}

func handleSingleFileUpload(ctx context.Context, payload payload) error {
	projectID := strings.TrimSpace(payload.ProjectID)
	filePath := strings.TrimSpace(payload.FilePath)
	if projectID == "" {
		return fmt.Errorf("project_id is required")
	}
	if filePath == "" {
		return fmt.Errorf("file_path is required")
	}

	if !s3Enabled() {
		log.Printf("📦 S3 未启用，保留本地文件 project_id=%s path=%s", projectID, filePath)
		return nil
	}

	objectKey := fmt.Sprintf("projects/%s/%s", projectID, filepath.Base(filePath))
	_, err := storageS3.UploadFile(ctx, s3ConfigFromEnv(), filePath, objectKey)
	return err
}

func handlePodcastUpload(ctx context.Context, payload payload) error {
	projectID := strings.TrimSpace(payload.ProjectID)
	if projectID == "" {
		return fmt.Errorf("project_id is required")
	}
	if payload.RunMode != 0 && payload.RunMode != 1 {
		return fmt.Errorf("podcast upload only supports run_mode 0 or 1")
	}
	if payload.RunMode == 1 || strings.TrimSpace(payload.SourceProjectID) != "" {
		if payload.RunMode != 1 {
			return fmt.Errorf("upload replay entry requires run_mode=1")
		}
		normalizedTasks, err := podcastreplay.ValidateSpecifyTasks(payload.TTSType, payload.RunMode, payload.SpecifyTasks)
		if err != nil {
			return err
		}
		payload.SpecifyTasks = normalizedTasks
		if err := podcastreplay.EnsureReplayProjectDirForProject(payload.ProjectID, payload.SourceProjectID); err != nil {
			return err
		}
	}

	projectDir := filepath.Join(conf.Get[string]("worker.ffmpeg_work_dir"), "projects", projectID)
	pdfPath := filepath.Join(projectDir, "chat_script.pdf")
	if _, err := os.Stat(pdfPath); err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("chat_script.pdf not found for project %s", projectID)
		}
		return err
	}

	downloads := []downloadAsset{{
		Label:  "下载聊天脚本 PDF",
		Format: "pdf",
		Ready:  false,
	}}

	if s3Enabled() {
		objectKey := fmt.Sprintf("podcast/my/%s/%s", projectID, filepath.Base(pdfPath))
		result, err := storageS3.UploadFile(ctx, s3ConfigFromEnv(), pdfPath, objectKey)
		if err != nil {
			return err
		}
		downloads[0].URL = result
		downloads[0].Ready = true
	} else {
		log.Printf("📦 S3 未启用，保留本地 chat pdf project_id=%s path=%s", projectID, pdfPath)
	}

	downloadsJSON, err := json.Marshal(downloads)
	if err != nil {
		return err
	}
	if err := task.UpdatePodcastProjectUpload(projectID); err != nil {
		return err
	}
	store, err := persistence.DefaultStore()
	if err != nil {
		return err
	}
	if err := store.UpdatePodcastScriptPageDownloads(projectID, downloadsJSON); err != nil {
		return err
	}
	return task.FinalizePodcastProjectUpload(projectID)
}

func s3Enabled() bool {
	return conf.Get[bool]("worker.s3_enabled") && strings.TrimSpace(conf.Get[string]("worker.s3_bucket")) != ""
}

func s3ConfigFromEnv() storageS3.Config {
	return storageS3.Config{
		Endpoint:  conf.Get[string]("worker.s3_endpoint"),
		Region:    conf.Get[string]("worker.s3_region"),
		Bucket:    conf.Get[string]("worker.s3_bucket"),
		AccessKey: conf.Get[string]("worker.s3_access_key"),
		SecretKey: conf.Get[string]("worker.s3_secret_key"),
		PublicURL: conf.Get[string]("worker.s3_public_url"),
	}
}

func decodePayload(raw map[string]interface{}) (payload, error) {
	data, err := json.Marshal(raw)
	if err != nil {
		return payload{}, err
	}
	var out payload
	if err := json.Unmarshal(data, &out); err != nil {
		return payload{}, err
	}
	if strings.TrimSpace(out.ProjectID) == "" {
		out.ProjectID = strings.TrimSpace(mapx.GetString(raw, "project_id"))
	}
	if strings.TrimSpace(out.ContentType) == "" {
		out.ContentType = strings.TrimSpace(mapx.GetString(raw, "content_type"))
	}
	return out, nil
}
