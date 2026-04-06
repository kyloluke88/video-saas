export interface ConversationRuby {
  surface: string;
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
  body?: string;
  lines: PodcastScriptLine[];
}

export interface VocabularyItem {
  term: string;
  pinyin?: string;
  meaning: string;
  notes?: string;
  example?: string;
  example_translation?: string;
}

export interface GrammarItem {
  pattern: string;
  explanation: string;
  examples?: Array<{
    zh: string;
    en?: string;
  }>;
}

export interface DownloadAsset {
  label: string;
  format: "pdf" | "html" | "txt" | "json" | string;
  url?: string;
  ready?: boolean;
}

export interface SidebarProduct {
  id: number | string;
  title: string;
  description?: string;
  image_url?: string;
  price_label?: string;
  href?: string;
}

export interface PodcastScriptPage {
  id: number;
  slug: string;
  title: string;
  subtitle?: string;
  summary?: string;
  language: string;
  audience_language?: string;
  project_id?: string;
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
    intro?: string;
    sections: PodcastScriptSection[];
  };
  vocabulary?: VocabularyItem[];
  grammar?: GrammarItem[];
  sidebar?: {
    products?: SidebarProduct[];
  };
}
