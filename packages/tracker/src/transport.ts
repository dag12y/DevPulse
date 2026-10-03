import type { PageViewEvent } from "./types";

export interface SendOptions {
  /** Number of fetch retries after the initial attempt. Default 2 (3 attempts total). */
  maxRetries?: number;
  /** Base backoff delay in ms; actual delay is base * 2^attempt. Default 500. */
  baseDelayMs?: number;
}

export function sendEvent(endpoint: string, event: PageViewEvent, navigator: Navigator, fetcher: typeof fetch | undefined = globalThis.fetch, options: SendOptions = {}): void {
  try {
    const body = JSON.stringify(event);
    try {
      if (navigator.sendBeacon?.(endpoint, body)) return;
    } catch {
      // Fall through to fetch when beacon delivery is unavailable.
    }
    if (!fetcher) return;
    const maxRetries = Math.min(Math.max(options.maxRetries ?? 2, 0), 3);
    const baseDelay = Math.min(Math.max(options.baseDelayMs ?? 500, 0), 5000);

    const attemptFetch = (attempt: number): void => {
      let controller: AbortController | undefined;
      let timeout: ReturnType<typeof setTimeout> | undefined;
      try {
        if (typeof AbortController !== "undefined") {
          controller = new AbortController();
          timeout = globalThis.setTimeout(() => {
            try { controller?.abort(); } catch { /* never break host */ }
          }, 5000);
        }
        void fetcher(endpoint, { method: "POST", body, keepalive: true, headers: { "Content-Type": "text/plain;charset=UTF-8" }, signal: controller?.signal })
          .then(
            () => { if (timeout !== undefined) globalThis.clearTimeout(timeout); },
            () => {
              if (timeout !== undefined) globalThis.clearTimeout(timeout);
              if (attempt < maxRetries) {
                const delay = baseDelay * 2 ** attempt;
                try {
                  globalThis.setTimeout(() => attemptFetch(attempt + 1), delay);
                } catch { /* never break host */ }
              }
            },
          );
      } catch {
        if (timeout !== undefined) {
          try { globalThis.clearTimeout(timeout); } catch { /* never break host */ }
        }
        // Synchronous fetch construction failure: schedule one bounded retry.
        if (attempt < maxRetries) {
          try {
            globalThis.setTimeout(() => attemptFetch(attempt + 1), baseDelay * 2 ** attempt);
          } catch { /* never break host */ }
        }
      }
    };

    attemptFetch(0);
  } catch {
    // Tracking must never affect the host application.
  }
}
