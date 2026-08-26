.PHONY: fmt test vet build snapshot
fmt:
	gofmt -w cmd internal

test:
	go test -race ./...

vet:
	go vet ./...

build:
	go build -trimpath -o bin/nanoharness ./cmd/nanoharness

snapshot:
	goreleaser release --snapshot --clean
