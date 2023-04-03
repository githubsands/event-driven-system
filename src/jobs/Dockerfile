# Start from the official Golang base image
FROM golang:1.17 AS builder

# Set the working directory
WORKDIR /app

# Copy the Go source files
COPY . .

# Build the Golang app
RUN go build -o /usr/local/bin/jobs main.go

# Start a new stage from the minimal 'scratch' image
FROM scratch

# Copy the compiled binary from the builder stage
COPY --from=builder /usr/local/bin/jobs /usr/local/bin/jobs

# Set the entrypoint to the compiled binary
ENTRYPOINT ["/usr/local/bin/jobs"]
