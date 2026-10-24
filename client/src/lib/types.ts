// Shared types mirroring the server API + Rust command payloads.

export interface Game {
  id: string;
  name: string;
  exe_pattern: string;
  save_dir_hint: string;
  notes: string;
  created_at: number;
  updated_at: number;
}

export interface SaveMeta {
  game_id: string;
  filename: string;
  sha256: string;
  size: number;
  updated_at: number;
}

/** Local file state as reported by scan_saves. */
export type SaveState = "synced" | "changed" | "local_only" | "remote_only";

export interface SaveStatus {
  filename: string;
  state: SaveState;
  local_size?: number;
  local_mtime?: number; // unix seconds
  local_sha256?: string;
  remote_size?: number;
  remote_sha256?: string;
  remote_updated_at?: number;
}

export interface Settings {
  server_url: string;
  api_token: string;
  auto_sync: boolean;
}

export interface RunningMatch {
  pid: number;
  name: string;
  exe_stem: string;
  game_id: string | null;
  game_name: string | null;
}
