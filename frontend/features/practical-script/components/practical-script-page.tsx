import type { ReactNode } from "react";
import Link from "next/link";
import { ChevronDown, Download, FileText, PlayCircle, ScrollText } from "lucide-react";

import { Badge } from "@/shared/ui/badge";
import { Button } from "@/shared/ui/button";
import { Card, CardContent, CardHeader, CardTitle } from "@/shared/ui/card";
import { Separator } from "@/shared/ui/separator";
import type {
  DownloadAsset,
  GrammarItem,
  ConversationRuby,
  PhoneticToken,
  PracticalScriptBlock,
  PracticalScriptListItem,
  PracticalScriptPage,
  PracticalScriptSpeaker,
  PracticalScriptTurn,
  VocabularyItem,
} from "@/shared/types/public";
import styles from "@/features/podcast-script/components/podcast-script-page.module.css";

type ScriptPageCopy = {
  badge: string;
  video: string;
  videoUnavailable: string;
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
  sidebarRecentTitle: string;
  noRecent: string;
};

const SCRIPT_PAGE_COPY: Record<string, ScriptPageCopy> = {
  zh: {
    badge: "实用会话脚本",
    video: "视频",
    videoUnavailable: "这里优先展示 YouTube 视频；当前这条内容还没有绑定视频地址。",
    downloadPending: "下载文件待生成",
    navScript: "脚本",
    navVocabulary: "词语总结",
    navGrammar: "语法总结",
    transcriptEyebrow: "脚本",
    transcriptTitle: "场景脚本",
    vocabularyTitle: "词语总结",
    vocabularyEmpty: "暂时还没有词语总结。",
    grammarTitle: "语法总结",
    grammarEmpty: "暂时还没有语法总结。",
    meaning: "释义",
    explanation: "说明",
    example: "例句",
    sidebarRecentTitle: "Practical 列表",
    noRecent: "暂无其他脚本。",
  },
  ja: {
    badge: "実用会話スクリプト",
    video: "動画",
    videoUnavailable: "ここでは YouTube 動画を優先表示します。現在このコンテンツには動画リンクがまだ設定されていません。",
    downloadPending: "ダウンロードファイルは準備中です",
    navScript: "スクリプト",
    navVocabulary: "単語まとめ",
    navGrammar: "文法まとめ",
    transcriptEyebrow: "スクリプト",
    transcriptTitle: "シーン別スクリプト",
    vocabularyTitle: "単語まとめ",
    vocabularyEmpty: "単語まとめはまだありません。",
    grammarTitle: "文法まとめ",
    grammarEmpty: "文法まとめはまだありません。",
    meaning: "意味",
    explanation: "解説",
    example: "例文",
    sidebarRecentTitle: "Practical 一覧",
    noRecent: "他のスクリプトはまだありません。",
  },
  en: {
    badge: "Practical Script",
    video: "Video",
    videoUnavailable: "YouTube is preferred here, but this page does not have a video URL yet.",
    downloadPending: "Downloads are not ready yet",
    navScript: "Script",
    navVocabulary: "Vocabulary",
    navGrammar: "Grammar",
    transcriptEyebrow: "Script",
    transcriptTitle: "Scenario Script",
    vocabularyTitle: "Vocabulary",
    vocabularyEmpty: "No vocabulary notes yet.",
    grammarTitle: "Grammar",
    grammarEmpty: "No grammar notes yet.",
    meaning: "Meaning",
    explanation: "Explanation",
    example: "Example",
    sidebarRecentTitle: "Recent Scripts",
    noRecent: "No practical scripts found.",
  },
};

function scriptPageCopy(language?: string) {
  const normalized = language?.trim().toLowerCase();
  return SCRIPT_PAGE_COPY[normalized || ""] ?? SCRIPT_PAGE_COPY.en;
}

function summaryParagraphs(summary?: string) {
  return (summary ?? "")
    .split(/\n\s*\n/)
    .map((item) => item.trim())
    .filter(Boolean);
}

function extractYouTubeVideoId(page: PracticalScriptPage) {
  if (page.youtube_video_id?.trim()) {
    return page.youtube_video_id.trim();
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

function extractYouTubeVideoIDFromListItem(item: PracticalScriptListItem) {
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

function resolveCoverURL(item: PracticalScriptListItem) {
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

function tokenText(tokens?: PhoneticToken[]) {
  return (tokens ?? []).map((token) => token.char).join("");
}

function rubyTokensFromPhoneticTokens(tokens?: PhoneticToken[]) {
  return (tokens ?? [])
    .map((token) => ({
      surface: token.char?.trim(),
      reading: token.reading?.trim(),
    }))
    .filter((token) => token.surface && token.reading);
}

function renderTokenizedText(text?: string, tokens?: PhoneticToken[]) {
  const content = text?.trim() || tokenText(tokens);
  if (!content) {
    return null;
  }
  return renderTextWithRuby(content, rubyTokensFromPhoneticTokens(tokens));
}

function speakerNameMap(speakers?: PracticalScriptSpeaker[]) {
  const out = new Map<string, string>();
  for (const speaker of speakers ?? []) {
    const role = speaker.speaker_role?.trim();
    if (!role) {
      continue;
    }
    out.set(role, speaker.name?.trim() || role.replaceAll("_", " "));
  }
  return out;
}

function displaySpeaker(turn: PracticalScriptTurn, block: PracticalScriptBlock) {
  const role = turn.speaker_role?.trim() || turn.speaker_id?.trim() || "speaker";
  const name = speakerNameMap(block.speakers).get(role);
  return name || role.replaceAll("_", " ");
}

const SPEAKER_BADGE_TONES = [
  "border-[#f1ca97] bg-[#f9e1bc] text-[#9c6a28]",
  "border-[#c8cef4] bg-[#d9dcff] text-[#4c689c]",
  "border-[#bfded8] bg-[#d7ece7] text-[#196d73]",
  "border-[#ebc1ca] bg-[#f5d4db] text-[#9d586b]",
];

function stableSpeakerToneIndex(role: string, block: PracticalScriptBlock) {
  const speakerRoles = (block.speakers ?? [])
    .map((speaker) => speaker.speaker_role?.trim())
    .filter((speakerRole): speakerRole is string => Boolean(speakerRole));

  const directIndex = speakerRoles.indexOf(role);
  if (directIndex >= 0) {
    return directIndex % SPEAKER_BADGE_TONES.length;
  }

  let hash = 0;
  for (const char of role) {
    hash = (hash * 31 + char.charCodeAt(0)) >>> 0;
  }
  return hash % SPEAKER_BADGE_TONES.length;
}

function speakerBadgeClassName(turn: PracticalScriptTurn, block: PracticalScriptBlock) {
  const role = turn.speaker_role?.trim() || turn.speaker_id?.trim() || "speaker";
  return SPEAKER_BADGE_TONES[stableSpeakerToneIndex(role, block)];
}

function preferredLocales(page: PracticalScriptPage) {
  const candidates = [
    page.audience_language?.trim(),
    "en",
    "zh-Hans",
    "zh",
    "ja",
    ...(page.translation_locales ?? []),
  ];
  const out: string[] = [];
  const seen = new Set<string>();
  for (const candidate of candidates) {
    if (!candidate || seen.has(candidate)) {
      continue;
    }
    seen.add(candidate);
    out.push(candidate);
  }
  return out;
}

function firstTranslation(record: Record<string, string> | undefined, locales: string[]) {
  if (!record) {
    return "";
  }
  for (const locale of locales) {
    const value = record[locale]?.trim();
    if (value) {
      return value;
    }
  }
  for (const value of Object.values(record)) {
    const trimmed = value?.trim();
    if (trimmed) {
      return trimmed;
    }
  }
  return "";
}

function readyDownloads(downloads?: DownloadAsset[]) {
  return (downloads ?? []).filter((asset) => asset.url && asset.ready !== false);
}

function renderTurnText(turn: PracticalScriptTurn) {
  const content = turn.text?.trim();
  if (!content) {
    return null;
  }
  return renderTextWithRuby(content, rubyTokensFromPhoneticTokens(turn.tokens));
}

export default function PracticalScriptPageView({
  page,
  sidebarPages,
}: {
  page: PracticalScriptPage;
  sidebarPages: PracticalScriptListItem[];
}) {
  const copy = scriptPageCopy(page.language);
  const blocks = page.script.blocks ?? [];
  const downloads = readyDownloads(page.downloads);
  const vocabulary = page.vocabulary ?? [];
  const grammar = page.grammar ?? [];
  const summaryBlocks = summaryParagraphs(page.summary);
  const translationLocales = preferredLocales(page);
  const youtubeVideoId = extractYouTubeVideoId(page);
  const youtubeEmbedUrl = youtubeVideoId ? `https://www.youtube.com/embed/${youtubeVideoId}` : null;
  const difficulty = page.script.difficulty_level?.trim();

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
            <video className="w-full rounded-[1.75rem] border border-border/60 bg-black/90" controls preload="metadata" src={page.video_url} />
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
            {difficulty ? <Badge variant="secondary">{difficulty}</Badge> : null}
          </div>

          <h1 className="mt-5 font-display text-5xl font-bold leading-none tracking-tight md:text-6xl">{page.title}</h1>

          {page.subtitle ? <p className="mt-4 text-lg leading-8 text-foreground/80">{page.subtitle}</p> : null}

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
                <Button asChild key={`${asset.format}-${asset.label}`}>
                  <Link href={asset.url!}>
                    <Download className="mr-2 h-4 w-4" />
                    {asset.label || asset.format.toUpperCase()}
                  </Link>
                </Button>
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
              {blocks.map((block, blockIndex) => {
                const blockTranslation = firstTranslation(block.topic_translations, translationLocales);

                return (
                  <article key={block.block_id || `block-${blockIndex}`}>
                    <div className="space-y-2">
                      <p className="text-xs font-semibold uppercase tracking-[0.26em] text-primary/80">Section {blockIndex + 1}</p>
                      <h3 className="font-display text-2xl font-semibold tracking-tight md:text-3xl">{block.topic}</h3>
                      {blockTranslation && blockTranslation !== block.topic ? (
                        <p className="text-base leading-8 text-muted-foreground md:text-lg">{blockTranslation}</p>
                      ) : null}
                    </div>

                    <div className="mt-6 overflow-hidden rounded-[1.8rem] border border-border/60 bg-background/55">
                      {block.chapters.map((chapter, chapterIndex) => {
                        const chapterTranslation = firstTranslation(chapter.scene_translations, translationLocales);

                        return (
                          <section
                            className={`${chapterIndex > 0 ? "border-t border-border/60" : ""} p-5 md:p-6`}
                            key={chapter.chapter_id || `chapter-${chapterIndex}`}
                          >
                            <div className="mb-5 border-b border-border/50 pb-4">
                              <p className="text-xs font-semibold uppercase tracking-[0.2em] text-primary/75">Chapter {chapterIndex + 1}</p>
                              {chapter.scene ? <h4 className="mt-2 text-2xl font-semibold tracking-tight">{chapter.scene}</h4> : null}
                              {chapterTranslation && chapterTranslation !== chapter.scene ? (
                                <p className="mt-2 text-base leading-8 text-muted-foreground">{chapterTranslation}</p>
                              ) : null}
                            </div>

                            <div className="space-y-6">
                              {chapter.turns.map((turn, turnIndex) => {
                                const translation = firstTranslation(turn.translations, translationLocales);
                                return (
                                  <div className="transcript-line" key={turn.turn_id || `${chapter.chapter_id}-${turnIndex}`}>
                                    <div className="grid grid-cols-[auto_minmax(0,1fr)] items-start gap-x-3 gap-y-1.5">
                                      <div
                                        className={`inline-flex min-h-11 items-center justify-center whitespace-nowrap rounded-[1.2rem] border px-3 py-1.5 text-base font-semibold leading-none shadow-[inset_0_1px_0_rgba(255,255,255,0.42)] md:min-h-12 md:px-4 md:py-1.5 md:text-[1.15rem] ${speakerBadgeClassName(turn, block)}`}
                                      >
                                        {displaySpeaker(turn, block)}：
                                      </div>

                                      <div className="space-y-1.5 pt-1">
                                        <p className={`${styles.transcriptLineText} text-[1.2rem] leading-[2.2] text-foreground`}>{renderTurnText(turn)}</p>
                                        {translation ? (
                                          <div className="rounded-[1.35rem] border border-secondary/60 bg-secondary/45 px-3.5 py-1.5 md:px-4 md:py-2">
                                            <p className="text-[0.98rem] leading-6 text-muted-foreground md:text-base md:leading-7">{translation}</p>
                                          </div>
                                        ) : null}
                                      </div>
                                    </div>
                                  </div>
                                );
                              })}
                            </div>
                          </section>
                        );
                      })}
                    </div>
                  </article>
                );
              })}
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
                      <div className="text-[2rem] text-foreground">{renderTokenizedText(item.term, item.tokens)}</div>
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
                                <span className="font-semibold">{copy.example}:</span> {renderTokenizedText(example.text, example.tokens)}
                              </p>
                              {example.translation ? <p className="mt-3 text-base leading-8 text-muted-foreground">{example.translation}</p> : null}
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
                      <div className="text-[2rem] leading-[2.05] text-foreground md:text-[2.15rem]">{renderTokenizedText(item.pattern, item.tokens)}</div>
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
                              <span className="font-semibold">{copy.example}:</span> {renderTokenizedText(example.text, example.tokens)}
                            </p>
                            {example.translation ? <p className="mt-3 text-base leading-8 text-muted-foreground">{example.translation}</p> : null}
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
        <Card className={`${styles.sidebarCard} ${styles.sidebarRecentCard}`}>
          <CardContent className={styles.sidebarCardContent}>
            <h3 className={styles.sidebarTitle}>{copy.sidebarRecentTitle}</h3>
            <div className={styles.sidebarRecentList}>
              {sidebarPages.length > 0 ? (
                sidebarPages.map((item) => {
                  const coverURL = resolveCoverURL(item);
                  return (
                    <Link className={styles.recentItem} href={`/practical/scripts/${item.slug}`} key={`recent-${item.id}`}>
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
                <p className={styles.recentEmpty}>{copy.noRecent}</p>
              )}
            </div>
          </CardContent>
        </Card>
      </aside>
    </div>
  );
}
