import type { Metadata } from "next";
import { notFound } from "next/navigation";

import ProductCatalog from "@/features/commerce/product/components/product-catalog";
import { getLocalizedProductList } from "@/features/commerce/product/api.server";

export const dynamic = "force-dynamic";
export const revalidate = 0;

type Locale = "zh" | "ja";

type Props = {
  params: Promise<{
    locale: string;
  }>;
};

function normalizeLocale(raw: string): Locale | null {
  const value = raw.trim().toLowerCase();
  if (value === "zh" || value === "ja") {
    return value;
  }
  return null;
}

function metadataTitle(locale: Locale) {
  return locale === "zh" ? "Chinese Product Catalog" : "Japanese Product Catalog";
}

function metadataDescription(locale: Locale) {
  return locale === "zh"
    ? "Chinese product catalog with podcast recommendations, localized browsing, and curated product discovery."
    : "Japanese product catalog with podcast recommendations, localized browsing, and curated product discovery.";
}

export async function generateMetadata({ params }: Props): Promise<Metadata> {
  const { locale } = await params;
  const normalizedLocale = normalizeLocale(locale);
  if (!normalizedLocale) {
    return {
      title: "Product Catalog",
    };
  }

  return {
    // 列表页是聚合入口，SEO 以语言维度为主，不绑定具体实体。
    title: metadataTitle(normalizedLocale),
    description: metadataDescription(normalizedLocale),
    alternates: {
      canonical: `/product/${normalizedLocale}`,
    },
    keywords:
      normalizedLocale === "zh"
        ? ["Chinese product catalog", "podcast recommendations", "localized products"]
        : ["Japanese product catalog", "podcast recommendations", "localized products"],
    openGraph: {
      title: metadataTitle(normalizedLocale),
      description: metadataDescription(normalizedLocale),
      type: "website",
      url: `/product/${normalizedLocale}`,
    },
    twitter: {
      card: "summary_large_image",
      title: metadataTitle(normalizedLocale),
      description: metadataDescription(normalizedLocale),
    },
  };
}

export default async function LocalizedProductListPage({ params }: Props) {
  const { locale } = await params;
  const normalizedLocale = normalizeLocale(locale);
  if (!normalizedLocale) {
    notFound();
  }

  const payload = await getLocalizedProductList(normalizedLocale, 24);

  return (
    <main className="page-shell">
      <ProductCatalog
        locale={normalizedLocale}
        products={payload.products}
        recommendedPodcasts={payload.recommended_podcasts}
      />
    </main>
  );
}
