BINARY_NAME := probable-eureka
MODULE := github.com/innfi/probable-eureka
GO := go

GOFLAGS := -v
LDFLAGS := -s -w

CNI_BIN_DIR := /opt/cni/bin
CNI_CONF_DIR := /etc/cni/net.d
CONFLIST := deployments/10-eureka.conflist

.PHONY: all build clean test vet fmt lint tidy install uninstall image help

all: fmt vet build

build:
	$(GO) build $(GOFLAGS) -ldflags "$(LDFLAGS)" -o $(BINARY_NAME) .

build-debug:
	$(GO) build $(GOFLAGS) -gcflags="all=-N -l" -o $(BINARY_NAME) .

build-race:
	$(GO) build $(GOFLAGS) -race -o $(BINARY_NAME) .

test:
	$(GO) test $(GOFLAGS) ./...

test-cover:
	$(GO) test -coverprofile=coverage.out ./...
	$(GO) tool cover -html=coverage.out -o coverage.html

vet:
	$(GO) vet ./...

fmt:
	$(GO) fmt ./...

lint:
	golangci-lint run ./...

tidy:
	$(GO) mod tidy

clean:
	rm -f $(BINARY_NAME)
	rm -f coverage.out coverage.html

install: build
	[ $$(id -u) -eq 0 ] || (echo "install requires root"; exit 1)
	install -d $(CNI_BIN_DIR)
	install -m 755 $(BINARY_NAME) $(CNI_BIN_DIR)/$(BINARY_NAME)
	install -d $(CNI_CONF_DIR)
	install -m 644 $(CONFLIST) $(CNI_CONF_DIR)/10-eureka.conflist

uninstall:
	[ $$(id -u) -eq 0 ] || (echo "uninstall requires root"; exit 1)
	rm -f $(CNI_BIN_DIR)/$(BINARY_NAME)
	rm -f $(CNI_CONF_DIR)/10-eureka.conflist

image:
	docker build -t $(BINARY_NAME):latest .

help:
	@echo "Available targets:"
	@echo "  all        Run fmt, vet, and build"
	@echo "  build      Compile the CNI plugin binary"
	@echo "  build-debug  Build with debug symbols (no optimisations)"
	@echo "  build-race Build with race detector"
	@echo "  test       Run all unit tests"
	@echo "  test-cover Run tests and generate HTML coverage report"
	@echo "  vet        Run go vet"
	@echo "  fmt        Run go fmt"
	@echo "  lint       Run golangci-lint"
	@echo "  tidy       Run go mod tidy"
	@echo "  clean      Remove build artefacts"
	@echo "  install    (root) Install binary to $(CNI_BIN_DIR) and conflist to $(CNI_CONF_DIR)"
	@echo "  uninstall  (root) Remove installed binary and conflist"
	@echo "  image      Build Docker installer image $(BINARY_NAME):latest"
