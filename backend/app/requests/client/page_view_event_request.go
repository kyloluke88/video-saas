package client

import "encoding/json"

// PageViewEventRequest 只承载前端能直接采集或推断的字段。
// IP、User-Agent 等可信请求信息由后端补齐，避免前端伪造。
type PageViewEventRequest struct {
	VisitorKey     string  `json:"visitor_key" binding:"required,max=64"`
	SessionKey     string  `json:"session_key" binding:"required,max=64"`
	PageType       int16   `json:"page_type" binding:"required,min=1"`
	PageEntityID   *uint64 `json:"page_entity_id,omitempty"`
	PagePath       string  `json:"page_path" binding:"required,max=2048"`
	Referer        string  `json:"referer,omitempty" binding:"omitempty,max=2048"`
	AcceptLanguage string  `json:"accept_language,omitempty" binding:"omitempty,max=255"`
	UserAgent      string  `json:"user_agent,omitempty" binding:"omitempty,max=512"`
	// 这块不做结构化强校验，后端只要求它是可解析的 JSON。
	ClientHints json.RawMessage `json:"client_hints,omitempty"`
}
