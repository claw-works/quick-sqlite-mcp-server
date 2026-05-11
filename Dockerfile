FROM golang:1.22-alpine AS builder

# Install build dependencies for CGO (go-sqlite3 requires gcc)
RUN apk add --no-cache gcc musl-dev sqlite-dev

WORKDIR /build

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=1 GOOS=linux go build -ldflags="-s -w" -o quick-sqlite-mcp-server .

# ── Final image ───────────────────────────────────────────────────────────────
FROM alpine:3.19

RUN apk add --no-cache sqlite-libs ca-certificates

WORKDIR /app
COPY --from=builder /build/quick-sqlite-mcp-server .

# Default data directory (mount your databases here)
VOLUME ["/data"]

ENV ROOT_DIR=/data
ENV ALLOW_ABSOLUTE_PATH=false

ENTRYPOINT ["/app/quick-sqlite-mcp-server"]
