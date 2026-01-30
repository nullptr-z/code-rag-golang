VERSION := 1.0.0
BINARY  := crag
MODULE  := github.com/zheng/crag

GOOS    ?= $(shell go env GOOS)
GOARCH  ?= $(shell go env GOARCH)

LDFLAGS := -s -w -X main.version=$(VERSION)

.PHONY: build clean install test

build:
	go build -ldflags "$(LDFLAGS)" -o $(BINARY) .

install:
	go install -ldflags "$(LDFLAGS)" .

test:
	go test ./...

clean:
	rm -f $(BINARY)
