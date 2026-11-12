# syntax=docker/dockerfile:1

FROM golang:1.22-alpine AS builder

RUN apk add --no-cache git ca-certificates

WORKDIR /src
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
