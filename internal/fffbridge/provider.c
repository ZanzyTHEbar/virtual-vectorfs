#include "provider.h"
#include <dlfcn.h>
#include <stdlib.h>
#include <string.h>

typedef FffbHandle *(*open_fn)(const char *, const char *, char **);
typedef char *(*json_fn)(FffbHandle *);
typedef int32_t (*wait_rescan_fn)(FffbHandle *, uint64_t, uint32_t);

struct FffbApi {
    void *provider;
    open_fn open_root_only;
    void (*close)(FffbHandle *);
    void (*free_string)(char *);
    int32_t (*wait_ready)(FffbHandle *, uint32_t);
    json_fn status;
    char *(*inventory)(FffbHandle *, uint64_t, uint32_t);
    char *(*search_paths)(FffbHandle *, const char *, uint64_t, uint32_t);
    char *(*live_grep)(FffbHandle *, const char *, const char *, uint64_t, uint32_t, uint64_t, uint64_t);
    char *(*poll_events)(FffbHandle *, uint32_t, uint32_t);
    json_fn request_rescan;
    wait_rescan_fn wait_rescan;
};

static int same_provider(void *function, void *base) {
    Dl_info info;
    return function != NULL && dladdr(function, &info) != 0 && info.dli_fbase == base;
}

FffbApi *fffb_api_bind_root_only(char **error) {
    Dl_info anchor;
    *error = NULL;
    FffbApi *api = calloc(1, sizeof(*api));
    const char *failure = "RootOnly unavailable: cannot identify bound native provider";
    if (api == NULL) {
        *error = strdup("RootOnly unavailable: provider allocation failed");
        return NULL;
    }
    /* These addresses are the bound legacy APIs, not independent global
     * lookups. A provider mismatch is refused before strict open is called. */
    if (dladdr((void *)fffb_open, &anchor) == 0 || anchor.dli_fname == NULL)
        goto fail;
#define CAPTURE(member, symbol) \
    api->member = symbol; \
    if (!same_provider((void *)api->member, anchor.dli_fbase)) goto fail
    failure = "RootOnly unavailable: bound legacy native providers differ";
    CAPTURE(close, fffb_close);
    CAPTURE(free_string, fffb_free_string);
    CAPTURE(wait_ready, fffb_wait_ready);
    CAPTURE(status, fffb_status);
    CAPTURE(inventory, fffb_inventory);
    CAPTURE(search_paths, fffb_search_paths);
    CAPTURE(live_grep, fffb_live_grep);
    CAPTURE(poll_events, fffb_poll_events);
#undef CAPTURE
    failure = "RootOnly unavailable: cannot pin bound native provider";
    api->provider = dlopen(anchor.dli_fname, RTLD_NOW | RTLD_LOCAL | RTLD_NOLOAD);
    if (api->provider == NULL) goto fail;
    /* A pathname is not provider identity: verify the reopened handle too. */
    if (dlsym(api->provider, "fffb_open") != (void *)fffb_open) goto fail;
    api->open_root_only = (open_fn)dlsym(api->provider, "fffb_open_root_only_v1");
    api->request_rescan = (json_fn)dlsym(api->provider, "fffb_request_rescan_v2");
    api->wait_rescan = (wait_rescan_fn)dlsym(api->provider, "fffb_wait_rescan_v2");
    failure = "RootOnly unavailable: bound native provider lacks the scoped-read v2 ABI";
    if (api->open_root_only == NULL || api->request_rescan == NULL || api->wait_rescan == NULL)
        goto fail;
    failure = "RootOnly unavailable: native extension provider differs from bound legacy provider";
    if (!same_provider((void *)api->open_root_only, anchor.dli_fbase) ||
        !same_provider((void *)api->request_rescan, anchor.dli_fbase) ||
        !same_provider((void *)api->wait_rescan, anchor.dli_fbase)) goto fail;
    return api;
fail:
    *error = strdup(failure);
    fffb_api_release(api);
    return NULL;
}

void fffb_api_release(FffbApi *api) {
    if (api == NULL) return;
    if (api->provider != NULL) dlclose(api->provider);
    free(api);
}

/* NULL dispatch preserves Ambient's linked ABI. Strict handles always retain
 * their pinned, validated dispatch, including error-string free and destruction. */
FffbHandle *fffb_api_open(FffbApi *a, const char *r, const char *o, char **e) {
    return a ? a->open_root_only(r, o, e) : fffb_open(r, o, e);
}
void fffb_api_close(FffbApi *a, FffbHandle *h) { if (a) a->close(h); else fffb_close(h); }
void fffb_api_free_string(FffbApi *a, char *s) { if (a) a->free_string(s); else fffb_free_string(s); }
int32_t fffb_api_wait_ready(FffbApi *a, FffbHandle *h, uint32_t t) {
    return a ? a->wait_ready(h, t) : fffb_wait_ready(h, t);
}
char *fffb_api_status(FffbApi *a, FffbHandle *h) { return a ? a->status(h) : fffb_status(h); }
char *fffb_api_inventory(FffbApi *a, FffbHandle *h, uint64_t o, uint32_t n) {
    return a ? a->inventory(h, o, n) : fffb_inventory(h, o, n);
}
char *fffb_api_search_paths(FffbApi *a, FffbHandle *h, const char *q, uint64_t o, uint32_t n) {
    return a ? a->search_paths(h, q, o, n) : fffb_search_paths(h, q, o, n);
}
char *fffb_api_live_grep(FffbApi *a, FffbHandle *h, const char *q, const char *m, uint64_t o, uint32_t n, uint64_t b, uint64_t c) {
    return a ? a->live_grep(h, q, m, o, n, b, c) : fffb_live_grep(h, q, m, o, n, b, c);
}
char *fffb_api_poll_events(FffbApi *a, FffbHandle *h, uint32_t t, uint32_t n) {
    return a ? a->poll_events(h, t, n) : fffb_poll_events(h, t, n);
}
char *fffb_api_request_rescan(FffbApi *a, FffbHandle *h) { return a ? a->request_rescan(h) : NULL; }
int32_t fffb_api_wait_rescan(FffbApi *a, FffbHandle *h, uint64_t ticket, uint32_t t) {
    return a ? a->wait_rescan(h, ticket, t) : -1;
}
