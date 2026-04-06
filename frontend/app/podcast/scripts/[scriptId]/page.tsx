import type { Metadata } from "next";
import { notFound } from "next/navigation";

import PodcastScriptPageView from "@/components/podcast-script-page";
import { getPodcastScriptPage } from "@/lib/api";

export const dynamic = "force-dynamic";

type Props = {
  params: Promise<{
    scriptId: string;
  }>;
};

function normalizeResourceId(value: string) {
  return value.trim();
}

export async function generateMetadata({ params }: Props): Promise<Metadata> {
  const { scriptId } = await params;
  const page = await getPodcastScriptPage(normalizeResourceId(scriptId)).catch(() => null);

  if (!page) {
    return {
      title: "Podcast Script Not Found",
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

export default async function PodcastScriptDetailPage({ params }: Props) {
  const { scriptId } = await params;
  const page = await getPodcastScriptPage(normalizeResourceId(scriptId)).catch(() => null);

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

  return (
    <>
      <script
        dangerouslySetInnerHTML={{ __html: JSON.stringify(articleJsonLd) }}
        type="application/ld+json"
      />
      <main className="shell">
        <PodcastScriptPageView page={page} />
      </main>
    </>
  );
}
