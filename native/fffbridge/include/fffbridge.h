#ifndef FFFBRIDGE_H
#define FFFBRIDGE_H

#include <stdint.h>

#ifdef __cplusplus
extern "C" {
#endif

typedef struct FffbHandle FffbHandle;

/*
 * root_utf8 and opts_json are NUL-terminated UTF-8. opts_json may be NULL.
 * Options: enable_content_indexing (false), enable_mmap_cache (false), and
 * cache_budget {max_files, max_bytes, max_file_size}. No frecency database,
 * Git recency, query history, or AI mode is enabled by this bridge.
 * On failure, returns NULL and writes an allocated error string when err is
 * non-NULL. Free all returned strings with fffb_free_string.
 */
FffbHandle *fffb_open(const char *root_utf8, const char *opts_json, char **err);
/* Additive v1 ABI; old fffb_open always retains Ambient behavior.
 * RootOnly is a LOCAL PROTOTYPE, unsupported by default native builds. It fails
 * explicitly unless built with scoped-reads-prototype and the reviewed local
 * FFF patch. No Git or ignore interpretation; hidden files included, symlinks
 * excluded. Requires trusted stable ancestry AND indexed leaves, not an OS
 * sandbox/caller capability boundary. opts_json has the same cache-only schema.
 * Do not fall back to fffb_open if this entry point is absent or fails. */
FffbHandle *fffb_open_root_only_v1(const char *root_utf8, const char *opts_json,
                                char **err);
/* Retained v1 symbol; now explicitly returns a JSON error because generic idle
 * flags cannot fence its request. Use the v2 pair, not a v1/readiness fallback. */
char *fffb_request_rescan_v1(FffbHandle *handle);
/* Returns allocated {"ticket":N} for a real native request, or a JSON error.
 * At most one outstanding ticket per handle; no unbounded ticket registry. */
char *fffb_request_rescan_v2(FffbHandle *handle);
/* 1 completed by a qualifying successful scan, 0 timeout, -1 failed/invalid or
 * cancelled. Tickets belong to this handle/picker; readiness is NOT its fence. */
int32_t fffb_wait_rescan_v2(FffbHandle *handle, uint64_t ticket, uint32_t timeout_ms);
void fffb_close(FffbHandle *handle);

/* Returns 1 when scan/post-scan indexing and watcher are ready, 0 on timeout,
 * -2 for terminal watch-subscription failure, and -1 for an invalid handle or
 * caught panic. */
int32_t fffb_wait_ready(FffbHandle *handle, uint32_t timeout_ms);

/* JSON methods return allocated JSON; operational errors (including a NULL
 * handle) are returned as {"error":"..."}. A caught panic returns NULL.
 * Inventory/search pages are capped at FFFB_MAX_PAGE_SIZE items. */
#define FFFB_MAX_PAGE_SIZE 10000u
char *fffb_status(FffbHandle *handle);
/* Inventory offset and next_offset index the raw path-sorted FFF slice,
 * including tombstones. Each page scans at most limit raw entries; items omit
 * deleted entries. total is the live-file count, and done follows the raw
 * slice cursor (so a page may have no items and still have a continuation). */
char *fffb_inventory(FffbHandle *handle, uint64_t offset, uint32_t limit);
char *fffb_search_paths(FffbHandle *handle, const char *query_utf8,
                        uint64_t offset, uint32_t limit);
char *fffb_live_grep(FffbHandle *handle, const char *query_utf8,
                     const char *mode_utf8, uint64_t file_offset,
                     uint32_t limit, uint64_t context_before,
                     uint64_t context_after);
char *fffb_poll_events(FffbHandle *handle, uint32_t timeout_ms,
                       uint32_t max_events);
void fffb_free_string(char *value);

/* Paths are UTF-8 in this initial ABI. Inventory/search results flag U+FFFD
 * paths as possibly lossy; watch events with non-UTF-8 paths contain
 * path:null and path_bytes_hex so callers can report/rescan them explicitly.
 * A rescan_required event with reason "watch_subscription_gap" tells callers
 * to establish a baseline after the FFF subscription is installed.
 * Calls on one handle may run concurrently, except close requires all calls
 * to have returned. Handles and input pointers must be valid for each call. */
/* Every operation, string-free and destructor MUST use the same loaded provider
 * that opened the handle. Never mix independently resolved global providers.
 * RootOnly validates absolute non-symlink ancestry BEFORE canonicalizing, but
 * still requires stable ancestry/indexed leaves; it does not pin identities. */

#ifdef __cplusplus
}
#endif

#endif
