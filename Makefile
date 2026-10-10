GO ?= go
CARGO ?= cargo
GO_PACKAGES := ./search/... ./internal/...
FAISS_DIR ?= build/deps/faiss-$(FAISS_COMMIT)
FAISS_COMMIT := fff814dea2bdda020363506904979b204ee201aa
NATIVE_PREFIX ?= $(CURDIR)/build/native
NATIVE_LIB_DIR := $(NATIVE_PREFIX)/lib
NATIVE_INCLUDE_DIR := $(NATIVE_PREFIX)/include
NATIVE_PKG_CONFIG_DIR := $(NATIVE_LIB_DIR)/pkgconfig
FAISS_LIB_DIR := $(NATIVE_LIB_DIR)
FAISS_INCLUDE_DIR := $(NATIVE_INCLUDE_DIR)
JOBS ?= 4

.DEFAULT_GOAL := build

.PHONY: help build native-fff native-faiss native rust-test test test-fast race vet fmt

help:
	@printf '%s\n' \
	  'build           Build all packages with vector support enabled' \
	  'native-fff      Build the pinned FFF Rust bridge' \
	  'native-faiss    Build Bleve's pinned FAISS C API' \
	  'native          Build both native dependencies' \
	  'rust-test       Test, lint, and format-check the FFF bridge' \
	  'test            Run the full Linux suite, including vectors and race checks' \
	  'test-fast       Run vector-enabled tests without the race detector' \
	  'race            Run the full suite with the race detector' \
	  'vet             Run go vet with vector support' \
	  'fmt             Format Go and Rust sources'

native-fff:
	$(CARGO) build --manifest-path native/fffbridge/Cargo.toml --release --target-dir "$(CURDIR)/build/cargo"
	install -d "$(NATIVE_LIB_DIR)" "$(NATIVE_INCLUDE_DIR)" "$(NATIVE_PKG_CONFIG_DIR)"
	install -m 755 build/cargo/release/libfffbridge.so "$(NATIVE_LIB_DIR)/libfffbridge.so"
	install -m 644 native/fffbridge/include/fffbridge.h "$(NATIVE_INCLUDE_DIR)/fffbridge.h"
	install -m 644 native/fffbridge/fffbridge.pc "$(NATIVE_PKG_CONFIG_DIR)/fffbridge.pc"

native-faiss:
	@if [ ! -d "$(FAISS_DIR)/.git" ]; then \
		mkdir -p "$(dir $(FAISS_DIR))" && \
		git clone https://github.com/blevesearch/faiss.git "$(FAISS_DIR)" && \
		git -C "$(FAISS_DIR)" checkout --detach $(FAISS_COMMIT); \
	else \
		actual=$$(git -C "$(FAISS_DIR)" rev-parse HEAD); \
		test "$$actual" = "$(FAISS_COMMIT)" || { \
			printf 'FAISS checkout at %s is %s, expected %s; refusing to alter it\n' "$(FAISS_DIR)" "$$actual" "$(FAISS_COMMIT)" >&2; \
			exit 1; \
		}; \
	fi
	git -C "$(FAISS_DIR)" submodule update --init --recursive
	cmake -S "$(FAISS_DIR)" -B "$(FAISS_DIR)/build" \
		-DFAISS_ENABLE_GPU=OFF \
		-DFAISS_ENABLE_C_API=ON \
		-DBUILD_SHARED_LIBS=ON \
		-DFAISS_ENABLE_PYTHON=OFF \
		-DBUILD_TESTING=OFF \
		-DFAISS_OPT_LEVEL=generic \
		-DCMAKE_CXX_FLAGS=-I$(CURDIR)/$(FAISS_DIR)
	cmake --build "$(FAISS_DIR)/build" --parallel $(JOBS)
	cmake --install "$(FAISS_DIR)/build" --prefix "$(NATIVE_PREFIX)"
	@test -f "$(FAISS_LIB_DIR)/libfaiss_c.so"
	@test -f "$(FAISS_LIB_DIR)/libfaiss.so"

native:
	$(MAKE) native-fff
	$(MAKE) native-faiss

build: native
	PKG_CONFIG_PATH="$(NATIVE_PKG_CONFIG_DIR):$${PKG_CONFIG_PATH:-}" \
	CGO_ENABLED=1 CGO_CFLAGS="-I$(FAISS_INCLUDE_DIR)" CGO_LDFLAGS="-L$(FAISS_LIB_DIR) -Wl,-rpath,$(FAISS_LIB_DIR)" \
	LD_LIBRARY_PATH="$(NATIVE_LIB_DIR):$${LD_LIBRARY_PATH:-}" \
		$(GO) build -tags=vectors $(GO_PACKAGES)

rust-test:
	cargo fmt --manifest-path native/fffbridge/Cargo.toml -- --check
	CARGO_TARGET_DIR="$(CURDIR)/build/cargo" cargo test --manifest-path native/fffbridge/Cargo.toml
	CARGO_TARGET_DIR="$(CURDIR)/build/cargo" cargo clippy --manifest-path native/fffbridge/Cargo.toml --all-targets -- -D warnings

test: native
	PKG_CONFIG_PATH="$(NATIVE_PKG_CONFIG_DIR):$${PKG_CONFIG_PATH:-}" \
	CGO_ENABLED=1 CGO_CFLAGS="-I$(FAISS_INCLUDE_DIR)" CGO_LDFLAGS="-L$(FAISS_LIB_DIR) -Wl,-rpath,$(FAISS_LIB_DIR)" \
	LD_LIBRARY_PATH="$(NATIVE_LIB_DIR):$${LD_LIBRARY_PATH:-}" \
		$(GO) test -tags=vectors -race -count=1 -timeout=10m $(GO_PACKAGES)

test-fast: native
	PKG_CONFIG_PATH="$(NATIVE_PKG_CONFIG_DIR):$${PKG_CONFIG_PATH:-}" \
	CGO_ENABLED=1 CGO_CFLAGS="-I$(FAISS_INCLUDE_DIR)" CGO_LDFLAGS="-L$(FAISS_LIB_DIR) -Wl,-rpath,$(FAISS_LIB_DIR)" \
	LD_LIBRARY_PATH="$(NATIVE_LIB_DIR):$${LD_LIBRARY_PATH:-}" \
		$(GO) test -tags=vectors -count=1 $(GO_PACKAGES)

race: native
	PKG_CONFIG_PATH="$(NATIVE_PKG_CONFIG_DIR):$${PKG_CONFIG_PATH:-}" \
	CGO_ENABLED=1 CGO_CFLAGS="-I$(FAISS_INCLUDE_DIR)" CGO_LDFLAGS="-L$(FAISS_LIB_DIR) -Wl,-rpath,$(FAISS_LIB_DIR)" \
	LD_LIBRARY_PATH="$(NATIVE_LIB_DIR):$${LD_LIBRARY_PATH:-}" \
		$(GO) test -tags=vectors -race -count=1 -timeout=10m $(GO_PACKAGES)

vet: native
	PKG_CONFIG_PATH="$(NATIVE_PKG_CONFIG_DIR):$${PKG_CONFIG_PATH:-}" \
	CGO_ENABLED=1 CGO_CFLAGS="-I$(FAISS_INCLUDE_DIR)" CGO_LDFLAGS="-L$(FAISS_LIB_DIR) -Wl,-rpath,$(FAISS_LIB_DIR)" \
	LD_LIBRARY_PATH="$(NATIVE_LIB_DIR):$${LD_LIBRARY_PATH:-}" \
		$(GO) vet -tags=vectors $(GO_PACKAGES)

fmt:
	gofmt -w search/*.go internal/fffbridge/*.go
	$(CARGO) fmt --manifest-path native/fffbridge/Cargo.toml
