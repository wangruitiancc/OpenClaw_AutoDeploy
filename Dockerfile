# syntax=docker/dockerfile:1

FROM golang:1.24 AS builder
WORKDIR /src
COPY go.mod go.sum ./
RUN /usr/local/go/bin/go mod download
COPY . .

ARG TARGET=control-plane-api
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 /usr/local/go/bin/go build -o /out/app ./cmd/${TARGET}

FROM debian:bookworm-slim
RUN apt-get update && apt-get install -y --no-install-recommends ca-certificates docker.io && rm -rf /var/lib/apt/lists/*
WORKDIR /app
COPY --from=builder /out/app /usr/local/bin/app
ENTRYPOINT ["/usr/local/bin/app"]
