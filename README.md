# DevPulse

DevPulse is a lightweight, privacy-conscious web analytics platform for developers. It will let website owners add a small tracking script and view useful traffic analytics in a simple dashboard.

## Status

The Go API exposes health checks, project CRUD, and analytics event ingestion with CORS enabled. The browser tracker is implemented and produces events compatible with the API. PostgreSQL and the API are defined in Docker Compose. The Next.js dashboard is still the default scaffold — analytics views are planned work.

## Planned Features

- Page-view tracking, unique visitors, and sessions
- New and returning visitors, session duration, and bounce rate
- Referrers, traffic sources, and UTM campaigns
- Countries, devices, browsers, operating systems, and screen information
- Real-time visitors and configurable retention
- Privacy-conscious, first-party visitor tracking

## Technology

- Dashboard: Next.js, TypeScript, and Tailwind CSS
- API: Go
- Data store: PostgreSQL
- Tooling: pnpm, Docker, and Docker Compose

## Intended Architecture

```text
Website → browser tracker → Go API → PostgreSQL
                              ↑        ↓
                         Dashboard ← aggregated analytics API
```

For the detailed design, see [architecture.md](docs/architecture.md), [tracking.md](docs/tracking.md), and [privacy.md](docs/privacy.md).

## Repository Layout

```text
apps/dashboard/     Next.js dashboard
services/api/        Go API
docs/                Product and technical design notes
docker-compose.yml   Local PostgreSQL and API services
```

The future tracker package will live in `packages/tracker/`.

## Local Development

### Requirements

- Node.js and pnpm
- Go
- Docker and Docker Compose

### Start services

1. Copy `.env.example` to `.env` and adjust values if necessary.
2. Start PostgreSQL: `docker compose up -d postgres`
3. Start the API: `docker compose up -d api`
4. Start the dashboard: `pnpm dev`

The dashboard runs at <http://localhost:3000>. The API health endpoint is available at <http://localhost:8080/health>.

## Development Principles

- Privacy by default and data minimization
- Simple architecture and reliable tracking
- Fast analytics ingestion and useful metrics
- Self-hosting and developer control

## License

TBD
