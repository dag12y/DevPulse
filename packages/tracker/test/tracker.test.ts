import assert from "node:assert/strict";
import test from "node:test";
import { createTracker } from "../src/index";
import { getSessionID, getVisitorID, sessionTimeout } from "../src/identity";
import { createPageView } from "../src/pageview";
import { sendEvent } from "../src/transport";

class MemoryStorage {
  values = new Map<string, string>();
  getItem(key: string) { return this.values.get(key) ?? null; }
  setItem(key: string, value: string) { this.values.set(key, value); }
}

import { configFromScript, endpointFromScriptSrc, ingestPath } from "../src/config";

function fakeDocument(script: Record<string, unknown> | null): Document {
  return {
    currentScript: script,
    querySelector: () => script,
  } as unknown as Document;
}

function scriptElement(dataset: Record<string, string>, src = ""): Record<string, unknown> {
  return { tagName: "SCRIPT", src, dataset };
}

test("ingest endpoint is derived from the script origin, never hardcoded", () => {
  assert.equal(
    endpointFromScriptSrc("https://analytics.example.com/analytics.js"),
    `https://analytics.example.com${ingestPath}`,
  );
  assert.equal(
    endpointFromScriptSrc("https://analytics.example.com/analytics-0.2.0.js"),
    `https://analytics.example.com${ingestPath}`,
  );
  // Port and path must not leak into the endpoint.
  assert.equal(
    endpointFromScriptSrc("http://localhost:5000/analytics.js"),
    `http://localhost:5000${ingestPath}`,
  );
});

test("unusable script URLs yield no endpoint instead of a wrong host", () => {
  assert.equal(endpointFromScriptSrc(undefined), undefined);
  assert.equal(endpointFromScriptSrc(""), undefined);
  assert.equal(endpointFromScriptSrc("   "), undefined);
  assert.equal(endpointFromScriptSrc("/analytics.js"), undefined);
  assert.equal(endpointFromScriptSrc("not a url"), undefined);
  assert.equal(endpointFromScriptSrc("javascript:alert(1)"), undefined);
});

test("config infers the endpoint from the script src when none is given", () => {
  const config = configFromScript(fakeDocument(scriptElement({ project: "dp_abc" }, "https://analytics.example.com/analytics.js")));
  assert.deepEqual(config, { projectId: "dp_abc", endpoint: `https://analytics.example.com${ingestPath}` });
});

test("explicit data-endpoint overrides the derived origin", () => {
  const config = configFromScript(fakeDocument(
    scriptElement({ project: "dp_abc", endpoint: "https://api.example.com/v1/analytics/events" }, "https://cdn.example.com/analytics.js"),
  ));
  assert.equal(config?.endpoint, "https://api.example.com/v1/analytics/events");
});

test("config is rejected when no endpoint can be determined", () => {
  assert.equal(configFromScript(fakeDocument(scriptElement({ project: "dp_abc" }))), null);
  assert.equal(configFromScript(fakeDocument(scriptElement({ project: "dp_abc" }, "/relative.js"))), null);
});

test("config still requires a project id", () => {
  const config = configFromScript(fakeDocument(scriptElement({}, "https://analytics.example.com/analytics.js")));
  assert.equal(config, null);
});

test("visitor ID is created and reused through the first-party cookie", () => {
  const document = { cookie: "", location: { protocol: "https:" } } as unknown as Document;
  const first = getVisitorID(document);
  const second = getVisitorID(document);
  assert.equal(first, second);
  assert.match(first, /^[0-9a-f-]{36}$/);
});

test("session ID is reused until thirty minutes of inactivity", () => {
  const storage = new MemoryStorage() as unknown as Storage;
  const first = getSessionID(storage, 1_000);
  assert.equal(getSessionID(storage, 1_000 + sessionTimeout - 1), first);
  assert.notEqual(getSessionID(storage, 1_000 + sessionTimeout * 2), first);
});

test("blocked storage falls back without throwing", () => {
  const document = {} as Document;
  Object.defineProperty(document, "cookie", { get() { throw new Error("blocked"); }, set() { throw new Error("blocked"); } });
  assert.equal(getVisitorID(document), getVisitorID(document));
  assert.equal(getSessionID(undefined, 1_000), getSessionID(undefined, 1_000 + sessionTimeout - 1));
});

test("page view strips fragments and extracts UTM values", () => {
  const document = { cookie: "", location: { protocol: "https:" }, title: "About", referrer: "" } as unknown as Document;
  const location = new URL("https://example.com/about?utm_source=google&utm_medium=organic#private") as unknown as Location;
  const event = createPageView({ projectId: "dp_test", endpoint: "https://api.test/events" }, document, location, { language: "en-US" } as Navigator, { width: 1920, height: 1080 } as Screen, new MemoryStorage() as unknown as Storage);
  assert.equal(event.page.url.includes("#"), false);
  assert.equal(event.campaign.source, "google");
  assert.equal(event.campaign.medium, "organic");
});

test("page view payload contains the backend event contract", () => {
  const document = { cookie: "", location: { protocol: "https:" }, title: "Home", referrer: "https://referrer.example/" } as unknown as Document;
  const location = new URL("https://example.com/") as unknown as Location;
  const event = createPageView({ projectId: "dp_test", endpoint: "https://api.test/events" }, document, location, { language: "en-US" } as Navigator, { width: 1920, height: 1080 } as Screen, undefined, { width: 1200, height: 800 });
  assert.equal(event.type, "page_view");
  assert.equal(event.page.path, "/");
  assert.deepEqual(event.viewport, { width: 1200, height: 800 });
  assert.equal(event.page.referrer, "https://referrer.example/");
  assert.match(event.sdk_version ?? "", /^\d+\.\d+\.\d+$/);
});

test("SPA navigation sends once for a changed URL and ignores duplicates", () => {
  let current = new URL("https://example.com/");
  let beaconCount = 0;
  const listeners = new Map<string, () => void>();
  const history = {
    pushState(_state: unknown, _title: string, url?: string | URL | null) { if (url) current = new URL(url, current); },
    replaceState(_state: unknown, _title: string, url?: string | URL | null) { if (url) current = new URL(url, current); },
  };
  const window = {
    get location() { return { href: current.href, pathname: current.pathname }; },
    history,
    navigator: { language: "en-US", sendBeacon() { beaconCount += 1; return true; } },
    screen: { width: 1920, height: 1080 }, innerWidth: 1200, innerHeight: 800,
    addEventListener(type: string, listener: () => void) { listeners.set(type, listener); },
    setTimeout(callback: () => void) { callback(); return 0; },
  } as unknown as Window;
  const document = { cookie: "", location: { protocol: "https:" }, title: "Home", referrer: "" } as unknown as Document;

  createTracker({ projectId: "dp_test", endpoint: "https://api.test/events" }, document, window);
  history.pushState({}, "", "/");
  history.pushState({}, "", "/pricing");
  assert.equal(beaconCount, 2);
  assert.equal(listeners.has("popstate"), true);
});

test("transport failures are silent", () => {
  assert.doesNotThrow(() => sendEvent("https://api.test/events", {} as never, { sendBeacon: () => false } as Navigator, () => Promise.reject(new Error("offline")), { baseDelayMs: 0 }));
});

test("fetch failure retries with bounded attempts then stops", async () => {
  let calls = 0;
  const fetcher = () => {
    calls += 1;
    return Promise.reject(new Error("offline"));
  };
  sendEvent("https://api.test/events", {} as never, { sendBeacon: () => false } as Navigator, fetcher as typeof fetch, { maxRetries: 2, baseDelayMs: 0 });
  await new Promise((resolve) => setTimeout(resolve, 50));
  assert.equal(calls, 3);
});

test("beacon success never touches fetch", () => {
  let fetchCalls = 0;
  sendEvent("https://api.test/events", {} as never, { sendBeacon: () => true } as unknown as Navigator, (() => { fetchCalls += 1; return Promise.resolve(new Response()); }) as typeof fetch);
  assert.equal(fetchCalls, 0);
});
