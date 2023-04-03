FROM golang:1.17-alpine AS build

WORKDIR /app

RUN apk add --no-cache gcc musl-dev

COPY . .

RUN go mod download && go build -o keymanager

FROM alpine:latest

COPY --from=build /app/keymanager /usr/local/bin/keymanager

EXPOSE 8080

ENTRYPOINT ["/usr/local/bin/keymanager"]
