#ifndef FFFBRIDGE_PROVIDER_H
#define FFFBRIDGE_PROVIDER_H
#include "fffbridge.h"

typedef struct FffbApi FffbApi;
/* Bind/pin one coherent strict provider BEFORE constructing any Rust handle.
 * Binding errors use libc allocation, not a Rust provider's string allocator. */
FffbApi *fffb_api_bind_root_only(char **error);
void fffb_api_release(FffbApi *api);
FffbHandle *fffb_api_open(FffbApi *, const char *, const char *, char **);
void fffb_api_close(FffbApi *, FffbHandle *);
void fffb_api_free_string(FffbApi *, char *);
int32_t fffb_api_wait_ready(FffbApi *, FffbHandle *, uint32_t);
char *fffb_api_status(FffbApi *, FffbHandle *);
char *fffb_api_inventory(FffbApi *, FffbHandle *, uint64_t, uint32_t);
char *fffb_api_search_paths(FffbApi *, FffbHandle *, const char *, uint64_t, uint32_t);
char *fffb_api_live_grep(FffbApi *, FffbHandle *, const char *, const char *, uint64_t, uint32_t, uint64_t, uint64_t);
char *fffb_api_poll_events(FffbApi *, FffbHandle *, uint32_t, uint32_t);
char *fffb_api_request_rescan(FffbApi *, FffbHandle *);
int32_t fffb_api_wait_rescan(FffbApi *, FffbHandle *, uint64_t, uint32_t);
#endif
