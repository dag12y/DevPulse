FROM golang:1.25-alpine AS builder

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -o /devpulse-api ./cmd/server

FROM alpine:3.22

WORKDIR /app

COPY --from=builder /devpulse-api /devpulse-api
# Production tracker bundle served at /analytics.js and
# /analytics-<version>.js. Staged by ./scripts/build-tracker-dist.sh
# (or in CI from packages/tracker); a missing bundle 404s until
# TRACKER_DIR is populated, and the API logs a warning at startup.
COPY tracker-dist /tracker-dist

RUN adduser -D -H -u 10001 devpulse

USER devpulse

EXPOSE 8080

HEALTHCHECK --interval=30s --timeout=5s --start-period=10s --retries=3 \
  CMD wget -q -O /dev/null "http://127.0.0.1:${API_PORT:-8080}/ready" || exit 1

CMD ["/devpulse-api"]