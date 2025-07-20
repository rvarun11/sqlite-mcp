FROM golang:1.24.4-alpine AS builder

# Install build dependencies including GCC for CGO
RUN apk add --no-cache git sqlite gcc musl-dev

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=1 GOOS=linux go build -o bin/sqlite-mcp-server cmd/server/main.go

# Final stage
FROM alpine:latest

# Install sqlite and ca-certificates
RUN apk add --no-cache sqlite ca-certificates

WORKDIR /app

COPY --from=builder /app/bin/sqlite-mcp-server /usr/local/bin/sqlite-mcp-server

COPY example.sql /app/example.sql
RUN sqlite3 /app/database.db < /app/example.sql

RUN cat > /app/entrypoint.sh << 'EOF'
#!/bin/sh

DB_PATH="/app/database.db"

if [ -f "/data/schema.sql" ]; then
    sqlite3 "$DB_PATH" < "/data/schema.sql" 2>/dev/null || true
fi

exec sqlite-mcp-server --database "$DB_PATH" "$@"
EOF

RUN chmod +x /app/entrypoint.sh
RUN mkdir -p /data

ENTRYPOINT ["/app/entrypoint.sh"]
