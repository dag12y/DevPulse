# DevPulse Tracking

This document defines the tracker behavior. The tracker is implemented in `packages/tracker/`.

## Basic Installation

A website will install the tracker with a public project tracking ID:

```html
<script
  src="https://analytics.devpulse.example/analytics.js"
  data-project="PROJECT_TRACKING_ID"
  async
></script>
```

The tracking ID identifies the analytics project; it is not a secret credential.

## Page Views

The initial MVP will primarily track page views. Each event may include:

- Event ID and type, project ID, visitor ID, session ID, and timestamp
- URL, path, page title, and referrer
- Screen and viewport dimensions, browser language, and timezone
- UTM parameters

The server may enrich an event with country, region, device type, browser, operating system, and bot status.

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

The tracker should detect `pushState`, `replaceState`, and `popstate`. It should send a new page view when the URL changes while avoiding duplicates.

## Sending Events

The tracker should:

- Prefer `navigator.sendBeacon()` when appropriate
- Use `fetch(..., { keepalive: true })` when necessary
- Fail silently without blocking the website
- Use limited retries and never retry indefinitely

## UTM Parameters

The tracker supports `utm_source`, `utm_medium`, `utm_campaign`, `utm_term`, and `utm_content`.

## Referrers and Traffic Sources

Referrer information should be normalized. For example, `https://www.google.com/search?q=example` may be represented as `google.com`.

Traffic sources can be classified as Direct, Organic Search, Social, Referral, Campaign, or Other.
