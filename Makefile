.PHONY: fmt test vet build install snapshot

fmt:
	gofmt -w cmd internal

test:
	go test -race ./...

vet:
	go vet ./...

build:
	go build -trimpath -ldflags="-s -w -X main.version=dev" -o bin/nanoharness ./cmd/nanoharness

install:
	go install -trimpath -ldflags="-s -w" ./cmd/nanoharness

snapshot:
	goreleaser release --snapshot --clean
