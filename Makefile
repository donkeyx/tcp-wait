# Go parameters
BINARY_PATH = ./bin/linux/
BINARY_NAME = tcp-wait
VERSION ='$(shell git describe --tags)'
VERSION ='$(shell git symbolic-ref -q --short HEAD || git describe --tags --exact-match)'
BUILD_DATE='$(shell date)'
HASH = '$(shell git rev-parse --short HEAD)'
BUILD_FLAGS = go build -ldflags "-w -s -X main.Version=$(VERSION) -X main.GitHash=$(HASH)"


all: clean deps test build-all
build:
	CGO_ENABLED=0 $(BUILD_FLAGS) -o bin/$(BINARY_NAME) -v
	# CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o tcp-wait .
test:
	go test

	mkdir -p tmp/test-coverage
	go test -coverprofile=tmp/test-coverage/coverage.out
	go tool cover -html=tmp/test-coverage/coverage.out -o ./tmp/test-coverage/coverage.html
clean:
	go clean
	find ./bin/ -type f | grep -v keep | xargs rm

deps:
	go get .

update:
	go get ./...
	go mod tidy

# Cross compilation
build-all:
	$(info    version is $(VERSION))
	$(info    build_date is $(BUILD_DATE))
	$(info    ld-flags is $(BUILD_FLAGS))

	CGO_ENABLED=0 GOARCH=386   GOOS=windows  $(BUILD_FLAGS) -o bin/$(BINARY_NAME).windows.amd64 -v
	CGO_ENABLED=0 GOARCH=amd64 GOOS=linux  $(BUILD_FLAGS) -o bin/$(BINARY_NAME).linux.amd64 -v
	CGO_ENABLED=0 GOARCH=amd64 GOOS=darwin  $(BUILD_FLAGS) -o bin/$(BINARY_NAME).darwin.amd64 -v
	CGO_ENABLED=0 GOARCH=arm64 GOOS=android $(BUILD_FLAGS) -o bin/$(BINARY_NAME).arm64 -v

docker-build:
	docker build . -t tcp-wait
