# Define the variables for the build
GO=go
BUILD_DIR=bin
TARGETS=windows/amd64 linux/amd64 linux/arm darwin/arm64
APP_NAME=meshtastic_go
VERSION=$(shell git describe --tags --always)
GO_FILES=$(shell find . -name '*.go' -not -path './vendor/*' -not -path './.git/*')
FMT_FILES=$(shell find . -name '*.go' -not -path './vendor/*' -not -path './.git/*' -not -path './pkg/generated/*')

# Default target
.PHONY: all
all: test lint build

# Test target
.PHONY: test
test:
	@echo "Running tests..."
	@$(GO) test ./...
	@echo "Tests completed."

# Linting target
.PHONY: lint
lint:
	@echo "Running linters..."
	@if command -v trunk >/dev/null 2>&1; then \
		trunk check; \
	else \
		echo "trunk not found; running gofmt and go vet fallback"; \
		unformatted="$$(gofmt -l $(FMT_FILES))"; \
		if [ -n "$$unformatted" ]; then \
			echo "$$unformatted"; \
			echo "gofmt reported unformatted files"; \
			exit 1; \
		fi; \
		$(GO) vet ./...; \
	fi
	@echo "Linting completed."

# Build target
.PHONY: build
build: $(TARGETS)

# Create build directory if it doesn't exist
$(BUILD_DIR):
	@mkdir -p $(BUILD_DIR)

# Build for each target
windows/amd64: | $(BUILD_DIR)
	GOOS=windows GOARCH=amd64 $(GO) build -o $(BUILD_DIR)/$(APP_NAME)_windows_amd64.exe \
		-ldflags="-X main.version=$(VERSION)" -tags netgo -installsuffix netgo ./cmd/main.go

linux/amd64: | $(BUILD_DIR)
	GOOS=linux GOARCH=amd64 $(GO) build -o $(BUILD_DIR)/$(APP_NAME)_linux_amd64 \
		-ldflags="-X main.version=$(VERSION)" -tags netgo -installsuffix netgo ./cmd/main.go

linux/arm: | $(BUILD_DIR)
	GOOS=linux GOARCH=arm $(GO) build -o $(BUILD_DIR)/$(APP_NAME)_linux_arm \
		-ldflags="-X main.version=$(VERSION)" -tags netgo -installsuffix netgo ./cmd/main.go

darwin/arm64: | $(BUILD_DIR)
	GOOS=darwin GOARCH=arm64 $(GO) build -o $(BUILD_DIR)/$(APP_NAME)_darwin_arm64 \
		-ldflags="-X main.version=$(VERSION)" -tags netgo -installsuffix netgo ./cmd/main.go


# Clean target
.PHONY: clean
clean:
	@echo "Cleaning up..."
	@rm -rf $(BUILD_DIR)
	@echo "Cleaned up."

# Help target
.PHONY: help
help:
	@echo "Makefile for building Go application."
	@echo ""
	@echo "Usage:"
	@echo "  make           Build all targets"
	@echo "  make lint      Run linters"
	@echo "  make build     Build all executables"
	@echo "  make clean     Remove build artifacts"
	@echo "  make help      Show this help message"
