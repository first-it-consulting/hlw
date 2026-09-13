.PHONY: build install test check fmt vet clean dist docs

BINARY_NAME=hlw
MODULE=github.com/first-it-consulting/hlw

# Version comes from the tag when building from a checkout, and falls back to
# the VERSION file so a source tarball still stamps something meaningful.
VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || cat VERSION 2>/dev/null || echo "dev")
COMMIT  ?= $(shell git rev-parse HEAD 2>/dev/null || echo "none")
DATE    ?= $(shell date -u +%Y-%m-%dT%H:%M:%SZ)

LDFLAGS = -X $(MODULE)/cmd.Version=$(VERSION) \
          -X $(MODULE)/cmd.Commit=$(COMMIT) \
          -X $(MODULE)/cmd.Date=$(DATE)

# Platforms built by `make dist`.
PLATFORMS = darwin/arm64 darwin/amd64 linux/amd64 linux/arm64

# Build the binary for the host platform
build:
	go build -ldflags="$(LDFLAGS)" -o $(BINARY_NAME) .

# Install into $(go env GOPATH)/bin
install:
	go install -ldflags="$(LDFLAGS)" .

# Run tests
test:
	go test -race ./...

# Format code
fmt:
	go fmt ./...

# Vet code
vet:
	go vet ./...

# Everything CI checks, locally
check: fmt vet test

# Cross-compile every platform into dist/, with checksums
dist:
	rm -rf dist && mkdir -p dist
	@for platform in $(PLATFORMS); do \
		os=$${platform%/*}; arch=$${platform#*/}; \
		echo "building $$os/$$arch"; \
		CGO_ENABLED=0 GOOS=$$os GOARCH=$$arch \
			go build -ldflags="$(LDFLAGS) -s -w" -o dist/$(BINARY_NAME) . || exit 1; \
		tar -czf dist/$(BINARY_NAME)-$(VERSION)-$$os-$$arch.tar.gz -C dist $(BINARY_NAME) || exit 1; \
		rm -f dist/$(BINARY_NAME); \
	done
	@cd dist && shasum -a 256 *.tar.gz > checksums.txt
	@echo "---"; cat dist/checksums.txt

# Remove build artifacts
clean:
	rm -rf dist
	rm -f $(BINARY_NAME)

# Generate documentation
docs:
	go doc ./...
