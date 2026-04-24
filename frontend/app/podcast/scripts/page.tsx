import type { Metadata } from "next";

import PodcastScriptList from "@/features/podcast-script/components/podcast-script-list";
import PageViewTracker from "@/features/analytics/page-view-tracker";
import { getPodcastScriptList } from "@/features/podcast-script/api.server";
import { PAGE_VIEW_PAGE_TYPE } from "@/features/analytics/page-view.client";

export const dynamic = "force-dynamic";
export const revalidate = 0;

export const metadata: Metadata = {
  title: "Podcast Scripts",
  description: "Browse all published podcast scripts with title, English subtitle, and summary previews.",
};

export default async function PodcastScriptListPage() {
  const pages = await getPodcastScriptList(30);

  return (
    <main className="page-shell">
      {/* 列表页归类为聚合页，不绑定实体 ID。 */}
      <PageViewTracker pageType={PAGE_VIEW_PAGE_TYPE.COLLECTION_PAGE} />
      <PodcastScriptList
        copy="Browse all published podcast scripts. Click any card to open the full transcript page."
        heading="Podcast Scripts"
        items={pages}
      />
    </main>
  );
}
