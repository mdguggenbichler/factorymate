# syntax=docker/dockerfile:1

FROM golang:1.26-alpine AS build

WORKDIR /src

RUN apk add --no-cache ca-certificates

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /out/server ./cmd/server

FROM alpine:3.21

RUN apk add --no-cache ca-certificates wget

WORKDIR /app

COPY --from=build /out/server /app/server
COPY data/ /app/data/

RUN adduser -D -H -u 10001 appuser \
    && mkdir -p /data \
    && chown appuser:appuser /data

USER appuser

ENV PORT=8080
ENV DATABASE_PATH=/data/factorymate.db

EXPOSE 8080

ENTRYPOINT ["/app/server"]
