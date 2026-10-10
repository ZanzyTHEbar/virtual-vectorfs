use crate::bridge::{capped_limit, offset_to_usize, parse_grep_mode, BridgeOptions, FffbHandle};
use serde_json::{json, Value};
use std::ffi::{CStr, CString};
use std::os::raw::{c_char, c_int};
use std::ptr;
use std::time::Duration;

unsafe fn input_utf8<'a>(value: *const c_char, name: &str) -> Result<&'a str, String> {
    if value.is_null() {
        return Err(format!("{name} must not be NULL"));
    }
    // The C ABI contract requires a readable NUL-terminated buffer.
    unsafe { CStr::from_ptr(value) }
        .to_str()
        .map_err(|_| format!("{name} must be UTF-8"))
}

fn allocated_string(value: String) -> *mut c_char {
    CString::new(value.replace('\0', "�"))
        .map(CString::into_raw)
        .unwrap_or(ptr::null_mut())
}

fn json_string(value: Value) -> *mut c_char {
    match serde_json::to_string(&value) {
        Ok(value) => allocated_string(value),
        Err(error) => allocated_string(json!({"error": error.to_string()}).to_string()),
    }
}

fn json_error(error: impl Into<String>) -> *mut c_char {
    json_string(json!({"error": error.into()}))
}

fn ffi_json(operation: impl FnOnce() -> Result<Value, String>) -> *mut c_char {
    std::panic::catch_unwind(std::panic::AssertUnwindSafe(|| match operation() {
        Ok(value) => json_string(value),
        Err(error) => json_error(error),
    }))
    .unwrap_or(ptr::null_mut())
}

#[no_mangle]
/// Open an FFF-backed inventory and watcher for a UTF-8 directory path.
///
/// # Safety
/// `root_utf8` and non-NULL `opts_json` must be valid NUL-terminated UTF-8;
/// `err`, when non-NULL, must point to writable pointer storage.
pub unsafe extern "C" fn fffb_open(
    root_utf8: *const c_char,
    opts_json: *const c_char,
    err: *mut *mut c_char,
) -> *mut FffbHandle {
    match std::panic::catch_unwind(std::panic::AssertUnwindSafe(|| {
        if !err.is_null() {
            unsafe { *err = ptr::null_mut() };
        }
        match (|| -> Result<*mut FffbHandle, String> {
            let root = unsafe { input_utf8(root_utf8, "root_utf8")? };
            let options_text = if opts_json.is_null() {
                None
            } else {
                Some(unsafe { input_utf8(opts_json, "opts_json")? })
            };
            FffbHandle::open(root, BridgeOptions::parse(options_text)?)
                .map(|handle| Box::into_raw(Box::new(handle)))
        })() {
            Ok(handle) => handle,
            Err(error) => {
                if !err.is_null() {
                    unsafe { *err = allocated_string(error) };
                }
                ptr::null_mut()
            }
        }
    })) {
        Ok(handle) => handle,
        Err(_) => {
            if !err.is_null() {
                let _ = std::panic::catch_unwind(std::panic::AssertUnwindSafe(|| unsafe {
                    *err =
                        allocated_string("Rust panic caught while opening FFF bridge".to_owned());
                }));
            }
            ptr::null_mut()
        }
    }
}

#[no_mangle]
/// Close a bridge handle.
///
/// # Safety
/// `handle` must be NULL or a live handle returned by `fffb_open`, with no
/// concurrent calls in progress. A non-NULL handle must be closed once.
pub unsafe extern "C" fn fffb_close(handle: *mut FffbHandle) {
    let _ = std::panic::catch_unwind(std::panic::AssertUnwindSafe(|| {
        if !handle.is_null() {
            // Handle must be a live pointer returned by fffb_open and closed once.
            unsafe { drop(Box::from_raw(handle)) };
        }
    }));
}

#[no_mangle]
/// Wait for the scan, post-scan work, and FFF watcher to become ready.
///
/// # Safety
/// `handle` must be NULL or a live handle returned by `fffb_open`; it must not
/// be closed concurrently.
pub unsafe extern "C" fn fffb_wait_ready(handle: *mut FffbHandle, timeout_ms: u32) -> c_int {
    std::panic::catch_unwind(std::panic::AssertUnwindSafe(|| {
        let Some(handle) = (unsafe { handle.as_ref() }) else {
            return -1;
        };
        match handle.wait_ready(Duration::from_millis(timeout_ms.into())) {
            Ok(true) => 1,
            Ok(false) => 0,
            Err(_) => -2,
        }
    }))
    .unwrap_or(-1)
}

#[no_mangle]
/// Return the current FFF scan and watcher status as JSON.
///
/// # Safety
/// `handle` must be NULL or a live handle returned by `fffb_open`; it must not
/// be closed concurrently.
pub unsafe extern "C" fn fffb_status(handle: *mut FffbHandle) -> *mut c_char {
    ffi_json(|| {
        let handle =
            unsafe { handle.as_ref() }.ok_or_else(|| "handle must not be NULL".to_owned())?;
        Ok(handle.status())
    })
}

#[no_mangle]
/// Return one UTF-8 path inventory page as JSON.
///
/// # Safety
/// `handle` must be NULL or a live handle returned by `fffb_open`; it must not
/// be closed concurrently.
pub unsafe extern "C" fn fffb_inventory(
    handle: *mut FffbHandle,
    offset: u64,
    limit: u32,
) -> *mut c_char {
    ffi_json(|| {
        let handle =
            unsafe { handle.as_ref() }.ok_or_else(|| "handle must not be NULL".to_owned())?;
        handle.inventory(offset_to_usize(offset)?, capped_limit(limit))
    })
}

#[no_mangle]
/// Run FFF fuzzy file-path search and return one page as JSON.
///
/// # Safety
/// `handle` must be NULL or a live handle returned by `fffb_open` and not
/// closed concurrently. `query_utf8` and `mode_utf8` must be valid
/// NUL-terminated UTF-8 strings.
pub unsafe extern "C" fn fffb_search_paths(
    handle: *mut FffbHandle,
    query_utf8: *const c_char,
    offset: u64,
    limit: u32,
) -> *mut c_char {
    ffi_json(|| {
        let handle =
            unsafe { handle.as_ref() }.ok_or_else(|| "handle must not be NULL".to_owned())?;
        let query = unsafe { input_utf8(query_utf8, "query_utf8")? };
        handle.search_paths(query, offset_to_usize(offset)?, capped_limit(limit))
    })
}

#[no_mangle]
/// Run FFF live grep and return one page of content matches as JSON.
///
/// # Safety
/// `handle` must be NULL or a live handle returned by `fffb_open` and not
/// closed concurrently. `query_utf8` must be a valid NUL-terminated UTF-8
/// string.
pub unsafe extern "C" fn fffb_live_grep(
    handle: *mut FffbHandle,
    query_utf8: *const c_char,
    mode_utf8: *const c_char,
    file_offset: u64,
    limit: u32,
    context_before: u64,
    context_after: u64,
) -> *mut c_char {
    ffi_json(|| {
        let handle =
            unsafe { handle.as_ref() }.ok_or_else(|| "handle must not be NULL".to_owned())?;
        let query = unsafe { input_utf8(query_utf8, "query_utf8")? };
        let mode = parse_grep_mode(unsafe { input_utf8(mode_utf8, "mode_utf8")? })?;
        handle.live_grep(
            query,
            mode,
            offset_to_usize(file_offset)?,
            capped_limit(limit),
            offset_to_usize(context_before)?,
            offset_to_usize(context_after)?,
        )
    })
}

#[no_mangle]
/// Block for and drain up to `max_events` FFF watch events as JSON.
///
/// # Safety
/// `handle` must be NULL or a live handle returned by `fffb_open`; it must not
/// be closed concurrently.
pub unsafe extern "C" fn fffb_poll_events(
    handle: *mut FffbHandle,
    timeout_ms: u32,
    max_events: u32,
) -> *mut c_char {
    ffi_json(|| {
        let handle =
            unsafe { handle.as_ref() }.ok_or_else(|| "handle must not be NULL".to_owned())?;
        Ok(handle.poll_events(
            Duration::from_millis(timeout_ms.into()),
            capped_limit(max_events),
        ))
    })
}

#[no_mangle]
/// Free a string returned by this library.
///
/// # Safety
/// `value` must be NULL or an unfreed string returned by this library.
pub unsafe extern "C" fn fffb_free_string(value: *mut c_char) {
    let _ = std::panic::catch_unwind(std::panic::AssertUnwindSafe(|| {
        if !value.is_null() {
            // String must be a live pointer returned by this library and freed once.
            unsafe { drop(CString::from_raw(value)) };
        }
    }));
}
