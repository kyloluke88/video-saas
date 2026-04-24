import { normalizeBaseUrl } from "@/shared/lib/url";
import type { PodcastScriptListItem, ProductDetail, ProductListItem } from "@/shared/types/public";

const DEFAULT_SERVER_API_BASE_URL =
  process.env.API_BASE_URL || process.env.NEXT_PUBLIC_API_BASE_URL || "http://localhost:8080";

export async function getLocalizedProductList(locale: "zh" | "ja", limit = 20): Promise<{
  locale: "zh" | "ja";
  products: ProductListItem[];
  recommended_podcasts: PodcastScriptListItem[];
}> {
  const query = new URLSearchParams({
    limit: String(limit),
  });
  const response = await fetch(
    `${normalizeBaseUrl(DEFAULT_SERVER_API_BASE_URL)}/api/public/products/${encodeURIComponent(locale)}?${query.toString()}`,
    { cache: "no-store" },
  );

  if (!response.ok) {
    throw new Error(`Product list request failed: ${response.status} ${response.statusText}`);
  }

  const payload = (await response.json()) as {
    locale?: "zh" | "ja";
    products?: ProductListItem[];
    recommended_podcasts?: PodcastScriptListItem[];
  };

  return {
    locale: payload.locale || locale,
    products: payload.products ?? [],
    recommended_podcasts: payload.recommended_podcasts ?? [],
  };
}

export async function getLocalizedProductDetail(locale: "zh" | "ja", slug: string): Promise<{
  locale: "zh" | "ja";
  product: ProductDetail;
  recommend_products: ProductListItem[];
  recommended_podcasts: PodcastScriptListItem[];
} | null> {
  const response = await fetch(
    `${normalizeBaseUrl(DEFAULT_SERVER_API_BASE_URL)}/api/public/products/${encodeURIComponent(locale)}/${encodeURIComponent(slug.trim())}`,
    { cache: "no-store" },
  );

  if (response.status === 404) {
    return null;
  }
  if (!response.ok) {
    throw new Error(`Product detail request failed: ${response.status} ${response.statusText}`);
  }

  const payload = (await response.json()) as {
    locale?: "zh" | "ja";
    product?: ProductDetail;
    recommend_products?: ProductListItem[];
    recommended_podcasts?: PodcastScriptListItem[];
  };
  if (!payload.product) {
    return null;
  }

  return {
    locale: payload.locale || locale,
    product: payload.product,
    recommend_products: payload.recommend_products ?? [],
    recommended_podcasts: payload.recommended_podcasts ?? [],
  };
}
