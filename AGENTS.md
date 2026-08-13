# Agent Instructions

## Repository Shape

- This is the Go module `github.com/donkeyx/tcp-wait/v2`; it requires Go 1.26+ and has no external module dependencies or `go.sum`.
- The module root is the importable `tcpwait` library. The CLI entrypoint is `cmd/tcp-wait`.
- Supported probes are TCP, HTTP, Redis, PostgreSQL, and filesystem paths. Redis and PostgreSQL probes speak enough protocol to verify readiness, so do not reduce them to a TCP-connect check.

## Development

- Run `make test` for the standard test command; it runs `go test -race -count=1 ./...`.
- Run `go test -race ./...` for a focused direct verification, or `go test -race -run TestName ./...` for one named test.
- Run `make build` to build the CLI with `CGO_ENABLED=0` and version/hash linker flags into `bin/tcp-wait`.
- `make` runs `go mod tidy`, then tests, then build. Do not assume `go mod tidy` is harmless if intentionally changing module metadata.
- CI additionally builds and runs `bin/tcp-wait -version`, then verifies an unreachable TCP target exits non-zero within the timeout.
- There is no repository lint or formatter configuration; use standard `gofmt` on changed Go files.

## Release And Docker

- `make docker-build` builds the local source image `tcp-wait:local`; it requires Docker.
- `make snapshot` requires Docker and GoReleaser and creates a local snapshot release. `make release-dry-run` validates/builds without publishing.
- Releases are tag-driven: pushing a `v*` tag runs GoReleaser for binaries and multi-architecture Docker images. Do not trigger this casually.
- The release image uses `Dockerfile.goreleaser`; the ordinary `Dockerfile` is only for local source builds.

## Behavior Constraints

- CLI wait mode exits `0` when all checks pass, `1` on timeout/bad input, and `130` on interruption. Watch mode cannot be combined with a command to exec and treats Ctrl-C as success.
- `-t 0` means no deadline, primarily for watch mode. The CLI defaults to JSON logs; use `-o text` for human-readable output.
- `Wait` and `Watch` validate non-empty checks and targets. `Wait` runs checks concurrently and returns an error wrapping `ErrTimeout` on deadline; library callers should use `errors.Is` rather than match error text.
- When changing the public API or probe semantics, update the README examples and relevant tests; the README is also published as the Docker Hub description.

## Future Feature Ideas

- Environment-variable configuration for container deployments; flags should remain authoritative if added.
- Configurable retry budgets or exponential backoff while preserving the current predictable fixed-interval defaults.
- Unix-socket checks, TCP banner matching, and richer final result summaries.
- Redis authentication and PostgreSQL TLS/authentication if protocol-level readiness requirements justify them.
- Shell completion or subcommands only if the flat CLI grows substantially.
