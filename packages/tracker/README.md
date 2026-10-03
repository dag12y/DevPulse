# DevPulse Tracker

A small, privacy-conscious browser tracker that sends page views to the DevPulse API.

## Install on a website

Prefer the versioned bundle (immutable, cacheable for a year):

```html
<script
  defer
  src="https://analytics.example.com/analytics-0.1.0.js"
  data-project="dp_your_project_id"
></script>
```

`data-project` is required and must be the public tracking ID returned when the analytics project is created. To override the endpoint, set `data-endpoint`:

```html
<script async src="/analytics.js" data-project="dp_your_project_id" data-endpoint="https://api.example.com/v1/analytics/events"></script>
```

The development default is `http://localhost:5000/v1/analytics/events`.

## Build and test

From the repository root:

```bash
pnpm --filter @devpulse/tracker typecheck
pnpm --filter @devpulse/tracker build
pnpm --filter @devpulse/tracker size
pnpm --filter @devpulse/tracker test
```

The minified browser bundle is written to `dist/analytics.js` plus versioned
`dist/analytics-<version>.js`. Size budget: 10 KB gzipped (enforced by `size`).

## Data collected

The tracker sends page-view event IDs, the public project ID, first-party visitor and session IDs, timestamp, URL without fragments, path, title, referrer, screen/viewport size, language, timezone, and present UTM parameters.

It does **not** collect IP addresses, precise location, form or DOM contents, names, emails, phone numbers, authentication tokens, keystrokes, mouse movements, clipboard data, camera/microphone data, or browser fingerprints.
