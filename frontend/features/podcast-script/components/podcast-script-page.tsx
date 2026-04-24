import type { ReactNode } from "react";
import Link from "next/link";
import { ChevronDown, Download, FileText, PlayCircle, ScrollText, Search } from "lucide-react";

import { Badge } from "@/shared/ui/badge";
import { Button } from "@/shared/ui/button";
import { Card, CardContent, CardHeader, CardTitle } from "@/shared/ui/card";
import { Separator } from "@/shared/ui/separator";
import type {
  ConversationRuby,
  DownloadAsset,
  GrammarItem,
  PhoneticToken,
  PodcastScriptListItem,
  PodcastScriptPage,
  VocabularyItem,
} from "@/shared/types/public";
import styles from "@/features/podcast-script/components/podcast-script-page.module.css";

type ScriptPageCopy = {
  badge: string;
  video: string;
  videoUnavailable: string;
  downloadPDF: string;
  downloadPending: string;
  navScript: string;
  navVocabulary: string;
  navGrammar: string;
  transcriptEyebrow: string;
  transcriptTitle: string;
  vocabularyTitle: string;
  vocabularyEmpty: string;
  grammarTitle: string;
  grammarEmpty: string;
  meaning: string;
  explanation: string;
  example: string;
  sidebarSearchTitle: string;
  sidebarSearchPlaceholder: string;
  sidebarRecentTitle: string;
};

const SCRIPT_PAGE_COPY: Record<string, ScriptPageCopy> = {
  zh: {
    badge: "播客脚本",
    video: "视频",
    videoUnavailable: "这里优先展示 YouTube 视频；当前这条内容还没有绑定 YouTube 地址。",
    downloadPDF: "下载聊天脚本 PDF",
    downloadPending: "下载文件待生成",
    navScript: "聊天脚本",
    navVocabulary: "词语总结",
    navGrammar: "语法总结",
    transcriptEyebrow: "文本",
    transcriptTitle: "文本",
    vocabularyTitle: "词语总结",
    vocabularyEmpty: "暂时还没有词语总结。",
    grammarTitle: "语法总结",
    grammarEmpty: "暂时还没有语法总结。",
    meaning: "释义",
    explanation: "说明",
    example: "例句",
    sidebarSearchTitle: "搜索播客",
    sidebarSearchPlaceholder: "Search Our Podcast",
    sidebarRecentTitle: "播客列表",
  },
  ja: {
    badge: "ポッドキャストスクリプト",
    video: "動画",
    videoUnavailable: "ここでは YouTube 動画を優先表示します。現在このコンテンツには YouTube のリンクがまだ設定されていません。",
    downloadPDF: "チャットスクリプト PDF をダウンロード",
    downloadPending: "ダウンロードファイルは準備中です",
    navScript: "会話スクリプト",
    navVocabulary: "単語まとめ",
    navGrammar: "文法まとめ",
    transcriptEyebrow: "文字起こし",
    transcriptTitle: "文字起こし",
    vocabularyTitle: "単語まとめ",
    vocabularyEmpty: "単語まとめはまだありません。",
    grammarTitle: "文法まとめ",
    grammarEmpty: "文法まとめはまだありません。",
    meaning: "意味",
    explanation: "解説",
    example: "例文",
    sidebarSearchTitle: "ポッドキャスト検索",
    sidebarSearchPlaceholder: "Search Our Podcast",
    sidebarRecentTitle: "ポッドキャスト一覧",
  },
  en: {
    badge: "Podcast Script",
    video: "Video",
    videoUnavailable: "YouTube is preferred here, but this page does not have a YouTube URL yet.",
    downloadPDF: "Download Chat Script PDF",
    downloadPending: "Download is not ready yet",
    navScript: "Transcript",
    navVocabulary: "Vocabulary",
    navGrammar: "Grammar",
    transcriptEyebrow: "Transcript",
    transcriptTitle: "Transcript",
    vocabularyTitle: "Vocabulary",
    vocabularyEmpty: "No vocabulary notes yet.",
    grammarTitle: "Grammar",
    grammarEmpty: "No grammar notes yet.",
    meaning: "Meaning",
    explanation: "Explanation",
    example: "Example",
    sidebarSearchTitle: "Search Podcast",
    sidebarSearchPlaceholder: "Search Our Podcast",
    sidebarRecentTitle: "Recent Scripts",
  },
};

function scriptPageCopy(language?: string) {
  const normalized = language?.trim().toLowerCase();
  return SCRIPT_PAGE_COPY[normalized || ""] ?? SCRIPT_PAGE_COPY.en;
}

function normalizeRubyTokens(rubyTokens?: ConversationRuby[]) {
  if (!rubyTokens?.length) {
    return [];
  }

  return rubyTokens.flatMap((token) => {
    const surface = token.surface?.trim();
    const reading = token.reading?.trim();

    if (!surface || !reading) {
      return [];
    }

    const chars = Array.from(surface);
    const syllables = reading.split(/\s+/).filter(Boolean);

    // Chinese test data sometimes arrives as phrase-level ruby like "发现 / fā xiàn".
    // If the syllable count matches the Hanzi count, split it to per-character ruby.
    if (chars.length > 1 && syllables.length === chars.length) {
      return chars.map((char, index) => ({
        surface: char,
        reading: syllables[index],
      }));
    }

    return [{ surface, reading }];
  });
}

function renderTextWithRuby(text: string, rubyTokens?: ConversationRuby[]) {
  const normalizedTokens = normalizeRubyTokens(rubyTokens);
  if (!normalizedTokens.length) {
    return text;
  }

  const content: ReactNode[] = [];
  let cursor = 0;

  normalizedTokens.forEach((token, index) => {
    if (!token.surface || !token.reading) {
      return;
    }

    const position = text.indexOf(token.surface, cursor);
    if (position === -1) {
      return;
    }

    if (position > cursor) {
      content.push(text.slice(cursor, position));
    }

    content.push(
      <ruby key={`${token.surface}-${token.reading}-${index}`} className="font-medium text-foreground mx-1.5 mb-1">
        {token.surface}
        <rt>{token.reading}</rt>
      </ruby>,
    );

    cursor = position + token.surface.length;
  });

  if (cursor < text.length) {
    content.push(text.slice(cursor));
  }

  return content;
}

function extractYouTubeVideoId(page: PodcastScriptPage) {
  if (page.youtube_video_id) {
    return page.youtube_video_id;
  }

  const rawURL = page.youtube_video_url?.trim();
  if (!rawURL) {
    return null;
  }

  const patterns = [/[?&]v=([A-Za-z0-9_-]{11})/, /youtu\.be\/([A-Za-z0-9_-]{11})/, /embed\/([A-Za-z0-9_-]{11})/];
  for (const pattern of patterns) {
    const match = rawURL.match(pattern);
    if (match?.[1]) {
      return match[1];
    }
  }

  return null;
}

function extractYouTubeVideoIDFromListItem(item: PodcastScriptListItem) {
  if (item.youtube_video_id?.trim()) {
    return item.youtube_video_id.trim();
  }

  const rawURL = item.youtube_video_url?.trim();
  if (!rawURL) {
    return "";
  }

  const patterns = [/[?&]v=([A-Za-z0-9_-]{11})/, /youtu\.be\/([A-Za-z0-9_-]{11})/, /embed\/([A-Za-z0-9_-]{11})/];
  for (const pattern of patterns) {
    const match = rawURL.match(pattern);
    if (match?.[1]) {
      return match[1];
    }
  }

  return "";
}

function resolveCoverURL(item: PodcastScriptListItem) {
  const videoID = extractYouTubeVideoIDFromListItem(item);
  if (videoID) {
    return `https://i.ytimg.com/vi/${videoID}/mqdefault.jpg`;
  }

  const rawURL = item.youtube_video_url?.trim() || "";
  if (/\.(png|jpe?g|webp|gif)(\?.*)?$/i.test(rawURL)) {
    return rawURL;
  }

  return "";
}

function formatPublishedAt(value?: string) {
  if (!value) {
    return "No date";
  }

  const date = new Date(value);
  if (Number.isNaN(date.getTime())) {
    return value;
  }

  return new Intl.DateTimeFormat("en-US", {
    year: "numeric",
    month: "short",
    day: "2-digit",
  }).format(date);
}

function normalizeVocabularyExamples(item: VocabularyItem) {
  return item.examples ?? [];
}

function normalizeGrammarExamples(item: GrammarItem) {
  return item.examples ?? [];
}

function tokenText(tokens?: PhoneticToken[]) {
  return (tokens ?? []).map((token) => token.char).join("");
}

function exampleTranslation(example: { translation?: string }) {
  return example.translation?.trim() || "";
}

function rubyTokensFromPhoneticTokens(tokens?: PhoneticToken[]) {
  return (tokens ?? [])
    .map((token) => ({
      surface: token.char?.trim(),
      reading: token.reading?.trim(),
    }))
    .filter((token) => token.surface);
}

function renderTokenizedText(text?: string, tokens?: PhoneticToken[]) {
  const content = text?.trim() || tokenText(tokens);
  if (!content) {
    return null;
  }

  return renderTextWithRuby(content, rubyTokensFromPhoneticTokens(tokens));
}

function summaryParagraphs(summary?: string) {
  return (summary ?? "")
    .split(/\n\s*\n/)
    .map((item) => item.trim())
    .filter(Boolean);
}

function downloadLabel(asset: DownloadAsset | undefined, copy: ScriptPageCopy) {
  if (!asset) {
    return copy.downloadPDF;
  }
  if (asset.format?.toLowerCase() === "pdf") {
    return copy.downloadPDF;
  }
  return asset.label || copy.downloadPDF;
}

export default function PodcastScriptPageView({
  page,
  sidebarPages,
}: {
  page: PodcastScriptPage;
  sidebarPages: PodcastScriptListItem[];
}) {

  const copy = scriptPageCopy(page.language);
  const sections = page.script.sections ?? [];
  const downloads = (page.downloads ?? []).filter((asset) => asset.format.toLowerCase() === "pdf");
  const vocabulary = page.vocabulary ?? [];
  const grammar = page.grammar ?? [];
  const summaryBlocks = summaryParagraphs(page.summary);
  const youtubeVideoId = extractYouTubeVideoId(page);
  const youtubeEmbedUrl = youtubeVideoId ? `https://www.youtube.com/embed/${youtubeVideoId}` : null;

  return (
    <div className={styles.pageLayout}>
      <section className={styles.mainColumn}>
        <section className="glass-panel overflow-hidden p-4 md:p-6">
          <div className="mb-4 flex items-center gap-3">
            <PlayCircle className="h-5 w-5 text-primary" />
            <h2 className="section-title text-2xl">{copy.video}</h2>
          </div>

          {youtubeEmbedUrl ? (
            <div className="overflow-hidden rounded-[1.75rem] border border-border/60 bg-background/70">
              <div className="aspect-video w-full">
                <iframe
                  allow="accelerometer; autoplay; clipboard-write; encrypted-media; gyroscope; picture-in-picture; web-share"
                  allowFullScreen
                  className="h-full w-full"
                  referrerPolicy="strict-origin-when-cross-origin"
                  src={youtubeEmbedUrl}
                  title={page.title}
                />
              </div>
            </div>
          ) : page.video_url ? (
            <video
              className="w-full rounded-[1.75rem] border border-border/60 bg-black/90"
              controls
              preload="metadata"
              src={page.video_url}
            />
          ) : (
            <div className="rounded-[1.75rem] border border-dashed border-border/70 bg-background/70 p-6 text-sm leading-7 text-muted-foreground">
              {copy.videoUnavailable}
            </div>
          )}

        </section>

        <header className="glass-panel overflow-hidden p-8 md:p-10">
          <div className="flex flex-wrap items-center gap-2">
            <Badge>{copy.badge}</Badge>
            <Badge variant="secondary">{page.language}</Badge>
            {page.audience_language ? <Badge variant="secondary">{page.audience_language}</Badge> : null}
          </div>

          <h1 className="mt-5 font-display text-5xl font-bold leading-none tracking-tight md:text-6xl">
            {page.title}
          </h1>

          {page.subtitle ? (
            <p className="mt-4 text-lg leading-8 text-foreground/80">{page.subtitle}</p>
          ) : null}

          {summaryBlocks.length > 0 ? (
            <div className="mt-5 space-y-4 text-base leading-8 text-muted-foreground md:text-lg">
              {summaryBlocks.map((paragraph, index) => (
                <p key={`${paragraph.slice(0, 40)}-${index}`}>{paragraph}</p>
              ))}
            </div>
          ) : null}

          <div className="mt-8 flex flex-wrap gap-3">
            {downloads.length > 0 ? (
              downloads.map((asset) => (
                asset.url && asset.ready !== false ? (
                  <Button asChild key={`${asset.format}-${asset.label}`}>
                    <Link href={asset.url}>
                      <Download className="mr-2 h-4 w-4" />
                      {downloadLabel(asset, copy)}
                    </Link>
                  </Button>
                ) : (
                  <Button disabled key={`${asset.format}-${asset.label}`}>
                    <Download className="mr-2 h-4 w-4" />
                    {downloadLabel(asset, copy)}
                  </Button>
                )
              ))
            ) : (
              <Button disabled>
                <Download className="mr-2 h-4 w-4" />
                {copy.downloadPending}
              </Button>
            )}
          </div>
        </header>

        <nav className="glass-panel p-4 md:p-5">
          <div className="flex flex-wrap gap-3 text-sm font-medium text-muted-foreground">
            <Link className="rounded-full bg-secondary px-4 py-2 transition hover:bg-secondary/80" href="#script">
              {copy.navScript}
            </Link>
            <Link className="rounded-full bg-secondary px-4 py-2 transition hover:bg-secondary/80" href="#vocabulary">
              {copy.navVocabulary}
            </Link>
            <Link className="rounded-full bg-secondary px-4 py-2 transition hover:bg-secondary/80" href="#grammar">
              {copy.navGrammar}
            </Link>
          </div>
        </nav>

        <details className={`${styles.transcriptShell} glass-panel overflow-hidden`} id="script" open>
          <summary className={`${styles.transcriptSummary} list-none cursor-pointer select-none px-6 py-5 md:px-8 md:py-6`}>
            <div className="flex items-center gap-4">
              <span className="transcript-summary-icon flex h-11 w-11 items-center justify-center rounded-2xl bg-secondary/70 text-primary">
                <ScrollText className="h-5 w-5" />
              </span>
              <div className="min-w-0 flex-1">
                <p className="text-xs font-semibold uppercase tracking-[0.26em] text-primary/80">{copy.transcriptEyebrow}</p>
                <h2 className="font-display text-3xl font-semibold tracking-tight md:text-4xl">{copy.transcriptTitle}</h2>
              </div>
              <span className={`${styles.transcriptSummaryChevron} flex h-11 w-11 items-center justify-center rounded-2xl border border-border/70 bg-background/75 text-foreground/70`}>
                <ChevronDown className="h-5 w-5" />
              </span>
            </div>
          </summary>

          <div className="border-t border-border/60 px-6 py-7 md:px-8 md:py-8">
            <div className="space-y-9">
              {sections.map((section, sectionIndex) => (
                <article key={`${section.heading || "section"}-${sectionIndex}`}>
                  {section.heading ? (
                    <h3 className="font-display text-2xl font-semibold tracking-tight">{section.heading}</h3>
                  ) : null}

                  <div className="mt-5 space-y-8">
                    {section.lines.map((line, lineIndex) => (
                      <div className="transcript-line" key={`${line.speaker}-${lineIndex}`}>
                        <div className="grid grid-cols-[auto_minmax(0,1fr)] items-start gap-x-3 gap-y-2">
                          <p className="pt-2 text-lg font-semibold tracking-[0.16em] text-primary md:text-xl">
                            {(line.speaker_name || line.speaker) + "："}
                          </p>

                          <div className="space-y-2">
                            <p className={`${styles.transcriptLineText} text-[1.5rem] leading-[2.2] text-foreground`}>
                              {renderTextWithRuby(line.text, line.ruby)}
                            </p>
                            {line.translation ? (
                              <div className="rounded-[1.55rem] border border-secondary/60 bg-secondary/45 px-4 py-2">
                                <p className="text-base leading-8 text-muted-foreground md:text-[1.05rem]">
                                  {line.translation}
                                </p>
                              </div>
                            ) : null}
                          </div>
                        </div>
                      </div>
                    ))}
                  </div>
                </article>
              ))}
            </div>
          </div>
        </details>

        <section className="glass-panel p-6 md:p-8" id="vocabulary">
          <div className="flex items-center gap-3">
            <FileText className="h-5 w-5 text-primary" />
            <h2 className="section-title text-3xl">{copy.vocabularyTitle}</h2>
          </div>

          {vocabulary.length === 0 ? (
            <p className="mt-4 text-base leading-8 text-muted-foreground">{copy.vocabularyEmpty}</p>
          ) : (
            <div className="mt-6 grid gap-5 md:grid-cols-2">
              {vocabulary.map((item, index) => (
                <Card className="overflow-hidden border-border/60 bg-card/95" key={`${item.term}-${index}`}>
                  <CardHeader className="border-b border-border/60 bg-background/55">
                    <CardTitle className="space-y-3 text-2xl">
                      <div className="text-[2rem] text-foreground">
                        {renderTokenizedText(item.term, item.tokens)}
                      </div>
                    </CardTitle>
                  </CardHeader>
                  <CardContent className="space-y-5 text-base leading-8 text-muted-foreground">
                    <p className="text-lg leading-9 text-foreground">
                      <span className="font-semibold">{copy.meaning}:</span> {item.meaning}
                    </p>
                    <p className="text-lg leading-9 text-foreground/85">
                      <span className="font-semibold">{copy.explanation}:</span> {item.explanation}
                    </p>
                    {normalizeVocabularyExamples(item).length > 0 ? (
                      <div className="space-y-3">
                        <Separator />
                        <div className="space-y-3">
                          {normalizeVocabularyExamples(item).map((example, exampleIndex) => (
                            <div className="rounded-2xl bg-background/75 p-4" key={`${example.text}-${exampleIndex}`}>
                              <p className="text-lg leading-9 text-foreground">
                                <span className="font-semibold">{copy.example}:</span>{" "}
                                <span className="align-middle">{renderTokenizedText(example.text, example.tokens)}</span>
                              </p>
                              {exampleTranslation(example) ? (
                                <p className="mt-3 text-base leading-8 text-muted-foreground">{exampleTranslation(example)}</p>
                              ) : null}
                            </div>
                          ))}
                        </div>
                      </div>
                    ) : null}
                  </CardContent>
                </Card>
              ))}
            </div>
          )}
        </section>

        <section className="glass-panel p-6 md:p-8" id="grammar">
          <div className="flex items-center gap-3">
            <FileText className="h-5 w-5 text-primary" />
            <h2 className="section-title text-3xl">{copy.grammarTitle}</h2>
          </div>

          {grammar.length === 0 ? (
            <p className="mt-4 text-base leading-8 text-muted-foreground">{copy.grammarEmpty}</p>
          ) : (
            <div className="mt-6 space-y-5">
              {grammar.map((item, index) => (
                <Card className="overflow-hidden border-border/60 bg-card/95" key={`${item.pattern}-${index}`}>
                  <CardHeader className="border-b border-border/60 bg-background/55">
                    <CardTitle className="space-y-2">
                      <div className="text-[2rem] leading-[2.05] text-foreground md:text-[2.15rem]">
                        {renderTokenizedText(item.pattern, item.tokens)}
                      </div>
                      <p className="text-lg font-medium leading-8 text-foreground/85">{item.meaning}</p>
                    </CardTitle>
                  </CardHeader>
                  <CardContent className="space-y-5 text-base leading-8 text-muted-foreground">
                    <p className="text-lg leading-9 text-foreground/85">
                      <span className="font-semibold text-foreground">{copy.explanation}:</span> {item.explanation}
                    </p>
                    {normalizeGrammarExamples(item).length ? (
                      <div className="space-y-4">
                        <Separator />
                        {normalizeGrammarExamples(item).map((example, exampleIndex) => (
                          <div className="rounded-[1.5rem] border border-border/60 bg-background/75 p-4" key={`${example.text}-${exampleIndex}`}>
                            <p className="text-lg leading-9 text-foreground">
                              <span className="font-semibold">{copy.example}:</span>{" "}
                              <span className="align-middle">{renderTokenizedText(example.text, example.tokens)}</span>
                            </p>
                            {exampleTranslation(example) ? (
                              <p className="mt-3 text-base leading-8 text-muted-foreground">{exampleTranslation(example)}</p>
                            ) : null}
                          </div>
                        ))}
                      </div>
                    ) : null}
                  </CardContent>
                </Card>
              ))}
            </div>
          )}
        </section>
      </section>

      <aside className={styles.sidebar}>
        {/* <Card className={styles.sidebarCard}>
          <CardContent className="p-6">
            <h3 className={styles.sidebarTitle}>{copy.sidebarSearchTitle}</h3>
            <label className={styles.searchWrap}>
              <input
                className={styles.searchInput}
                placeholder={copy.sidebarSearchPlaceholder}
                readOnly
                type="text"
              />
              <Search className={styles.searchIcon} />
            </label>
          </CardContent>
        </Card> */}

        <Card className={`${styles.sidebarCard} ${styles.sidebarRecentCard}`}>
          <CardContent className={styles.sidebarCardScroll}>
            <h3 className={styles.sidebarTitle}>{copy.sidebarRecentTitle}</h3>
            <div className="space-y-1">
              {sidebarPages.length > 0 ? (
                sidebarPages.map((item) => {
                  const coverURL = resolveCoverURL(item);
                  return (
                    <Link className={styles.recentItem} href={`/podcast/scripts/${item.slug}`} key={`recent-${item.id}`}>
                      <div className={styles.recentRow}>
                        <div className={styles.recentThumb}>
                          {coverURL ? (
                            <img alt={item.title} className={styles.recentThumbImage} loading="lazy" src={coverURL} />
                          ) : (
                            <div className={styles.recentThumbFallback}>No image</div>
                          )}
                        </div>
                        <div className="min-w-0">
                          <p className={styles.recentEnTitle}>{item.en_title}</p>
                          <p className={styles.recentTitle}>{item.title}</p>
                          <p className={styles.recentMeta}>{formatPublishedAt(item.published_at)}</p>
                        </div>
                      </div>
                    </Link>
                  );
                })
              ) : (
                <p className={styles.recentEmpty}>No podcast scripts found.</p>
              )}
            </div>
          </CardContent>
        </Card>
      </aside>
    </div>
  );
}
