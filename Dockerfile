# syntax=docker/dockerfile:1
#
# GoIrbis подключается через replace в go.mod (../GoIrbis).
# Собирайте через docker compose (additional_contexts) или:
#   docker build -f Dockerfile --build-context goirbis=../GoIrbis .

FROM golang:1.22-alpine AS builder

RUN apk add --no-cache git ca-certificates

WORKDIR /src/irbis-api

# Локальный модуль рядом с irbis-api — путь как в replace: ../GoIrbis
COPY --from=goirbis / /src/GoIrbis

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /out/irbis-api ./cmd/server

FROM alpine:3.20

RUN apk add --no-cache ca-certificates tzdata \
	&& adduser -D -H -u 10001 app

WORKDIR /app

COPY --from=builder /out/irbis-api /app/irbis-api
COPY templates /app/templates

USER app
EXPOSE 8080

ENV HTTP_ADDR=:8080 \
	TEMPLATES_DIR=/app/templates

ENTRYPOINT ["/app/irbis-api"]
