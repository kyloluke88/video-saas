import type { ReactNode } from "react";
import Link from "next/link";
import { Download, ExternalLink, FileText, Package2, PlayCircle, ScrollText } from "lucide-react";

import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Separator } from "@/components/ui/separator";
import type { ConversationRuby, PodcastScriptPage } from "@/types/public";

function renderTextWithRuby(text: string, rubyTokens?: ConversationRuby[]) {
  if (!rubyTokens?.length) {
    return text;
  }

  const content: ReactNode[] = [];
  let cursor = 0;

  rubyTokens.forEach((token, index) => {
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
      <ruby key={`${token.surface}-${token.reading}-${index}`} className="font-medium text-foreground">
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

export default function PodcastScriptPageView({ page }: { page: PodcastScriptPage }) {
  const sections = page.script.sections ?? [];
  const downloads = page.downloads ?? [];
  const vocabulary = page.vocabulary ?? [];
  const grammar = page.grammar ?? [];
  const products = page.sidebar?.products ?? [];
  const youtubeVideoId = extractYouTubeVideoId(page);
  const youtubeEmbedUrl = youtubeVideoId ? `https://www.youtube.com/embed/${youtubeVideoId}` : null;

  return (
    <div className="grid gap-6 lg:grid-cols-[minmax(0,1fr)_320px]">
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

          {page.youtube_video_url ? (
            <div className="mt-4">
              <Button asChild variant="outline">
                <Link href={page.youtube_video_url}>
                  <ExternalLink className="mr-2 h-4 w-4" />
                  在 YouTube 打开
                </Link>
              </Button>
            </div>
          ) : null}
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

          {page.summary ? (
            <p className="mt-5 max-w-3xl text-base leading-8 text-muted-foreground md:text-lg">
              {page.summary}
            </p>
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

        <section className="glass-panel p-6 md:p-8" id="script">
          <div className="flex items-center gap-3">
            <ScrollText className="h-5 w-5 text-primary" />
            <h2 className="section-title text-3xl">聊天脚本</h2>
          </div>
          {page.script.intro ? (
            <p className="mt-4 max-w-3xl text-base leading-8 text-muted-foreground">{page.script.intro}</p>
          ) : null}

          <div className="mt-8 space-y-8">
            {sections.map((section, sectionIndex) => (
              <article key={`${section.heading || "section"}-${sectionIndex}`}>
                {section.heading ? (
                  <h3 className="font-display text-2xl font-semibold tracking-tight">{section.heading}</h3>
                ) : null}
                {section.body ? (
                  <p className="mt-3 text-sm leading-7 text-muted-foreground md:text-base">{section.body}</p>
                ) : null}

                <div className="mt-5 space-y-4">
                  {section.lines.map((line, lineIndex) => (
                    <div
                      className="rounded-[1.75rem] border border-border/60 bg-background/85 p-5"
                      key={`${line.speaker}-${lineIndex}`}
                    >
                      <div className="flex flex-wrap items-center gap-3">
                        <span className="rounded-full bg-primary px-3 py-1 text-xs font-semibold uppercase tracking-[0.2em] text-primary-foreground">
                          {line.speaker_name || line.speaker}
                        </span>
                        {line.note ? (
                          <span className="text-xs uppercase tracking-[0.2em] text-muted-foreground">
                            {line.note}
                          </span>
                        ) : null}
                      </div>
                      <p className="mt-4 text-lg leading-9 text-foreground md:text-xl">
                        {renderTextWithRuby(line.text, line.ruby)}
                      </p>
                      {line.translation ? (
                        <p className="mt-3 text-sm leading-7 text-muted-foreground md:text-base">
                          {line.translation}
                        </p>
                      ) : null}
                    </div>
                  ))}
                </div>
              </article>
            ))}
          </div>
        </section>

        <section className="glass-panel p-6 md:p-8" id="vocabulary">
          <div className="flex items-center gap-3">
            <FileText className="h-5 w-5 text-primary" />
            <h2 className="section-title text-3xl">词语总结</h2>
          </div>

          {vocabulary.length === 0 ? (
            <p className="mt-4 text-base leading-8 text-muted-foreground">暂时还没有词语总结。</p>
          ) : (
            <div className="mt-6 grid gap-4 md:grid-cols-2">
              {vocabulary.map((item, index) => (
                <Card key={`${item.term}-${index}`}>
                  <CardHeader>
                    <CardTitle className="flex flex-wrap items-end gap-3 text-2xl">
                      <span>{item.term}</span>
                      {item.pinyin ? (
                        <span className="text-sm font-medium text-primary/80">{item.pinyin}</span>
                      ) : null}
                    </CardTitle>
                  </CardHeader>
                  <CardContent className="space-y-3 text-sm leading-7 text-muted-foreground">
                    <p className="text-foreground">{item.meaning}</p>
                    {item.notes ? <p>{item.notes}</p> : null}
                    {item.example ? (
                      <>
                        <Separator />
                        <div className="space-y-2">
                          <p className="text-foreground">{item.example}</p>
                          {item.example_translation ? <p>{item.example_translation}</p> : null}
                        </div>
                      </>
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
            <div className="mt-6 space-y-4">
              {grammar.map((item, index) => (
                <Card key={`${item.pattern}-${index}`}>
                  <CardHeader>
                    <CardTitle className="text-2xl">{item.pattern}</CardTitle>
                  </CardHeader>
                  <CardContent className="space-y-4 text-sm leading-7 text-muted-foreground">
                    <p>{item.explanation}</p>
                    {item.examples?.length ? (
                      <div className="space-y-3">
                        <Separator />
                        {item.examples.map((example, exampleIndex) => (
                          <div className="rounded-2xl bg-background/80 p-4" key={`${example.zh}-${exampleIndex}`}>
                            <p className="text-foreground">{example.zh}</p>
                            {example.en ? <p className="mt-2">{example.en}</p> : null}
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

      <aside className="space-y-6 lg:sticky lg:top-6 lg:self-start">
        <Card>
          <CardHeader>
            <CardTitle className="flex items-center gap-2 text-xl">
              <Download className="h-5 w-5 text-primary" />
              下载资源
            </CardTitle>
          </CardHeader>
          <CardContent className="space-y-3">
            {downloads.length > 0 ? (
              downloads.map((asset) => (
                <div
                  className="flex items-center justify-between rounded-2xl border border-border/60 bg-background/70 px-4 py-3"
                  key={`${asset.format}-${asset.label}`}
                >
                  <div>
                    <p className="font-medium text-foreground">{asset.label}</p>
                    <p className="text-xs uppercase tracking-[0.2em] text-muted-foreground">{asset.format}</p>
                  </div>
                  {asset.url && asset.ready !== false ? (
                    <Link className="text-primary transition hover:text-primary/80" href={asset.url}>
                      <ExternalLink className="h-4 w-4" />
                    </Link>
                  ) : (
                    <span className="text-xs font-medium text-muted-foreground">待生成</span>
                  )}
                </div>
              ))
            ) : (
              <p className="text-sm leading-7 text-muted-foreground">
                你可以在这里展示 PDF、纯文本、HTML 或 JSON 下载按钮。
              </p>
            )}
          </CardContent>
        </Card>

        <Card>
          <CardHeader>
            <CardTitle className="flex items-center gap-2 text-xl">
              <Package2 className="h-5 w-5 text-primary" />
              推荐商品
            </CardTitle>
          </CardHeader>
          <CardContent className="space-y-4">
            {products.length > 0 ? (
              products.map((product) => (
                <div
                  className="rounded-[1.5rem] border border-border/60 bg-background/75 p-4"
                  key={`${product.id}`}
                >
                  <h3 className="font-medium text-foreground">{product.title}</h3>
                  {product.description ? (
                    <p className="mt-2 text-sm leading-7 text-muted-foreground">{product.description}</p>
                  ) : null}
                  {product.price_label ? (
                    <p className="mt-3 text-sm font-semibold text-primary">{product.price_label}</p>
                  ) : null}
                  {product.href ? (
                    <Button asChild className="mt-4 w-full" variant="outline">
                      <Link href={product.href}>查看商品</Link>
                    </Button>
                  ) : null}
                </div>
              ))
            ) : (
              <div className="rounded-[1.5rem] border border-dashed border-border/70 bg-background/70 p-5 text-sm leading-7 text-muted-foreground">
                商品位已经预留，当前先保持为空。后续你把商品数据接进数据库后，这里会直接渲染推荐列表。
              </div>
            )}
          </CardContent>
        </Card>
      </aside>
    </div>
  );
}
