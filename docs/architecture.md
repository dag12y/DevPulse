# DevPulse Architecture

## Overview

DevPulse is planned as three application components:

1. A browser tracking script
2. A Go analytics API
3. A Next.js analytics dashboard

PostgreSQL is the primary data store.

## Current Foundation

The repository contains a Go API with health checks, project CRUD, and analytics event ingestion. The browser tracker is implemented and sends page-view events compatible with the API. Docker Compose services are defined for the API and PostgreSQL. The Next.js dashboard is still the default scaffold — analytics views are not yet built.

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

The Go API receives and validates analytics events, manages visitors and sessions, and persists data. CORS is enabled for cross-origin tracker requests. Rate limits, bot detection, and approximate geography enrichment are planned.

### PostgreSQL

PostgreSQL will store projects, visitors, sessions, and page views. Dashboard requests should use aggregated database queries rather than raw events.

### Dashboard

The Next.js dashboard will provide overview metrics, traffic trends, top pages, traffic sources, countries, devices, browsers, operating systems, real-time visitors, and project settings.
