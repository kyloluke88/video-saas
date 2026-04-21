import type { Metadata } from "next";

import PodcastScriptList from "@/components/podcast-script-list";
import PageViewTracker from "@/components/page-view-tracker";
import { getPodcastScriptList } from "@/lib/api";
import { PAGE_VIEW_PAGE_TYPE } from "@/lib/page-view";

export const dynamic = "force-dynamic";
export const revalidate = 0;

export const metadata: Metadata = {
  title: "Japanese Podcast Scripts",
  description: "Japanese chat script list with title, English subtitle, and summary previews.",
};

export default async function JapanesePodcastScriptListPage() {
  const pages = await getPodcastScriptList(30, "ja");

  return (
    <main className="page-shell">
      {/* 列表页归类为聚合页，不绑定实体 ID。 */}
      <PageViewTracker pageType={PAGE_VIEW_PAGE_TYPE.COLLECTION_PAGE} />
      <PodcastScriptList
        copy="Browse all published Japanese podcast scripts. Click any card to open the full transcript page."
        heading="Japanese Podcast Scripts"
        items={pages}
      />
    </main>
  );
}
