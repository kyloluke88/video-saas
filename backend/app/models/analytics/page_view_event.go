package analytics

import (
	"encoding/json"
	"time"
)

type PageType int16

const (
	// PageType 的数值要和前端常量保持一致，避免埋点口径漂移。
	// PageTypePodcastScriptDetail = 1: podcast 详情页。
	PageTypePodcastScriptDetail PageType = 1
	// PageTypeProductDetail = 2: product 详情页。
	PageTypeProductDetail PageType = 2
	// PageTypeCollectionPage = 3: 列表页、首页、分类页等聚合页。
	PageTypeCollectionPage PageType = 3
	// PageTypeStaticPage = 4: about/contact/terms 等静态页。
	PageTypeStaticPage PageType = 4
)

func NormalizePageType(value int16) (PageType, bool) {
	pageType := PageType(value)
	switch pageType {
	case PageTypePodcastScriptDetail, PageTypeProductDetail, PageTypeCollectionPage, PageTypeStaticPage:
		return pageType, true
	default:
		return 0, false
	}
}

func (t PageType) String() string {
	switch t {
	case PageTypePodcastScriptDetail:
		return "podcast_script_detail"
	case PageTypeProductDetail:
		return "product_detail"
	case PageTypeCollectionPage:
		return "collection_page"
	case PageTypeStaticPage:
		return "static_page"
	default:
		return "unknown"
	}
}

func (t PageType) RequiresEntityID() bool {
	switch t {
	case PageTypePodcastScriptDetail, PageTypeProductDetail:
		return true
	default:
		return false
	}
}

type PageViewEvent struct {
	ID uint64 `gorm:"column:id;primaryKey;autoIncrement" json:"id,omitempty"`

	// 访客和会话标识都由前端生成；后端只做校验和入库。
	VisitorKey string  `gorm:"column:visitor_key;type:uuid;not null;index:idx_page_view_events_visitor" json:"visitor_key"`
	SessionKey string  `gorm:"column:session_key;type:uuid;not null;index:idx_page_view_events_session" json:"session_key"`
	UserID     *uint64 `gorm:"column:user_id;index:idx_page_view_events_user" json:"user_id,omitempty"`

	// PageType + PageEntityID 组合是主分析维度；列表页/静态页可不填实体 ID。
	PageType     int16     `gorm:"column:page_type;not null;index:idx_page_view_events_page" json:"page_type"`
	PageEntityID *uint64   `gorm:"column:page_entity_id;index:idx_page_view_events_page" json:"page_entity_id,omitempty"`
	PagePath     string    `gorm:"column:page_path;not null" json:"page_path"`
	ViewedAt     time.Time `gorm:"column:viewed_at;not null" json:"viewed_at"`
	Referer      *string   `gorm:"column:referer" json:"referer,omitempty"`
	IP           *string   `gorm:"column:ip;type:inet" json:"ip,omitempty"`
	UserAgent    *string   `gorm:"column:user_agent" json:"user_agent,omitempty"`
	// client_hints 保留前端能直接采到的设备信息，不强依赖后端请求头。
	ClientHints    json.RawMessage `gorm:"column:client_hints;type:jsonb" json:"client_hints,omitempty"`
	AcceptLanguage *string         `gorm:"column:accept_language" json:"accept_language,omitempty"`

	CreatedAt time.Time `gorm:"column:created_at;not null" json:"created_at"`
}

func (PageViewEvent) TableName() string {
	return "page_view_events"
}
