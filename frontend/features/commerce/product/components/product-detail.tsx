import Link from "next/link";

import { Card, CardContent } from "@/shared/ui/card";
import styles from "@/features/commerce/product/components/product-detail.module.css";
import type { PodcastScriptListItem, ProductDetail, ProductListItem, ProductSKUItem } from "@/shared/types/public";

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

function pickDefaultSKU(skus?: ProductSKUItem[]) {
  if (!skus?.length) {
    return undefined;
  }
  return skus.find((sku) => sku.is_default) || skus[0];
}

function productInfo(product: ProductDetail) {
  const defaultSKU = pickDefaultSKU(product.skus);
  const currentPrice = defaultSKU?.price ?? product.min_price ?? product.max_price;
  const oldPrice = defaultSKU?.original_price ?? product.max_price;
  const hasDiscount = typeof currentPrice === "number" && typeof oldPrice === "number" && oldPrice > currentPrice;
  const discountPercent = hasDiscount
    ? Math.round(((oldPrice - currentPrice) / oldPrice) * 100)
    : undefined;

  return {
    defaultSKU,
    currentPrice,
    oldPrice: hasDiscount ? oldPrice : undefined,
    discountPercent,
  };
}

function metadataEntries(metadata?: Record<string, unknown>) {
  if (!metadata || typeof metadata !== "object") {
    return [];
  }
  return Object.entries(metadata)
    .filter((entry) => entry[1] !== null && entry[1] !== "")
    .slice(0, 6);
}

function normalizedText(value: unknown) {
  if (typeof value === "string") {
    return value;
  }
  if (typeof value === "number" || typeof value === "boolean") {
    return String(value);
  }
  return JSON.stringify(value);
}

export default function ProductDetailPageView({
  locale,
  product,
  recommendProducts,
  recommendedPodcasts,
}: {
  locale: "zh" | "ja";
  product: ProductDetail;
  recommendProducts: ProductListItem[];
  recommendedPodcasts: PodcastScriptListItem[];
}) {
  const info = productInfo(product);
  const metaItems = metadataEntries(product.metadata);
  const featureItems =
    metaItems.length > 0
      ? metaItems.map(([key, value]) => `${key}: ${normalizedText(value)}`)
      : [
          `Type: ${product.product_type || "General"}`,
          `Status: ${product.status || "active"}`,
          "Shipping: Standard delivery available",
          "Material: Please see full specification",
        ];
  const skuPills = (product.skus || []).slice(0, 4);
  const productThumbs = [product, ...recommendProducts].slice(0, 4);
  const stripProducts = recommendProducts.slice(0, 3);
  const stripPodcasts = recommendedPodcasts.slice(0, 3);
  const stockQty = info.defaultSKU?.stock_qty;
  const isInStock = typeof stockQty !== "number" || stockQty > 0;

  return (
    <section className={styles.page}>
      <div className={styles.topGrid}>
        <Card className={styles.galleryCard}>
          <div className={styles.mainImageWrap}>
            <div className={styles.mainImageInner}>
              {product.cover_image_url ? (
                <img alt={product.name} className={styles.mainImage} src={product.cover_image_url} />
              ) : (
                <div className={styles.mainImageFallback}>No image</div>
              )}
            </div>
          </div>

          <div className={styles.thumbRail}>
            {productThumbs.map((item, index) => (
              <Link
                className={`${styles.thumbItem} ${index === 0 ? styles.thumbItemActive : ""}`}
                href={`/product/${locale}/${item.slug}`}
                key={`${item.slug}-${index}`}
              >
                {item.cover_image_url ? (
                  <img alt={item.name} className={styles.thumbImage} loading="lazy" src={item.cover_image_url} />
                ) : (
                  <span className={styles.thumbFallback}>No image</span>
                )}
              </Link>
            ))}
          </div>
        </Card>

        <Card className={styles.detailCard}>
          <CardContent className="p-7 md:p-8">
            <h1 className={styles.productName}>{product.name}</h1>

            <div className={styles.ratingRow}>
              <span className={styles.stars}>★★★★☆</span>
              <span className={styles.ratingText}>992 Ratings</span>
            </div>

            <div className={styles.priceHeader}>
              <div>
                <div className={styles.priceRow}>
                  <span className={styles.currentPrice}>{formatMoney(info.currentPrice, product.currency)}</span>
                  {typeof info.oldPrice === "number" ? <span className={styles.oldPrice}>{formatMoney(info.oldPrice, product.currency)}</span> : null}
                  {typeof info.discountPercent === "number" ? <span className={styles.discount}>-{info.discountPercent}%</span> : null}
                </div>
                {typeof info.oldPrice === "number" ? (
                  <p className={styles.mrp}>M.R.P.: {formatMoney(info.oldPrice, product.currency)}</p>
                ) : null}
              </div>

              <div className={styles.stockBox}>
                <p className={styles.skuLine}>SKU#: {info.defaultSKU?.sku_code || product.product_code}</p>
                <p className={isInStock ? styles.stockOk : styles.stockLow}>{isInStock ? "IN STOCK" : "LOW STOCK"}</p>
              </div>
            </div>

            <p className={styles.desc}>
              {product.description?.trim() || "This product description will be updated after content enrichment."}
            </p>

            <ul className={styles.featureList}>
              {featureItems.map((item) => (
                <li key={item}>{item}</li>
              ))}
            </ul>

            {skuPills.length > 0 ? (
              <>
                <p className={styles.pillTitle}>WEIGHT</p>
                <div className={styles.pills}>
                  {skuPills.map((sku, index) => (
                    <button
                      className={`${styles.pill} ${index === 0 ? styles.pillActive : ""}`}
                      key={sku.id}
                      type="button"
                    >
                      {sku.name}
                    </button>
                  ))}
                </div>
              </>
            ) : null}

            <div className={styles.buyRow}>
              <div className={styles.qtyBox}>
                <button type="button">-</button>
                <span>1</span>
                <button type="button">+</button>
              </div>

              <button className={styles.cartButton} type="button">ADD TO CART</button>
              <button className={styles.iconButton} type="button">♡</button>
              <Link className={styles.iconButton} href={`/product/${locale}`}>↩</Link>
            </div>
          </CardContent>
        </Card>
      </div>

      {stripProducts.length > 0 ? (
        <Card className={styles.recommendStrip}>
          <CardContent className="p-4 md:p-5">
            <div className={styles.stripGrid}>
              {stripProducts.map((item) => (
                <Link className={styles.stripItem} href={`/product/${locale}/${item.slug}`} key={`rp-${item.id}`}>
                  <div className={styles.stripThumb}>
                    {item.cover_image_url ? (
                      <img alt={item.name} className={styles.stripThumbImage} loading="lazy" src={item.cover_image_url} />
                    ) : (
                      <span className={styles.stripThumbFallback}>No image</span>
                    )}
                  </div>
                  <div className="min-w-0">
                    <p className={styles.stripName}>{item.name}</p>
                    <p className={styles.stripStars}>★★★☆☆</p>
                    <p className={styles.stripPrice}>{formatMoney(item.min_price ?? item.max_price, item.currency)}</p>
                  </div>
                </Link>
              ))}
            </div>
          </CardContent>
        </Card>
      ) : null}

      {stripPodcasts.length > 0 ? (
        <Card className={styles.podcastStrip}>
          <CardContent className="p-5 md:p-6">
            <h3 className={styles.podcastStripTitle}>YouTube Podcast Recommendations</h3>
            <div className={styles.podcastGrid}>
              {stripPodcasts.map((item) => {
                const cover = resolvePodcastCoverURL(item);
                return (
                  <Link className={styles.podcastItem} href={`/podcast/scripts/${item.slug}`} key={`pod-${item.id}`}>
                    <div className={styles.podcastThumb}>
                      {cover ? (
                        <img alt={item.title} className={styles.podcastThumbImage} loading="lazy" src={cover} />
                      ) : (
                        <span className={styles.podcastThumbFallback}>No image</span>
                      )}
                    </div>
                    <div className="min-w-0">
                      <p className={styles.podcastTitle}>{item.title}</p>
                      <p className={styles.podcastMeta}>{formatPublishedAt(item.published_at)}</p>
                    </div>
                  </Link>
                );
              })}
            </div>
          </CardContent>
        </Card>
      ) : null}
    </section>
  );
}
