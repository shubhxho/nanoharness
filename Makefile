.PHONY: fmt test vet build install snapshot version

VERSION := $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
COMMIT  := $(shell git rev-parse --short HEAD 2>/dev/null || echo unknown)

LDFLAGS := -s -w \
	-X main.versionFlag=$(VERSION) \
	-X main.commitFlag=$(COMMIT) \
	-X github.com/shubhxho/nanoharness/internal/version.Version=$(VERSION) \
	-X github.com/shubhxho/nanoharness/internal/version.Commit=$(COMMIT)

fmt:
	gofmt -w cmd internal

test:
	go test -race ./...

vet:
	go vet ./...

version:
	@echo version=$(VERSION)
	@echo commit=$(COMMIT)
	@git rev-parse HEAD
	@git rev-parse --short HEAD

build:
	go build -trimpath -ldflags="$(LDFLAGS)" -o bin/nanoharness ./cmd/nanoharness

install:
	go install -trimpath -ldflags="$(LDFLAGS)" ./cmd/nanoharness

snapshot:
	goreleaser release --snapshot --clean
