# Build stage
FROM golang:1.24-alpine AS builder

WORKDIR /app

# Install build dependencies
RUN apk add --no-cache git ca-certificates tzdata

# Cache Go modules
COPY go.mod ./
RUN go mod download || true

# Copy source code and build statically linked binary
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-w -s" -o /app/bin/api ./cmd/api

# Minimal runtime image
FROM alpine:3.21

RUN apk --no-cache add ca-certificates tzdata
WORKDIR /app

COPY --from=builder /app/bin/api /app/api

EXPOSE 8080

CMD ["/app/api"]

