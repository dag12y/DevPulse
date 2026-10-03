# DevPulse Production Runbook (single Azure VPS)

Target topology: one VPS running the whole stack via compose — Postgres
(container + local volume), API (tracker bundle baked in), dashboard,
with an optional Caddy sidecar for TLS once domains exist.

```
                    ┌──────────────────────────────────────┐
                    │               Azure VPS              │
  visitors ────────►│  :5000 API  /analytics.js + /v1/...  │
  you ─────────────►│  :3000 dashboard                     │
                    │  postgres (no published port)        │
                    └──────────────────────────────────────┘
```

> Until HTTPS is enabled (see below), restrict dashboard/API access to
> your own IP or a VPN. Session tokens over plain HTTP can be
> intercepted on untrusted networks.

## 1. Provision the VPS

- Size: 2 vCPU / 4 GB RAM (e.g. `Standard_B2s`) is plenty to start;
  30–50 GB disk (analytics + Postgres grow with traffic × retention).
- Networking (NSG): allow inbound **22** (your IP only), **5000** and
  **3000** (your IP only, until TLS). Deny everything else inbound.
- OS: Ubuntu 24.04 LTS. Create a non-root sudo user, disable SSH
  password auth (`PasswordAuthentication no`), enable
  `unattended-upgrades`.
- Install Docker Engine via the official Docker apt repository
  (includes the compose v2 plugin). Enable + start the service.
- Optional: enable UFW as a second layer (`ufw allow 22/tcp`,
  `ufw allow 5000/tcp`, `ufw allow 3000/tcp`, `ufw enable`).

## 2. First deploy

```bash
# On the VPS as your deploy user:
git clone <your-repo-url> /opt/DevPulse
cd /opt/DevPulse

# Generate .env.prod (writes a random Postgres password, mode 600):
./scripts/new-prod-env.sh http://<VPS_IP>:5000 http://<VPS_IP>:3000

# Optional: enable country/region enrichment:
#   place GeoLite2-City.mmdb in ./geoip/ and set in .env.prod:
#   GEOIP_DB_PATH=/geoip/GeoLite2-City.mmdb

# Build images (dashboard bakes in PUBLIC_API_URL) and start:
docker compose --env-file .env.prod \
  -f docker-compose.yml -f docker-compose.prod.yml up -d --build
```

Migrations run automatically inside the API on startup; the retention
worker starts with it. Nothing else to initialize.

## 3. Smoke test (do this on every deploy)

```bash
API=http://<VPS_IP>:5000

# Liveness + readiness (readiness pings Postgres, ~2s timeout):
curl -sf $API/health && echo
curl -sf $API/ready && echo

# Tracker bundle serves with cache headers:
curl -sI $API/analytics.js | grep -i cache-control
```

Then in the dashboard (`http://<VPS_IP>:3000`): register an account,
create a workspace + project, copy the Install snippet onto any page
(the snippet includes the correct `data-endpoint`), load the page,
and confirm you appear under Real-time within ~15 seconds.

Useful probes afterwards:

```bash
# Ingestion outcome counters (accepted / invalid / rate_limited / ...):
curl -s $API/metrics | grep -E "events_ingested|report_requests|retention"
docker compose --env-file .env.prod -f docker-compose.yml -f docker-compose.prod.yml logs --tail=50 api
```

## 4. Backups

Nightly dumps via cron on the host:

```bash
# As root or the deploy user with docker access:
crontab -e
# 0 2 * * * cd /opt/DevPulse && ./scripts/backup-postgres.sh >> /var/log/devpulse-backup.log 2>&1
```

- Dumps land in `./backups/` (add more disk before they fill it;
  `BACKUP_KEEP=14` by default, override via env).
- Off-VPS copy: install `azcopy`, create an Azure Blob container,
  set `AZCOPY_DEST="<container SAS URL>"` in the cron environment.
  Without this step a dead disk still loses everything.
- Verify monthly: check dump sizes are non-zero and growing, and run
  a trial restore (next section) against a scratch database.

## 5. Restore procedure

```bash
cd /opt/DevPulse
FILE=backups/devpulse-<STAMP>.sql.gz   # newest good dump

# Stop writers (keeps the schema intact, drops data first):
docker compose --env-file .env.prod \
  -f docker-compose.yml -f docker-compose.prod.yml stop api dashboard

# Wipe + reload (destructive — confirm the filename twice):
set -a; source .env.prod; set +a
docker exec devpulse-postgres psql -U "$POSTGRES_USER" -d "$POSTGRES_DB" \
  -c "DROP SCHEMA public CASCADE; CREATE SCHEMA public;"
gunzip -c $FILE | docker exec -i devpulse-postgres \
  psql -U "$POSTGRES_USER" -d "$POSTGRES_DB" -v ON_ERROR_STOP=1

# Restart and re-run smoke tests:
docker compose --env-file .env.prod \
  -f docker-compose.yml -f docker-compose.prod.yml start api dashboard
curl -sf http://<VPS_IP>:5000/ready
```

`POSTGRES_USER`/`POSTGRES_DB` come from `.env.prod` (sourced above).
Migrations are idempotent, so a dump taken at any version restores
cleanly; the API re-applies any missing migrations on boot.

## 6. Rollback

Data and code deploy independently here:

- **Bad code deploy**: `git log` → `git revert <sha>` (or checkout the
  previous tag) → `up -d --build` again → smoke tests. Postgres data
  is untouched by rebuilds.
- **Bad data event** (e.g. wrong retention setting wiped rows):
  restore from the newest pre-incident dump (section 5). There is no
  undelete; this is what the off-VPS blob copy is for.
- Never `docker volume rm devpulse_postgres_data` unless you have a
  verified dump elsewhere.

## 7. Enabling HTTPS (when you own domains)

1. Create A records: dashboard host and API host → VPS IP.
2. Regenerate env with https origins and rebuild (dashboard bundle
   bakes the origins in):
   ```bash
   ./scripts/new-prod-env.sh https://api.example.com https://pulse.example.com
   # add to .env.prod:
   #   DASHBOARD_HOST=pulse.example.com
   #   API_HOST=api.example.com
   docker compose --env-file .env.prod \
     -f docker-compose.yml -f docker-compose.prod.yml -f docker-compose.tls.yml \
     up -d --build
   ```
3. Caddy issues certificates on first request; watch
   `docker logs devpulse-caddy` for issuance errors.
4. Close ports 5000/3000 in the NSG; open 80/443 to the world.
5. Update `CORS_ALLOWED_ORIGINS` if the dashboard host ever changes
   (API restart picks it up; no rebuild needed for the API).

## 8. Routine operations

- **Logs**: `docker compose -f docker-compose.yml -f docker-compose.prod.yml logs -f api`
  (JSON in production; capped at 5×10 MB per container by the overlay).
- **Disk**: watch `df -h` and `docker system df`. Postgres grows with
  traffic × retention days; lower a project's retention or prune
  `./backups/` if needed. `docker system prune` monthly.
- **Updates**: `apt upgrade` (unattended-upgrades covers security),
  `docker compose pull` for base images (postgres/caddy) quarterly,
  then `up -d` + smoke tests.
- **Metrics to alert on** (scrape `/metrics` or curl in cron):
  `events_ingested_total{outcome="error"}` rising,
  `retention_runs_total{status="error"}` > 0,
  `/ready` non-200, disk > 80%.
