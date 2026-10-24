mod api;
mod processes;
mod state;
mod watcher;

use std::collections::{HashMap, HashSet};
use std::path::{Path, PathBuf};
use std::sync::Arc;


use sha2::{Digest, Sha256};
use tauri::{AppHandle, State};

use api::{Api, CreateGameInput, Game, GamePatch, SaveStatus, Settings};
use state::{AppState, games_of, remote_of, set_games, set_remote};
use watcher::Watcher;

pub fn run() {
    let watch: std::sync::Arc<std::sync::Mutex<Option<Watcher>>> =
        Arc::new(std::sync::Mutex::new(None));
    let dirs: Arc<watcher::DirMapInner> = Arc::new(std::sync::Mutex::new(HashMap::new()));

    let setup_watch = Arc::clone(&watch);
    let setup_dirs = Arc::clone(&dirs);

    tauri::Builder::default()
        .plugin(tauri_plugin_store::Builder::default().build())
        .plugin(tauri_plugin_dialog::init())
        .manage(AppState::new())
        .manage(Watch2 { poll: watch, dirs })
        .setup(move |app| {
            let handle = app.handle().clone();
            let watcher = Watcher::start(handle, setup_dirs);
            *setup_watch.lock().unwrap() = Some(watcher);
            Ok(())
        })
        .invoke_handler(tauri::generate_handler![
            get_settings,
            save_settings,
            check_connection,
            fetch_games,
            list_processes,
            get_running_matches,
            create_game,
            update_game,
            delete_game,
            scan_saves,
            upload_saves,
            restore_save
        ])
        .run(tauri::generate_context!())
        .expect("error while running Cracked Saves");
}

// --- settings ---

#[tauri::command]
fn get_settings(app: AppHandle) -> Result<Settings, String> {
    Ok(state::load_settings(&app))
}

#[tauri::command]
fn save_settings(app: AppHandle, patch: Settings) -> Result<Settings, String> {
    state::save_settings(&app, &patch)?;
    Ok(patch)
}

// --- connection / games ---

#[tauri::command]
async fn check_connection(app: AppHandle) -> Result<bool, String> {
    let settings = state::load_settings(&app);
    Api::from_settings(&settings)?.health().await
}

#[tauri::command]
async fn fetch_games(app: AppHandle, state: State<'_, AppState>, watch: State<'_, Watch2>) -> Result<Vec<Game>, String> {
    let settings = state::load_settings(&app);
    let api = Api::from_settings(&settings)?;
    let games = api.list_games().await?;
    set_games(&state, games.clone());
    refresh_watchers(&watch, &games);
    Ok(games)
}

#[tauri::command]
fn list_processes() -> Vec<api::ProcessEntry> {
    processes::list_processes()
}

#[tauri::command]
fn get_running_matches(state: State<'_, AppState>) -> Vec<api::RunningMatch> {
    processes::running_matches(&games_of(&state))
}

#[tauri::command]
async fn create_game(
    app: AppHandle,
    state: State<'_, AppState>,
    watch: State<'_, Watch2>,
    input: CreateGameInput,
) -> Result<Game, String> {
    let settings = state::load_settings(&app);
    let api = Api::from_settings(&settings)?;
    let game = api.create_game(&input).await?;
    let games = api.list_games().await.unwrap_or_else(|_| vec![game.clone()]);
    set_games(&state, games.clone());
    refresh_watchers(&watch, &games);
    Ok(game)
}

#[tauri::command]
async fn update_game(
    app: AppHandle,
    state: State<'_, AppState>,
    watch: State<'_, Watch2>,
    id: String,
    patch: GamePatch,
) -> Result<Game, String> {
    let settings = state::load_settings(&app);
    let api = Api::from_settings(&settings)?;
    let game = api.update_game(&id, &patch).await?;
    let games = api.list_games().await.unwrap_or_else(|_| vec![game.clone()]);
    set_games(&state, games.clone());
    refresh_watchers(&watch, &games);
    Ok(game)
}

#[tauri::command]
async fn delete_game(
    app: AppHandle,
    state: State<'_, AppState>,
    watch: State<'_, Watch2>,
    id: String,
) -> Result<(), String> {
    let settings = state::load_settings(&app);
    let api = Api::from_settings(&settings)?;
    api.delete_game(&id).await?;
    let games = api.list_games().await.unwrap_or_default();
    set_games(&state, games.clone());
    refresh_watchers(&watch, &games);
    Ok(())
}

// --- saves: scan / sync / restore ---

#[tauri::command]
async fn scan_saves(app: AppHandle, state: State<'_, AppState>, game_id: String) -> Result<Vec<SaveStatus>, String> {
    let games = games_of(&state);
    let game = games
        .iter()
        .find(|g| g.id == game_id)
        .ok_or_else(|| format!("unknown game {game_id}"))?;
    let Some(save_dir) = non_empty(&game.save_dir_hint) else {
        return Ok(Vec::new());
    };

    // Fresh remote state (fall back to cached when offline).
    let known: HashMap<String, api::KnownRemote> = match Api::from_settings(&state::load_settings(&app))?
        .remote_state(&game_id)
        .await
    {
        Ok(fresh) => {
            set_remote(&state, &game_id, fresh.clone());
            fresh
        }
        Err(_) => remote_of(&state, &game_id),
    };

    let local = walk_dir(Path::new(save_dir));
    let mut statuses: Vec<SaveStatus> = local
        .iter()
        .map(|(rel, abs, size, mtime)| {
            let hash = sha256_file(abs);
            match known.get(rel) {
                Some(remote) => SaveStatus {
                    filename: rel.clone(),
                    state: if remote.sha256 == hash { "synced".into() } else { "changed".into() },
                    local_size: Some(*size),
                    local_mtime: Some(*mtime),
                    local_sha256: Some(hash),
                    remote_size: Some(remote.size),
                    remote_sha256: Some(remote.sha256.clone()),
                    remote_updated_at: Some(remote.updated_at),
                },
                None => SaveStatus {
                    filename: rel.clone(),
                    state: "local_only".into(),
                    local_size: Some(*size),
                    local_mtime: Some(*mtime),
                    local_sha256: Some(hash),
                    remote_size: None,
                    remote_sha256: None,
                    remote_updated_at: None,
                },
            }
        })
        .collect();

    // Files present only on the server (restore candidates).
    let local_names: HashSet<String> = local.iter().map(|(rel, _, _, _)| rel.clone()).collect();
    for (filename, remote) in &known {
        if !local_names.contains(filename) {
            statuses.push(SaveStatus {
                filename: filename.clone(),
                state: "remote_only".into(),
                local_size: None,
                local_mtime: None,
                local_sha256: None,
                remote_size: Some(remote.size),
                remote_sha256: Some(remote.sha256.clone()),
                remote_updated_at: Some(remote.updated_at),
            });
        }
    }

    statuses.sort_by(|a, b| a.filename.cmp(&b.filename));
    Ok(statuses)
}

#[tauri::command]
async fn upload_saves(
    app: AppHandle,
    state: State<'_, AppState>,
    game_id: String,
    filenames: Option<Vec<String>>,
) -> Result<u32, String> {
    let games = games_of(&state);
    let game = games.iter().find(|g| g.id == game_id).ok_or_else(|| format!("unknown game {game_id}"))?;
    let save_dir = non_empty(&game.save_dir_hint).ok_or_else(|| "game has no saves directory".to_string())?;
    let settings = state::load_settings(&app);
    let api = Api::from_settings(&settings)?;

    // Which files to upload: explicit list, or every local file whose hash differs from the known remote.
    let local = walk_dir(Path::new(&save_dir));
    let known = remote_of(&state, &game_id);

    let targets: Vec<(String, PathBuf)> = match filenames {
        Some(list) => list
            .into_iter()
            .filter_map(|name| local.iter().find(|(rel, _, _, _)| rel == &name).map(|(_, abs, _, _)| (name, abs.clone())))
            .collect(),
        None => local
            .iter()
            .filter_map(|(rel, abs, _, _)| {
                let hash = sha256_file(abs);
                match known.get(rel) {
                    Some(remote) if remote.sha256 == hash => None,
                    _ => Some((rel.clone(), abs.clone())),
                }
            })
            .collect(),
    };

    let mut uploaded = 0u32;
    for (rel, abs) in targets {
        let bytes = std::fs::read(&abs).map_err(|e| format!("read {rel}: {e}"))?;
        let meta = api.upload_save(&game_id, &rel, bytes).await?;
        if let Ok(mut map) = state.remote.lock() {
            map.entry(game_id.clone())
                .or_default()
                .insert(meta.filename.clone(), api::KnownRemote {
                    sha256: meta.sha256.clone(),
                    size: meta.size,
                    updated_at: meta.updated_at,
                });
        }
        uploaded += 1;
    }
    Ok(uploaded)
}

#[tauri::command]
async fn restore_save(app: AppHandle, state: State<'_, AppState>, game_id: String, filename: String) -> Result<(), String> {
    let games = games_of(&state);
    let game = games.iter().find(|g| g.id == game_id).ok_or_else(|| format!("unknown game {game_id}"))?;
    let save_dir = non_empty(&game.save_dir_hint).ok_or_else(|| "game has no saves directory".to_string())?;
    let settings = state::load_settings(&app);
    let api = Api::from_settings(&settings)?;

    let bytes = api.download_save(&game_id, &filename).await?;
    let dest = Path::new(save_dir).join(filename.replace('/', std::path::MAIN_SEPARATOR_STR));
    if let Some(parent) = dest.parent() {
        std::fs::create_dir_all(parent).map_err(|e| e.to_string())?;
    }
    std::fs::write(&dest, &bytes).map_err(|e| format!("write {}: {e}", dest.display()))?;

    // Cache the now-known state so the next scan reports "synced".
    let mut hasher = Sha256::new();
    hasher.update(&bytes);
    let hash = hex::encode(hasher.finalize());
    if let Ok(mut map) = state.remote.lock() {
        map.entry(game_id.clone())
            .or_default()
            .insert(filename, api::KnownRemote {
                sha256: hash,
                size: bytes.len() as i64,
                updated_at: std::time::SystemTime::now()
                    .duration_since(std::time::UNIX_EPOCH)
                    .map(|d| d.as_secs() as i64)
                    .unwrap_or(0),
            });
    }
    Ok(())
}

// --- helpers ---

fn non_empty(s: &str) -> Option<&str> {
    let trimmed = s.trim();
    if trimmed.is_empty() { None } else { Some(trimmed) }
}

/// Walk a directory recursively, returning (relative slash-separated path, absolute path, size, mtime unix seconds).
fn walk_dir(root: &Path) -> Vec<(String, PathBuf, i64, i64)> {
    let mut out = Vec::new();
    walk_inner(root, root, &mut out);
    out.sort_by(|a, b| a.0.cmp(&b.0));
    out
}

fn walk_inner(root: &Path, dir: &Path, out: &mut Vec<(String, PathBuf, i64, i64)>) {
    let Ok(entries) = std::fs::read_dir(dir) else { return };
    for entry in entries.flatten() {
        let path = entry.path();
        let name = path.file_name().and_then(|n| n.to_str());
        if matches!(name, Some(n) if n.starts_with('.')) {
            continue; // skip hidden files/dirs
        }
        let Ok(metadata) = entry.metadata() else { continue };
        if metadata.is_dir() {
            walk_inner(root, &path, out);
        } else if metadata.is_file() {
            if path.extension().is_some_and(|e| e == "part") {
                continue; // in-flight upload temp files
            }
            let rel = path.strip_prefix(root).unwrap_or(&path).to_string_lossy().replace('\\', "/");
            let size = metadata.len() as i64;
            let mtime = metadata
                .modified()
                .ok()
                .and_then(|t| t.duration_since(std::time::UNIX_EPOCH).ok())
                .map(|d| d.as_secs() as i64)
                .unwrap_or(0);
            out.push((rel, path, size, mtime));
        }
    }
}

fn sha256_file(path: &Path) -> String {
    let Ok(bytes) = std::fs::read(path) else {
        return String::new();
    };
    let mut hasher = Sha256::new();
    hasher.update(&bytes);
    hex::encode(hasher.finalize())
}

/// Sync the poll watcher with the current set of games + their save directories.
fn refresh_watchers(watch: &State<'_, Watch2>, games: &[Game]) {
    let mut poll_guard = watch.poll.lock().unwrap();
    let Some(poll) = poll_guard.as_mut() else { return };
    let dirs = &watch.dirs;

    // Add (or keep) every game's saves directory.
    for game in games {
        let Some(dir) = non_empty(&game.save_dir_hint) else { continue };
        if let Ok(canonical) = std::fs::canonicalize(dir) {
            let already = dirs.lock().unwrap().contains_key(&canonical);
            if !already && poll.watch(&canonical).is_ok() {
                dirs.lock().unwrap().insert(canonical, game.id.clone());
            }
        }
    }

    // Remove entries whose game no longer exists.
    let live_ids: HashSet<&str> = games.iter().map(|g| g.id.as_str()).collect();
    let mut stale: Vec<(PathBuf, String)> = Vec::new();
    {
        let g = dirs.lock().unwrap();
        for (path, game_id) in g.iter() {
            if !live_ids.contains(game_id.as_str()) {
                stale.push((path.clone(), game_id.clone()));
            }
        }
    }
    {
        let mut g = dirs.lock().unwrap();
        for (path, _) in &stale {
            let _ = poll.unwatch(path);
            g.remove(path);
        }
    }
}

// Extra runtime state: file watcher for save directories.
pub struct Watch2 {
    pub poll: std::sync::Arc<std::sync::Mutex<Option<Watcher>>>,
    pub dirs: Arc<watcher::DirMapInner>,
}
