# ==============================================================================
# Stage 1: Frontend Build (Vue 3 + Vite)
# ==============================================================================
FROM node:22-alpine AS frontend-builder
WORKDIR /app/web

# Copy package descriptors first to leverage caching
COPY web/package*.json ./
RUN if [ -f package.json ]; then npm install; fi

# Copy source code and build
COPY web/ ./
RUN if [ -f package.json ]; then npm run build; else mkdir -p dist && echo "<h1>TeslaCost</h1>" > dist/index.html; fi

# ==============================================================================
# Stage 2: Development environment for Go
# ==============================================================================
FROM golang:alpine AS dev
WORKDIR /app
ENV GOTOOLCHAIN=auto
RUN apk add --no-cache git curl build-base
COPY go.mod go.sum* ./
RUN go mod download
COPY . .
CMD ["go", "run", "./cmd/server/main.go"]

# ==============================================================================
# Stage 3: Backend Build (Go binary)
# ==============================================================================
FROM golang:alpine AS backend-builder
WORKDIR /app
ENV GOTOOLCHAIN=auto
RUN apk add --no-cache ca-certificates git

COPY go.mod go.sum* ./
RUN go mod download

COPY . .
# Copy compiled frontend assets to web/dist
COPY --from=frontend-builder /app/web/dist ./web/dist

# Build statically linked Go binary
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o /app/teslacost ./cmd/server

# ==============================================================================
# Stage 4: Production Runner (Scratch or Minimal Alpine)
# ==============================================================================
FROM alpine:3.20 AS prod
WORKDIR /app
RUN apk add --no-cache ca-certificates tzdata

COPY --from=backend-builder /app/teslacost /app/teslacost
COPY --from=backend-builder /app/migrations /app/migrations

EXPOSE 8080
ENV PORT=8080

CMD ["/app/teslacost"]
