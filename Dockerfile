# Stage 1: Build the Go application
FROM golang:1.23 AS builder

WORKDIR /app

# Copy Go module files and download dependencies
COPY go.mod go.sum ./
RUN go mod download

# Copy the rest of the application code
COPY . ./

# Build the Go application with static linking
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -a -installsuffix cgo -o main .

# Stage 2: Create a minimal image to run the application
FROM alpine:latest

WORKDIR /app

# Install runtime dependencies
RUN apk add --no-cache ca-certificates

# Copy the compiled binary from the builder stage
COPY --from=builder /app/main .

# Copy the .env file
COPY .env /app/.env

# Ensure the binary has execute permissions
RUN chmod +x main

# Expose the application port
EXPOSE 8080

# Set the binary as the entry point
CMD ["./main"]
