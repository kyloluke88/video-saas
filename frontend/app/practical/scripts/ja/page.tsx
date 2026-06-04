import type { Metadata } from "next";

import { getPracticalScriptList } from "@/features/practical-script/api.server";
import PracticalScriptList from "@/features/practical-script/components/practical-script-list";

export const dynamic = "force-dynamic";
export const revalidate = 0;

export const metadata: Metadata = {
  title: "Japanese Practical Scripts",
  description: "Browse all published Japanese practical conversation scripts.",
};

export default async function JapanesePracticalScriptListPage() {
  const pages = await getPracticalScriptList(30, "ja");

  return (
    <main className="page-shell">
      <PracticalScriptList
        copy="Browse all published Japanese practical conversation scripts. Click any card to open the full transcript page."
        heading="Japanese Practical Scripts"
        items={pages}
      />
    </main>
  );
}
