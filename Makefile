# Makefile for Virtual VectorFS with Embedded LibSQL
.PHONY: help build-libsql-amd64 build-libsql-arm64 build-libsql-all clean-libsql test-libsql smoke-test build-app-amd64 build-app-arm64 build-app-all fetch-sqlean extract-artifacts-amd64 extract-artifacts-arm64 build-libsql-amd64-full build-libsql-arm64-full validate-artifacts clean-build-cache models-download models-download-v2 models-validate models-clean models-clean-cache models-info models-check models-update-check ci-build release clean-all deps info

# Build configuration
LIBSQL_VERSION := v0.9.23
LIBSQL_COMMIT := 7eed898
SQLEAN_VERSION := 0.27.1
BUILD_DIR := ./build
LIB_DIR := ./lib
SQLEAN_DIR := $(BUILD_DIR)/sqlean_local
DOCKER_BUILDKIT := 1
export DOCKER_BUILDKIT

# Model configuration
MODELS_DIR := vvfs/generation/models/gguf
MODELS_SCRIPT_DIR := scripts
REQUIRED_MODELS := open-embed.gguf open-chat-qwen3-1_7b.gguf open-vision.gguf
MODEL_CACHE_DIR := $(HOME)/.cache/vvfs-models
PARALLEL_DOWNLOADS := 3
CUSTOM_MODEL_REPO :=

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
	@echo "Model Management:"
	@echo "  models-check            - Check if required models exist (idempotent)"
	@echo "  models-download         - Download open-source models (skips if exist)"
	@echo "  models-download-v2      - Enhanced download (parallel, checksums, caching)"
	@echo "  models-validate         - Validate downloaded models"
	@echo "  models-update-check     - Check for model updates"
	@echo "  models-info             - Show model information"
	@echo "  models-clean            - Remove downloaded models"
	@echo "  models-clean-cache      - Clean model cache directory"
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
	@echo "  Models Directory: $(MODELS_DIR)"
	@echo "  Architectures: linux/amd64, linux/arm64"

# ============================================================================
# Model Management Targets
# ============================================================================

# Check if required models exist (idempotent, used by other targets)
models-check:
	@echo "🔍 Checking for required models..."
	@missing=0; \
	models="$(REQUIRED_MODELS)"; \
	for model in $$models; do \
		if [ -f "$(MODELS_DIR)/$$model" ]; then \
			size=$$(du -h "$(MODELS_DIR)/$$model" | cut -f1); \
			echo "  ✅ $$model ($$size)"; \
		else \
			echo "  ❌ $$model (missing)"; \
			missing=$$((missing + 1)); \
		fi; \
	done; \
	if [ $$missing -eq 0 ]; then \
		echo "✅ All required models present"; \
	else \
		echo "⚠️  $$missing model(s) missing"; \
		exit 1; \
	fi

# Download models only if they don't exist (idempotent)
models-download: $(MODELS_DIR)
	@if $(MAKE) -s models-check 2>/dev/null; then \
		echo "✅ Models already present, skipping download"; \
	else \
		echo "📥 Downloading open-source models..."; \
		bash $(MODELS_SCRIPT_DIR)/download_open_source_models.sh || { \
			echo "❌ Model download failed"; \
			exit 1; \
		}; \
		echo ""; \
		echo "🔍 Validating downloaded models..."; \
		$(MAKE) models-validate || { \
			echo "❌ Model validation failed after download"; \
			exit 1; \
		}; \
		echo ""; \
		echo "✅ Models downloaded and validated successfully"; \
		$(MAKE) -s models-check; \
	fi

# Validate models (checks GGUF headers and file integrity)
models-validate:
	@echo "🔍 Validating models..."
	@if ! $(MAKE) -s models-check 2>/dev/null; then \
		echo "❌ Models missing, run 'make models-download' first"; \
		exit 1; \
	fi
	@echo "📋 Checking GGUF headers..."
	@all_valid=1; \
	for model in $(MODELS_DIR)/*.gguf; do \
		if [ -f "$$model" ]; then \
			header=$$(xxd -l 4 -p "$$model" 2>/dev/null | head -1); \
			if [ "$$header" = "47475546" ]; then \
				echo "  ✅ $$(basename $$model): Valid GGUF"; \
			else \
				echo "  ❌ $$(basename $$model): Invalid header ($$header)"; \
				all_valid=0; \
			fi; \
		fi; \
	done; \
	if [ $$all_valid -eq 1 ]; then \
		echo "✅ All models validated successfully"; \
	else \
		echo "❌ Model validation failed"; \
		exit 1; \
	fi

# Show model information
models-info:
	@echo "📊 Model Information"
	@echo "===================="
	@echo "Directory: $(MODELS_DIR)"
	@echo "Required models: $(REQUIRED_MODELS)"
	@echo ""
	@if [ -d "$(MODELS_DIR)" ] && [ -n "$$(ls -A $(MODELS_DIR)/*.gguf 2>/dev/null)" ]; then \
		echo "Installed models:"; \
		for model in $(MODELS_DIR)/*.gguf; do \
			if [ -f "$$model" ]; then \
				size=$$(du -h "$$model" | cut -f1); \
				name=$$(basename "$$model"); \
				modified=$$(stat -c %y "$$model" 2>/dev/null | cut -d' ' -f1); \
				echo "  • $$name ($$size) - $$modified"; \
			fi; \
		done; \
	else \
		echo "No models installed yet."; \
		echo "Run 'make models-download' to download models."; \
	fi

# Clean models (remove downloaded files)
models-clean:
	@echo "🧹 Cleaning models..."
	@if [ -d "$(MODELS_DIR)" ]; then \
		rm -rf $(MODELS_DIR)/*.gguf; \
		rm -f $(MODELS_DIR)/.checksums $(MODELS_DIR)/.model_versions; \
		echo "✅ Models cleaned"; \
	else \
		echo "✅ No models to clean"; \
	fi

# Enhanced download with parallel, checksums, and caching (v2)
models-download-v2: $(MODELS_DIR)
	@export MODEL_CACHE_DIR="$(MODEL_CACHE_DIR)"; \
	export PARALLEL_DOWNLOADS="$(PARALLEL_DOWNLOADS)"; \
	export CUSTOM_MODEL_REPO="$(CUSTOM_MODEL_REPO)"; \
	if $(MAKE) -s models-check 2>/dev/null; then \
		echo "✅ Models already present"; \
		echo "   Run 'make models-update-check' to check for updates"; \
	else \
		echo "📥 Starting enhanced download..."; \
		bash $(MODELS_SCRIPT_DIR)/download_open_source_models_v2.sh || { \
			echo "❌ Enhanced download failed"; \
			exit 1; \
		}; \
	fi

# Check for model updates
models-update-check:
	@echo "🔍 Checking for model updates..."
	@if [ ! -f "$(MODELS_DIR)/.model_versions" ]; then \
		echo "⚠️  No version file found - use 'make models-download-v2' for version tracking"; \
		exit 0; \
	fi
	@echo "Current versions:"
	@cat $(MODELS_DIR)/.model_versions 2>/dev/null || echo "  No versions recorded"
	@echo ""
	@echo "💡 To update models, run: make models-clean && make models-download-v2"

# Clean model cache
models-clean-cache:
	@echo "🧹 Cleaning model cache..."
	@if [ -d "$(MODEL_CACHE_DIR)" ]; then \
		rm -rf $(MODEL_CACHE_DIR)/*; \
		echo "✅ Cache cleaned: $(MODEL_CACHE_DIR)"; \
		echo "   Freed: $$(du -sh $(MODEL_CACHE_DIR) 2>/dev/null | cut -f1 || echo 'unknown')"; \
	else \
		echo "✅ No cache to clean"; \
	fi
