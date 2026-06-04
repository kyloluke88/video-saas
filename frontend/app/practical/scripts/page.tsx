import type { Metadata } from "next";

import { getPracticalScriptList } from "@/features/practical-script/api.server";
import PracticalScriptList from "@/features/practical-script/components/practical-script-list";

export const dynamic = "force-dynamic";
export const revalidate = 0;

export const metadata: Metadata = {
  title: "Practical Scripts",
  description: "Browse all published practical conversation scripts with title, English subtitle, and summary previews.",
};

export default async function PracticalScriptListPage() {
  const pages = await getPracticalScriptList(30);

  return (
    <main className="page-shell">
      <PracticalScriptList
        copy="Browse all published practical conversation scripts. Click any card to open the full transcript page."
        heading="Practical Scripts"
        items={pages}
      />
    </main>
  );
}
