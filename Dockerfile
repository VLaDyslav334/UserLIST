FROM golang:1.24.3 as builder

WORKDIR /app

# Copy Go dependencies
COPY go.mod go.sum ./

# Download Go module dependencies
RUN go mod download

# Copy application source code
COPY . .

# Build Go app binary
RUN go build -o main ./cmd/

# Copy the config file
COPY config.yml ./config.yml


# Final stage
FROM debian:bookworm-slim

WORKDIR /app

# Copy the binary from builder
COPY --from=builder /app/main .

# Copy database wait script
COPY wait-for-postgres.sh /app/wait-for-postgres.sh

# Make the wait-for-postgres.sh script executable
RUN chmod +x /app/wait-for-postgres.sh

# Expose the application port
EXPOSE 8080

# Start the application
CMD ["./main"]