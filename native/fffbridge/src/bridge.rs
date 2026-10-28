use fff_search::file_picker::{FFFMode, FilePicker, FilePickerOptions, FuzzySearchOptions};
use fff_search::grep::{GrepMode, GrepSearchOptions};
use fff_search::shared::{SharedFilePicker, SharedFrecency};
use fff_search::types::{ContentCacheBudget, PaginationArgs};
use fff_search::watch::{WatchEvent, WatchOptions};
use fff_search::{GitRecencyConfig, GrepConfig, QueryParser};
use serde_json::{json, Map, Value};
use std::collections::VecDeque;
use std::os::unix::ffi::OsStrExt;
use std::path::Path;
use std::sync::atomic::{AtomicBool, Ordering};
use std::sync::{Arc, Condvar, Mutex};
use std::thread::JoinHandle;
use std::time::{Duration, Instant};

const EVENT_QUEUE_CAPACITY: usize = 4096;
const MAX_PAGE_SIZE: usize = 10_000;
const PATH_REPLACEMENT: char = '\u{fffd}';
// FFF's page_limit is a match target; file_offset is its cursor. The per-file cap
// bounds result-file overshoot, while the time budget bounds sparse candidate scans.
const MAX_GREP_FILES_PER_PAGE: usize = 100;
const MAX_GREP_MATCHES_PER_FILE: usize = 20;
const MAX_GREP_MATCH_TARGET: usize = MAX_GREP_FILES_PER_PAGE - MAX_GREP_MATCHES_PER_FILE;
const MAX_GREP_CONTEXT_LINES: usize = 5;
// Keep each displayed text line and context line bounded after FFF's own line truncation.
const MAX_GREP_LINE_BYTES: usize = 384;
const MAX_GREP_FILE_BYTES: u64 = 16 * 1024 * 1024;
const MAX_GREP_TIME_BUDGET_MS: u64 = 500;

#[derive(Debug, Default, Clone, Copy, PartialEq, Eq)]
pub(super) enum ReadPolicy {
    #[default]
    Ambient,
    RootOnly,
}

#[derive(Debug)]
pub(super) struct BridgeOptions {
    read_policy: ReadPolicy,
    enable_content_indexing: bool,
    enable_mmap_cache: bool,
    cache_budget: Option<ContentCacheBudget>,
}

impl Default for BridgeOptions {
    fn default() -> Self {
        Self {
            read_policy: ReadPolicy::Ambient,
            enable_content_indexing: false,
            enable_mmap_cache: false,
            cache_budget: Some(ContentCacheBudget::with_max_files(0)),
        }
    }
}

impl BridgeOptions {
    pub(super) fn with_read_policy(mut self, read_policy: ReadPolicy) -> Self {
        self.read_policy = read_policy;
        self
    }

    pub(super) fn parse(json_text: Option<&str>) -> Result<Self, String> {
        let Some(json_text) = json_text else {
            return Ok(Self::default());
        };
        let value: Value = serde_json::from_str(json_text)
            .map_err(|error| format!("invalid options JSON: {error}"))?;
        if value.is_null() {
            return Ok(Self::default());
        }
        let object = value
            .as_object()
            .ok_or_else(|| "options JSON must be an object or null".to_owned())?;
        for key in object.keys() {
            if !matches!(
                key.as_str(),
                "enable_content_indexing" | "enable_mmap_cache" | "cache_budget"
            ) {
                return Err(format!("unknown option: {key}"));
            }
        }

        let mut options = Self {
            enable_content_indexing: optional_bool(object, "enable_content_indexing")?
                .unwrap_or(false),
            enable_mmap_cache: optional_bool(object, "enable_mmap_cache")?.unwrap_or(false),
            ..Self::default()
        };

        if let Some(budget) = object.get("cache_budget") {
            if !budget.is_null() {
                let budget = budget
                    .as_object()
                    .ok_or_else(|| "cache_budget must be an object or null".to_owned())?;
                for key in budget.keys() {
                    if !matches!(key.as_str(), "max_files" | "max_bytes" | "max_file_size") {
                        return Err(format!("unknown cache_budget field: {key}"));
                    }
                }
                let max_files = optional_usize(budget, "max_files")?.unwrap_or(0);
                let max_bytes = optional_u64(budget, "max_bytes")?.unwrap_or(0);
                let max_file_size = optional_u64(budget, "max_file_size")?.unwrap_or(0);
                options.cache_budget =
                    Some(if max_files == 0 && max_bytes == 0 && max_file_size == 0 {
                        ContentCacheBudget::with_max_files(0)
                    } else {
                        ContentCacheBudget::from_overrides(max_files, max_bytes, max_file_size)
                            .ok_or_else(|| "invalid cache_budget".to_owned())?
                    });
            }
        }

        Ok(options)
    }
}

fn optional_bool(object: &Map<String, Value>, key: &str) -> Result<Option<bool>, String> {
    object
        .get(key)
        .map(|value| {
            value
                .as_bool()
                .ok_or_else(|| format!("{key} must be a boolean"))
        })
        .transpose()
}

fn optional_u64(object: &Map<String, Value>, key: &str) -> Result<Option<u64>, String> {
    object
        .get(key)
        .map(|value| {
            value
                .as_u64()
                .ok_or_else(|| format!("{key} must be a non-negative integer"))
        })
        .transpose()
}

fn optional_usize(object: &Map<String, Value>, key: &str) -> Result<Option<usize>, String> {
    optional_u64(object, key)?
        .map(|value| usize::try_from(value).map_err(|_| format!("{key} is too large")))
        .transpose()
}

#[derive(Default)]
struct EventQueue {
    events: Mutex<VecDeque<Value>>,
    ready: Condvar,
}

impl EventQueue {
    fn push(&self, event: Value) {
        let mut events = self
            .events
            .lock()
            .unwrap_or_else(|error| error.into_inner());
        if events.len() == EVENT_QUEUE_CAPACITY {
            events.clear();
            events.push_back(json!({
                "kind": "rescan",
                "path": "",
                "reason": "bridge_queue_overflow",
                "rescan_required": true
            }));
        } else {
            events.push_back(event);
        }
        drop(events);
        self.ready.notify_all();
    }

    fn drain(&self, timeout: Duration, max_events: usize) -> Vec<Value> {
        if max_events == 0 {
            return Vec::new();
        }
        let mut events = self
            .events
            .lock()
            .unwrap_or_else(|error| error.into_inner());
        let start = Instant::now();
        while events.is_empty() {
            let remaining = timeout.saturating_sub(start.elapsed());
            if remaining.is_zero() {
                break;
            }
            events = match self.ready.wait_timeout(events, remaining) {
                Ok((events, _)) => events,
                Err(error) => error.into_inner().0,
            };
        }
        let count = max_events.min(events.len());
        events.drain(..count).collect()
    }
}

#[derive(Default)]
struct WatchState {
    watch_id: Option<fff_search::watch::WatchId>,
    error: Option<String>,
    finished: bool,
}

#[derive(Default)]
struct WatchStartup {
    state: Mutex<WatchState>,
    ready: Condvar,
    cancelled: AtomicBool,
}

pub(super) struct FffbHandle {
    picker: SharedFilePicker,
    events: Arc<EventQueue>,
    watch_startup: Arc<WatchStartup>,
    watch_worker: Option<JoinHandle<()>>,
    #[cfg(feature = "scoped-reads-prototype")]
    rescan_ticket: Mutex<Option<fff_search::RescanTicket>>,
}

impl FffbHandle {
    pub(super) fn open(root: &str, options: BridgeOptions) -> Result<Self, String> {
        #[cfg(not(feature = "scoped-reads-prototype"))]
        if options.read_policy == ReadPolicy::RootOnly {
            return Err("RootOnly requires the local scoped-reads-prototype native build and reviewed FFF patch".to_owned());
        }
        if root.is_empty() {
            return Err("root path must not be empty".to_owned());
        }
        let input_root = Path::new(root);
        if options.read_policy == ReadPolicy::RootOnly {
            validate_root_only_root(input_root)?;
        }
        let root = input_root
            .canonicalize()
            .map_err(|error| format!("cannot resolve root path: {error}"))?;
        if !root.is_dir() {
            return Err("root path is not a directory".to_owned());
        }
        let root_utf8 = root
            .to_str()
            .ok_or_else(|| "canonical root path is not UTF-8".to_owned())?;

        let picker = SharedFilePicker::default();
        let events = Arc::new(EventQueue::default());

        let shared_frecency = SharedFrecency::default();
        let fff_options = FilePickerOptions {
            base_path: root_utf8.to_owned(),
            enable_content_indexing: options.enable_content_indexing,
            enable_mmap_cache: options.enable_mmap_cache,
            cache_budget: options.cache_budget,
            mode: FFFMode::Neovim,
            git_recency: GitRecencyConfig {
                enabled: false,
                ..GitRecencyConfig::default()
            },
            watch: true,
            follow_symlinks: false,
            enable_fs_root_scanning: false,
            enable_home_dir_scanning: false,
        };
        let start = match options.read_policy {
            ReadPolicy::Ambient => {
                FilePicker::new_with_shared_state(picker.clone(), shared_frecency, fff_options)
            }
            ReadPolicy::RootOnly => {
                #[cfg(feature = "scoped-reads-prototype")]
                {
                    FilePicker::new_with_shared_state_and_read_policy(
                        picker.clone(),
                        shared_frecency,
                        fff_options,
                        fff_search::ReadPolicy::RootOnly,
                    )
                }
                #[cfg(not(feature = "scoped-reads-prototype"))]
                unreachable!("RootOnly was rejected before native startup")
            }
        };
        if let Err(error) = start {
            stop_picker(&picker);
            return Err(format!("cannot start FFF picker: {error}"));
        }

        let watch_startup = Arc::new(WatchStartup::default());
        let immediate_watch = subscribe_events(&picker, &events, &root);
        let watch_worker = match immediate_watch {
            Ok(watch_id) => {
                set_watch_result(&watch_startup, Ok(watch_id));
                None
            }
            Err(error) if !picker.wait_for_watcher(Duration::ZERO) => {
                let startup = Arc::clone(&watch_startup);
                let worker_picker = picker.clone();
                let worker_events = Arc::clone(&events);
                let worker_root = root.clone();
                match std::thread::Builder::new()
                    .name("fffbridge-watch-startup".to_owned())
                    .spawn(move || {
                        while !startup.cancelled.load(Ordering::Acquire) {
                            if worker_picker.wait_for_watcher(Duration::from_millis(50)) {
                                if startup.cancelled.load(Ordering::Acquire) {
                                    break;
                                }
                                let result =
                                    subscribe_events(&worker_picker, &worker_events, &worker_root)
                                        .map_err(|error| {
                                            format!("cannot subscribe to FFF watch events: {error}")
                                        });
                                if result.is_err() {
                                    stop_picker(&worker_picker);
                                }
                                set_watch_result(&startup, result);
                                break;
                            }
                        }
                    }) {
                    Ok(worker) => Some(worker),
                    Err(spawn_error) => {
                        stop_picker(&picker);
                        return Err(format!(
                            "cannot start FFF watch subscription worker after {error}: {spawn_error}"
                        ));
                    }
                }
            }
            Err(error) => {
                stop_picker(&picker);
                return Err(format!("cannot subscribe to FFF watch events: {error}"));
            }
        };

        Ok(Self {
            picker,
            events,
            watch_startup,
            watch_worker,
            #[cfg(feature = "scoped-reads-prototype")]
            rescan_ticket: Mutex::new(None),
        })
    }

    pub(super) fn wait_ready(&self, timeout: Duration) -> Result<bool, String> {
        let start = Instant::now();
        if !self.picker.wait_for_watcher(timeout) {
            self.wait_for_subscription(Duration::ZERO)?;
            return Ok(false);
        }
        let remaining = timeout.saturating_sub(start.elapsed());
        if !self.wait_for_subscription(remaining)? {
            return Ok(false);
        }
        Ok(self
            .picker
            .wait_for_indexing_complete(timeout.saturating_sub(start.elapsed())))
    }

    pub(super) fn request_rescan(&self) -> Result<Value, String> {
        Err("unfenced v1 rescan is unavailable; use the v2 request/completion ABI".to_owned())
    }

    pub(super) fn request_rescan_v2(&self) -> Result<Value, String> {
        #[cfg(feature = "scoped-reads-prototype")]
        {
            let mut pending = self
                .rescan_ticket
                .lock()
                .map_err(|_| "rescan lock poisoned")?;
            if let Some(ticket) = pending.as_ref() {
                if matches!(ticket.wait(Duration::ZERO), Ok(false)) {
                    return Err("a fenced rescan is already outstanding".to_owned());
                }
            }
            let ticket = self
                .picker
                .request_rescan(&SharedFrecency::default())
                .map_err(|error| format!("cannot request fenced FFF rescan: {error}"))?;
            let sequence = ticket.sequence();
            *pending = Some(ticket);
            Ok(json!({"ticket": sequence}))
        }
        #[cfg(not(feature = "scoped-reads-prototype"))]
        {
            Err("fenced rescan requires the local scoped-reads-prototype native build".to_owned())
        }
    }

    pub(super) fn wait_rescan_v2(&self, sequence: u64, timeout: Duration) -> Result<bool, String> {
        #[cfg(feature = "scoped-reads-prototype")]
        {
            let pending = self
                .rescan_ticket
                .lock()
                .map_err(|_| "rescan lock poisoned")?;
            let ticket = pending
                .as_ref()
                .filter(|ticket| ticket.sequence() == sequence)
                .ok_or_else(|| "unknown or superseded native rescan ticket".to_owned())?;
            ticket
                .wait(timeout)
                .map_err(|error| format!("fenced FFF rescan failed: {error}"))
        }
        #[cfg(not(feature = "scoped-reads-prototype"))]
        {
            let _ = (sequence, timeout);
            Err("fenced rescan requires the local scoped-reads-prototype native build".to_owned())
        }
    }

    fn wait_for_subscription(&self, timeout: Duration) -> Result<bool, String> {
        let start = Instant::now();
        let mut state = self
            .watch_startup
            .state
            .lock()
            .unwrap_or_else(|error| error.into_inner());
        loop {
            if watch_subscription_ready(&state)? {
                return Ok(true);
            }
            let remaining = timeout.saturating_sub(start.elapsed());
            if remaining.is_zero() {
                return Ok(false);
            }
            state = match self.watch_startup.ready.wait_timeout(state, remaining) {
                Ok((state, _)) => state,
                Err(error) => error.into_inner().0,
            };
        }
    }

    pub(super) fn status(&self) -> Value {
        let guard = match self.picker.read() {
            Ok(guard) => guard,
            Err(error) => return json!({"error": error.to_string()}),
        };
        let Some(picker) = guard.as_ref() else {
            let watch_state = self
                .watch_startup
                .state
                .lock()
                .unwrap_or_else(|error| error.into_inner());
            return json!({
                "scan_active": false,
                "scan_complete": false,
                "post_scan_active": false,
                "watcher_ready": false,
                "watch_subscribed": watch_state.watch_id.is_some(),
                "watch_error": watch_state.error,
                "indexed_files": 0,
                "path_encoding": "UTF-8; U+FFFD paths are flagged as possibly lossy"
            });
        };
        let scan_active = picker.is_scan_active();
        let post_scan_active = picker.is_post_scan_active();
        let watcher_ready = picker.is_watcher_ready();
        let watch_state = self
            .watch_startup
            .state
            .lock()
            .unwrap_or_else(|error| error.into_inner());
        json!({
            "scan_active": scan_active,
            "scan_complete": !scan_active && !post_scan_active,
            "post_scan_active": post_scan_active,
            "watcher_ready": watcher_ready,
            "watch_subscribed": watch_state.watch_id.is_some(),
            "watch_error": watch_state.error,
            "indexed_files": picker.live_file_count(),
            "path_encoding": "UTF-8; U+FFFD paths are flagged as possibly lossy"
        })
    }

    pub(super) fn inventory(&self, offset: usize, limit: usize) -> Result<Value, String> {
        let guard = self
            .picker
            .read()
            .map_err(|error| format!("cannot read FFF picker: {error}"))?;
        let picker = guard
            .as_ref()
            .ok_or_else(|| "FFF picker is not initialized".to_owned())?;
        let total = picker.live_file_count();
        let files = picker.get_files();
        let start = offset.min(files.len());
        let next_offset = start.saturating_add(limit).min(files.len());
        let items: Vec<Value> = files[start..next_offset]
            .iter()
            .filter(|file| !file.is_deleted())
            .map(|file| {
                let relative_path = file.relative_path(picker);
                json!({
                    "path": relative_path,
                    "path_encoding_warning": relative_path.contains(PATH_REPLACEMENT),
                    "size": file.size,
                    "modified": file.modified
                })
            })
            .collect();
        let path_encoding_warning_count = items
            .iter()
            .filter(|item| item["path_encoding_warning"] == true)
            .count();
        let scan_active = picker.is_scan_active();
        let post_scan_active = picker.is_post_scan_active();
        let watch_state = self
            .watch_startup
            .state
            .lock()
            .unwrap_or_else(|error| error.into_inner());
        Ok(json!({
            "offset": offset,
            "limit": limit,
            "total": total,
            "next_offset": next_offset,
            "done": next_offset == files.len(),
            "items": items,
            "scan_complete": !scan_active && !post_scan_active,
            "watcher_ready": picker.is_watcher_ready(),
            "watch_subscribed": watch_state.watch_id.is_some(),
            "watch_error": watch_state.error,
            "path_encoding_warning_count": path_encoding_warning_count
        }))
    }

    pub(super) fn search_paths(
        &self,
        query_text: &str,
        offset: usize,
        limit: usize,
    ) -> Result<Value, String> {
        let guard = self
            .picker
            .read()
            .map_err(|error| format!("cannot read FFF picker: {error}"))?;
        let picker = guard
            .as_ref()
            .ok_or_else(|| "FFF picker is not initialized".to_owned())?;
        let parser = QueryParser::default();
        let query = parser.parse(query_text);
        let result = picker.fuzzy_search(
            &query,
            None,
            FuzzySearchOptions {
                pagination: PaginationArgs { offset, limit },
                ..FuzzySearchOptions::default()
            },
        );
        let matches: Vec<Value> = result
            .items
            .iter()
            .enumerate()
            .map(|(index, file)| {
                let path = file.relative_path(picker);
                json!({
                    "path": path,
                    "path_encoding_warning": path.contains(PATH_REPLACEMENT),
                    "size": file.size,
                    "modified": file.modified,
                    "rank": offset.saturating_add(index).saturating_add(1),
                    "score": result.scores.get(index).map(|score| score.total),
                    "match_byte_offsets": result.match_byte_offsets.get(index)
                        .map(|ranges| ranges.iter().map(|(start, end)| json!([start, end])).collect::<Vec<_>>())
                        .unwrap_or_default()
                })
            })
            .collect();
        let next_offset = offset.saturating_add(result.items.len());
        let next_offset = if next_offset < result.total_matched {
            next_offset
        } else {
            0
        };
        Ok(json!({
            "query": query_text,
            "offset": offset,
            "limit": limit,
            "next_offset": next_offset,
            "total_matched": result.total_matched,
            "total_files": result.total_files,
            "matches": matches
        }))
    }

    pub(super) fn live_grep(
        &self,
        query_text: &str,
        mode: GrepMode,
        file_offset: usize,
        limit: usize,
        context_before: usize,
        context_after: usize,
    ) -> Result<Value, String> {
        let guard = self
            .picker
            .read()
            .map_err(|error| format!("cannot read FFF picker: {error}"))?;
        let picker = guard
            .as_ref()
            .ok_or_else(|| "FFF picker is not initialized".to_owned())?;
        let parser = QueryParser::new(GrepConfig);
        let query = parser.parse(query_text);
        let match_target = capped_grep_match_target(limit);
        let after_context = if context_after == 0 {
            grep_newline_count(&query.grep_text()).min(MAX_GREP_CONTEXT_LINES)
        } else {
            context_after.min(MAX_GREP_CONTEXT_LINES)
        };
        let result = picker.grep(
            &query,
            &GrepSearchOptions {
                file_offset,
                max_file_size: MAX_GREP_FILE_BYTES,
                max_matches_per_file: MAX_GREP_MATCHES_PER_FILE,
                page_limit: match_target, // FFF's page limit counts matches, not scanned files.
                mode,
                time_budget_ms: MAX_GREP_TIME_BUDGET_MS,
                enforce_time_budget: true,
                before_context: context_before.min(MAX_GREP_CONTEXT_LINES),
                after_context,
                ..GrepSearchOptions::default()
            },
        );
        let matches: Vec<Value> = result
            .matches
            .iter()
            .map(|matched| {
                let path = result
                    .files
                    .get(matched.file_index)
                    .map(|file| file.relative_path(picker));
                let path_encoding_warning = path
                    .as_deref()
                    .is_some_and(|path| path.contains(PATH_REPLACEMENT));
                let (line_content, line_start, mut truncated) = bounded_grep_line(
                    &matched.line_content,
                    &matched.match_byte_offsets,
                    MAX_GREP_LINE_BYTES,
                );
                let match_byte_offsets = adjusted_grep_ranges(
                    &matched.match_byte_offsets,
                    line_start,
                    line_content.len(),
                );
                let (context_before, before_truncated) = bounded_context_lines(
                    &matched.context_before,
                    MAX_GREP_CONTEXT_LINES,
                    MAX_GREP_LINE_BYTES,
                    true,
                );
                let (context_after, after_truncated) = bounded_context_lines(
                    &matched.context_after,
                    MAX_GREP_CONTEXT_LINES,
                    MAX_GREP_LINE_BYTES,
                    false,
                );
                truncated |= before_truncated || after_truncated;
                json!({
                    "path": path,
                    "path_encoding_warning": path_encoding_warning,
                    "line_number": matched.line_number,
                    "column": matched.col.saturating_sub(line_start).min(line_content.len()),
                    "byte_offset": matched.byte_offset,
                    "line_content": line_content,
                    "match_byte_offsets": match_byte_offsets,
                    "context_before": context_before,
                    "context_after": context_after,
                    "truncated": truncated
                })
            })
            .collect();
        Ok(json!({
            "query": query_text,
            "file_offset": file_offset,
            "limit": match_target,
            "matches": matches,
            "total_files": result.total_files,
            "total_files_searched": result.total_files_searched,
            "filtered_file_count": result.filtered_file_count,
            "files_with_matches": result.files_with_matches,
            "next_file_offset": result.next_file_offset,
            "literal_fallback": if result.literal_fallback { query_text } else { "" },
            "regex_fallback_error": result.regex_fallback_error
        }))
    }

    pub(super) fn poll_events(&self, timeout: Duration, max_events: usize) -> Value {
        Value::Array(self.events.drain(timeout, max_events))
    }
}

fn validate_root_only_root(root: &Path) -> Result<(), String> {
    use std::path::Component;
    if !root.is_absolute()
        || root
            .components()
            .any(|c| !matches!(c, Component::RootDir | Component::Normal(_)))
    {
        return Err("RootOnly requires an absolute root without parent traversal".to_owned());
    }
    let mut current = std::path::PathBuf::new();
    for component in root.components() {
        current.push(component.as_os_str());
        let metadata = std::fs::symlink_metadata(&current)
            .map_err(|error| format!("cannot inspect RootOnly ancestry: {error}"))?;
        if metadata.file_type().is_symlink() || !metadata.is_dir() {
            return Err("RootOnly root and ancestry must be non-symlink directories".to_owned());
        }
    }
    Ok(())
}

impl Drop for FffbHandle {
    fn drop(&mut self) {
        self.watch_startup.cancelled.store(true, Ordering::Release);
        if let Some(worker) = self.watch_worker.take() {
            let _ = worker.join();
        }
        let watch_id = self
            .watch_startup
            .state
            .lock()
            .unwrap_or_else(|error| error.into_inner())
            .watch_id
            .take();
        if let Some(watch_id) = watch_id {
            self.picker.unwatch(watch_id);
            self.picker.shutdown_watches_and_wait();
        }
        stop_picker(&self.picker);
    }
}

fn stop_picker(picker: &SharedFilePicker) {
    picker.cancel();
    if let Ok(mut guard) = picker.write() {
        if let Some(picker) = guard.as_mut() {
            picker.stop_background_monitor();
        }
    }
}

fn set_watch_result(startup: &WatchStartup, result: Result<fff_search::watch::WatchId, String>) {
    let mut state = startup
        .state
        .lock()
        .unwrap_or_else(|error| error.into_inner());
    match result {
        Ok(watch_id) => state.watch_id = Some(watch_id),
        Err(error) => state.error = Some(error),
    }
    state.finished = true;
    drop(state);
    startup.ready.notify_all();
}

fn watch_subscription_ready(state: &WatchState) -> Result<bool, String> {
    if state.watch_id.is_some() {
        return Ok(true);
    }
    if let Some(error) = &state.error {
        return Err(error.clone());
    }
    if state.finished {
        return Err("watch subscription worker stopped before subscribing".to_owned());
    }
    Ok(false)
}

fn subscribe_events(
    picker: &SharedFilePicker,
    events: &Arc<EventQueue>,
    root: &Path,
) -> Result<fff_search::watch::WatchId, String> {
    let callback_events = Arc::clone(events);
    let callback_root = root.to_path_buf();
    let watch_id = picker
        .watch("", WatchOptions::default(), move |_, batch| {
            let callback_result = std::panic::catch_unwind(std::panic::AssertUnwindSafe(|| {
                for event in batch {
                    callback_events.push(watch_event_json(event, &callback_root));
                }
            }));
            if callback_result.is_err() {
                callback_events.push(json!({
                    "kind": "rescan",
                    "path": "",
                    "reason": "bridge_event_conversion_panic",
                    "rescan_required": true
                }));
            }
        })
        .map_err(|error| error.to_string())?;
    // FFF doesn't replay events from before this public subscription was installed.
    events.push(json!({
        "kind": "rescan",
        "path": "",
        "reason": "watch_subscription_gap",
        "rescan_required": true
    }));
    Ok(watch_id)
}

fn watch_event_json(event: &WatchEvent, root: &Path) -> Value {
    let (path, path_encoding, path_bytes_hex, path_warning) = encode_watch_path(&event.path, root);
    let (from, from_encoding, from_bytes_hex, from_warning) = event
        .from
        .as_deref()
        .map(|path| encode_watch_path(path, root))
        .unwrap_or((Value::Null, "utf8", None, false));
    let mut value = json!({
        "kind": event.kind.as_str(),
        "path": path,
        "path_encoding": path_encoding,
        "path_encoding_warning": path_warning,
        "from": from,
        "from_path_encoding": from_encoding,
        "from_path_encoding_warning": from_warning,
        "rescan_required": event.kind.as_str() == "rescan"
    });
    if let Some(bytes) = path_bytes_hex {
        value["path_bytes_hex"] = json!(bytes);
    }
    if let Some(bytes) = from_bytes_hex {
        value["from_path_bytes_hex"] = json!(bytes);
    }
    value
}

fn encode_watch_path(path: &Path, root: &Path) -> (Value, &'static str, Option<String>, bool) {
    let Ok(relative) = path.strip_prefix(root) else {
        return (
            Value::Null,
            "outside_root",
            Some(hex_bytes(path.as_os_str().as_bytes())),
            false,
        );
    };
    match relative.to_str() {
        Some(path) => (json!(path), "utf8", None, path.contains(PATH_REPLACEMENT)),
        None => (
            Value::Null,
            "unsupported",
            Some(hex_bytes(relative.as_os_str().as_bytes())),
            false,
        ),
    }
}

fn hex_bytes(bytes: &[u8]) -> String {
    const HEX: &[u8; 16] = b"0123456789abcdef";
    let mut output = String::with_capacity(bytes.len() * 2);
    for byte in bytes {
        output.push(HEX[(byte >> 4) as usize] as char);
        output.push(HEX[(byte & 0x0f) as usize] as char);
    }
    output
}

fn bounded_grep_line(line: &str, ranges: &[(u32, u32)], max_bytes: usize) -> (String, usize, bool) {
    if line.len() <= max_bytes {
        return (line.to_owned(), 0, false);
    }
    let max_start = line.len().saturating_sub(max_bytes);
    let first_match = ranges
        .iter()
        .map(|(start, _)| *start as usize)
        .min()
        .unwrap_or(0);
    let mut start = first_match.saturating_sub(max_bytes / 4).min(max_start);
    while start > 0 && !line.is_char_boundary(start) {
        start -= 1;
    }
    let mut end = (start + max_bytes).min(line.len());
    while end > start && !line.is_char_boundary(end) {
        end -= 1;
    }
    (line[start..end].to_owned(), start, true)
}

fn bounded_context_lines(
    lines: &[String],
    max_lines: usize,
    max_bytes: usize,
    keep_nearest: bool,
) -> (Vec<String>, bool) {
    let omitted = lines.len() > max_lines;
    let selected = if keep_nearest {
        &lines[lines.len().saturating_sub(max_lines)..]
    } else {
        &lines[..lines.len().min(max_lines)]
    };
    let mut truncated = omitted;
    let output = selected
        .iter()
        .map(|line| {
            if line.len() <= max_bytes {
                return line.clone();
            }
            truncated = true;
            let mut end = max_bytes;
            while end > 0 && !line.is_char_boundary(end) {
                end -= 1;
            }
            line[..end].to_owned()
        })
        .collect();
    (output, truncated)
}

fn adjusted_grep_ranges(
    ranges: &[(u32, u32)],
    start: usize,
    visible_bytes: usize,
) -> Vec<[usize; 2]> {
    let end = start + visible_bytes;
    ranges
        .iter()
        .filter_map(|(range_start, range_end)| {
            let range_start = (*range_start as usize).max(start);
            let range_end = (*range_end as usize).min(end);
            (range_start < range_end).then_some([range_start - start, range_end - start])
        })
        .collect()
}

fn capped_grep_match_target(limit: usize) -> usize {
    limit.min(MAX_GREP_MATCH_TARGET)
}

fn grep_newline_count(pattern: &str) -> usize {
    let bytes = pattern.as_bytes();
    let mut count = 0;
    let mut i = 0;
    while i < bytes.len() {
        if bytes[i] == b'\n' {
            count += 1;
            i += 1;
            continue;
        }
        if bytes[i] == b'\\' && i + 1 < bytes.len() && bytes[i + 1] == b'n' {
            let mut slashes = 1;
            let mut previous = i;
            while previous > 0 && bytes[previous - 1] == b'\\' {
                slashes += 1;
                previous -= 1;
            }
            if slashes % 2 == 1 {
                count += 1;
                i += 2;
                continue;
            }
        }
        i += 1;
    }
    count
}

pub(super) fn capped_limit(limit: u32) -> usize {
    (limit as usize).min(MAX_PAGE_SIZE)
}

pub(super) fn parse_grep_mode(mode: &str) -> Result<GrepMode, String> {
    match mode {
        "plain" => Ok(GrepMode::PlainText),
        "regex" => Ok(GrepMode::Regex),
        "fuzzy" => Ok(GrepMode::Fuzzy),
        _ => Err(format!("invalid grep mode: {mode}")),
    }
}

pub(super) fn offset_to_usize(offset: u64) -> Result<usize, String> {
    usize::try_from(offset).map_err(|_| "offset is too large".to_owned())
}

#[cfg(test)]
#[path = "bridge_tests.rs"]
mod tests;
