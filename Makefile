.PHONY: build build-all test lint install clean docs

BINARY := mate
VERSION := $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
LDFLAGS := -ldflags "-X main.version=$(VERSION)"

build:
	go build $(LDFLAGS) -o $(BINARY) ./cmd/mate

build-all: build-darwin-arm64 build-linux-amd64

build-darwin-arm64:
	GOOS=darwin GOARCH=arm64 go build $(LDFLAGS) -o dist/$(BINARY)-darwin-arm64 ./cmd/mate

build-linux-amd64:
	GOOS=linux GOARCH=amd64 go build $(LDFLAGS) -o dist/$(BINARY)-linux-amd64 ./cmd/mate

test:
	go test -race -cover ./...

lint:
	golangci-lint run

install: build
	cp $(BINARY) $(GOPATH)/bin/

clean:
	rm -f $(BINARY)
	rm -f coverage.out coverage.html
	rm -rf docs/ dist/

coverage:
	go test -coverprofile=coverage.out ./...
	go tool cover -html=coverage.out -o coverage.html

docs:
	go run ./cmd/gendocs docs/
