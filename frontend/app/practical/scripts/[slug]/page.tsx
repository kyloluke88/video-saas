import type { Metadata } from "next";
import { notFound } from "next/navigation";

import PracticalScriptPageView from "@/features/practical-script/components/practical-script-page";
import { getPracticalScriptList, getPracticalScriptPage } from "@/features/practical-script/api.server";

export const dynamic = "force-dynamic";
export const revalidate = 0;

type Props = {
  params: Promise<{
    slug: string;
  }>;
};

function normalizeSlug(value: string) {
  return value.trim();
}

function normalizeScriptLanguage(value?: string): "zh" | "ja" | undefined {
  const normalized = value?.trim().toLowerCase();
  if (normalized === "zh" || normalized === "ja") {
    return normalized;
  }
  return undefined;
}

export async function generateMetadata({ params }: Props): Promise<Metadata> {
  const { slug } = await params;
  const page = await getPracticalScriptPage(normalizeSlug(slug)).catch(() => null);

  if (!page) {
    return {
      title: "Practical Script Not Found",
    };
  }

  return {
    title: page.seo_title || page.title,
    description: page.seo_description || page.summary,
    keywords: page.seo_keywords,
    alternates: page.canonical_url
      ? {
          canonical: page.canonical_url,
        }
      : undefined,
    openGraph: {
      title: page.seo_title || page.title,
      description: page.seo_description || page.summary,
      type: "article",
      url: page.canonical_url,
      images: page.cover_image_url ? [page.cover_image_url] : undefined,
    },
  };
}

export default async function PracticalScriptDetailPage({ params }: Props) {
  const sidebarPageLimit = 25;
  const { slug } = await params;
  const page = await getPracticalScriptPage(normalizeSlug(slug)).catch(() => null);

  if (!page) {
    notFound();
  }

  const articleJsonLd = {
    "@context": "https://schema.org",
    "@type": "Article",
    headline: page.seo_title || page.title,
    description: page.seo_description || page.summary,
    inLanguage: page.language,
    datePublished: page.published_at,
    mainEntityOfPage: page.canonical_url,
  };

  const language = normalizeScriptLanguage(page.language);
  const sidebarPages = await getPracticalScriptList(sidebarPageLimit + 1, language)
    .then((items) => items.filter((item) => item.slug !== page.slug).slice(0, sidebarPageLimit))
    .catch(() => []);

  return (
    <>
      <script dangerouslySetInnerHTML={{ __html: JSON.stringify(articleJsonLd) }} type="application/ld+json" />
      <main className="page-shell">
        <PracticalScriptPageView page={page} sidebarPages={sidebarPages} />
      </main>
    </>
  );
}
