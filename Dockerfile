# syntax=docker/dockerfile:1

FROM golang:1.26-alpine AS go-build

WORKDIR /src/backend

RUN apk add --no-cache ca-certificates

COPY VERSION /VERSION
COPY backend/go.mod backend/go.sum ./
RUN go mod download

COPY backend/ .

COPY docs/FactoryGame-Docs.json /planner-data/FactoryGame-Docs.json
COPY assets/icons /planner-data/icons
COPY assets/icons.json /planner-data/icons.json

RUN APP_VERSION="$(cat /VERSION | tr -d '[:space:]')" && \
    CGO_ENABLED=0 GOOS=linux go build -trimpath \
    -ldflags="-s -w -X main.appVersion=${APP_VERSION}" \
    -o /out/server ./cmd/server

FROM node:22-alpine AS node-deps

WORKDIR /app

COPY frontend/package.json frontend/package-lock.json ./
RUN npm ci

FROM node:22-alpine AS node-build

WORKDIR /app

COPY --from=node-deps /app/node_modules ./node_modules
COPY frontend/ .
COPY VERSION /VERSION
COPY assets/icons /src/assets/icons
COPY assets/icons.json /src/assets/icons.json
COPY scripts/sync-item-icons.mjs /src/scripts/sync-item-icons.mjs

ENV NEXT_TELEMETRY_DISABLED=1
RUN node /src/scripts/sync-item-icons.mjs && npm run build

FROM node:22-alpine AS runtime

RUN apk add --no-cache ca-certificates wget

WORKDIR /app

ENV NODE_ENV=production
ENV NEXT_TELEMETRY_DISABLED=1
ENV HOSTNAME=0.0.0.0
ENV PORT=3000
ENV BACKEND_URL=http://127.0.0.1:8080
ENV DATABASE_PATH=/data/factorymate.db
ENV PLANNER_CATALOG_PATH=/app/data/factory_catalog.json
ENV PLANNER_DOCS_PATH=/app/planner-data/FactoryGame-Docs.json
ENV PLANNER_ICONS_DIR=/app/planner-data/icons
ENV PLANNER_ICONS_JSON=/app/planner-data/icons.json

COPY --from=go-build /out/server /app/server
COPY --from=go-build /src/backend/data /app/data
COPY --from=go-build /planner-data /app/planner-data
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
