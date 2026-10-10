use super::*;
use std::fs;

fn open_ready(root: &Path) -> FffbHandle {
    let bridge = FffbHandle::open(root.to_str().unwrap(), BridgeOptions::default()).unwrap();
    assert!(bridge.wait_ready(Duration::from_secs(10)).unwrap());
    bridge
}

#[test]
fn failed_watch_subscription_is_terminal() {
    let startup = WatchStartup::default();
    set_watch_result(&startup, Err("synthetic watch failure".to_owned()));
    let state = startup.state.lock().unwrap();
    assert_eq!(
        watch_subscription_ready(&state),
        Err("synthetic watch failure".to_owned())
    );
}

#[test]
fn inventory_and_path_search_use_fff_index() {
    let temp = tempfile::tempdir().unwrap();
    fs::create_dir_all(temp.path().join("src")).unwrap();
    fs::write(temp.path().join("src/main.rs"), "fn main() {}\n").unwrap();
    fs::write(temp.path().join("README.md"), "bridge\n").unwrap();

    let bridge = open_ready(temp.path());
    let inventory = bridge.inventory(0, 10).unwrap();
    assert_eq!(inventory["total"], 2);
    assert!(inventory["items"]
        .as_array()
        .unwrap()
        .iter()
        .any(|item| item["path"] == "src/main.rs"));

    let search = bridge.search_paths("main.rs", 0, 10).unwrap();
    assert!(search["matches"]
        .as_array()
        .unwrap()
        .iter()
        .any(|item| item["path"] == "src/main.rs"));

    let grep = bridge
        .live_grep("main", GrepMode::PlainText, 0, 10, 0, 0)
        .unwrap();
    assert!(
        grep["matches"]
            .as_array()
            .unwrap()
            .iter()
            .any(|matched| matched["path"] == "src/main.rs"),
        "{grep}"
    );
}

#[test]
fn inventory_offsets_include_tombstones_and_empty_pages_advance() {
    let temp = tempfile::tempdir().unwrap();
    for name in ["a.txt", "b.txt", "c.txt", "d.txt"] {
        fs::write(temp.path().join(name), "content\n").unwrap();
    }

    let bridge = open_ready(temp.path());
    {
        let mut guard = bridge.picker.write().unwrap();
        let picker = guard.as_mut().unwrap();
        assert!(picker.remove_file_by_path(temp.path().join("a.txt")));
        assert!(picker.remove_file_by_path(temp.path().join("b.txt")));
    }

    let first = bridge.inventory(0, 2).unwrap();
    assert_eq!(first["items"].as_array().unwrap().len(), 0);
    assert_eq!(first["next_offset"], 2);
    assert!(!first["done"].as_bool().unwrap());

    let second = bridge.inventory(2, 2).unwrap();
    let paths: Vec<_> = second["items"]
        .as_array()
        .unwrap()
        .iter()
        .map(|item| item["path"].as_str().unwrap())
        .collect();
    assert_eq!(paths, ["c.txt", "d.txt"]);
    assert_eq!(second["total"], 2);
    assert_eq!(second["next_offset"], 4);
    assert!(second["done"].as_bool().unwrap());
}

#[test]
fn inventory_paginates_large_raw_slice_linearly() {
    const FILE_COUNT: usize = 513;
    const PAGE_SIZE: usize = 64;

    let temp = tempfile::tempdir().unwrap();
    for index in 0..FILE_COUNT {
        fs::write(
            temp.path().join(format!("file-{index:04}.txt")),
            "content\n",
        )
        .unwrap();
    }

    let bridge = open_ready(temp.path());
    let mut offset = 0;
    let mut visited = 0;
    let mut pages = 0;
    let mut paths = std::collections::HashSet::new();
    loop {
        let page = bridge.inventory(offset, PAGE_SIZE).unwrap();
        let next_offset = usize::try_from(page["next_offset"].as_u64().unwrap()).unwrap();
        let done = page["done"].as_bool().unwrap();
        assert!(next_offset > offset, "inventory cursor stalled at {offset}");
        assert!(next_offset - offset <= PAGE_SIZE);
        visited += next_offset - offset;
        pages += 1;
        for item in page["items"].as_array().unwrap() {
            let path = item["path"].as_str().unwrap().to_owned();
            assert!(
                paths.insert(path.clone()),
                "duplicate inventory path: {path}"
            );
        }
        if done {
            break;
        }
        offset = next_offset;
    }

    assert_eq!(visited, FILE_COUNT);
    assert_eq!(paths.len(), FILE_COUNT);
    assert_eq!(pages, FILE_COUNT.div_ceil(PAGE_SIZE));
}

#[test]
fn fff_watch_events_reach_the_bounded_bridge_queue() {
    let temp = tempfile::tempdir().unwrap();
    let bridge = open_ready(temp.path());
    fs::write(temp.path().join("created.txt"), "hello\n").unwrap();

    let deadline = Instant::now() + Duration::from_secs(5);
    let mut observed = false;
    while Instant::now() < deadline {
        let events = bridge.poll_events(Duration::from_millis(200), 32);
        observed |= events.as_array().unwrap().iter().any(|event| {
            event["path"] == "created.txt"
                && matches!(event["kind"].as_str(), Some("created" | "modified"))
        });
        if observed {
            break;
        }
    }
    assert!(observed, "watch event for created.txt was not delivered");
}

#[test]
fn open_returns_while_watcher_warms_and_subscribes_before_ready() {
    let temp = tempfile::tempdir().unwrap();
    for index in 0..30_000 {
        fs::write(temp.path().join(format!("{index:05}.txt")), []).unwrap();
    }

    let bridge = FffbHandle::open(temp.path().to_str().unwrap(), BridgeOptions::default()).unwrap();
    let status = bridge.status();
    assert_eq!(
        status["watcher_ready"], false,
        "open waited for watcher: {status}"
    );
    assert_eq!(status["watch_subscribed"], false);
    assert!(bridge.wait_ready(Duration::from_secs(30)).unwrap());
    assert_eq!(bridge.status()["watch_subscribed"], true);
    let events = bridge.poll_events(Duration::ZERO, 32);
    assert!(events
        .as_array()
        .unwrap()
        .iter()
        .any(|event| event["reason"] == "watch_subscription_gap"));
}

#[test]
fn event_queue_ignores_spurious_condvar_wakeups_until_deadline() {
    let queue = Arc::new(EventQueue::default());
    let notify_queue = Arc::clone(&queue);
    let notifier = std::thread::spawn(move || {
        std::thread::sleep(Duration::from_millis(5));
        notify_queue.ready.notify_all();
    });

    let start = Instant::now();
    assert!(queue.drain(Duration::from_millis(80), 1).is_empty());
    assert!(start.elapsed() >= Duration::from_millis(60));
    notifier.join().unwrap();
}

#[test]
fn event_queue_overflow_replaces_lost_events_with_rescan() {
    let queue = EventQueue::default();
    for _ in 0..=EVENT_QUEUE_CAPACITY {
        queue.push(json!({"kind": "modified"}));
    }

    let events = queue.drain(Duration::ZERO, EVENT_QUEUE_CAPACITY);
    assert_eq!(events.len(), 1);
    assert_eq!(events[0]["kind"], "rescan");
    assert_eq!(events[0]["reason"], "bridge_queue_overflow");
}

#[test]
fn grep_output_limits_preserve_utf8_and_adjust_match_ranges() {
    let line = format!("{}needle{}", "é".repeat(220), "x".repeat(300));
    let ranges = [(440, 446)];
    let (display, window_start, truncated) = bounded_grep_line(&line, &ranges, MAX_GREP_LINE_BYTES);
    let adjusted = adjusted_grep_ranges(&ranges, window_start, display.len());
    assert!(truncated);
    assert!(display.len() <= MAX_GREP_LINE_BYTES);
    assert!(std::str::from_utf8(display.as_bytes()).is_ok());
    assert_eq!(adjusted, vec![[96, 102]]);
    assert_eq!(&display[adjusted[0][0]..adjusted[0][1]], "needle");

    let contexts = vec!["é".repeat(300); MAX_GREP_CONTEXT_LINES + 2];
    let (contexts, context_truncated) =
        bounded_context_lines(&contexts, MAX_GREP_CONTEXT_LINES, MAX_GREP_LINE_BYTES, true);
    assert!(context_truncated);
    assert_eq!(contexts.len(), MAX_GREP_CONTEXT_LINES);
    assert!(contexts
        .iter()
        .all(|line| line.len() <= MAX_GREP_LINE_BYTES));
    assert!(contexts
        .iter()
        .all(|line| std::str::from_utf8(line.as_bytes()).is_ok()));
}

#[test]
fn grep_multiline_context_count_respects_escaped_newlines() {
    assert_eq!(grep_newline_count("first\nsecond"), 1);
    assert_eq!(grep_newline_count(r"first\nsecond"), 1);
    assert_eq!(grep_newline_count(r"first\\nsecond"), 0);
}
