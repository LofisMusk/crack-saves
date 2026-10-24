import { invoke } from "@tauri-apps/api/core";
import type { Game, SaveStatus, Settings, RunningMatch } from "./types";

export async function getSettings(): Promise<Settings> {
  return invoke("get_settings");
}

export async function saveSettings(patch: Partial<Settings>): Promise<Settings> {
  return invoke("save_settings", { patch });
}

export async function checkConnection(): Promise<boolean> {
  return invoke("check_connection");
}

export async function fetchGames(): Promise<Game[]> {
  return invoke("fetch_games");
}

export async function getRunningMatches(): Promise<RunningMatch[]> {
  return invoke("get_running_matches");
}

export async function listProcesses(): Promise<{ name: string; count: number }[]> {
  return invoke("list_processes");
}

export async function createGame(input: {
  name: string;
  exe_pattern: string;
  save_dir_hint: string;
  notes: string;
}): Promise<Game> {
  return invoke("create_game", { input });
}

export async function updateGame(
  id: string,
  patch: Partial<Pick<Game, "name" | "exe_pattern" | "save_dir_hint" | "notes">>
): Promise<Game> {
  return invoke("update_game", { id, patch });
}

export async function deleteGame(id: string): Promise<void> {
  return invoke("delete_game", { id });
}

/** Scan the local save dir of a game and compare with server state. */
export async function scanSaves(gameId: string): Promise<SaveStatus[]> {
  return invoke("scan_saves", { gameId });
}

/** Upload one or (null = all dirty) saves of a game to the server. */
export async function uploadSaves(
  gameId: string,
  filenames: string[] | null
): Promise<number> {
  return invoke("upload_saves", { gameId, filenames });
}

/** Download a single save from the server back into the local dir. */
export async function restoreSave(gameId: string, filename: string): Promise<void> {
  return invoke("restore_save", { gameId, filename });
}
