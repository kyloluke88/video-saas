import type { Metadata } from "next";

import { getPracticalScriptList } from "@/features/practical-script/api.server";
import PracticalScriptList from "@/features/practical-script/components/practical-script-list";

export const dynamic = "force-dynamic";
export const revalidate = 0;

export const metadata: Metadata = {
  title: "Chinese Practical Scripts",
  description: "Browse all published Chinese practical conversation scripts.",
};

export default async function ChinesePracticalScriptListPage() {
  const pages = await getPracticalScriptList(30, "zh");

  return (
    <main className="page-shell">
      <PracticalScriptList
        copy="Browse all published Chinese practical conversation scripts. Click any card to open the full transcript page."
        heading="Chinese Practical Scripts"
        items={pages}
      />
    </main>
  );
}
