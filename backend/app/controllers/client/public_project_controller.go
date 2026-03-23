package client

import (
	appconfig "api/pkg/config"
	"api/pkg/response"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
)

type PublicProjectController struct {
	BaseAPIController
}

type publicProjectSummary struct {
	ProjectID        string              `json:"project_id"`
	ConversationID   string              `json:"conversation_id"`
	Title            string              `json:"title"`
	Language         string              `json:"language"`
	AudienceLanguage string              `json:"audience_language"`
	TurnCount        int                 `json:"turn_count"`
	SegmentCount     int                 `json:"segment_count"`
	UpdatedAt        string              `json:"updated_at"`
	DetailPath       string              `json:"detail_path"`
	Assets           publicProjectAssets `json:"assets"`
}

type publicProjectDetail struct {
	publicProjectSummary
	Conversation conversationMinimal `json:"conversation"`
}

type publicProjectAssets struct {
	VideoPath    string `json:"video_path,omitempty"`
	AudioPath    string `json:"audio_path,omitempty"`
	SubtitlePath string `json:"subtitle_path,omitempty"`
}

type conversationMinimal struct {
	ConversationID   string             `json:"conversation_id"`
	Language         string             `json:"language"`
	AudienceLanguage string             `json:"audience_language"`
	Title            string             `json:"title"`
	Turns            []conversationTurn `json:"turns"`
}

type conversationTurn struct {
	TurnID      string                `json:"turn_id"`
	Role        string                `json:"role"`
	Speaker     string                `json:"speaker"`
	SpeakerName string                `json:"speaker_name"`
	Segments    []conversationSegment `json:"segments"`
}

type conversationSegment struct {
	SegmentID   string             `json:"segment_id"`
	DisplayText string             `json:"display_text"`
	English     string             `json:"english"`
	Ruby        []conversationRuby `json:"ruby,omitempty"`
}

type conversationRuby struct {
	Surface string `json:"surface"`
	Reading string `json:"reading"`
}

type scriptInputMetadata struct {
	Title string `json:"title"`
}

var allowedPublicAssets = map[string]struct{}{
	"dialogue.mp3":          {},
	"podcast_base.mp4":      {},
	"podcast_content.mp4":   {},
	"podcast_final.mp4":     {},
	"podcast_subtitles.ass": {},
}

func (ctrl *PublicProjectController) ListProjects(c *gin.Context) {
	projectsRoot, err := resolveProjectsRoot()
	if err != nil {
		response.Abort500(c, err.Error())
		return
	}

	entries, err := os.ReadDir(projectsRoot)
	if err != nil {
		response.Abort500(c, "read projects directory failed: "+err.Error())
		return
	}

	projects := make([]publicProjectSummary, 0, len(entries))
	for _, entry := range entries {
		if !entry.IsDir() || strings.HasPrefix(entry.Name(), ".") {
			continue
		}

		project, err := loadPublicProject(projectsRoot, entry.Name())
		if err != nil {
			continue
		}

		projects = append(projects, project.publicProjectSummary)
	}

	sort.Slice(projects, func(i, j int) bool {
		return projects[i].UpdatedAt > projects[j].UpdatedAt
	})

	response.JSON(c, gin.H{
		"count":    len(projects),
		"projects": projects,
	})
}

func (ctrl *PublicProjectController) ShowProject(c *gin.Context) {
	projectsRoot, err := resolveProjectsRoot()
	if err != nil {
		response.Abort500(c, err.Error())
		return
	}

	projectID := strings.TrimSpace(c.Param("projectID"))
	project, err := loadPublicProject(projectsRoot, projectID)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			response.Abort404(c, "project not found")
			return
		}
		response.Abort500(c, err.Error())
		return
	}

	response.JSON(c, gin.H{
		"project": project,
	})
}

func (ctrl *PublicProjectController) ServeProjectAsset(c *gin.Context) {
	projectsRoot, err := resolveProjectsRoot()
	if err != nil {
		response.Abort500(c, err.Error())
		return
	}

	projectID := strings.TrimSpace(c.Param("projectID"))
	assetName := strings.TrimSpace(c.Param("assetName"))
	if !isAllowedAsset(assetName) {
		response.Abort404(c, "asset not found")
		return
	}

	projectDir, err := resolveProjectDir(projectsRoot, projectID)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			response.Abort404(c, "project not found")
			return
		}
		response.Abort500(c, err.Error())
		return
	}

	assetPath, err := resolveChildPath(projectDir, assetName)
	if err != nil {
		response.Abort404(c, "asset not found")
		return
	}

	info, err := os.Stat(assetPath)
	if err != nil || info.IsDir() {
		response.Abort404(c, "asset not found")
		return
	}

	c.Header("Cache-Control", "public, max-age=300")
	c.File(assetPath)
}

func loadPublicProject(projectsRoot string, projectID string) (*publicProjectDetail, error) {
	projectDir, err := resolveProjectDir(projectsRoot, projectID)
	if err != nil {
		return nil, err
	}

	conversationPath, err := resolveChildPath(projectDir, "conversation_minimal.json")
	if err != nil {
		return nil, err
	}

	payload, err := os.ReadFile(conversationPath)
	if err != nil {
		return nil, err
	}

	var conversation conversationMinimal
	if err := json.Unmarshal(payload, &conversation); err != nil {
		return nil, fmt.Errorf("decode conversation_minimal.json for %s failed: %w", projectID, err)
	}

	if strings.TrimSpace(conversation.ConversationID) == "" {
		conversation.ConversationID = projectID
	}
	if strings.TrimSpace(conversation.Title) == "" {
		conversation.Title = loadFallbackTitle(projectDir, projectID)
	}

	info, err := os.Stat(conversationPath)
	if err != nil {
		return nil, err
	}

	assets := publicProjectAssets{}
	if fileExists(filepath.Join(projectDir, "podcast_final.mp4")) {
		assets.VideoPath = buildAssetPath(projectID, "podcast_final.mp4")
	}
	if fileExists(filepath.Join(projectDir, "dialogue.mp3")) {
		assets.AudioPath = buildAssetPath(projectID, "dialogue.mp3")
	}
	if fileExists(filepath.Join(projectDir, "podcast_subtitles.ass")) {
		assets.SubtitlePath = buildAssetPath(projectID, "podcast_subtitles.ass")
	}

	project := &publicProjectDetail{
		publicProjectSummary: publicProjectSummary{
			ProjectID:        projectID,
			ConversationID:   conversation.ConversationID,
			Title:            defaultProjectTitle(conversation.Title, projectID),
			Language:         strings.TrimSpace(conversation.Language),
			AudienceLanguage: strings.TrimSpace(conversation.AudienceLanguage),
			TurnCount:        len(conversation.Turns),
			SegmentCount:     countSegments(conversation.Turns),
			UpdatedAt:        info.ModTime().UTC().Format(time.RFC3339),
			DetailPath:       buildDetailPath(projectID),
			Assets:           assets,
		},
		Conversation: conversation,
	}

	return project, nil
}

func resolveProjectsRoot() (string, error) {
	candidates := uniqueNonEmptyStrings(
		appconfig.Get[string]("content.projects_dir"),
		"../worker/outputs/projects",
		"worker/outputs/projects",
		"/data/worker-outputs/projects",
	)

	for _, candidate := range candidates {
		absolutePath, err := filepath.Abs(candidate)
		if err != nil {
			continue
		}

		info, err := os.Stat(absolutePath)
		if err == nil && info.IsDir() {
			return absolutePath, nil
		}
	}

	return "", fmt.Errorf("projects directory not found; checked: %s", strings.Join(candidates, ", "))
}

func resolveProjectDir(projectsRoot string, projectID string) (string, error) {
	if !isSafeProjectID(projectID) {
		return "", os.ErrNotExist
	}
	return resolveChildPath(projectsRoot, projectID)
}

func resolveChildPath(root string, child string) (string, error) {
	rootAbs, err := filepath.Abs(root)
	if err != nil {
		return "", err
	}

	targetAbs, err := filepath.Abs(filepath.Join(rootAbs, child))
	if err != nil {
		return "", err
	}

	if targetAbs != rootAbs && !strings.HasPrefix(targetAbs, rootAbs+string(os.PathSeparator)) {
		return "", os.ErrNotExist
	}

	return targetAbs, nil
}

func isSafeProjectID(projectID string) bool {
	if projectID == "" || projectID == "." || projectID == ".." {
		return false
	}
	if strings.Contains(projectID, "/") || strings.Contains(projectID, "\\") {
		return false
	}
	return true
}

func isAllowedAsset(assetName string) bool {
	_, ok := allowedPublicAssets[assetName]
	return ok
}

func buildDetailPath(projectID string) string {
	return "/api/public/projects/" + projectID
}

func buildAssetPath(projectID string, assetName string) string {
	return "/api/public/projects/" + projectID + "/assets/" + assetName
}

func loadFallbackTitle(projectDir string, projectID string) string {
	scriptPath := filepath.Join(projectDir, "script_input.json")
	payload, err := os.ReadFile(scriptPath)
	if err != nil {
		return projectID
	}

	var metadata scriptInputMetadata
	if err := json.Unmarshal(payload, &metadata); err != nil {
		return projectID
	}

	if strings.TrimSpace(metadata.Title) == "" {
		return projectID
	}

	return strings.TrimSpace(metadata.Title)
}

func countSegments(turns []conversationTurn) int {
	total := 0
	for _, turn := range turns {
		total += len(turn.Segments)
	}
	return total
}

func fileExists(path string) bool {
	info, err := os.Stat(path)
	if err != nil {
		return false
	}
	return !info.IsDir()
}

func defaultProjectTitle(title string, fallback string) string {
	if strings.TrimSpace(title) == "" {
		return fallback
	}
	return strings.TrimSpace(title)
}

func uniqueNonEmptyStrings(values ...string) []string {
	seen := make(map[string]struct{}, len(values))
	result := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		result = append(result, value)
	}
	return result
}
