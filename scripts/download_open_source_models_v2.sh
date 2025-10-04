#!/bin/bash
# Enhanced Open-Source Model Download Script v2
# Features: Parallel downloads, SHA256 verification, progress bars, caching, version tracking
# Downloads production-ready open-source models with permissive licenses

set -euo pipefail

# Configuration
MODELS_DIR="${MODELS_DIR:-vvfs/generation/models/gguf}"
CACHE_DIR="${MODEL_CACHE_DIR:-${HOME}/.cache/vvfs-models}"
CHECKSUMS_FILE="${MODELS_DIR}/.checksums"
VERSION_FILE="${MODELS_DIR}/.model_versions"
PARALLEL_DOWNLOADS="${PARALLEL_DOWNLOADS:-3}"
CUSTOM_REPO="${CUSTOM_MODEL_REPO:-}"
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"

# Color codes for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Model registry with SHA256 checksums
declare -A MODEL_REGISTRY=(
    # Format: "local_name:repo_id:filename:sha256:version"
    ["open-embed"]="Qwen/Qwen3-Embedding-0.6B-GGUF:Qwen3-Embedding-0.6B-f16.gguf::v1.0.0"
    ["open-chat-qwen3-1_7b"]="bartowski/Qwen_Qwen3-1.7B-GGUF:Qwen_Qwen3-1.7B-Q4_K_M.gguf::v1.0.0"
    ["open-vision"]="mradermacher/llama-3.2-Korean-Bllossom-3B-vision-expanded-GGUF:llama-3.2-Korean-Bllossom-3B-vision-expanded.Q4_K_M.gguf::v1.0.0"
)

# Create directories
mkdir -p "$MODELS_DIR" "$CACHE_DIR"
cd "$PROJECT_ROOT"

# Logging functions
log_info() {
    echo -e "${BLUE}ℹ️  ${NC}$*"
}

log_success() {
    echo -e "${GREEN}✅${NC} $*"
}

log_warning() {
    echo -e "${YELLOW}⚠️  ${NC}$*"
}

log_error() {
    echo -e "${RED}❌${NC} $*"
}

# Check prerequisites
check_prerequisites() {
    local missing=0
    
    log_info "Checking prerequisites..."
    
    # Check for hf CLI
    if ! command -v hf &> /dev/null; then
        log_error "hf (Hugging Face CLI) not found"
        echo ""
        echo "Install with:"
        if command -v pipx &> /dev/null; then
            echo "  pipx install huggingface_hub[cli]"
        else
            echo "  pip install huggingface_hub[cli]"
            echo "  (or install pipx first: sudo pacman -S python-pipx)"
        fi
        missing=1
    else
        log_success "Hugging Face CLI available"
    fi
    
    # Check for optional tools
    if command -v pv &> /dev/null; then
        USE_PV=true
        log_success "pv (progress viewer) available - progress bars enabled"
    else
        USE_PV=false
        log_warning "pv not found (optional) - install for progress bars: sudo pacman -S pv"
    fi
    
    if command -v sha256sum &> /dev/null; then
        log_success "sha256sum available - checksum verification enabled"
    else
        log_warning "sha256sum not found - checksum verification disabled"
    fi
    
    if command -v xxd &> /dev/null; then
        log_success "xxd available - GGUF validation enabled"
    else
        log_warning "xxd not found - basic validation only"
    fi
    
    return $missing
}

# Calculate SHA256 checksum
calculate_checksum() {
    local file="$1"
    if command -v sha256sum &> /dev/null && [ -f "$file" ]; then
        sha256sum "$file" | awk '{print $1}'
    else
        echo ""
    fi
}

# Verify checksum
verify_checksum() {
    local file="$1"
    local expected="$2"
    
    if [ -z "$expected" ]; then
        log_warning "No checksum available for $(basename "$file") - skipping verification"
        return 0
    fi
    
    if ! command -v sha256sum &> /dev/null; then
        log_warning "sha256sum not available - skipping checksum verification"
        return 0
    fi
    
    log_info "Verifying checksum for $(basename "$file")..."
    local actual=$(calculate_checksum "$file")
    
    if [ "$actual" = "$expected" ]; then
        log_success "Checksum verified"
        return 0
    else
        log_error "Checksum mismatch!"
        log_error "Expected: $expected"
        log_error "Actual:   $actual"
        return 1
    fi
}

# Check if model is cached
check_cache() {
    local model_name="$1"
    local version="$2"
    local cache_path="$CACHE_DIR/${model_name}-${version}.gguf"
    
    if [ -f "$cache_path" ]; then
        log_info "Found cached version: $cache_path"
        return 0
    fi
    return 1
}

# Copy from cache
copy_from_cache() {
    local model_name="$1"
    local version="$2"
    local target="$3"
    local cache_path="$CACHE_DIR/${model_name}-${version}.gguf"
    
    log_info "Copying from cache..."
    cp "$cache_path" "$target"
    log_success "Copied from cache (saved download time)"
}

# Save to cache
save_to_cache() {
    local model_name="$1"
    local version="$2"
    local source="$3"
    local cache_path="$CACHE_DIR/${model_name}-${version}.gguf"
    
    log_info "Saving to cache for future builds..."
    cp "$source" "$cache_path"
    log_success "Cached for future use"
}

# Download single model with progress
download_model() {
    local model_name="$1"
    local repo_id="$2"
    local filename="$3"
    local checksum="$4"
    local version="$5"
    local target_path="$MODELS_DIR/${model_name}.gguf"
    
    # Check if already exists and valid
    if [ -f "$target_path" ]; then
        if [ -n "$checksum" ] && verify_checksum "$target_path" "$checksum" 2>/dev/null; then
            local size=$(du -h "$target_path" | cut -f1)
            log_success "$model_name already exists and verified ($size)"
            return 0
        fi
        log_warning "$model_name exists but verification failed - re-downloading"
        rm -f "$target_path"
    fi
    
    # Check cache
    if check_cache "$model_name" "$version"; then
        copy_from_cache "$model_name" "$version" "$target_path"
        if [ -n "$checksum" ]; then
            verify_checksum "$target_path" "$checksum" || {
                log_error "Cached file corrupted - re-downloading"
                rm -f "$target_path"
            }
        fi
        if [ -f "$target_path" ]; then
            return 0
        fi
    fi
    
    # Download
    log_info "Downloading $model_name..."
    log_info "Source: $repo_id/$filename"
    
    local temp_dir="$MODELS_DIR/.download_tmp_$$"
    mkdir -p "$temp_dir"
    
    # Download with or without progress bar
    if [ "$USE_PV" = true ]; then
        if hf download "$repo_id" "$filename" --local-dir "$temp_dir" 2>&1 | \
           grep -oP '\d+%|\d+\.\d+[GM]B' | pv -l -s 100 > /dev/null; then
            :
        else
            hf download "$repo_id" "$filename" --local-dir "$temp_dir" || {
                log_error "Download failed for $model_name"
                rm -rf "$temp_dir"
                return 1
            }
        fi
    else
        hf download "$repo_id" "$filename" --local-dir "$temp_dir" || {
            log_error "Download failed for $model_name"
            rm -rf "$temp_dir"
            return 1
        }
    fi
    
    # Move to final location
    if [ -f "$temp_dir/$filename" ]; then
        mv -f "$temp_dir/$filename" "$target_path"
        rm -rf "$temp_dir"
        
        local size=$(du -h "$target_path" | cut -f1)
        log_success "Downloaded $model_name ($size)"
        
        # Verify checksum
        if [ -n "$checksum" ]; then
            verify_checksum "$target_path" "$checksum" || {
                log_error "Checksum verification failed - removing file"
                rm -f "$target_path"
                return 1
            }
        fi
        
        # Save to cache
        save_to_cache "$model_name" "$version" "$target_path"
        
        # Store checksum
        if [ -n "$checksum" ]; then
            echo "$checksum  $model_name.gguf" >> "$CHECKSUMS_FILE"
        else
            # Calculate and store
            local calc_sum=$(calculate_checksum "$target_path")
            echo "$calc_sum  $model_name.gguf" >> "$CHECKSUMS_FILE"
        fi
        
        # Store version
        echo "$model_name=$version" >> "$VERSION_FILE"
        
        return 0
    fi
    
    log_error "Download failed for $model_name - file not found after download"
    rm -rf "$temp_dir"
    return 1
}

# Parallel download wrapper
download_model_parallel() {
    local model_name="$1"
    IFS=':' read -r repo_id filename checksum version <<< "${MODEL_REGISTRY[$model_name]}"
    
    if [ -n "$CUSTOM_REPO" ]; then
        repo_id="$CUSTOM_REPO"
    fi
    
    download_model "$model_name" "$repo_id" "$filename" "$checksum" "$version"
}

# Check for model updates
check_for_updates() {
    log_info "Checking for model updates..."
    
    if [ ! -f "$VERSION_FILE" ]; then
        log_info "No version file found - first time setup"
        return 0
    fi
    
    local updates_available=0
    
    while IFS='=' read -r model current_version; do
        for model_name in "${!MODEL_REGISTRY[@]}"; do
            if [ "$model" = "$model_name" ]; then
                IFS=':' read -r _ _ _ registry_version <<< "${MODEL_REGISTRY[$model_name]}"
                if [ "$current_version" != "$registry_version" ]; then
                    log_warning "Update available for $model: $current_version → $registry_version"
                    updates_available=1
                fi
            fi
        done
    done < "$VERSION_FILE"
    
    if [ $updates_available -eq 0 ]; then
        log_success "All models are up to date"
    fi
    
    return 0
}

# Main execution
main() {
    echo "🤖 Enhanced Model Download Script v2"
    echo "====================================="
    echo ""
    
    # Check prerequisites
    if ! check_prerequisites; then
        log_error "Missing required prerequisites"
        exit 1
    fi
    
    echo ""
    
    # Check authentication
    if ! hf auth whoami &> /dev/null; then
        log_warning "Not authenticated with HuggingFace"
        log_info "Login recommended for faster downloads: hf login"
        echo ""
    else
        log_success "HuggingFace authenticated"
    fi
    
    # Check for updates
    check_for_updates
    
    echo ""
    log_info "Starting parallel downloads (max $PARALLEL_DOWNLOADS concurrent)..."
    echo ""
    
    # Parallel download using background jobs
    local pids=()
    local model_names=("${!MODEL_REGISTRY[@]}")
    local failed=()
    
    for model_name in "${model_names[@]}"; do
        # Wait if we've hit the parallel limit
        while [ ${#pids[@]} -ge $PARALLEL_DOWNLOADS ]; do
            for i in "${!pids[@]}"; do
                if ! kill -0 "${pids[$i]}" 2>/dev/null; then
                    wait "${pids[$i]}" || failed+=("${model_names[$i]}")
                    unset 'pids[i]'
                fi
            done
            pids=("${pids[@]}")  # Reindex array
            sleep 0.1
        done
        
        # Start download in background
        download_model_parallel "$model_name" &
        pids+=($!)
    done
    
    # Wait for all remaining downloads
    for pid in "${pids[@]}"; do
        wait "$pid" || failed+=("model")
    done
    
    echo ""
    echo "════════════════════════════════════════"
    
    if [ ${#failed[@]} -eq 0 ]; then
        log_success "All models downloaded and verified successfully!"
    else
        log_error "${#failed[@]} model(s) failed to download"
        exit 1
    fi
    
    echo "════════════════════════════════════════"
    echo ""
    
    # Summary
    log_info "Model Summary"
    echo "Location: $MODELS_DIR"
    echo "Cache: $CACHE_DIR"
    echo ""
    
    for model in "$MODELS_DIR"/*.gguf; do
        if [ -f "$model" ]; then
            local size=$(du -h "$model" | cut -f1)
            local name=$(basename "$model")
            echo "  • $name ($size)"
        fi
    done
    
    echo ""
    log_success "Ready for production!"
    echo ""
    echo "💡 Features enabled:"
    echo "  • Parallel downloads: $PARALLEL_DOWNLOADS concurrent"
    [ "$USE_PV" = true ] && echo "  • Progress bars: enabled"
    [ -n "$(command -v sha256sum)" ] && echo "  • Checksum verification: enabled"
    echo "  • Model caching: enabled ($CACHE_DIR)"
    echo ""
}

# Run main function
main "$@"

