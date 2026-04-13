package client

import (
	"encoding/json"
	"errors"
	"strings"

	commerceModel "api/app/models/commerce"
	"api/pkg/database"
	"api/pkg/response"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type PublicProductListItem struct {
	ID            uint64   `json:"id"`
	Slug          string   `json:"slug"`
	ProductCode   string   `json:"product_code"`
	Name          string   `json:"name"`
	Locale        string   `json:"locale"`
	ProductType   string   `json:"product_type"`
	Status        string   `json:"status"`
	Currency      string   `json:"currency"`
	MinPrice      *float64 `json:"min_price,omitempty"`
	MaxPrice      *float64 `json:"max_price,omitempty"`
	CoverImageURL *string  `json:"cover_image_url,omitempty"`
	Description   *string  `json:"description,omitempty"`
}

type PublicProductSKUItem struct {
	ID            uint64   `json:"id"`
	SKUCode       string   `json:"sku_code"`
	Name          string   `json:"name"`
	Price         float64  `json:"price"`
	OriginalPrice *float64 `json:"original_price,omitempty"`
	Currency      string   `json:"currency"`
	Status        string   `json:"status"`
	IsDefault     bool     `json:"is_default"`
	StockQty      *int     `json:"stock_qty,omitempty"`
}

type PublicProductDetail struct {
	PublicProductListItem
	Metadata json.RawMessage        `json:"metadata,omitempty"`
	SKUs     []PublicProductSKUItem `json:"skus,omitempty"`
}

type PublicProductController struct {
	BaseAPIController
}

func (ctrl *PublicProductController) ListProducts(c *gin.Context) {
	locale, ok := normalizePublicLocale(c.Param("locale"))
	if !ok || locale == "" {
		response.Abort404(c, "locale not supported")
		return
	}

	limit := parsePositiveIntQuery(c.Query("limit"), 20, 1, 120)
	excludeSlug := strings.TrimSpace(c.Query("exclude_slug"))

	query := database.DB.
		Model(&commerceModel.Product{}).
		Select("id", "slug", "product_code", "name", "locale", "product_type", "status", "currency", "min_price", "max_price", "cover_image_url", "description").
		Where("locale = ?", locale)
	if excludeSlug != "" {
		query = query.Where("slug <> ?", excludeSlug)
	}

	var products []PublicProductListItem
	if err := query.
		Order("created_at DESC").
		Limit(limit).
		Find(&products).
		Error; err != nil {
		response.Abort500(c, err.Error())
		return
	}

	recommendedPodcasts, err := listPublishedPodcastScripts(locale, 8)
	if err != nil {
		response.Abort500(c, err.Error())
		return
	}

	response.JSON(c, gin.H{
		"locale":               locale,
		"products":             products,
		"recommended_podcasts": recommendedPodcasts,
	})
}

func (ctrl *PublicProductController) ShowProduct(c *gin.Context) {
	locale, ok := normalizePublicLocale(c.Param("locale"))
	if !ok || locale == "" {
		response.Abort404(c, "locale not supported")
		return
	}

	slug := strings.TrimSpace(c.Param("slug"))
	if slug == "" {
		response.Abort404(c, "product not found")
		return
	}

	var product PublicProductDetail
	if err := database.DB.
		Model(&commerceModel.Product{}).
		Select("id", "slug", "product_code", "name", "locale", "product_type", "status", "currency", "min_price", "max_price", "cover_image_url", "description", "metadata").
		Where("locale = ?", locale).
		Where("slug = ?", slug).
		First(&product).
		Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			response.Abort404(c, "product not found")
			return
		}
		response.Abort500(c, err.Error())
		return
	}

	var skus []PublicProductSKUItem
	if err := database.DB.
		Model(&commerceModel.ProductSKU{}).
		Select("id", "sku_code", "name", "price", "original_price", "currency", "status", "is_default", "stock_qty").
		Where("product_id = ?", product.ID).
		Order("is_default DESC").
		Order("created_at ASC").
		Limit(30).
		Find(&skus).
		Error; err != nil {
		response.Abort500(c, err.Error())
		return
	}
	product.SKUs = skus

	var recommendProducts []PublicProductListItem
	if err := database.DB.
		Model(&commerceModel.Product{}).
		Select("id", "slug", "product_code", "name", "locale", "product_type", "status", "currency", "min_price", "max_price", "cover_image_url", "description").
		Where("locale = ?", locale).
		Where("slug <> ?", slug).
		Order("created_at DESC").
		Limit(6).
		Find(&recommendProducts).
		Error; err != nil {
		response.Abort500(c, err.Error())
		return
	}

	recommendedPodcasts, err := listPublishedPodcastScripts(locale, 8)
	if err != nil {
		response.Abort500(c, err.Error())
		return
	}

	response.JSON(c, gin.H{
		"locale":               locale,
		"product":              product,
		"recommend_products":   recommendProducts,
		"recommended_podcasts": recommendedPodcasts,
	})
}
