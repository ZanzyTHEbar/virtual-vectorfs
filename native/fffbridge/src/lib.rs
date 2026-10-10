#![cfg_attr(not(target_os = "linux"), allow(dead_code))]

#[cfg(not(target_os = "linux"))]
compile_error!("fffbridge currently supports Linux only");

mod bridge;
mod ffi;
