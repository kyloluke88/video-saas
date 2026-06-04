import Link from "next/link";
import { Search } from "lucide-react";

import { Card, CardContent } from "@/shared/ui/card";
import styles from "@/features/podcast-script/components/podcast-script-list.module.css";
import type { PracticalScriptListItem } from "@/shared/types/public";

function extractYouTubeVideoID(item: PracticalScriptListItem) {
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
  const videoID = extractYouTubeVideoID(item);
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

type PracticalScriptListProps = {
  items: PracticalScriptListItem[];
  heading?: string;
  copy?: string;
};

export default function PracticalScriptList({
  items,
  heading = "Practical Scripts",
  copy = "Browse all published practical conversation scripts. Click any card to open the full transcript page.",
}: PracticalScriptListProps) {
  const recentItems = items.slice(0, 6);

  if (!items.length) {
    return (
      <section className={styles.page}>
        <header className={styles.pageHeader}>
          <p className={styles.headerEyebrow}>Practical Archive</p>
          <h1 className={styles.headerTitle}>{heading}</h1>
          <p className={styles.headerCopy}>暂时没有可展示的 practical 内容。</p>
        </header>

        <Card className={styles.emptyCard}>
          <CardContent className="p-0">暂无已发布内容。</CardContent>
        </Card>
      </section>
    );
  }

  return (
    <section className={styles.page}>
      <header className={styles.pageHeader}>
        <p className={styles.headerEyebrow}>Practical Archive</p>
        <h1 className={styles.headerTitle}>{heading}</h1>
        <p className={styles.headerCopy}>{copy}</p>
      </header>

      <div className={styles.layout}>
        <div className={styles.listGrid}>
          {items.map((item) => {
            const coverURL = resolveCoverURL(item);

            return (
              <Card className={styles.blogCard} key={item.id}>
                <Link className={styles.blogLink} href={`/practical/scripts/${item.slug}`}>
                  <div className={styles.coverWrap}>
                    {coverURL ? (
                      <img alt={item.title} className={styles.coverImage} loading="lazy" src={coverURL} />
                    ) : (
                      <div className={styles.coverFallback}>No preview</div>
                    )}
                  </div>

                  <div className={styles.cardBody}>
                    <p className={styles.meta}>{formatPublishedAt(item.published_at)} - Practical</p>
                    <h2 className={styles.title}>{item.title}</h2>
                    {item.en_title ? <p className={styles.enTitle}>{item.en_title}</p> : null}
                    <p className={styles.summary}>{item.summary?.trim() || "No summary available."}</p>
                    <p className={styles.readMore}>Read More »</p>
                  </div>
                </Link>
              </Card>
            );
          })}
        </div>

        <aside className={styles.sidebar}>
          <Card className={styles.sidebarCard}>
            <CardContent className="p-6">
              <h3 className={styles.sidebarTitle}>Search Practical Scripts</h3>
              <label className={styles.searchWrap}>
                <input className={styles.searchInput} placeholder="Search Practical Scripts" readOnly type="text" />
                <Search className={styles.searchIcon} />
              </label>
            </CardContent>
          </Card>

          <Card className={styles.sidebarCard}>
            <CardContent className="p-6">
              <h3 className={styles.sidebarTitle}>Recent Articles</h3>
              <div className="space-y-1">
                {recentItems.map((item) => {
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
                          <p className={styles.recentTitle}>{item.title}</p>
                          <p className={styles.recentMeta}>{formatPublishedAt(item.published_at)}</p>
                        </div>
                      </div>
                    </Link>
                  );
                })}
              </div>
            </CardContent>
          </Card>
        </aside>
      </div>
    </section>
  );
}
