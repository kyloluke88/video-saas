import { normalizeBaseUrl } from "@/shared/lib/url";

// 与后端 analytics.PageType 保持同值，避免前后端统计口径不一致。
export const PAGE_VIEW_PAGE_TYPE = {
  PODCAST_SCRIPT_DETAIL: 1,
  PRODUCT_DETAIL: 2,
  COLLECTION_PAGE: 3,
  STATIC_PAGE: 4,
} as const;

export type PageViewPageType = (typeof PAGE_VIEW_PAGE_TYPE)[keyof typeof PAGE_VIEW_PAGE_TYPE];

const VISITOR_COOKIE_NAME = "video_saas_visitor_key";
const SESSION_COOKIE_NAME = "video_saas_session_key";
const VISITOR_MAX_AGE_SECONDS = 60 * 60 * 24 * 365;
const SESSION_MAX_AGE_SECONDS = 60 * 60;

export function getAnalyticsBaseUrl() {
  const configuredBaseUrl = process.env.NEXT_PUBLIC_API_BASE_URL?.trim();
  if (configuredBaseUrl) {
    return normalizeBaseUrl(configuredBaseUrl);
  }

  // 线上默认走同域 API，由反向代理转发到 backend，避免浏览器端退回 localhost。
  if (typeof window !== "undefined" && window.location.origin) {
    return normalizeBaseUrl(window.location.origin);
  }

  return "";
}

export function getPageViewEndpoint() {
  const baseUrl = getAnalyticsBaseUrl();
  if (!baseUrl) {
    return "/api/analytics/page-view";
  }
  return `${baseUrl}/api/analytics/page-view`;
}

export function createUUID() {
  const webCrypto = globalThis.crypto;
  if (webCrypto && typeof webCrypto.randomUUID === "function") {
    return webCrypto.randomUUID();
  }

  const bytes = new Uint8Array(16);
  if (webCrypto) {
    webCrypto.getRandomValues(bytes);
  } else {
    for (let index = 0; index < bytes.length; index += 1) {
      bytes[index] = Math.floor(Math.random() * 256);
    }
  }
  bytes[6] = (bytes[6] & 0x0f) | 0x40;
  bytes[8] = (bytes[8] & 0x3f) | 0x80;

  const hex = Array.from(bytes, (byte) => byte.toString(16).padStart(2, "0")).join("");
  return `${hex.slice(0, 8)}-${hex.slice(8, 12)}-${hex.slice(12, 16)}-${hex.slice(16, 20)}-${hex.slice(20)}`;
}

function isSecureCookie() {
  return typeof window !== "undefined" && window.location.protocol === "https:";
}

function isUUIDLike(value: string) {
  if (value.length !== 36) {
    return false;
  }

  for (let index = 0; index < value.length; index += 1) {
    const char = value[index];
    switch (index) {
      case 8:
      case 13:
      case 18:
      case 23:
        if (char !== "-") {
          return false;
        }
        break;
      default:
        if (!/[0-9a-fA-F]/.test(char)) {
          return false;
        }
    }
  }

  return true;
}

export function getCookie(name: string) {
  if (typeof document === "undefined") {
    return "";
  }

  const escapedName = `${name}=`;
  const cookies = document.cookie.split(";");
  for (const cookie of cookies) {
    const trimmed = cookie.trim();
    if (trimmed.startsWith(escapedName)) {
      return decodeURIComponent(trimmed.slice(escapedName.length));
    }
  }
  return "";
}

export function setCookie(name: string, value: string, maxAgeSeconds: number) {
  if (typeof document === "undefined") {
    return;
  }

  const parts = [
    `${name}=${encodeURIComponent(value)}`,
    `Max-Age=${maxAgeSeconds}`,
    "Path=/",
    "SameSite=Lax",
  ];
  if (isSecureCookie()) {
    parts.push("Secure");
  }
  document.cookie = parts.join("; ");
}

export function getOrCreateVisitorKey() {
  const existing = getCookie(VISITOR_COOKIE_NAME);
  if (isUUIDLike(existing)) {
    // visitor_key 是长期访客标识，命中后要顺手续期。
    setCookie(VISITOR_COOKIE_NAME, existing, VISITOR_MAX_AGE_SECONDS);
    return existing;
  }

  const visitorKey = createUUID();
  setCookie(VISITOR_COOKIE_NAME, visitorKey, VISITOR_MAX_AGE_SECONDS);
  return visitorKey;
}

export function getOrCreateSessionKey() {
  const existing = getCookie(SESSION_COOKIE_NAME);
  if (isUUIDLike(existing)) {
    // session_key 是滑动会话标识，每次上报都刷新 60 分钟窗口。
    setCookie(SESSION_COOKIE_NAME, existing, SESSION_MAX_AGE_SECONDS);
    return existing;
  }

  const sessionKey = createUUID();
  setCookie(SESSION_COOKIE_NAME, sessionKey, SESSION_MAX_AGE_SECONDS);
  return sessionKey;
}

function buildClientHints() {
  if (typeof navigator === "undefined") {
    return {};
  }

  const navigatorWithHints = navigator as Navigator & {
    userAgentData?: {
      brands?: Array<{ brand: string; version: string }>;
      mobile?: boolean;
      platform?: string;
    };
  };

  return {
    languages: navigator.languages ?? [],
    language: navigator.language ?? "",
    platform: navigator.platform ?? "",
    user_agent_data: navigatorWithHints.userAgentData
      ? {
          brands: navigatorWithHints.userAgentData.brands ?? [],
          mobile: navigatorWithHints.userAgentData.mobile ?? false,
          platform: navigatorWithHints.userAgentData.platform ?? "",
        }
      : undefined,
    device_pixel_ratio: typeof window !== "undefined" ? window.devicePixelRatio : undefined,
    viewport: typeof window !== "undefined" ? { width: window.innerWidth, height: window.innerHeight } : undefined,
    screen:
      typeof window !== "undefined"
        ? {
            width: window.screen.width,
            height: window.screen.height,
            avail_width: window.screen.availWidth,
            avail_height: window.screen.availHeight,
            color_depth: window.screen.colorDepth,
          }
        : undefined,
    // 只采集前端可见信息，不强依赖后端请求头。
    time_zone: Intl.DateTimeFormat().resolvedOptions().timeZone,
  };
}

export type TrackPageViewInput = {
  pageType: PageViewPageType;
  pageEntityId?: number;
  referer?: string;
};

export async function trackPageView(input: TrackPageViewInput) {
  if (typeof window === "undefined" || typeof document === "undefined") {
    return;
  }

  const payload: Record<string, unknown> = {
    visitor_key: getOrCreateVisitorKey(),
    session_key: getOrCreateSessionKey(),
    page_type: input.pageType,
    page_path: `${window.location.pathname}${window.location.search}`,
    referer: input.referer ?? document.referrer ?? "",
    accept_language: navigator.language ?? "",
    user_agent: navigator.userAgent ?? "",
    client_hints: buildClientHints(),
  };

  if (typeof input.pageEntityId === "number") {
    payload.page_entity_id = input.pageEntityId;
  }

  const body = JSON.stringify(payload);

  // sendBeacon 适合“只管送达，不关心响应”的埋点。
  // 它会尽量在页面卸载时完成发送，且通常不会触发我们之前遇到的 JSON preflight 问题。
  if (typeof navigator !== "undefined" && typeof navigator.sendBeacon === "function") {
    // sendBeacon 只关心“有没有成功提交到浏览器队列”，不关心响应内容。
    const sent = navigator.sendBeacon(getPageViewEndpoint(), body);
    if (sent) {
      return;
    }
  }

  // 兜底方案：某些浏览器不支持 sendBeacon，或者 beacon 队列满了。
  // 这里使用 text/plain 发送 JSON 字符串，避免 application/json 触发 CORS 预检。
  await fetch(getPageViewEndpoint(), {
    method: "POST",
    mode: "cors",
    cache: "no-store",
    credentials: "omit",
    keepalive: true,
    headers: {
      "Content-Type": "text/plain;charset=UTF-8",
    },
    body,
  }).catch(() => undefined);
}
