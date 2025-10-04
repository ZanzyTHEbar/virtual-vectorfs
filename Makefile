# Makefile for Virtual VectorFS with Embedded LibSQL
.PHONY: help build-libsql-amd64 build-libsql-arm64 build-libsql-all clean-libsql test-libsql smoke-test build-app-amd64 build-app-arm64 build-app-all fetch-sqlean extract-artifacts-amd64 extract-artifacts-arm64 build-libsql-amd64-full build-libsql-arm64-full validate-artifacts clean-build-cache

# Build configuration
LIBSQL_VERSION := v0.9.23
LIBSQL_COMMIT := 7eed898
SQLEAN_VERSION := 0.27.1
BUILD_DIR := ./build
LIB_DIR := ./lib
SQLEAN_DIR := $(BUILD_DIR)/sqlean_local
DOCKER_BUILDKIT := 1
export DOCKER_BUILDKIT

# Default target
help:
	@echo "Virtual VectorFS with Embedded LibSQL - Build Targets"
	@echo ""
	@echo "LibSQL Static Library Builds:"
	@echo "  build-libsql-amd64-full - Build libsql static lib for Linux AMD64 (with SQLean fetch + extract)"
	@echo "  build-libsql-arm64-full - Build libsql static lib for Linux ARM64 (with SQLean fetch + extract)"
	@echo "  build-libsql-all        - Build libsql static libs for both architectures"
	@echo ""
	@echo "SQLean Dependency Management:"
	@echo "  fetch-sqlean            - Download and extract SQLean source (cached in build/)"
	@echo ""
	@echo "Artifact Management:"
	@echo "  extract-artifacts-amd64 - Extract build artifacts from Docker image (AMD64)"
	@echo "  extract-artifacts-arm64 - Extract build artifacts from Docker image (ARM64)"
	@echo "  validate-artifacts      - Validate all required artifacts are present"
	@echo ""
	@echo "Application Builds:"
	@echo "  build-app-amd64         - Build Go app for AMD64 with embedded libsql"
	@echo "  build-app-arm64         - Build Go app for ARM64 with embedded libsql"
	@echo "  build-app-all           - Build Go app for both architectures"
	@echo ""
	@echo "Testing & Verification:"
	@echo "  test-libsql             - Run libsql integration tests"
	@echo "  smoke-test              - Run comprehensive smoke tests"
	@echo ""
	@echo "Cleanup:"
	@echo "  clean-libsql            - Remove built libsql libraries"
	@echo "  clean-build-cache       - Clean build intermediate files only"
	@echo "  clean-all               - Remove all build artifacts"
	@echo ""
	@echo "CI/CD:"
	@echo "  ci-build                - Full CI pipeline (libs + app + tests)"
	@echo "  release                 - Create release artifacts"

# Create necessary directories
$(LIB_DIR):
	mkdir -p $(LIB_DIR)

# SQLean fetch and prepare
fetch-sqlean:
	@echo "Fetching SQLean $(SQLEAN_VERSION)..."
	@mkdir -p $(SQLEAN_DIR)
	@if [ ! -f $(SQLEAN_DIR)/Makefile ]; then \
		echo "Downloading SQLean from GitHub..."; \
		curl -L "https://github.com/nalgeon/sqlean/archive/refs/tags/$(SQLEAN_VERSION).tar.gz" -o /tmp/sqlean.tar.gz && \
		tar -xzf /tmp/sqlean.tar.gz -C $(SQLEAN_DIR) --strip-components=1 && \
		rm /tmp/sqlean.tar.gz && \
		echo "✅ SQLean source downloaded"; \
	else \
		echo "✅ SQLean already present (cached)"; \
	fi

# Extract artifacts from Docker builds
extract-artifacts-amd64:
	@echo "Extracting AMD64 artifacts from Docker image..."
	@mkdir -p $(BUILD_DIR)/artifacts/amd64
	docker create --name libsql-extract-amd64 libsql-builder-amd64:latest
	docker cp libsql-extract-amd64:/artifacts/. $(BUILD_DIR)/artifacts/amd64/
	docker rm libsql-extract-amd64
	@echo "✅ AMD64 artifacts extracted to $(BUILD_DIR)/artifacts/amd64/"

extract-artifacts-arm64:
	@echo "Extracting ARM64 artifacts from Docker image..."
	@mkdir -p $(BUILD_DIR)/artifacts/arm64
	docker create --name libsql-extract-arm64 libsql-builder-arm64:latest
	docker cp libsql-extract-arm64:/artifacts/. $(BUILD_DIR)/artifacts/arm64/
	docker rm libsql-extract-arm64
	@echo "✅ ARM64 artifacts extracted to $(BUILD_DIR)/artifacts/arm64/"

# Complete build with artifact extraction
build-libsql-amd64-full: fetch-sqlean
	@echo "Building LibSQL AMD64 with SQLean extensions..."
	docker build \
		--platform linux/amd64 \
		--build-arg LIBSQL_COMMIT=$(LIBSQL_COMMIT) \
		-f $(BUILD_DIR)/Dockerfile.libsql-amd64 \
		-t libsql-builder-amd64:latest \
		.
	@$(MAKE) extract-artifacts-amd64
	@echo "✅ Full AMD64 build complete"

build-libsql-arm64-full: fetch-sqlean
	@echo "Building LibSQL ARM64 with SQLean extensions..."
	docker build \
		--platform linux/arm64 \
		--build-arg LIBSQL_COMMIT=$(LIBSQL_COMMIT) \
		-f $(BUILD_DIR)/Dockerfile.libsql-arm64 \
		-t libsql-builder-arm64:latest \
		.
	@$(MAKE) extract-artifacts-arm64
	@echo "✅ Full ARM64 build complete"

# Validate artifacts
validate-artifacts:
	@echo "Validating build artifacts..."
	@test -f $(BUILD_DIR)/artifacts/amd64/libsql.h || (echo "❌ Missing libsql.h"; exit 1)
	@test -f $(BUILD_DIR)/artifacts/amd64/build-info.txt || (echo "❌ Missing build-info.txt"; exit 1)
	@ls $(BUILD_DIR)/artifacts/amd64/sqlean/*.so >/dev/null 2>&1 || (echo "❌ Missing SQLean extensions"; exit 1)
	@echo "✅ All artifacts validated"

# Clean build intermediate files only
clean-build-cache:
	@echo "Cleaning build cache..."
	rm -rf $(SQLEAN_DIR)/dist $(SQLEAN_DIR)/build
	@echo "✅ Build cache cleaned"

# Build libsql static library for AMD64
build-libsql-amd64: $(LIB_DIR)
	@echo "Building LibSQL static library for Linux AMD64..."
	docker build \
		--platform linux/amd64 \
		-f $(BUILD_DIR)/Dockerfile.libsql-amd64 \
		-t libsql-builder-amd64 \
		$(BUILD_DIR)
	docker run --rm -v $(PWD)/$(LIB_DIR):/output libsql-builder-amd64 \
		cp /libsql-amd64.a /output/
	docker run --rm -v $(PWD)/$(LIB_DIR):/output libsql-builder-amd64 \
		cp /build-info.txt /output/libsql-amd64-build-info.txt
	@echo "✅ LibSQL AMD64 build complete: $(LIB_DIR)/libsql-amd64.a"

# Build libsql static library for ARM64
build-libsql-arm64: $(LIB_DIR)
	@echo "Building LibSQL static library for Linux ARM64..."
	docker build \
		--platform linux/arm64 \
		-f $(BUILD_DIR)/Dockerfile.libsql-arm64 \
		-t libsql-builder-arm64 \
		$(BUILD_DIR)
	docker run --rm -v $(PWD)/$(LIB_DIR):/output libsql-builder-arm64 \
		cp /libsql-arm64.a /output/
	docker run --rm -v $(PWD)/$(LIB_DIR):/output libsql-builder-arm64 \
		cp /build-info.txt /output/libsql-arm64-build-info.txt
	@echo "✅ LibSQL ARM64 build complete: $(LIB_DIR)/libsql-arm64.a"

# Build libsql for both architectures
build-libsql-all: build-libsql-amd64 build-libsql-arm64
	@echo "✅ All LibSQL static libraries built"
	@ls -la $(LIB_DIR)/libsql-*.a

# Clean libsql build artifacts
clean-libsql:
	@echo "Cleaning LibSQL build artifacts..."
	rm -rf $(LIB_DIR)
	docker rmi libsql-builder-amd64 libsql-builder-arm64 2>/dev/null || true
	@echo "✅ LibSQL artifacts cleaned"

# Run libsql integration tests
test-libsql:
	@echo "Running LibSQL integration tests..."
	go run scripts/test-libsql-integration.go

# Run comprehensive smoke tests
smoke-test: test-libsql
	@echo "Running comprehensive smoke tests..."
	go run scripts/smoke-test-libsql.go
	go test ./vvfs/db/... -v -run TestLibSQL
	go test ./vvfs/memory/database/... -v -run TestCapabilities

# Build Go application for AMD64 with embedded libsql
build-app-amd64: $(LIB_DIR)/libsql-amd64.a
	@echo "Building Go application for AMD64..."
	CGO_ENABLED=1 \
	CGO_CFLAGS="-I$(PWD)/lib" \
	CGO_LDFLAGS="-L$(PWD)/lib -lsql -lm -ldl" \
	GOOS=linux \
	GOARCH=amd64 \
	go build -o bin/vvfs-amd64 -ldflags="-linkmode external -extldflags '-static'" .
	@echo "✅ AMD64 application built: bin/vvfs-amd64"
	@ls -lh bin/vvfs-amd64

# Build Go application for ARM64 with embedded libsql
build-app-arm64: $(LIB_DIR)/libsql-arm64.a
	@echo "Building Go application for ARM64..."
	CGO_ENABLED=1 \
	CGO_CFLAGS="-I$(PWD)/lib" \
	CGO_LDFLAGS="-L$(PWD)/lib -lsql -lm -ldl" \
	GOOS=linux \
	GOARCH=arm64 \
	go build -o bin/vvfs-arm64 -ldflags="-linkmode external -extldflags '-static'" .
	@echo "✅ ARM64 application built: bin/vvfs-arm64"
	@ls -lh bin/vvfs-arm64

# Build Go application for both architectures
build-app-all: build-app-amd64 build-app-arm64
	@echo "✅ Applications built for both architectures"
	@ls -lh bin/

# Full CI pipeline
ci-build: clean-libsql build-libsql-all smoke-test build-app-all
	@echo "✅ CI pipeline completed successfully"
	@echo "Build artifacts:"
	@ls -la $(LIB_DIR)/
	@ls -la bin/

# Create release artifacts
release: ci-build
	@echo "Creating release artifacts..."
	mkdir -p release
	cp bin/vvfs-amd64 release/
	cp bin/vvfs-arm64 release/
	cp $(LIB_DIR)/libsql-amd64.a release/
	cp $(LIB_DIR)/libsql-arm64.a release/
	cd release && tar czf vvfs-$(shell date +%Y%m%d).tar.gz *
	@echo "✅ Release created: release/vvfs-$(shell date +%Y%m%d).tar.gz"

# Clean all build artifacts
clean-all: clean-libsql
	@echo "Cleaning all build artifacts..."
	rm -rf bin/ release/
	@echo "✅ All artifacts cleaned"

# Development helpers
deps:
	@echo "Installing build dependencies..."
	go mod download
	command -v docker >/dev/null 2>&1 || { echo "Docker is required but not installed."; exit 1; }

info:
	@echo "Build Information:"
	@echo "  LibSQL Version: $(LIBSQL_VERSION)"
	@echo "  LibSQL Commit: $(LIBSQL_COMMIT)"
	@echo "  Build Directory: $(BUILD_DIR)"
	@echo "  Library Directory: $(LIB_DIR)"
	@echo "  Architectures: linux/amd64, linux/arm64"
