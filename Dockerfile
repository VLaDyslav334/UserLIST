# Start from the official Go image
FROM golang:1.25.3-bookworm AS builder

# Set working directory
WORKDIR /app

# Copy go.mod and go.sum files
COPY go.mod go.sum* ./

## COPY config.yml .
#COPY .env ./

# Download dependencies
RUN go mod download

# Copy the source code
COPY . .

# Build the application
RUN go build -o gin-app ./cmd/main.go

# Use a slim debian image for the final stage
FROM debian:bookworm-slim

# Set working directory
WORKDIR /usr/bin/app

# Copy the binary from the builder stage
COPY --from=builder /app/gin-app .

# COPY --from=builder /app/config.yml .

# Expose the application port
EXPOSE 8080

# Command to run the application
CMD ["./gin-app"]
