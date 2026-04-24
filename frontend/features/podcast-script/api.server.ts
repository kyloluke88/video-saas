import { normalizeBaseUrl } from "@/shared/lib/url";
import type { PodcastScriptListItem, PodcastScriptPage } from "@/shared/types/public";

const DEFAULT_SERVER_API_BASE_URL =
  process.env.API_BASE_URL || process.env.NEXT_PUBLIC_API_BASE_URL || "http://localhost:8080";

export async function getPodcastScriptPage(slug: string): Promise<PodcastScriptPage | null> {
  const response = await fetch(
    `${normalizeBaseUrl(DEFAULT_SERVER_API_BASE_URL)}/api/public/podcast/scripts/${encodeURIComponent(slug.trim())}`,
    {
      cache: "no-store",
    },
  );

  if (response.status === 404) {
    return null;
  }

  if (!response.ok) {
    throw new Error(`Podcast script request failed: ${response.status} ${response.statusText}`);
  }

  const payload = (await response.json()) as { page?: PodcastScriptPage };
  return payload.page ?? null;
}

export async function getPodcastScriptList(limit = 24, language?: "zh" | "ja"): Promise<PodcastScriptListItem[]> {
  const query = new URLSearchParams({
    limit: String(limit),
  });
  if (language) {
    query.set("language", language);
  }
  const response = await fetch(
    `${normalizeBaseUrl(DEFAULT_SERVER_API_BASE_URL)}/api/public/podcast/scripts?${query.toString()}`,
    {
      cache: "no-store",
    },
  );

  if (!response.ok) {
    throw new Error(`Podcast script list request failed: ${response.status} ${response.statusText}`);
  }

  const payload = (await response.json()) as { pages?: PodcastScriptListItem[] };
  return payload.pages ?? [];
}
