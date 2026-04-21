package client

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strings"
	"time"

	analyticsModel "api/app/models/analytics"
	clientreq "api/app/requests/client"
	"api/pkg/database"
	"api/pkg/redis"
	"api/pkg/response"

	"github.com/gin-gonic/gin"
)

const pageViewDedupWindow = 10 * time.Second

type AnalyticsController struct {
	BaseAPIController
}

func (ctrl *AnalyticsController) TrackPageView(c *gin.Context) {
	var req clientreq.PageViewEventRequest
	if !ctrl.BindJSON(c, &req) {
		return
	}

	// 先归一化核心页面字段，再根据页面类型决定是否必须携带实体 ID。
	pageType, ok := analyticsModel.NormalizePageType(req.PageType)
	if !ok {
		response.BadRequest(c, fmt.Errorf("unsupported page_type"), "page_type is invalid")
		return
	}

	pagePath := strings.TrimSpace(req.PagePath)
	if pagePath == "" {
		response.BadRequest(c, fmt.Errorf("page_path is empty"), "page_path is required")
		return
	}

	if pageType.RequiresEntityID() && req.PageEntityID == nil {
		response.BadRequest(c, fmt.Errorf("page_entity_id is required"), "page_entity_id is required for detail pages")
		return
	}

	event := analyticsModel.PageViewEvent{
		VisitorKey:     strings.TrimSpace(req.VisitorKey),
		SessionKey:     strings.TrimSpace(req.SessionKey),
		PageType:       int16(pageType),
		PageEntityID:   req.PageEntityID,
		PagePath:       pagePath,
		ViewedAt:       time.Now().UTC(),
		Referer:        optionalString(req.Referer),
		IP:             optionalString(c.ClientIP()),
		UserAgent:      optionalString(firstNonEmpty(c.Request.UserAgent(), req.UserAgent)),
		ClientHints:    normalizeClientHints(req.ClientHints),
		AcceptLanguage: optionalString(firstNonEmpty(c.GetHeader("Accept-Language"), req.AcceptLanguage)),
		CreatedAt:      time.Now().UTC(),
	}

	if event.VisitorKey == "" || event.SessionKey == "" {
		response.BadRequest(c, fmt.Errorf("visitor_key/session_key are required"), "visitor_key and session_key are required")
		return
	}

	if !isUUIDLike(event.VisitorKey) || !isUUIDLike(event.SessionKey) {
		response.BadRequest(c, fmt.Errorf("invalid uuid"), "visitor_key and session_key must be uuid values")
		return
	}

	if len(event.ClientHints) > 0 && len(event.ClientHints) > 4096 {
		response.BadRequest(c, fmt.Errorf("client_hints too large"), "client_hints is too large")
		return
	}

	if database.DB == nil {
		response.Abort500(c, "database is not initialized")
		return
	}

	// 去重窗口内的重复上报，通常来自刷新、重试、同一页面的重复挂载。
	// key 由 visitor/session/page_type/entity/path 组成，保证不同页面不会互相误伤。
	dedupeKey := buildPageViewDedupKey(event)
	deduped := false
	if redis.Redis != nil && redis.Redis.Client != nil {
		// SETNX + TTL：先占坑，窗口内第二次命中会直接返回，不进数据库。
		ok, err := redis.Redis.Client.SetNX(c.Request.Context(), dedupeKey, "1", pageViewDedupWindow).Result()
		if err != nil {
			response.Abort500(c, "page view dedupe failed: "+err.Error())
			return
		}
		if !ok {
			deduped = true
			response.JSON(c, gin.H{
				"accepted": true,
				"deduped":  true,
			})
			return
		}
	}

	// 埋点直接写库，避免额外的异步队列把一次 view 拆成两套状态机。
	// 如果写库失败，回滚 Redis key，避免“没写成功但被当成已上报”。
	if err := database.DB.WithContext(c.Request.Context()).Create(&event).Error; err != nil {
		if redis.Redis != nil && redis.Redis.Client != nil {
			_ = redis.Redis.Client.Del(c.Request.Context(), dedupeKey).Err()
		}
		response.Abort500(c, "create page view event failed: "+err.Error())
		return
	}

	response.JSON(c, gin.H{
		"accepted": true,
		"deduped":  deduped,
		"event_id": event.ID,
	})
}

func buildPageViewDedupKey(event analyticsModel.PageViewEvent) string {
	var entityPart string
	if event.PageEntityID != nil {
		entityPart = fmt.Sprintf("%d", *event.PageEntityID)
	}

	// 用稳定字段做 fingerprint，避免把同一会话里同一页面的重复挂载写成多条。
	sum := sha256.Sum256([]byte(strings.Join([]string{
		event.VisitorKey,
		event.SessionKey,
		fmt.Sprintf("%d", event.PageType),
		entityPart,
		strings.TrimSpace(event.PagePath),
	}, "|")))
	return "page-view-events:" + hex.EncodeToString(sum[:])
}

func normalizeClientHints(value []byte) []byte {
	if len(value) == 0 {
		return nil
	}
	return value
}

func optionalString(value string) *string {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return nil
	}
	return &trimmed
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if trimmed := strings.TrimSpace(value); trimmed != "" {
			return trimmed
		}
	}
	return ""
}

func isUUIDLike(value string) bool {
	if len(value) != 36 {
		return false
	}

	for index, r := range value {
		switch index {
		case 8, 13, 18, 23:
			if r != '-' {
				return false
			}
		default:
			if !((r >= '0' && r <= '9') || (r >= 'a' && r <= 'f') || (r >= 'A' && r <= 'F')) {
				return false
			}
		}
	}

	return true
}
