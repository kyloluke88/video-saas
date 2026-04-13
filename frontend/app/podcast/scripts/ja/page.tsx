import type { Metadata } from "next";

import PodcastScriptList from "@/components/podcast-script-list";
import { getPodcastScriptList } from "@/lib/api";

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
      <PodcastScriptList
        copy="Browse all published Japanese podcast scripts. Click any card to open the full transcript page."
        heading="Japanese Podcast Scripts"
        items={pages}
      />
    </main>
  );
}
