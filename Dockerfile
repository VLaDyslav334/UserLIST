FROM golang:1.24.3 as builder

WORKDIR /app

# Copy Go dependencies
COPY .. .

# Download Go module dependencies
RUN go mod tidy

# Build Go app binary
RUN go build -o main ./cmd/

# Expose the application port
EXPOSE 8080

# Start the application
CMD ["./main"]