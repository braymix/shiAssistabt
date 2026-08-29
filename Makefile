# shikA Makefile

BINARY := shikad
PKG := ./cmd/shikad
BINDIR := bin
VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo 0.1.0-dev)
LDFLAGS := -X main.version=$(VERSION)

.PHONY: all build run test check fmt vet clean cross dist android prima prima-android

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

# Release artifacts: cross binaries plus a SHA256SUMS manifest, the same set the
# release workflow publishes.
dist: cross
	cd $(BINDIR) && sha256sum $(BINARY)-* > SHA256SUMS
	@echo "dist ready in $(BINDIR)/ (binaries + SHA256SUMS)"

# Android: build the orchestrator as a native, NDK-free arm64 executable and drop
# it where the APK expects it (as libshikad.so). Build the APK from android/.
ANDROID_JNI := android/app/src/main/jniLibs/arm64-v8a
android:
	@mkdir -p $(ANDROID_JNI)
	CGO_ENABLED=0 GOOS=android GOARCH=arm64 go build -ldflags "$(LDFLAGS)" -o $(ANDROID_JNI)/libshikad.so $(PKG)
	@echo "android binary -> $(ANDROID_JNI)/libshikad.so"
	@echo "now: cd android && ./gradlew assembleRelease"

# Fetch & build prima.cpp (the data plane) into ~/prima.cpp.
prima:
	sh scripts/bootstrap-prima.sh

# Cross-compile the prima.cpp engine for Android arm64 into the APK's jniLibs.
# Needs ANDROID_NDK_HOME set. Bundled by the release CI automatically.
prima-android:
	sh scripts/build-prima-android.sh $(ANDROID_JNI)
