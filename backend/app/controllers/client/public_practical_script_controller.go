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

type PublicPracticalScriptController struct {
	BaseAPIController
}

func (ctrl *PublicPracticalScriptController) ListPages(c *gin.Context) {
	language, ok := normalizePublicLocale(c.Query("language"))
	if !ok {
		response.BadRequest(c, fmt.Errorf("invalid language"), "language must be zh or ja")
		return
	}

	limit := parsePositiveIntQuery(c.Query("limit"), 24, 1, 120)
	pages, err := listPublishedPracticalScripts(language, limit)
	if err != nil {
		response.Abort500(c, err.Error())
		return
	}

	response.JSON(c, gin.H{
		"pages": pages,
	})
}

func (ctrl *PublicPracticalScriptController) ShowPage(c *gin.Context) {
	slug := strings.TrimSpace(c.Param("slug"))
	logger.DebugString("slug", "", "Received request for practical script page with slug: "+slug)
	if slug == "" {
		response.Abort404(c, "page not found")
		return
	}

	page, err := findPublishedPracticalScriptPage(slug)
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

func findPublishedPracticalScriptPage(slug string) (*contentModel.PracticalScriptPage, error) {
	slug = strings.TrimSpace(slug)
	if slug == "" {
		return nil, gorm.ErrRecordNotFound
	}

	page := new(contentModel.PracticalScriptPage)
	if err := database.DB.
		Model(&contentModel.PracticalScriptPage{}).
		Where("status = ?", "published").
		Where("slug = ?", slug).
		First(page).
		Error; err != nil {
		return nil, err
	}
	return page, nil
}
