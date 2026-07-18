# Multi-stage build for DigitalOcean App Platform / any container host.
FROM golang:1.26-alpine AS builder

WORKDIR /src

RUN apk add --no-cache git ca-certificates

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /out/shadow-llm-evaluator .

FROM alpine:3.22

RUN apk add --no-cache ca-certificates tzdata \
  && adduser -D -H -u 10001 appuser

WORKDIR /app

COPY --from=builder /out/shadow-llm-evaluator /app/shadow-llm-evaluator
COPY env/ /app/env/

USER appuser

ENV APP_ENV=local \
    ADDR=:8080 \
    ENV_DIR=env

EXPOSE 8080

ENTRYPOINT ["/app/shadow-llm-evaluator"]
