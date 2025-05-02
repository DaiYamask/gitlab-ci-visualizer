.PHONY: build clean test

# Build variables
BINARY_NAME=gitlabviz
BUILD_DIR=bin
VERSION=$(shell git describe --tags --always --dirty 2>/dev/null || echo "0.1.0")
LDFLAGS=-ldflags "-X main.Version=$(VERSION)"

# Build targets for different platforms
PLATFORMS=linux darwin windows
ARCHITECTURES=amd64 arm64

# Default target
all: build

# Build for the current platform
build:
	@echo "Building $(BINARY_NAME)..."
	@mkdir -p $(BUILD_DIR)
	@go build $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY_NAME) ./cmd/gitlabviz

# Build for all platforms
build-all: $(PLATFORMS)

# Platform-specific builds
$(PLATFORMS):
	@echo "Building for $@..."
	@mkdir -p $(BUILD_DIR)/$@
	@for arch in $(ARCHITECTURES); do \
		echo "  Building for $@/$$arch..."; \
		GOOS=$@ GOARCH=$$arch go build $(LDFLAGS) -o $(BUILD_DIR)/$@/$(BINARY_NAME)-$@-$$arch ./cmd/gitlabviz; \
		if [ "$@" = "windows" ]; then \
			mv $(BUILD_DIR)/$@/$(BINARY_NAME)-$@-$$arch $(BUILD_DIR)/$@/$(BINARY_NAME)-$@-$$arch.exe; \
		fi; \
	done

# Run tests
test:
	@echo "Running tests..."
	@go test -v ./...

# Clean build artifacts
clean:
	@echo "Cleaning..."
	@rm -rf $(BUILD_DIR)

# Install locally
install: build
	@echo "Installing $(BINARY_NAME)..."
	@cp $(BUILD_DIR)/$(BINARY_NAME) /usr/local/bin/

# Run the application
run:
	@go run ./cmd/gitlabviz/main.go
