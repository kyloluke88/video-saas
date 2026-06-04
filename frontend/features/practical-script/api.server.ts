import { normalizeBaseUrl } from "@/shared/lib/url";
import type { PracticalScriptListItem, PracticalScriptPage } from "@/shared/types/public";

const DEFAULT_SERVER_API_BASE_URL =
  process.env.API_BASE_URL || process.env.NEXT_PUBLIC_API_BASE_URL || "http://localhost:8080";

export async function getPracticalScriptPage(slug: string): Promise<PracticalScriptPage | null> {
  const response = await fetch(
    `${normalizeBaseUrl(DEFAULT_SERVER_API_BASE_URL)}/api/public/practical/scripts/${encodeURIComponent(slug.trim())}`,
    {
      cache: "no-store",
    },
  );

  if (response.status === 404) {
    return null;
  }

  if (!response.ok) {
    throw new Error(`Practical script request failed: ${response.status} ${response.statusText}`);
  }

  const payload = (await response.json()) as { page?: PracticalScriptPage };
  return payload.page ?? null;
}

export async function getPracticalScriptList(limit = 24, language?: "zh" | "ja"): Promise<PracticalScriptListItem[]> {
  const query = new URLSearchParams({
    limit: String(limit),
  });
  if (language) {
    query.set("language", language);
  }

  const response = await fetch(
    `${normalizeBaseUrl(DEFAULT_SERVER_API_BASE_URL)}/api/public/practical/scripts?${query.toString()}`,
    {
      cache: "no-store",
    },
  );

  if (!response.ok) {
    throw new Error(`Practical script list request failed: ${response.status} ${response.statusText}`);
  }

  const payload = (await response.json()) as { pages?: PracticalScriptListItem[] };
  return payload.pages ?? [];
}
