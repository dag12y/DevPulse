import { configFromScript } from "./config";
import { createPageView } from "./pageview";
import { sendEvent } from "./transport";
import type { Tracker, TrackerConfig } from "./types";

export function isDNTEnabled(window = globalThis.window): boolean {
  try {
    const nav = window.navigator as unknown as { doNotTrack?: string; globalPrivacyControl?: boolean } | undefined;
    const win = window as unknown as { doNotTrack?: string } | undefined;
    if (nav?.globalPrivacyControl === true) return true;
    if (nav?.doNotTrack === "1" || nav?.doNotTrack === "yes") return true;
    if (win?.doNotTrack === "1") return true;
    return false;
  } catch {
    return false;
  }
}

export function createTracker(config: TrackerConfig, document = globalThis.document, window = globalThis.window): Tracker {
  if (config.respectDNT && isDNTEnabled(window)) {
    return { trackPageView: () => {} };
  }

  let lastURL = currentURL(window.location);
  const trackPageView = () => {
    try {
      sendEvent(config.endpoint, createPageView(config, document, window.location, window.navigator, window.screen, safeStorage(window), {
        width: window.innerWidth,
        height: window.innerHeight,
      }), window.navigator);
    } catch {
      // Tracking must never affect the host application.
    }
  };
  const trackNavigation = () => {
    const nextURL = currentURL(window.location);
    if (nextURL === lastURL) return;
    lastURL = nextURL;
    trackPageView();
  };
  for (const method of ["pushState", "replaceState"] as const) {
    const original = window.history[method];
    window.history[method] = function (...args) {
      const result = original.apply(this, args);
      trackNavigation();
      return result;
    };
  }
  window.addEventListener("popstate", trackNavigation);
  window.setTimeout(trackPageView, 0);
  return { trackPageView };
}

function currentURL(location: Location): string {
  const url = new URL(location.href);
  url.hash = "";
  return url.toString();
}

function safeStorage(window: Window): Storage | undefined {
  try { return window.sessionStorage; } catch { return undefined; }
}

if (typeof document !== "undefined" && typeof window !== "undefined") {
  const config = configFromScript(document);
  if (config) createTracker(config);
}
