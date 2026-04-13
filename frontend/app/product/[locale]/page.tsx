import type { Metadata } from "next";
import { notFound } from "next/navigation";

import ProductCatalog from "@/components/product-catalog";
import { getLocalizedProductList } from "@/lib/api";

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

export async function generateMetadata({ params }: Props): Promise<Metadata> {
  const { locale } = await params;
  const normalizedLocale = normalizeLocale(locale);
  if (!normalizedLocale) {
    return {
      title: "Product Catalog",
    };
  }

  return {
    title: metadataTitle(normalizedLocale),
    description: "Localized product catalog with podcast recommendations.",
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
