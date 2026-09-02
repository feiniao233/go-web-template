.PHONY: run test build release clean

VERSION ?= dev
COMMIT ?= unknown
BUILD_TIME ?= $(shell date -u +%Y-%m-%dT%H:%M:%SZ)
RELEASE_GOOS ?= linux
RELEASE_GOARCH ?= amd64
LDFLAGS := -X go-web-template/internal/buildinfo.Version=$(VERSION) -X go-web-template/internal/buildinfo.Commit=$(COMMIT) -X go-web-template/internal/buildinfo.BuildTime=$(BUILD_TIME)

run:
	go run ./cmd/server

test:
	go test ./...

build:
	go build -trimpath -ldflags "$(LDFLAGS)" -o server ./cmd/server

release:
	mkdir -p dist
	CGO_ENABLED=0 GOOS=$(RELEASE_GOOS) GOARCH=$(RELEASE_GOARCH) go build -trimpath -ldflags "-s -w $(LDFLAGS)" -o dist/server ./cmd/server
	cd dist && sha256sum server > server.sha256

clean:
	rm -f ./server ./dist/server ./dist/server.sha256
	rmdir ./dist 2>/dev/null || true
