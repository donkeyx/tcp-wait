BINARY_NAME = tcp-wait
VERSION     = $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
HASH        = $(shell git rev-parse --short HEAD 2>/dev/null || echo unknown)
LDFLAGS     = -ldflags "-s -w -X main.Version=$(VERSION) -X main.GitHash=$(HASH)"
MAIN_PKG    = ./cmd/tcp-wait

.PHONY: all build test clean tidy snapshot release-dry-run docker-build

all: tidy test build

build:
	CGO_ENABLED=0 go build $(LDFLAGS) -o bin/$(BINARY_NAME) $(MAIN_PKG)

test:
	go test -race -count=1 ./...

tidy:
	go mod tidy

clean:
	go clean
	rm -rf bin/ dist/

# Local multi-stage image from source (not the release path).
docker-build:
	docker build -t $(BINARY_NAME):local .

# GoReleaser snapshot (binaries + local docker load). Needs goreleaser + docker.
snapshot:
	goreleaser release --snapshot --clean

# Validate config / build without publishing.
release-dry-run:
	goreleaser release --clean --skip=publish
