# DevPulse dashboard production image.
#
# Build from the repository root so the pnpm workspace resolves:
#   docker build -f apps/dashboard/Dockerfile \
#     --build-arg NEXT_PUBLIC_API_URL=https://api.example.com \
#     -t devpulse-dashboard .
#
# NEXT_PUBLIC_* values bake into the bundle at build time: rebuilding
# (and redeploying) the image is required when the public API origin
# changes. NEXT_PUBLIC_TRACKER_URL falls back to
# <NEXT_PUBLIC_API_URL>/analytics.js when unset.
FROM node:22-alpine AS builder

ENV NEXT_TELEMETRY_DISABLED=1
WORKDIR /app

RUN corepack enable && corepack prepare pnpm@10.34.5 --activate

COPY pnpm-workspace.yaml pnpm-lock.yaml package.json ./
COPY apps/dashboard/package.json apps/dashboard/package.json
COPY packages/tracker/package.json packages/tracker/package.json
RUN pnpm install --frozen-lockfile

COPY . .

ARG NEXT_PUBLIC_API_URL=http://localhost:5000
ARG NEXT_PUBLIC_TRACKER_URL=""
ENV NEXT_PUBLIC_API_URL=${NEXT_PUBLIC_API_URL}
ENV NEXT_PUBLIC_TRACKER_URL=${NEXT_PUBLIC_TRACKER_URL}
RUN pnpm --filter dashboard build

FROM node:22-alpine AS runner

ENV NODE_ENV=production PORT=3000 NEXT_TELEMETRY_DISABLED=1
WORKDIR /app

RUN adduser -D -H -u 10001 nextjs

COPY --from=builder --chown=nextjs:nextjs /app/apps/dashboard/.next/standalone ./
COPY --from=builder --chown=nextjs:nextjs /app/apps/dashboard/.next/static ./apps/dashboard/.next/static
COPY --from=builder --chown=nextjs:nextjs /app/apps/dashboard/public ./apps/dashboard/public

USER nextjs

EXPOSE 3000

HEALTHCHECK --interval=30s --timeout=5s --start-period=10s --retries=3 \
  CMD wget -q -O /dev/null http://127.0.0.1:3000/ || exit 1

CMD ["node", "apps/dashboard/server.js"]
