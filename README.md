# tcp-wait

Go package to test and wait on the availability of a TCP host and port. This is mainly used as a pre
start script for containers which depend on other services before starting. You can run this first
with a defined interval and die if the host:port does not become available.

Feel free to contribute, as I will allow most merges as long as they don't drastically break the usage.

## Quickstart

Using docker:

```bash
# simply run the docker run command and you will be good to go immediately
$ docker run -it donkeyx/tcp-wait -version
Unable to find image 'donkeyx/tcp-wait:latest' locally
...
Version 'docker-build'
Git_Hash '6e567e4'
```

Using go get:

```bash
# get the package
go get github.com/donkeyx/tcp-wait

# add go bin path to your startup shell, bash in this case
echo 'export PATH="~/go/bin:$PATH"' >> ~/.bashrc

# quick run from there like this. If either fails it will return error code and messages to match
tcp-wait -hp localhost:8080,localhost:9090 -t 5

# for help just run with no flags
tcp-wait -h
Usage of /Users/dbinney/go/bin/tcp-wait:
  -hp value
    	<host:port> [host2:port,...] comma seperated list of host:ports
  -o string
    	output in format json/text (default "json")
  -t int
    	timeout (default 20)
  -version
    	version information
...
```

## Usage

```bash
### simple (success)
$ ./bin/tcp-wait -hp github.com:80
{"level":"info","msg":"services are ready!","services":["github.com:80"],"time":"2020-03-12T17:18:30+10:30"}

### multiple hosts with timeout(1sec) and text as the output
$ ./bin/tcp-wait -hp github.com:443,google.com:443 -t 1 -o text
INFO[2020-03-12T17:20:15+10:30] services are ready!  services="[github.com:443 google.com:443]"

### multiple hosts with timeout(2sec) and fail condition on one host:port
$ ./bin/tcp-wait -hp github.com:443,localhost:10000 -t 2
{"level":"warning","msg":"tcp ping failed","tcp-host":"localhost:10000","time":"2020-03-12T17:26:16+10:30"}
{"level":"warning","msg":"tcp ping failed","tcp-host":"localhost:10000","time":"2020-03-12T17:26:17+10:30"}
{"level":"error","msg":"services did not respond","time":"2020-03-12T17:26:18+10:30"}
```

## Building locally

You can just clone down the repo, then use the make file to build the packages. They will be placed
in a local bin folder, with linux/mac and a local env build. You can execute them directly from that
path.

```bash
# build and run using make all
$ make all
go get
go test -v ./...
=== RUN   TestSuccessSingle
--- PASS: TestSuccessSingle (0.01s)
=== RUN   TestFailureDouble
{"level":"warning","msg":"tcp ping failed","tcp-host":"nowhere:50","time":"2020-03-13T11:38:45+10:30"}
{"level":"warning","msg":"tcp ping failed","tcp-host":"nowhere:51","time":"2020-03-13T11:38:45+10:30"}
--- PASS: TestFailureDouble (0.50s)
PASS
ok      tcp-wait        (cached)
go build -o ./bin/tcp-wait -v
CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o bin/linux/tcp-wait -v
tcp-wait
CGO_ENABLED=0 GOOS=darwin GOARCH=amd64 go build -o bin/mac/tcp-wait -v
tcp-wait

# structure created with binaries in your local bin directory.
$ tree
├── Dockerfile
├── LICENSE
├── Makefile
├── README.md
├── bin
│   ├── tcp-wait
│   ├── tcp-wait.darwin.amd64
│   ├── tcp-wait.linux.amd64
│   └── tcp-wait.windows.amd64
├── go.mod
├── go.sum
├── main.go
├── main_test.go
└── tmp
    └── test-coverage
        ├── coverage.html
        └── coverage.out.


# Binary should be imediately executable
$ ./bin/tcp-wait
Usage of ./bin/tcp-wait:
  -hp value
        <host:port> [host2:port,...] comma seperated list of services
  -o string
        output in format json/text (default "json")
  -t int
        timeout (default 20)
```
