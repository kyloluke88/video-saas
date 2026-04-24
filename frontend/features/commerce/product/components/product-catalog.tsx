import Link from "next/link";

import { Card, CardContent } from "@/shared/ui/card";
import styles from "@/features/commerce/product/components/product-catalog.module.css";
import type { PodcastScriptListItem, ProductListItem } from "@/shared/types/public";

function extractYouTubeVideoID(item: PodcastScriptListItem) {
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

function resolvePodcastCoverURL(item: PodcastScriptListItem) {
  const videoID = extractYouTubeVideoID(item);
  if (videoID) {
    return `https://i.ytimg.com/vi/${videoID}/mqdefault.jpg`;
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

function formatMoney(value: number | undefined, currency = "USD") {
  if (typeof value !== "number") {
    return "";
  }
  return new Intl.NumberFormat("en-US", {
    style: "currency",
    currency,
    maximumFractionDigits: 2,
  }).format(value);
}

function localeTitle(locale: "zh" | "ja") {
  return locale === "zh" ? "Chinese Product Catalog" : "Japanese Product Catalog";
}

function localeCopy(locale: "zh" | "ja") {
  return locale === "zh"
    ? "Browse Chinese products on the left and discover recommended Chinese podcast scripts on the right."
    : "Browse Japanese products on the left and discover recommended Japanese podcast scripts on the right.";
}

export default function ProductCatalog({
  locale,
  products,
  recommendedPodcasts,
}: {
  locale: "zh" | "ja";
  products: ProductListItem[];
  recommendedPodcasts: PodcastScriptListItem[];
}) {
  return (
    <section className={styles.page}>
      <header className={styles.header}>
        <p className={styles.eyebrow}>Product Catalog</p>
        <h1 className={styles.title}>{localeTitle(locale)}</h1>
        <p className={styles.copy}>{localeCopy(locale)}</p>
      </header>

      <div className={styles.layout}>
        <div className={styles.cards}>
          {products.map((product) => {
            const currentPrice = product.min_price ?? product.max_price;
            const oldPrice = typeof product.max_price === "number" && typeof currentPrice === "number" && product.max_price > currentPrice
              ? product.max_price
              : undefined;

            return (
              <Card className={styles.productCard} key={product.id}>
                <Link className={styles.productLink} href={`/product/${locale}/${product.slug}`}>
                  <div className={styles.thumbWrap}>
                    {product.cover_image_url ? (
                      <img alt={product.name} className={styles.thumb} loading="lazy" src={product.cover_image_url} />
                    ) : (
                      <div className={styles.thumbFallback}>No Image</div>
                    )}
                  </div>

                  <div className={styles.body}>
                    <p className={styles.category}>{product.product_type || "General"}</p>
                    <h2 className={styles.name}>{product.name}</h2>
                    <p className={styles.desc}>{product.description?.trim() || "No product description yet."}</p>

                    <div className={styles.priceRow}>
                      <span className={styles.currentPrice}>{formatMoney(currentPrice, product.currency)}</span>
                      {typeof oldPrice === "number" ? <span className={styles.oldPrice}>{formatMoney(oldPrice, product.currency)}</span> : null}
                    </div>
                  </div>
                </Link>
              </Card>
            );
          })}
        </div>

        <aside className={styles.sidebar}>
          <Card className={styles.sidebarCard}>
            <CardContent className="p-6">
              <h3 className={styles.sidebarTitle}>Recommended Podcasts</h3>
              <div className="mt-4 space-y-1">
                {recommendedPodcasts.map((item) => {
                  const cover = resolvePodcastCoverURL(item);
                  return (
                    <Link className={styles.podcastItem} href={`/podcast/scripts/${item.slug}`} key={`podcast-${item.id}`}>
                      <div className={styles.podcastRow}>
                        <div className={styles.podcastThumb}>
                          {cover ? (
                            <img alt={item.title} className={styles.podcastThumbImage} loading="lazy" src={cover} />
                          ) : (
                            <div className={styles.podcastThumbFallback}>No image</div>
                          )}
                        </div>
                        <div className="min-w-0">
                          <p className={styles.podcastTitle}>{item.title}</p>
                          <p className={styles.podcastMeta}>{formatPublishedAt(item.published_at)}</p>
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
