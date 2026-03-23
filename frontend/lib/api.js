const DEFAULT_SERVER_API_BASE_URL =
  process.env.API_BASE_URL || process.env.NEXT_PUBLIC_API_BASE_URL || "http://localhost:8080";

const DEFAULT_PUBLIC_API_BASE_URL =
  process.env.NEXT_PUBLIC_API_BASE_URL || process.env.API_BASE_URL || "http://localhost:8080";

function normalizeBaseUrl(baseUrl) {
  return baseUrl.replace(/\/$/, "");
}

async function readJson(path) {
  const response = await fetch(`${normalizeBaseUrl(DEFAULT_SERVER_API_BASE_URL)}${path}`, {
    next: { revalidate: 60 },
  });

  if (!response.ok) {
    throw new Error(`Request failed: ${response.status} ${response.statusText}`);
  }

  return response.json();
}

export async function listProjects() {
  const payload = await readJson("/api/public/projects");
  return payload.projects || [];
}

export async function getProject(projectId) {
  const response = await fetch(
    `${normalizeBaseUrl(DEFAULT_SERVER_API_BASE_URL)}/api/public/projects/${projectId}`,
    {
      next: { revalidate: 60 },
    },
  );

  if (response.status === 404) {
    return null;
  }

  if (!response.ok) {
    throw new Error(`Project request failed: ${response.status} ${response.statusText}`);
  }

  const payload = await response.json();
  return payload.project || null;
}

export function buildPublicAssetUrl(path) {
  if (!path) {
    return null;
  }

  if (path.startsWith("http://") || path.startsWith("https://")) {
    return path;
  }

  return `${normalizeBaseUrl(DEFAULT_PUBLIC_API_BASE_URL)}${path}`;
}
