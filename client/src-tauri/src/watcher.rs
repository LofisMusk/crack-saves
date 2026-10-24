use std::collections::HashMap;
use std::path::{Path, PathBuf};
use std::sync::mpsc;
use std::sync::{Arc, Mutex};
use std::time::{Duration, Instant};

use notify::{Config, PollWatcher, RecursiveMode, Watcher as _};
use tauri::{AppHandle, Emitter};

use crate::state::SAVE_CHANGED_EVENT;

const POLL_INTERVAL: Duration = Duration::from_secs(3);
const DEBOUNCE: Duration = Duration::from_secs(2);

/// dir (absolute canonical path) -> game_id, shared with the event thread.
pub type DirMapInner = Mutex<HashMap<PathBuf, String>>;

pub struct Watcher {
    poll: PollWatcher,
}

impl Watcher {
    /// Create the poll watcher and spawn the debouncing event thread.
    pub fn start(app: AppHandle, dirs: Arc<DirMapInner>) -> Self {
        let (tx, rx) = mpsc::channel::<notify::Result<notify::Event>>();
        let poll = PollWatcher::new(
            move |result| {
                let _ = tx.send(result);
            },
            Config::default().with_poll_interval(POLL_INTERVAL),
        )
        .expect("failed to create poll watcher");

        std::thread::Builder::new()
            .name("cs-save-watcher".into())
            .spawn(move || event_loop(rx, dirs, app))
            .expect("failed to spawn watcher thread");

        Self { poll }
    }

    pub fn watch(&mut self, path: &Path) -> notify::Result<()> {
        let canonical = std::fs::canonicalize(path).unwrap_or_else(|_| path.to_path_buf());
        self.poll.watch(canonical.as_path(), RecursiveMode::Recursive)
    }

    pub fn unwatch(&mut self, path: &Path) -> notify::Result<()> {
        let canonical = std::fs::canonicalize(path).unwrap_or_else(|_| path.to_path_buf());
        self.poll.unwatch(canonical.as_path())
    }
}

fn event_loop(
    rx: mpsc::Receiver<notify::Result<notify::Event>>,
    dirs: Arc<DirMapInner>,
    app: AppHandle,
) {
    let mut last_emit: HashMap<String, Instant> = HashMap::new();

    while let Ok(result) = rx.recv() {
        if let Ok(event) = result {
            for path in &event.paths {
                // Find which registered directory this change belongs to.
                let game_id = {
                    let map = dirs.lock().unwrap();
                    map.iter()
                        .find(|(dir, _)| path.starts_with(dir))
                        .map(|(_, gid)| gid.clone())
                };
                let Some(game_id) = game_id else { continue };

                let now = Instant::now();
                let emit = match last_emit.get(&game_id) {
                    Some(last) => now.duration_since(*last) > DEBOUNCE,
                    None => true,
                };
                if emit {
                    last_emit.insert(game_id.clone(), now);
                    let _ = app.emit(SAVE_CHANGED_EVENT, serde_json::json!({ "gameId": game_id }));
                }
            }
        }
    }
}
