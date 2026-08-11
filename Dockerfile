# Local / ad-hoc image builds from source.
# Release images are built by GoReleaser via Dockerfile.goreleaser.
FROM golang:1.26-alpine AS builder
WORKDIR /src
COPY go.mod go.sum* ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -ldflags "-s -w" -o /tcp-wait ./cmd/tcp-wait

FROM alpine:3.24
LABEL org.opencontainers.image.source="https://github.com/donkeyx/tcp-wait"
LABEL org.opencontainers.image.title="tcp-wait"
LABEL org.opencontainers.image.description="Wait for network endpoints (tcp/http/redis/postgres/file)"
LABEL maintainer="David Binney <donkeysoft@gmail.com>"
RUN apk add --no-cache ca-certificates
COPY --from=builder /tcp-wait /usr/local/bin/tcp-wait
ENTRYPOINT ["/usr/local/bin/tcp-wait"]
