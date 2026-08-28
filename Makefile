# prima-mesh Makefile

BINARY := primeshd
PKG := ./cmd/primeshd
BINDIR := bin
VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo 0.1.0-dev)
LDFLAGS := -X main.version=$(VERSION)

.PHONY: all build run test check fmt vet clean cross

all: build

build:
	@mkdir -p $(BINDIR)
	go build -ldflags "$(LDFLAGS)" -o $(BINDIR)/$(BINARY) $(PKG)
	@echo "built $(BINDIR)/$(BINARY) ($(VERSION))"

run:
	go run $(PKG)

test:
	go test ./...

check: fmt vet

fmt:
	gofmt -l -w .

vet:
	go vet ./...

clean:
	rm -rf $(BINDIR)

# Cross-compile for the common home-device targets.
cross:
	@mkdir -p $(BINDIR)
	GOOS=darwin  GOARCH=arm64 go build -ldflags "$(LDFLAGS)" -o $(BINDIR)/$(BINARY)-darwin-arm64  $(PKG)
	GOOS=darwin  GOARCH=amd64 go build -ldflags "$(LDFLAGS)" -o $(BINDIR)/$(BINARY)-darwin-amd64  $(PKG)
	GOOS=linux   GOARCH=amd64 go build -ldflags "$(LDFLAGS)" -o $(BINDIR)/$(BINARY)-linux-amd64   $(PKG)
	GOOS=linux   GOARCH=arm64 go build -ldflags "$(LDFLAGS)" -o $(BINDIR)/$(BINARY)-linux-arm64   $(PKG)
	GOOS=windows GOARCH=amd64 go build -ldflags "$(LDFLAGS)" -o $(BINDIR)/$(BINARY)-windows-amd64.exe $(PKG)
	@echo "cross-built into $(BINDIR)/"
