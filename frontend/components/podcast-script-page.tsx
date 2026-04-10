import type { ReactNode } from "react";
import Link from "next/link";
import { ChevronDown, Download, ExternalLink, FileText, PlayCircle, ScrollText } from "lucide-react";

import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Separator } from "@/components/ui/separator";
import type { ConversationRuby, GrammarItem, PhoneticToken, PodcastScriptPage, VocabularyItem } from "@/types/public";
import styles from "@/components/podcast-script-page.module.css";

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

export default function PodcastScriptPageView({ page }: { page: PodcastScriptPage }) {
  const sections = page.script.sections ?? [];
  const downloads = (page.downloads ?? []).filter((asset) => asset.format.toLowerCase() === "pdf");
  const vocabulary = page.vocabulary ?? [];
  const grammar = page.grammar ?? [];
  const summaryBlocks = summaryParagraphs(page.summary);
  const youtubeVideoId = extractYouTubeVideoId(page);
  const youtubeEmbedUrl = youtubeVideoId ? `https://www.youtube.com/embed/${youtubeVideoId}` : null;

  return (
    <div className="space-y-6">
      <section className="space-y-6">
        <section className="glass-panel overflow-hidden p-4 md:p-6">
          <div className="mb-4 flex items-center gap-3">
            <PlayCircle className="h-5 w-5 text-primary" />
            <h2 className="section-title text-2xl">视频</h2>
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
              这里优先展示 YouTube 视频；当前这条内容还没有绑定 YouTube 地址。
            </div>
          )}

        </section>

        <header className="glass-panel overflow-hidden p-8 md:p-10">
          <div className="flex flex-wrap items-center gap-2">
            <Badge>Podcast Script</Badge>
            <Badge variant="secondary">{page.language}</Badge>
            {page.audience_language ? <Badge variant="secondary">{page.audience_language}</Badge> : null}
          </div>

          <h1 className="mt-5 max-w-4xl font-display text-5xl font-bold leading-none tracking-tight text-balance md:text-6xl">
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
                      {asset.label}
                    </Link>
                  </Button>
                ) : (
                  <Button disabled key={`${asset.format}-${asset.label}`}>
                    <Download className="mr-2 h-4 w-4" />
                    {asset.label}
                  </Button>
                )
              ))
            ) : (
              <Button disabled>
                <Download className="mr-2 h-4 w-4" />
                下载文件待生成
              </Button>
            )}
          </div>
        </header>

        <nav className="glass-panel p-4 md:p-5">
          <div className="flex flex-wrap gap-3 text-sm font-medium text-muted-foreground">
            <Link className="rounded-full bg-secondary px-4 py-2 transition hover:bg-secondary/80" href="#script">
              聊天脚本
            </Link>
            <Link className="rounded-full bg-secondary px-4 py-2 transition hover:bg-secondary/80" href="#vocabulary">
              词语总结
            </Link>
            <Link className="rounded-full bg-secondary px-4 py-2 transition hover:bg-secondary/80" href="#grammar">
              语法总结
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
                <p className="text-xs font-semibold uppercase tracking-[0.26em] text-primary/80">Transcript</p>
                <h2 className="font-display text-3xl font-semibold tracking-tight md:text-4xl">文本 / Transcript</h2>
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
            <h2 className="section-title text-3xl">词语总结</h2>
          </div>

          {vocabulary.length === 0 ? (
            <p className="mt-4 text-base leading-8 text-muted-foreground">暂时还没有词语总结。</p>
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
                      <span className="font-semibold">Meaning:</span> {item.meaning}
                    </p>
                    <p className="text-lg leading-9 text-foreground/85">
                      <span className="font-semibold">Explanation:</span> {item.explanation}
                    </p>
                    {normalizeVocabularyExamples(item).length > 0 ? (
                      <div className="space-y-3">
                        <Separator />
                        <div className="space-y-3">
                          {normalizeVocabularyExamples(item).map((example, exampleIndex) => (
                            <div className="rounded-2xl bg-background/75 p-4" key={`${example.text}-${exampleIndex}`}>
                              <p className="text-lg leading-9 text-foreground">
                                <span className="font-semibold">Example:</span>{" "}
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
            <h2 className="section-title text-3xl">语法总结</h2>
          </div>

          {grammar.length === 0 ? (
            <p className="mt-4 text-base leading-8 text-muted-foreground">暂时还没有语法总结。</p>
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
                      <span className="font-semibold text-foreground">Explanation:</span> {item.explanation}
                    </p>
                    {normalizeGrammarExamples(item).length ? (
                      <div className="space-y-4">
                        <Separator />
                        {normalizeGrammarExamples(item).map((example, exampleIndex) => (
                          <div className="rounded-[1.5rem] border border-border/60 bg-background/75 p-4" key={`${example.text}-${exampleIndex}`}>
                            <p className="text-lg leading-9 text-foreground">
                              <span className="font-semibold">Example:</span>{" "}
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
    </div>
  );
}
