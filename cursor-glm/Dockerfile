# Build stage
FROM golang:1.21-alpine AS builder

# Install necessary build tools
RUN apk add --no-cache git

# Set working directory
WORKDIR /app

# Copy go mod files first for better caching
COPY go.mod go.sum ./

# Download dependencies
RUN go mod download

# Copy source files
COPY . .

# Build the GLM proxy
RUN CGO_ENABLED=0 GOOS=linux go build -o proxy proxy-glm.go

# Final stage
FROM alpine:latest

# Install ca-certificates for HTTPS requests
RUN apk --no-cache add ca-certificates

# Set working directory
WORKDIR /app

# Copy the binary from builder
COPY --from=builder /app/proxy .

# Expose port 9000
EXPOSE 9000

# Run the application
CMD ["./proxy"]
