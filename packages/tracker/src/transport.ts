import type { PageViewEvent } from "./types";

export function sendEvent(endpoint: string, event: PageViewEvent, navigator: Navigator, fetcher: typeof fetch | undefined = globalThis.fetch): void {
  try {
    const body = JSON.stringify(event);
    try {
      if (navigator.sendBeacon?.(endpoint, body)) return;
    } catch {
      // Fall through to fetch when beacon delivery is unavailable.
    }
    if (!fetcher) return;
    const controller = typeof AbortController === "undefined" ? undefined : new AbortController();
    const timeout = controller ? globalThis.setTimeout(() => controller.abort(), 5000) : undefined;
    void fetcher(endpoint, { method: "POST", body, keepalive: true, headers: { "Content-Type": "text/plain;charset=UTF-8" }, signal: controller?.signal })
      .catch(() => undefined)
      .finally(() => { if (timeout !== undefined) globalThis.clearTimeout(timeout); });
  } catch {
    // Tracking must never affect the host application.
  }
}
