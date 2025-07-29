# -------- Stage 1: Build --------
FROM golang:1.24-alpine AS builder

# Install git (needed for go mod download sometimes)
RUN apk add --no-cache git

WORKDIR /app

# Cache go.mod and download dependencies
COPY go.mod go.sum ./
RUN go mod download

# Copy the rest of the code
COPY . .

# Build the Go binary statically
RUN CGO_ENABLED=0 GOOS=linux go build -o server .

# -------- Stage 2: Runtime --------
FROM alpine:latest

# Install mtr
RUN apk add --no-cache mtr

# Copy the compiled binary from builder
COPY --from=builder /app/server /server

# Set entrypoint
ENTRYPOINT ["/server"]

# Expose HTTP port
EXPOSE 8080
