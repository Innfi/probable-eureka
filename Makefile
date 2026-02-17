BINARY_NAME := probable-eureka
MODULE := github.com/innfi/probable-eureka
GO := go

GOFLAGS := -v
LDFLAGS := -s -w

CNI_BIN_DIR := /opt/cni/bin
CNI_CONF_DIR := /etc/cni/net.d

.PHONY: all build clean test vet fmt lint tidy install uninstall

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
	install -d $(CNI_BIN_DIR)
	install -m 755 $(BINARY_NAME) $(CNI_BIN_DIR)/$(BINARY_NAME)

uninstall:
	rm -f $(CNI_BIN_DIR)/$(BINARY_NAME)
