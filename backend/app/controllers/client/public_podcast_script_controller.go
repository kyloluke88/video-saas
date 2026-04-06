package client

import (
	"errors"
	"strconv"
	"strings"

	contentModel "api/app/models/content"
	"api/pkg/database"
	"api/pkg/response"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type PublicPodcastScriptController struct {
	BaseAPIController
}

func (ctrl *PublicPodcastScriptController) ShowPage(c *gin.Context) {
	resourceID := strings.TrimSpace(c.Param("resourceID"))
	if resourceID == "" {
		response.Abort404(c, "page not found")
		return
	}

	page, err := findPublishedPodcastScriptPage(resourceID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			response.Abort404(c, "page not found")
			return
		}
		response.Abort500(c, err.Error())
		return
	}

	response.JSON(c, gin.H{
		"page": page,
	})
}

func findPublishedPodcastScriptPage(resourceID string) (*contentModel.PodcastScriptPage, error) {
	resourceID = strings.TrimSpace(resourceID)
	if resourceID == "" {
		return nil, gorm.ErrRecordNotFound
	}

	page := new(contentModel.PodcastScriptPage)
	query := database.DB.Model(&contentModel.PodcastScriptPage{}).Where("status = ?", "published")

	normalizedID := resourceID
	if parts := strings.SplitN(resourceID, "-", 2); len(parts) > 0 && strings.TrimSpace(parts[0]) != "" {
		normalizedID = strings.TrimSpace(parts[0])
	}

	if numericID, err := strconv.ParseUint(normalizedID, 10, 64); err == nil {
		if err := query.Where("id = ?", numericID).First(page).Error; err == nil {
			return page, nil
		}
	}

	if err := database.DB.
		Model(&contentModel.PodcastScriptPage{}).
		Where("status = ?", "published").
		Where("slug = ?", resourceID).
		First(page).
		Error; err != nil {
		if err := database.DB.
			Model(&contentModel.PodcastScriptPage{}).
			Where("status = ?", "published").
			Where("project_id = ?", resourceID).
			First(page).
			Error; err != nil {
			return nil, err
		}
	}
	return page, nil
}
