export interface ConversationRuby {
  surface: string;
  reading: string;
}

export interface PhoneticToken {
  char: string;
  reading: string;
}

export interface ConversationSegment {
  segment_id: string;
  display_text: string;
  english?: string;
  ruby?: ConversationRuby[];
}

export interface ConversationTurn {
  turn_id: string;
  role?: string;
  speaker?: string;
  speaker_name?: string;
  segments?: ConversationSegment[];
}

export interface PodcastScriptLine {
  speaker: string;
  speaker_name?: string;
  text: string;
  ruby?: ConversationRuby[];
  translation?: string;
  note?: string;
}

export interface PodcastScriptSection {
  heading?: string;
  lines: PodcastScriptLine[];
}

export interface VocabularyItem {
  term: string;
  tokens?: PhoneticToken[];
  meaning: string;
  explanation: string;
  examples?: Array<{
    text: string;
    tokens?: PhoneticToken[];
    translation?: string;
  }>;
}

export interface GrammarItem {
  pattern: string;
  tokens?: PhoneticToken[];
  meaning: string;
  explanation: string;
  examples?: Array<{
    text: string;
    tokens?: PhoneticToken[];
    translation?: string;
  }>;
}

export interface DownloadAsset {
  label: string;
  format: "pdf" | "html" | "txt" | "json" | string;
  url?: string;
  ready?: boolean;
}

export interface PodcastScriptPage {
  id: number;
  slug: string;
  title: string;
  en_title?: string;
  subtitle?: string;
  summary?: string;
  language: string;
  audience_language?: string;
  project_id: string;
  cover_image_url?: string;
  video_url?: string;
  youtube_video_id?: string;
  youtube_video_url?: string;
  seo_title?: string;
  seo_description?: string;
  seo_keywords?: string[];
  canonical_url?: string;
  published_at?: string;
  downloads?: DownloadAsset[];
  script: {
    sections: PodcastScriptSection[];
  };
  vocabulary?: VocabularyItem[];
  grammar?: GrammarItem[];
}

export interface PodcastScriptListItem {
  id: number;
  slug: string;
  title: string;
  en_title?: string;
  summary?: string;
  youtube_video_id?: string;
  youtube_video_url?: string;
  published_at?: string;
}

export interface ProductListItem {
  id: number;
  slug: string;
  product_code: string;
  name: string;
  locale: "zh" | "ja" | string;
  product_type: string;
  status: string;
  currency: string;
  min_price?: number;
  max_price?: number;
  cover_image_url?: string;
  description?: string;
}

export interface ProductSKUItem {
  id: number;
  sku_code: string;
  name: string;
  price: number;
  original_price?: number;
  currency: string;
  status: string;
  is_default: boolean;
  stock_qty?: number;
}

export interface ProductDetail extends ProductListItem {
  metadata?: Record<string, unknown>;
  // 产品详情页需要的显式 SEO 字段，和后端 `products` 表一一对应。
  seo_title?: string;
  seo_description?: string;
  seo_keywords?: string[];
  canonical_url?: string;
  skus?: ProductSKUItem[];
}
