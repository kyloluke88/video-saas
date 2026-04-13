import type { Metadata } from "next";
import { notFound } from "next/navigation";

import ProductDetailPageView from "@/components/product-detail";
import { getLocalizedProductDetail } from "@/lib/api";

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
    title: payload.product.name,
    description: payload.product.description || payload.product.name,
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
      <ProductDetailPageView
        locale={normalizedLocale}
        product={payload.product}
        recommendProducts={payload.recommend_products}
        recommendedPodcasts={payload.recommended_podcasts}
      />
    </main>
  );
}
