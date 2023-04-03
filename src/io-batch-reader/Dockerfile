# Build stage
FROM golang:1.17-alpine AS build

WORKDIR /app

RUN apk add --no-cache gcc musl-dev

# Copy source files to the container
COPY . .

# Build the binary
RUN go mod download && go build -o io-batch-reader

# Final stage
FROM alpine:latest

# Copy the binary from the build stage to the final image
COPY --from=build /app/io-batch-reader /usr/local/bin/io-batch-reader

# Set the binary as the entrypoint
ENTRYPOINT ["/usr/local/bin/io-batch-reader"]
