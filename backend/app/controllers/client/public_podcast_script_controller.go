package client

import (
	"errors"
	"fmt"
	"strings"

	contentModel "api/app/models/content"
	"api/pkg/database"
	"api/pkg/logger"
	"api/pkg/response"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type PublicPodcastScriptController struct {
	BaseAPIController
}

func (ctrl *PublicPodcastScriptController) ListPages(c *gin.Context) {
	language, ok := normalizePublicLocale(c.Query("language"))
	if !ok {
		response.BadRequest(c, fmt.Errorf("invalid language"), "language must be zh or ja")
		return
	}

	limit := parsePositiveIntQuery(c.Query("limit"), 24, 1, 120)
	pages, err := listPublishedPodcastScripts(language, limit)
	if err != nil {
		response.Abort500(c, err.Error())
		return
	}

	response.JSON(c, gin.H{
		"pages": pages,
	})
}

func (ctrl *PublicPodcastScriptController) ShowPage(c *gin.Context) {
	slug := strings.TrimSpace(c.Param("slug"))
	logger.DebugString("slug", "", "Received request for podcast script page with slug: "+slug)
	if slug == "" {
		response.Abort404(c, "page not found")
		return
	}

	page, err := findPublishedPodcastScriptPage(slug)
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

func findPublishedPodcastScriptPage(slug string) (*contentModel.PodcastScriptPage, error) {
	slug = strings.TrimSpace(slug)
	if slug == "" {
		return nil, gorm.ErrRecordNotFound
	}

	page := new(contentModel.PodcastScriptPage)
	if err := database.DB.
		Model(&contentModel.PodcastScriptPage{}).
		Where("status = ?", "published").
		Where("slug = ?", slug).
		First(page).
		Error; err != nil {
		return nil, err
	}
	return page, nil
}
