import type { Metadata } from "next";

import PodcastScriptList from "@/components/podcast-script-list";
import { getPodcastScriptList } from "@/lib/api";

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
      <PodcastScriptList
        copy="Browse all published podcast scripts. Click any card to open the full transcript page."
        heading="Podcast Scripts"
        items={pages}
      />
    </main>
  );
}
