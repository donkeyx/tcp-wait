# tcp-wait

```
╭──────────────────────────────────────╮
│   🐴 DonkeyX's tcp-wait              │
╰──────────────────────────────────────╯

        //\\
       (/oo\)   .--------.
       (____)  | WAIT... |
        /||\   '--------'
       //||\\   📡 ping / ready / blip
      ^^ ^^ ^^
   "Is postgres back yet?"
```

Small tool to wait on stuff being up before you start the real process. Started life as a container pre-start helper; now it’s also an importable Go package, and has a watch mode for those upgrade / firewall blips.

Works as:

- **CLI** — `tcp-wait` in Docker / k8s / scripts
- **Library** — `import "github.com/donkeyx/tcp-wait/v2"`

| dockerhub | https://hub.docker.com/r/donkeyx/tcp-wait |
| ghcr | `ghcr.io/donkeyx/tcp-wait` |
| github | https://github.com/donkeyx/tcp-wait |
| docs | https://pkg.go.dev/github.com/donkeyx/tcp-wait/v2 |

Same donkey stable as [cluster-utils-api](https://github.com/donkeyx/cluster-utils-api) / [cluster-utils](https://github.com/donkeyx/cluster-utils) — this one’s the “wait until the other thing is actually up” bit.

## What it can wait on

| Kind | Ready when |
|------|------------|
| `tcp` | port accepts a connection |
| `http` | `GET` returns 200 (or whatever status you ask for) |
| `redis` | `PING` gets `PONG` (not just “port is open”) |
| `postgres` | answers a startup message (skips “starting up” / recovery noise) |
| `file` | path exists (marker file, socket path, etc.) |

TCP alone lies sometimes — redis can still be `LOADING`, postgres can accept sockets while it’s not ready. The redis/postgres checks speak just enough protocol that you don’t need `go-redis` / `pgx` hanging off this thing.

**License is MIT** — use it however you like; keep the copyright notice.
## Install

This is **v2** (library + probes + watch). Module path has the `/v2` suffix — that’s normal Go for majors.

```bash
# docker
docker run --rm donkeyx/tcp-wait:latest -version
docker run --rm ghcr.io/donkeyx/tcp-wait:latest -version

# go (note: cmd path — root is the library now)
go install github.com/donkeyx/tcp-wait/v2/cmd/tcp-wait@latest

# or grab a release binary
# https://github.com/donkeyx/tcp-wait/releases
```

Library:

```bash
go get github.com/donkeyx/tcp-wait/v2@latest
```

v1 was CLI-only at the module root (`go install github.com/donkeyx/tcp-wait@v1.0.2` still works if you need the old binary).

## CLI

```bash
# basic tcp (-hp is the old flag, still works; -tcp is the same thing)
tcp-wait -hp github.com:443
tcp-wait -tcp db:5432,cache:6379 -t 60

# http readiness
tcp-wait -http http://api:8080/readyz -t 30

# expected status and repeatable request headers
tcp-wait -http https://api:8443/readyz -http-status 204 \
  -http-header 'Authorization: Bearer token' -t 30

# TLS TCP/Redis probes; certificate verification remains enabled by default
tcp-wait -tls -tcp db:5432 -redis cache:6379 -t 60

# actually talk redis / postgres
tcp-wait -redis cache:6379 -postgres db:5432 -t 60

# wait for a file someone else writes
tcp-wait -file /var/run/app.ready -t 30

# wait, then exec your app (handy as a container entrypoint)
tcp-wait -tcp db:5432 -http http://api:8080/readyz \
  -t 60 -dial 2s -interval 500ms \
  -- /app/server --config /config.yaml
```

### Watch mode (the upgrade / firewall thing)

Normal mode exits as soon as everything is green. Watch mode keeps poking and only logs **changes** — up, down, or a **blip** (was up, now down). Super useful when you’re bouncing postgres/redis or waiting for a security group to actually open.

```bash
# sit on it until Ctrl-C (-t 0 = no time limit)
tcp-wait -watch -postgres db:5432 -redis cache:6379 -t 0 -interval 500ms -o text

# or cap the window (e.g. firewall change)
tcp-wait -w -tcp $HOST:5432 -t 600 -interval 1s -o text
```

You’ll see something like:

```text
level=INFO msg="check up" check=postgres://db:5432
level=WARN msg="check blip" check=postgres://db:5432 err="..."
level=INFO msg="check up" check=postgres://db:5432
level=INFO msg="watch stopped" status="all up" reason=signal
```

`check blip` = it was good, then it wasn’t. That’s the hole during the upgrade.

### Flags

| Flag | Default | Notes |
|------|---------|--------|
| `-hp` / `-tcp` | | `host:port,...` TCP |
| `-http` | | URLs, GET, expect 200 |
| `-http-status` | `200` | expected status for every `-http` URL |
| `-http-header` | | repeatable `Name: value` request header |
| `-redis` | | `host:port,...` PING |
| `-postgres` | | `host:port,...` startup probe |
| `-file` | | paths that must exist |
| `-t` | `20` | overall timeout seconds; **`0` = no limit** |
| `-dial` | `1s` | per-attempt timeout |
| `-interval` | `1s` | sleep between attempts |
| `-o` | `json` | `json` or `text` |
| `-q` | off | errors only (skip this for watch — you’ll hide the interesting lines) |
| `-w` / `-watch` | off | continuous up/down/blip logging |
| `-tls` | off | TLS for TCP/Redis; configure HTTPS TLS settings |
| `-tls-server-name` | | TLS certificate server-name override |
| `-tls-insecure-skip-verify` | off | disable TLS certificate verification; development only |
| `-version` | | version + git hash |
| `-- cmd…` | | after wait succeeds, `exec` the command (don’t mix with `-watch`) |

**Logs:** `-o json` (default, fine for k8s/loki) or `-o text` (nicer in a terminal). Final wait results include `status` (`ready`, `timeout`, or `error`) and `checks` fields for machine consumers.

**Exits:**

- wait: `0` ready, `1` timeout / bad flags, `130` if you Ctrl-C
- watch: `0` on Ctrl-C, or on timeout if everything’s up; `1` if the timer runs out still degraded

### k8s initContainer

```yaml
initContainers:
  - name: wait-deps
    image: donkeyx/tcp-wait:latest
    args: ["-postgres", "db:5432", "-redis", "cache:6379", "-t", "120", "-o", "text"]
```

### compose / entrypoint wrap

```yaml
command: ["tcp-wait", "-tcp", "db:5432", "-t", "60", "--", "/app/server"]
```

## Library

```go
package main

import (
	"context"
	"errors"
	"log"
	"time"

	tcpwait "github.com/donkeyx/tcp-wait/v2"
)

func main() {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	err := tcpwait.Wait(ctx, []tcpwait.Check{
		tcpwait.TCP("db:5432"),
		tcpwait.Redis("cache:6379"),
		tcpwait.Postgres("db:5432"),
		tcpwait.HTTP("http://api:8080/readyz"),
		tcpwait.File("/var/run/app.ready"),
	})
	if err != nil {
		// errors.Is(err, tcpwait.ErrTimeout) — message lists who never came up
		log.Fatal(err)
	}

	// knobs if you need them
	_ = tcpwait.WaitWithOptions(ctx, []tcpwait.Check{tcpwait.TCP("db:5432")}, tcpwait.Options{
		DialTimeout: time.Second,
		Interval:    500 * time.Millisecond,
	})

	// watch until ctx is done (timeout / cancel)
	res, _ := tcpwait.WatchWithOptions(ctx, []tcpwait.Check{
		tcpwait.Postgres("db:5432"),
		tcpwait.Redis("cache:6379"),
	}, tcpwait.Options{Interval: 500 * time.Millisecond})
	if !res.AllUp() {
		log.Printf("still down: %v", res.Down())
	}
}
```

TCP-only shortcut: `tcpwait.WaitTCP(ctx, []string{"db:5432", "redis:6379"})`.

Docs on [pkg.go.dev](https://pkg.go.dev/github.com/donkeyx/tcp-wait/v2) (`go doc .` works locally either way).

## Develop

Needs Go 1.26+.

```bash
go test -race ./...
go build -o bin/tcp-wait ./cmd/tcp-wait

make test
make build
make docker-build   # local image tcp-wait:local
```

### CI / release (same shape as cluster-utils-api)

| Workflow | When | What |
|----------|------|------|
| **CI** | PR / master | test, build, smoke, goreleaser check |
| **Docker** | PR / master | PR = amd64 build only; push = multi-arch + GHA cache → Hub/GHCR |
| **Release** | `v*` tag | GoReleaser binaries + multi-arch images + Hub README sync |
| **CodeQL** | PR / master / weekly | scan |

Docker Hub auth matches **cluster-utils-api** (no `USER`/`PASS` leftovers):

| Name | Type | Environments |
|------|------|----------------|
| `DOCKERHUB_USERNAME` | **variable** | `ci` (docker workflow) + `deployment` (release) |
| `DOCKERHUB_TOKEN` | **secret** | same — Hub access token (Read/Write) |

Same values as the API repo is fine. Without `DOCKERHUB_USERNAME` set, Hub login is skipped (GHCR still works).

Hub **image push** can work while **README sync** returns 403 — token needs description rights, or ignore it; images still publish.

```bash
goreleaser check
make snapshot   # needs docker + goreleaser

git tag v2.0.1
git push origin v2.0.1
```

## License

MIT — see [LICENSE](LICENSE).
