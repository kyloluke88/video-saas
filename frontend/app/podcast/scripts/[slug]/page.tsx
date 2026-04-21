import type { Metadata } from "next";
import { notFound } from "next/navigation";

import PageViewTracker from "@/components/page-view-tracker";
import PodcastScriptPageView from "@/components/podcast-script-page";
import { getPodcastScriptList, getPodcastScriptPage } from "@/lib/api";
import { PAGE_VIEW_PAGE_TYPE } from "@/lib/page-view";

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
  const page = await getPodcastScriptPage(normalizeSlug(slug)).catch(() => null);

  if (!page) {
    return {
      title: "Podcast Script Not Found",
    };
  }

  return {
    // 详情页优先使用内容里维护的 SEO 字段，避免和页面标题逻辑分叉。
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

export default async function PodcastScriptDetailPage({ params }: Props) {
  const { slug } = await params;
  const page = await getPodcastScriptPage(normalizeSlug(slug)).catch(() => null);

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
  const sidebarPages = await getPodcastScriptList(10, language)
    .then((items) => items.filter((item) => item.slug !== page.slug).slice(0, 6))
    .catch(() => []);

  return (
    <>
      <script
        dangerouslySetInnerHTML={{ __html: JSON.stringify(articleJsonLd) }}
        type="application/ld+json"
      />
      <main className="page-shell">
        {/* 详情页按实体 ID 上报，方便后端按页面粒度统计。 */}
        <PageViewTracker pageEntityId={page.id} pageType={PAGE_VIEW_PAGE_TYPE.PODCAST_SCRIPT_DETAIL} />
        <PodcastScriptPageView page={page} sidebarPages={sidebarPages} />
      </main>
    </>
  );
}
