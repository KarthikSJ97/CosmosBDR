# --- Build Stage ---
FROM golang:alpine AS builder

WORKDIR /app

# Install git and ca-certificates required for downloading private/public modules over HTTPS
RUN apk add --no-cache git ca-certificates

ENV CGO_ENABLED=0

# Install dependencies first for caching
COPY go.mod go.sum ./
RUN go mod download

# Copy source code
COPY . .

# Build the binary statically
RUN GOOS=linux go build -o /app/bin/cosmosbdr src/main.go

# --- Runtime Stage ---
FROM alpine:3.19

RUN apk --no-cache add ca-certificates curl

WORKDIR /app

# Copy binary from builder
COPY --from=builder /app/bin/cosmosbdr /app/cosmosbdr

# COPY the migration schema directory from the builder stage
COPY --from=builder /app/src/db/migrations /app/src/db/migrations

EXPOSE 8080

CMD ["./cosmosbdr"]
