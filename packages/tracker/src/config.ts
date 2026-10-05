import type { TrackerConfig } from "./types";

/** Path the API exposes for event ingestion. */
export const ingestPath = "/v1/analytics/events";

/**
 * Default ingest endpoint, derived from the tracker's own script URL.
 *
 * The bundle is served by the API itself (at `/analytics.js`), so the origin
 * it was loaded from is also the ingest origin. Deriving the endpoint means
 * the documented two-attribute snippet works with no `data-endpoint`, and no
 * development origin can ever be baked into a published bundle.
 *
 * Returns undefined when the script URL is missing or unparseable: the caller
 * then skips tracking rather than posting events to a wrong host.
 */
export function endpointFromScriptSrc(src: string | undefined): string | undefined {
  const trimmed = src?.trim();
  if (!trimmed) return undefined;
  let url: URL;
  try {
    url = new URL(trimmed);
  } catch {
    return undefined;
  }
  if (url.protocol !== "http:" && url.protocol !== "https:") return undefined;
  return `${url.origin}${ingestPath}`;
}

export function configFromScript(document: Document): TrackerConfig | null {
  const currentScript = document.currentScript as HTMLScriptElement | null;
  const script = currentScript && currentScript.tagName === "SCRIPT"
    ? currentScript
    : document.querySelector<HTMLScriptElement>("script[data-project]");
  if (!script) return null;

  const projectId = script.dataset.project?.trim();
  if (!projectId) return null;

  // An explicit data-endpoint always wins; otherwise infer from src.
  const endpoint = script.dataset.endpoint?.trim() || endpointFromScriptSrc(script.src);
  if (!endpoint) return null;

  return {
    projectId,
    endpoint,
  };
}
