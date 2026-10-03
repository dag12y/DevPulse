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
# /analytics-<version>.js. Built in CI from packages/tracker;
# missing files 404 until TRACKER_DIR is populated.
COPY tracker-dist /tracker-dist

EXPOSE 8080

CMD ["/devpulse-api"]