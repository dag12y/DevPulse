# DevPulse Architecture

## Overview

DevPulse is planned as three application components:

1. A browser tracking script
2. A Go analytics API
3. A Next.js analytics dashboard

PostgreSQL is the primary data store.

## Current Foundation

The repository currently contains the dashboard scaffold, a Go API with a `GET /health` endpoint, and Docker Compose services for the API and PostgreSQL. The tracker, event ingestion, persistence, and dashboard analytics are not implemented yet.

## Intended Data Flow

```text
Website
  │ page_view
  ▼
Browser tracker
  │ HTTPS
  ▼
Go API ── persists events ──► PostgreSQL
  ▲                              │ aggregated queries
  └──────── Next.js dashboard ◄──┘
```

## Planned Components

### Tracker

The tracker will be a small browser script installed on a website. Its responsibilities include:

- Generating a first-party visitor ID and maintaining a session
- Detecting page views, including client-side navigation
- Collecting page, referrer, UTM, device, and screen information
- Sending events without blocking or interfering with the website

### API

The Go API will receive and validate analytics events, apply rate limits and bot detection, manage visitors and sessions, enrich events with approximate geography, persist data, and provide dashboard analytics.

### PostgreSQL

PostgreSQL will store projects, visitors, sessions, and page views. Dashboard requests should use aggregated database queries rather than raw events.

### Dashboard

The Next.js dashboard will provide overview metrics, traffic trends, top pages, traffic sources, countries, devices, browsers, operating systems, real-time visitors, and project settings.
