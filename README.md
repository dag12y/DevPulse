# DevPulse

DevPulse is a lightweight, privacy-conscious web analytics platform for developers. Website owners add a small tracking script and view traffic analytics in a simple dashboard. No fingerprinting, no cross-site tracking, no ad profiles.

## Status

Working end to end on `main`:

- **Ingestion**: `POST /v1/analytics/events` validates, bot-filters, enriches (device, browser, OS, country/region via MaxMind when configured), and rate-limits per project+IP. The tracker never blocks the host site.
- **Auth**: workspace API keys (`dpk_…`) and human login sessions (`dps_…`) with owner/admin/viewer roles, workspace isolation on every private route, member management with last-owner guards. Signups must verify their email first: one-time `dpt_…` links go out through Resend (dev prints them to the API log), and password resets revoke every session on success. The dashboard keeps its session in an `HttpOnly` cookie and calls the API through a same-origin `/api` proxy with middleware route guards; direct API clients keep using `Authorization: Bearer <token>`.
- **Reports**: summary with previous-period comparison, traffic, top pages, sources, countries, devices, and 15s real-time — all scoped to the selected project, date range (24H/7/30/90D presets or a custom `start_date`/`end_date` range), and project timezone.
- **Dashboard**: project + workspace switchers (persisted to URL/localStorage), install screen with copyable script tag, login/register/account pages.
- **Privacy/retention**: raw IPs never stored, per-project retention (30/90/180/365d) enforced hourly by a cleanup worker with observable `retention_runs`.
- **CI**: vet, gofmt, full Go suite with `-race` (unit + Postgres integration), tracker typecheck/tests/bundle, dashboard lint/build, and production image build.

## Quickstart

Requirements: Node.js + pnpm, Go, Docker + Docker Compose.

```bash
cp .env.example .env
docker compose up -d          # postgres + api (http://localhost:5000)
pnpm dev                      # dashboard at http://localhost:3000
```

Create your first account in the dashboard (**Register**), or via the API:

```bash
curl -X POST localhost:5000/v1/auth/register \
  -H 'Content-Type: application/json' \
  -d '{"email":"you@example.com","password":"choose-12-plus-chars","workspace_name":"My workspace"}'
# → "verification_required": true. A one-time link is emailed; in dev
#   it is printed to the API log instead. Consume it:
curl -X POST localhost:5000/v1/auth/verify-email \
  -H 'Content-Type: application/json' \
  -d '{"token":"dpt_…"}'
# then log in (login refuses unverified accounts):
curl -X POST localhost:5000/v1/auth/login \
  -H 'Content-Type: application/json' \
  -d '{"email":"you@example.com","password":"choose-12-plus-chars"}'
# → save the "token", send it as: Authorization: Bearer <token>
#   plus: X-Workspace-ID: <workspace_id>
```

Add one tag to your site (see **Install** in the dashboard for your tracking ID —
the endpoint is derived from the tracker's own origin, so no `data-endpoint` needed):

```html
<script src="https://analytics.example.com/analytics.js" data-project="dp_xxx" defer></script>
```

Optional geography: place `GeoLite2-City.mmdb` in `./geoip/` and set `GEOIP_DB_PATH=/geoip/GeoLite2-City.mmdb` (see `docs/tracking.md`). Without it, countries report as `Unknown`.

## Layout

```text
apps/dashboard/     Next.js dashboard
packages/tracker/   browser tracker (Tracker API, SPA-aware)
services/api/       Go API (migrations embed in binary, run at startup)
docs/               architecture, tracking, privacy notes
docker-compose.yml  local PostgreSQL + API
```

## Testing

```bash
cd services/api && go test ./...        # unit tests
# integration tests need Postgres:
# TEST_DATABASE_URL=postgres://devpulse:devpulse_dev_password@localhost:5433/devpulse_test?sslmode=disable go test ./...
pnpm --filter @devpulse/tracker test
pnpm --filter dashboard lint && pnpm --filter dashboard build
```

Pushes to `main` run all of this in GitHub Actions (`.github/workflows/ci.yml`).

## Principles

- Privacy by default and data minimization
- Simple architecture and reliable tracking
- Fast aggregated queries, trustworthy numbers
- Self-hosting and developer control

## License

MIT — see [LICENSE](LICENSE).
