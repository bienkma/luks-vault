BINARY      := luks-vault
MODULE      := github.com/bienkma/luks-vault
MAIN        := ./main.go
GO          ?= go
GOOS        ?= linux
GOARCH      ?= amd64
CGO_ENABLED ?= 0
LDFLAGS     := -s -w
BUILD_FLAGS := -trimpath -ldflags "$(LDFLAGS)"
DIST_DIR    := dist
NFPM_CONFIG := packaging/nfpm.yaml
NFPM_PKG    := github.com/goreleaser/nfpm/v2/cmd/nfpm@v2.41.3
VERSION     ?= $(shell git describe --tags --always --dirty 2>/dev/null | sed 's/^v//' || echo 0.1.0-dev)
GOBIN       := $(shell $(GO) env GOPATH)/bin
NFPM        ?= $(shell command -v nfpm 2>/dev/null || echo $(GOBIN)/nfpm)

.PHONY: all build build-local build-linux test vet tidy clean docker-build \
	help package deb rpm install-nfpm

all: test build-local

help:
	@echo "Targets:"
	@echo "  build         Cross-compile $(BINARY) for $(GOOS)/$(GOARCH)"
	@echo "  build-local   Build $(BINARY) for the current platform"
	@echo "  build-linux   Build linux binary into $(DIST_DIR)/$(BINARY)"
	@echo "  test          Run unit tests"
	@echo "  vet           Run go vet"
	@echo "  tidy          Run go mod tidy"
	@echo "  clean         Remove build artifacts"
	@echo "  docker-build  Build $(BINARY) inside golang:1.26 container"
	@echo "  deb           Build .deb package"
	@echo "  rpm           Build .rpm package"
	@echo "  package       Build .deb and .rpm packages"
	@echo "  install-nfpm  Install nfpm packaging tool"
	@echo ""
	@echo "Variables:"
	@echo "  VERSION=$(VERSION)"
	@echo "  GOOS=$(GOOS) GOARCH=$(GOARCH) CGO_ENABLED=$(CGO_ENABLED)"

build:
	GOOS=$(GOOS) GOARCH=$(GOARCH) CGO_ENABLED=$(CGO_ENABLED) \
		$(GO) build $(BUILD_FLAGS) -o $(BINARY) $(MAIN)

build-local:
	$(GO) build $(BUILD_FLAGS) -o $(BINARY) $(MAIN)

$(DIST_DIR)/$(BINARY): $(MAIN) go.mod go.sum
	@mkdir -p $(DIST_DIR)
	GOOS=linux GOARCH=$(GOARCH) CGO_ENABLED=$(CGO_ENABLED) \
		$(GO) build $(BUILD_FLAGS) -o $(DIST_DIR)/$(BINARY) $(MAIN)

build-linux: $(DIST_DIR)/$(BINARY)

install-nfpm:
	$(GO) install $(NFPM_PKG)

$(NFPM):
	$(GO) install $(NFPM_PKG)

deb: build-linux $(NFPM)
	@mkdir -p $(DIST_DIR)
	VERSION=$(VERSION) NFPM_ARCH=$(GOARCH) \
		$(NFPM) pkg --packager deb --config $(NFPM_CONFIG) --target $(DIST_DIR)

rpm: build-linux $(NFPM)
	@mkdir -p $(DIST_DIR)
	VERSION=$(VERSION) NFPM_ARCH=$(GOARCH) \
		$(NFPM) pkg --packager rpm --config $(NFPM_CONFIG) --target $(DIST_DIR)

package: deb rpm

test:
	$(GO) test ./...

vet: test
	$(GO) vet ./...

tidy:
	$(GO) mod tidy

clean:
	rm -f $(BINARY)
	rm -rf $(DIST_DIR)

docker-build:
	docker run --rm \
		-v "$(CURDIR):/go/src/$(MODULE)" \
		-w /go/src/$(MODULE) \
		golang:1.26 sh -c \
		'GOOS=$(GOOS) GOARCH=$(GOARCH) CGO_ENABLED=$(CGO_ENABLED) $(GO) build -mod=mod $(BUILD_FLAGS) -o $(BINARY) $(MAIN)'
