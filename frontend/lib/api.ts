import type { PodcastScriptPage } from "@/types/public";

const DEFAULT_SERVER_API_BASE_URL =
  process.env.API_BASE_URL || process.env.NEXT_PUBLIC_API_BASE_URL || "http://localhost:8080";

function normalizeBaseUrl(baseUrl: string) {
  return baseUrl.replace(/\/$/, "");
}

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
