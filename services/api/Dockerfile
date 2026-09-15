FROM golang:1.25-alpine AS builder

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -o /devpulse-api ./cmd/server

FROM alpine:3.22

WORKDIR /app

COPY --from=builder /devpulse-api /devpulse-api

EXPOSE 4000

CMD ["/devpulse-api"]