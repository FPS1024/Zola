BINARY ?= zola
VERSION ?= v1.0.0
COMMIT ?= $(shell git rev-parse --short HEAD 2>/dev/null || echo unknown)
BUILD_TIME ?= $(shell date -u +%Y-%m-%dT%H:%M:%SZ)
LDFLAGS := -s -w \
	-X zola/internal/buildinfo.Version=$(VERSION) \
	-X zola/internal/buildinfo.Commit=$(COMMIT) \
	-X zola/internal/buildinfo.BuildTime=$(BUILD_TIME)

.PHONY: build release test vet clean

build:
	go build -buildvcs=false -ldflags "$(LDFLAGS)" -o bin/$(BINARY) ./cmd/zola

release:
	./scripts/build-all.sh

test:
	go test ./...

vet:
	go vet ./...

clean:
	rm -rf bin dist
