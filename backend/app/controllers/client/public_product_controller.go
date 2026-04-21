package client

import (
	"encoding/json"
	"errors"
	"strings"

	commerceModel "api/app/models/commerce"
	contentModel "api/app/models/content"
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
	// 详情页返回显式 SEO 列，前端 metadata 不需要再去解析 metadata JSON。
	SEOTitle       *string                  `json:"seo_title,omitempty"`
	SEODescription *string                  `json:"seo_description,omitempty"`
	SEOKeywords    contentModel.StringArray `json:"seo_keywords,omitempty"`
	CanonicalURL   *string                  `json:"canonical_url,omitempty"`
	Metadata       json.RawMessage          `json:"metadata,omitempty"`
	SKUs           []PublicProductSKUItem   `json:"skus,omitempty"`
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
		// 列表页只拿展示所需字段，避免把 SEO 字段和 JSON metadata 一起拉出。
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
		// 详情页把 SEO 字段一并取出，供前端 generateMetadata 直接使用。
		Select("id", "slug", "product_code", "name", "locale", "product_type", "status", "currency", "min_price", "max_price", "cover_image_url", "description", "seo_title", "seo_description", "seo_keywords", "canonical_url", "metadata").
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
		// 推荐位和主详情页解耦，只保留轻量列表字段。
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
