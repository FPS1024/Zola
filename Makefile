GO ?= go
VERSION ?= v1.0.0
VERSION_TAG := $(if $(filter v%,$(VERSION)),$(VERSION),v$(VERSION))
COMMIT ?= $(shell git rev-parse --short HEAD 2>/dev/null || echo unknown)
BUILD_TIME ?= $(shell date -u +%Y-%m-%dT%H:%M:%SZ)

HOST_OS := $(shell $(GO) env GOHOSTOS)
HOST_ARCH := $(shell $(GO) env GOHOSTARCH)
CURRENT_OS := $(shell $(GO) env GOOS)
CURRENT_ARCH := $(shell $(GO) env GOARCH)
CURRENT_ARM := $(shell $(GO) env GOARM)

TARGET_OS ?= $(CURRENT_OS)
TARGET_ARCH ?= $(CURRENT_ARCH)
TARGET_ARM ?= $(CURRENT_ARM)
TARGET_CGO ?= 0

ifeq ($(TARGET_OS),ios)
TARGET_CGO := 1
endif

ifeq ($(TARGET_ARCH),arm)
TARGET_PLATFORM := $(TARGET_OS)-armv$(if $(TARGET_ARM),$(TARGET_ARM),7)
TARGET_ARM_ENV := GOARM=$(TARGET_ARM)
else
TARGET_PLATFORM := $(TARGET_OS)-$(TARGET_ARCH)
TARGET_ARM_ENV :=
endif

ifeq ($(TARGET_OS),windows)
BINARY := zola-$(VERSION_TAG)-$(TARGET_PLATFORM).exe
else
BINARY := zola-$(VERSION_TAG)-$(TARGET_PLATFORM)
endif

OUTPUT ?= bin/$(BINARY)
LDFLAGS := -s -w \
	-X zola/internal/buildinfo.Version=$(VERSION_TAG) \
	-X zola/internal/buildinfo.Commit=$(COMMIT) \
	-X zola/internal/buildinfo.BuildTime=$(BUILD_TIME)

.PHONY: build build-current build-ios build-darwin build-linux build-windows release release-ios deb build-all check-env test vet clean

build: check-env
	@mkdir -p "$(dir $(OUTPUT))"
	env CGO_ENABLED=$(TARGET_CGO) GOOS=$(TARGET_OS) GOARCH=$(TARGET_ARCH) $(TARGET_ARM_ENV) \
		$(GO) build -trimpath -buildvcs=false -ldflags "$(LDFLAGS)" -o "$(OUTPUT)" ./cmd/zola
	@echo "Built $(OUTPUT)"

build-current: build

build-ios:
	@$(MAKE) build TARGET_OS=ios TARGET_ARCH=arm64 TARGET_ARM= TARGET_CGO=1

build-darwin:
	@$(MAKE) build TARGET_OS=darwin TARGET_ARCH=$(HOST_ARCH) TARGET_ARM=

build-linux:
	@$(MAKE) build TARGET_OS=linux TARGET_ARCH=$(HOST_ARCH) TARGET_ARM=$(CURRENT_ARM)

build-windows:
	@$(MAKE) build TARGET_OS=windows TARGET_ARCH=$(HOST_ARCH) TARGET_ARM=

check-env:
	@if [ "$(TARGET_OS)" != "$(HOST_OS)" ] || [ "$(TARGET_ARCH)" != "$(HOST_ARCH)" ]; then \
		echo "error: requested $(TARGET_OS)/$(TARGET_ARCH), but native Go host is $(HOST_OS)/$(HOST_ARCH)." >&2; \
		echo "Use 'make release' to cross-build all supported targets." >&2; \
		exit 1; \
	fi

release: build-all

release-ios:
	VERSION="$(VERSION_TAG)" ./scripts/build-all.sh ios-arm64

deb:
	VERSION="$(VERSION_TAG)" ./scripts/build-deb.sh

build-all:
	VERSION="$(VERSION_TAG)" ./scripts/build-all.sh

test:
	$(GO) test ./...

vet:
	$(GO) vet ./...

clean:
	rm -rf bin dist
