# syntax=docker/dockerfile:1

FROM golang:1.26-alpine AS go-build

WORKDIR /src/backend

RUN apk add --no-cache ca-certificates

COPY backend/go.mod backend/go.sum ./
RUN go mod download

COPY backend/ .

RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /out/server ./cmd/server

FROM node:22-alpine AS node-deps

WORKDIR /app

COPY frontend/package.json frontend/package-lock.json ./
RUN npm ci

FROM node:22-alpine AS node-build

WORKDIR /app

COPY --from=node-deps /app/node_modules ./node_modules
COPY frontend/ .

ENV NEXT_TELEMETRY_DISABLED=1
RUN npm run build

FROM node:22-alpine AS runtime

RUN apk add --no-cache ca-certificates wget

WORKDIR /app

ENV NODE_ENV=production
ENV NEXT_TELEMETRY_DISABLED=1
ENV HOSTNAME=0.0.0.0
ENV PORT=3000
ENV BACKEND_URL=http://127.0.0.1:8080
ENV DATABASE_PATH=/data/factorymate.db

COPY --from=go-build /out/server /app/server
COPY --from=go-build /src/backend/data /app/data
COPY --from=node-build /app/public /app/frontend/public
COPY --from=node-build /app/.next/standalone /app/frontend
COPY --from=node-build /app/.next/static /app/frontend/.next/static

RUN addgroup --system --gid 10001 app \
    && adduser --system --uid 10001 --ingroup app app \
    && mkdir -p /data \
    && chown -R app:app /data /app

COPY scripts/docker-entrypoint.sh /app/docker-entrypoint.sh
RUN chmod +x /app/docker-entrypoint.sh

USER app

EXPOSE 3000

ENTRYPOINT ["/app/docker-entrypoint.sh"]
