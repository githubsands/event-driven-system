# Build stage
FROM golang:1.17-alpine AS build

WORKDIR /app

RUN apk add --no-cache gcc musl-dev

# Copy source files to the container
COPY . .

# Build the binary
RUN go mod download && go build -o signer-worker

# Final stage
FROM alpine:latest

# Copy the binary from the build stage to the final image
COPY --from=build /app/signer-worker /usr/local/bin/signer-worker

EXPOSE 8080

# Set the binary as the entrypoint
ENTRYPOINT ["/usr/local/bin/signer-worker"]

