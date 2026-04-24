import type { Metadata } from "next";
import { notFound } from "next/navigation";

import PageViewTracker from "@/features/analytics/page-view-tracker";
import ProductDetailPageView from "@/features/commerce/product/components/product-detail";
import { getLocalizedProductDetail } from "@/features/commerce/product/api.server";
import { PAGE_VIEW_PAGE_TYPE } from "@/features/analytics/page-view.client";

export const dynamic = "force-dynamic";
export const revalidate = 0;

type Locale = "zh" | "ja";

type Props = {
  params: Promise<{
    locale: string;
    slug: string;
  }>;
};

function normalizeLocale(raw: string): Locale | null {
  const value = raw.trim().toLowerCase();
  if (value === "zh" || value === "ja") {
    return value;
  }
  return null;
}

export async function generateMetadata({ params }: Props): Promise<Metadata> {
  const { locale, slug } = await params;
  const normalizedLocale = normalizeLocale(locale);
  if (!normalizedLocale) {
    return {
      title: "Product Detail",
    };
  }

  const payload = await getLocalizedProductDetail(normalizedLocale, slug).catch(() => null);
  if (!payload?.product) {
    return {
      title: "Product Not Found",
    };
  }

  return {
    // 产品详情页同样优先使用显式 SEO 字段，canonical 也由内容侧控制。
    title: payload.product.seo_title || payload.product.name,
    description: payload.product.seo_description || payload.product.description || payload.product.name,
    keywords: payload.product.seo_keywords,
    alternates: payload.product.canonical_url
      ? {
          canonical: payload.product.canonical_url,
        }
      : undefined,
    openGraph: {
      title: payload.product.seo_title || payload.product.name,
      description: payload.product.seo_description || payload.product.description || payload.product.name,
      type: "website",
      url: payload.product.canonical_url,
      images: payload.product.cover_image_url ? [payload.product.cover_image_url] : undefined,
    },
    twitter: {
      card: "summary_large_image",
      title: payload.product.seo_title || payload.product.name,
      description: payload.product.seo_description || payload.product.description || payload.product.name,
      images: payload.product.cover_image_url ? [payload.product.cover_image_url] : undefined,
    },
  };
}

export default async function LocalizedProductDetailPage({ params }: Props) {
  const { locale, slug } = await params;
  const normalizedLocale = normalizeLocale(locale);
  if (!normalizedLocale) {
    notFound();
  }

  const payload = await getLocalizedProductDetail(normalizedLocale, slug).catch(() => null);
  if (!payload?.product) {
    notFound();
  }

  return (
    <main className="page-shell">
      {/* 产品详情页按产品实体上报。 */}
      <PageViewTracker pageEntityId={payload.product.id} pageType={PAGE_VIEW_PAGE_TYPE.PRODUCT_DETAIL} />
      <ProductDetailPageView
        locale={normalizedLocale}
        product={payload.product}
        recommendProducts={payload.recommend_products}
        recommendedPodcasts={payload.recommended_podcasts}
      />
    </main>
  );
}
