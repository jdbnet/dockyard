# ── Stage 1: Build Vue SPA ──
FROM node:20-alpine AS frontend
WORKDIR /build
COPY ui/package*.json ./
RUN npm ci --no-audit --no-fund 2>/dev/null || npm install --no-audit --no-fund
COPY ui/ ./
RUN npm run build

# ── Stage 2: Build Go server ──
FROM golang:1.27-alpine AS server
WORKDIR /build
COPY go.mod go.sum ./
RUN go mod download
COPY . .
COPY --from=frontend /build/dist ./ui/dist
ARG VERSION=dev
RUN CGO_ENABLED=0 go build -ldflags="-s -w -X github.com/jdbnet/dockyard/internal/version.Version=${VERSION}" -o dockyard ./cmd/dockyard

# ── Stage 3: Runtime ──
FROM alpine:3.24
RUN apk add --no-cache ca-certificates tzdata wget
WORKDIR /app
COPY --from=server /build/dockyard .
COPY config.yaml ./config.yaml
ENV DOCKYARD_WEB_BIND=0.0.0.0
EXPOSE 8080
HEALTHCHECK --interval=30s --timeout=5s --retries=3 \
  CMD wget -qO- http://127.0.0.1:8080/api/v1/health || exit 1
CMD ["./dockyard", "--headless", "--web"]
