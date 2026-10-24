use std::collections::HashMap;
use std::sync::Mutex;

use crate::api::{Game, KnownRemote, Settings};
use tauri_plugin_store::StoreExt;

/// Shared application state behind tauri::State.
pub struct AppState {
    /// Last known game list (server is source of truth; cached for offline + process matching).
    pub games: Mutex<Vec<Game>>,
    /// Per-game last-known remote save metadata (filename -> KnownRemote), refreshed on scan/upload.
    pub remote: Mutex<HashMap<String, HashMap<String, KnownRemote>>>,
}

impl AppState {
    pub fn new() -> Self {
        Self {
            games: Mutex::new(Vec::new()),
            remote: Mutex::new(HashMap::new()),
        }
    }
}

pub const STORE_FILE: &str = "cracked-saves.json";
pub const SETTINGS_KEY: &str = "settings";
pub const SAVE_CHANGED_EVENT: &str = "cs://save-changed";

/// Load settings from the tauri store, falling back to defaults.
pub fn load_settings(app: &tauri::AppHandle) -> Settings {
    let Ok(store) = app.store(STORE_FILE) else {
        return Settings::default();
    };
    if let Some(value) = store.get(SETTINGS_KEY) {
        if let Ok(settings) = serde_json::from_value::<Settings>(value) {
            return settings;
        }
    }
    Settings::default()
}

/// Persist settings to the tauri store.
pub fn save_settings(app: &tauri::AppHandle, settings: &Settings) -> Result<(), String> {
    let store = app.store(STORE_FILE).map_err(|e| e.to_string())?;
    let value = serde_json::to_value(settings).map_err(|e| e.to_string())?;
    store.set(SETTINGS_KEY, value);
    Ok(())
}

pub fn set_games(state: &AppState, games: Vec<Game>) {
    if let Ok(mut guard) = state.games.lock() {
        *guard = games;
    }
}

pub fn games_of(state: &AppState) -> Vec<Game> {
    state.games.lock().map(|g| g.clone()).unwrap_or_default()
}

pub fn set_remote(state: &AppState, game_id: &str, map: HashMap<String, KnownRemote>) {
    if let Ok(mut guard) = state.remote.lock() {
        guard.insert(game_id.to_string(), map);
    }
}

pub fn remote_of(state: &AppState, game_id: &str) -> HashMap<String, KnownRemote> {
    state
        .remote
        .lock()
        .ok()
        .and_then(|g| g.get(game_id).cloned())
        .unwrap_or_default()
}
