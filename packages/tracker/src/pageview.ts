import { createUUID, getSessionID, getVisitorID } from "./identity";
import type { Dimensions, PageViewEvent, TrackerConfig } from "./types";

const limits = { url: 2048, path: 1024, title: 512, referrer: 2048, language: 32, timezone: 64, utm: 256 };

export function createPageView(config: TrackerConfig, document: Document, location: Location, navigator: Navigator, screen: Screen | undefined, storage: Storage | undefined, viewport?: Dimensions): PageViewEvent {
  const url = new URL(location.href);
  url.hash = "";
  const parameter = (name: string) => truncate(url.searchParams.get(name) || "", limits.utm) || undefined;
  const timezone = truncate(safeTimezone(), limits.timezone) || "UTC";

  return {
    event_id: createUUID(), type: "page_view", project_id: config.projectId,
    visitor_id: getVisitorID(document), session_id: getSessionID(storage), timestamp: new Date().toISOString(),
    page: {
      url: safeURL(url, limits.url), path: truncate(location.pathname || "/", limits.path),
      title: truncate(document.title || "", limits.title), referrer: truncate(document.referrer || "", limits.referrer),
    },
    screen: { width: dimension(screen?.width), height: dimension(screen?.height) },
    viewport: viewport ?? { width: dimension(globalThis.innerWidth), height: dimension(globalThis.innerHeight) },
    language: truncate(navigator.language || "", limits.language), timezone,
    campaign: { source: parameter("utm_source"), medium: parameter("utm_medium"), campaign: parameter("utm_campaign"), term: parameter("utm_term"), content: parameter("utm_content") },
  };
}

function dimension(value: number | undefined): number {
  return Number.isFinite(value) && value && value > 0 ? Math.min(Math.floor(value), 10000) : 1;
}

function safeTimezone(): string {
  try { return Intl.DateTimeFormat().resolvedOptions().timeZone || "UTC"; } catch { return "UTC"; }
}

function truncate(value: string, maximum: number): string {
  return value.slice(0, maximum);
}

function safeURL(url: URL, maximum: number): string {
  const value = url.toString();
  if (value.length <= maximum) return value;

  url.search = "";
  const withoutQuery = url.toString();
  if (withoutQuery.length <= maximum) return withoutQuery;

  return `${url.origin}${truncate(url.pathname || "/", maximum - url.origin.length)}`;
}
