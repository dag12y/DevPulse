# DevPulse Tracking

This document defines the tracker behavior. The tracker is implemented in `packages/tracker/`.

## Basic Installation

A website installs the tracker with a public project tracking ID. Prefer the
versioned bundle (immutable cache, safe to cache for a year):

```html
<script
  src="https://analytics.example.com/analytics-0.1.0.js"
  data-project="PROJECT_TRACKING_ID"
  defer
></script>
```

During development, `/analytics.js` (short cache) is equivalent:

```html
<script
  src="https://analytics.example.com/analytics.js"
  data-project="PROJECT_TRACKING_ID"
  defer
></script>
```

The tracking ID identifies the analytics project; it is not a secret credential.

To override the endpoint (self-hosting, staging), set `data-endpoint`:

```html
<script
  defer
  src="/analytics.js"
  data-project="dp_your_project_id"
  data-endpoint="https://api.example.com/v1/analytics/events"
></script>
```

The development default is `http://localhost:5000/v1/analytics/events`.

## Production bundle

- Source: `packages/tracker/src/`; browser bundle: `dist/analytics.js`.
- Build: `pnpm --filter @devpulse/tracker build` writes minified
  `dist/analytics.js` plus versioned `dist/analytics-<version>.js`.
- Size budget: 10 KB gzipped, enforced by
  `pnpm --filter @devpulse/tracker size`. Keep it small; the tracker must
  never slow down host sites.
- Serving: the API serves `TRACKER_DIR` at `GET /analytics.js`
  (`Cache-Control: public, max-age=3600`) and
  `GET /analytics-<version>.js`
  (`Cache-Control: public, max-age=31536000, immutable`).
  The Docker image bakes the bundle into `/tracker-dist` and sets
  `TRACKER_DIR=/tracker-dist`. Without `TRACKER_DIR` the routes 404.
- Version reporting: every page view sends `sdk_version` (e.g. `"0.1.0"`).
  The API accepts and ignores it for storage; use it for debugging rollout.

## Page Views

The initial MVP primarily tracks page views. Each event includes:

- Event ID and type, project ID, visitor ID, session ID, timestamp, SDK version
- URL, path, page title, and referrer
- Screen and viewport dimensions, browser language, and timezone
- UTM parameters

The server enriches an event with country, region, device type, browser, operating system, and bot status.

## Geographic Detection

Country (ISO code) and region (subdivision name) come from a MaxMind City
database (GeoLite2-City or GeoIP2-City) when configured. The request IP is
used transiently for the lookup and never stored — see `docs/privacy.md`.

Setup:

1. Download `GeoLite2-City.mmdb` (free MaxMind account + license key):
   `https://www.maxmind.com/en/geolite2/signup`
2. Place it at `./geoip/GeoLite2-City.mmdb` (gitignored).
3. Set `GEOIP_DB_PATH=/geoip/GeoLite2-City.mmdb` (Docker) or the local
   path when running the API directly.
4. Restart the API. Startup logs confirm `GeoIP enrichment enabled`;
   without it, geography reports as `Unknown`.

Private, loopback, and unresolvable addresses always yield empty
geography rather than an error, so local development keeps working.

## Single-Page Applications

The tracker detects `pushState`, `replaceState`, and `popstate`. It sends a new
page view when the URL changes (fragment stripped) while avoiding duplicates
for the same URL.

## Sending Events

The tracker:

- Prefers `navigator.sendBeacon()` when available (no retry; the browser owns delivery)
- Falls back to `fetch(..., { keepalive: true })` with a 5s timeout
- Retries failed fetches at most 2 more times (3 attempts total) with
  exponential backoff (~500ms, ~1000ms), then gives up silently
- Never throws, never blocks rendering, never retries indefinitely

## Framework installation

### Plain HTML

Paste the versioned tag before `</body>`. Page views and SPA navigation are
tracked automatically. No further code is needed.

### Next.js (App Router)

Add once in `app/layout.tsx` so client-side route changes are observed:

```tsx
import Script from "next/script";

export default function RootLayout({ children }: { children: React.ReactNode }) {
  return (
    <html lang="en">
      <body>
        {children}
        <Script
          src="https://analytics.example.com/analytics-0.1.0.js"
          data-project="dp_your_project_id"
          strategy="afterInteractive"
        />
      </body>
    </html>
  );
}
```

`next/link` navigations use `pushState` under the hood, so each route change
sends one page view automatically.

### React (Vite / CRA / React Router)

Include the tag once in `index.html` (same as plain HTML). No
`useEffect` wrapper is needed — the bundle self-initializes from
`script[data-project]` and listens to `pushState`/`replaceState`/`popstate`,
so React Router route changes are tracked. For a custom endpoint:

```html
<script
  defer
  src="https://analytics.example.com/analytics-0.1.0.js"
  data-project="dp_your_project_id"
  data-endpoint="https://api.example.com/v1/analytics/events"
></script>
```

### Other SPAs (Vue, Svelte, etc.)

Same rule: include the tag once per document. Hash-only changes are ignored;
`pushState`/`replaceState` URL changes each send exactly one page view.

## UTM Parameters

The tracker supports `utm_source`, `utm_medium`, `utm_campaign`, `utm_term`, and `utm_content`.

## Referrers and Traffic Sources

Referrer information is normalized. For example, `https://www.google.com/search?q=example` may be represented as `google.com`.

Traffic sources can be classified as Direct, Organic Search, Social, Referral, Campaign, or Other.
