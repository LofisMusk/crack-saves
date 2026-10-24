# Cracked Saves

An open-source, self-hostable platform for managing **cracked game saves**.

The desktop client watches the save folders of your locally installed games and keeps
them in sync with a small server you run yourself (Docker). No cloud, no account —
your saves live on your hardware.

## Features

- **Autodetection** — the client polls running processes every 5 seconds and flags any
  game whose executable matches one of your configured patterns ("game detected" toast,
  running/idle badge in the UI).
- **File watching** — changes inside a game's saves directory trigger an automatic
  re-upload when auto-sync is enabled.
- **Manual control** — per-file *Sync* (upload) and *Restore* (download + write locally),
  plus "Sync all" for everything that changed or only exists locally.
- **Latest-only overwrite** (v1): the newest save wins, no version history yet.
- **Self-hosted server** — a single static Go binary backed by SQLite. Optional
  static-token auth (leave it empty on a trusted LAN).

## Repository layout

| Path      | What                                                            |
| --------- | --------------------------------------------------------------- |
| `server/` | Go + chi + SQLite REST server (pure-Go driver, no CGO)          |
| `client/` | Tauri 2 desktop app: Svelte 5 frontend + Rust core              |
| `docker/` | Multi-stage Dockerfile and docker-compose for the server        |

## Running the server

### With Docker (recommended)

```sh
cd docker
# optional auth: export CS_API_TOKEN="$(openssl rand -hex 16)"
docker compose up -d
```

The API listens on port **8090**. Data (SQLite + saved files) lives in the
`cracked-saves-data` named volume.

Configuration via environment variables:

| Variable         | Default   | Meaning                                    |
| ---------------- | --------- | ------------------------------------------ |
| `CS_PORT`        | `8090`    | HTTP listen port                            |
| `CS_DATA_DIR`    | `./data`  | Directory for SQLite DB + save files       |
| `CS_API_TOKEN`   | *(empty)* | Bearer token; empty = auth disabled        |

### Without Docker

```sh
cd server
go run .            # uses ./data by default
```

## REST API overview

All endpoints are under `/api`. Send `Authorization: Bearer <token>` when auth is enabled.

| Method   | Path                            | Description                       |
| -------- | ------------------------------- | --------------------------------- |
| `GET`    | `/health`                       | Liveness check (no auth)          |
| `GET`    | `/games`                        | List games                        |
| `POST`   | `/games`                        | Create game                       |
| `PATCH`  | `/games/{id}`                   | Update a game                     |
| `DELETE` | `/games/{id}`                   | Delete game + its saves           |
| `GET`    | `/games/{id}/saves`             | List save metadata for a game     |
| `PUT`    | `/games/{id}/saves/{filename}`  | Upload (replace) a save file      |
| `GET`    | `/games/{id}/saves/{filename}`  | Download a save file              |

## Building the desktop client

Prerequisites: [Rust](https://rustup.rs), Node.js, and your platform's Tauri
prerequisites (WebKit2GTK + GTK on Linux, Xcode CLT on macOS).

```sh
cd client
npm install
npm run tauri dev      # development build with hot reload
npm run tauri build    # release bundle for your platform
```

### First-run flow

1. Open **Settings**, enter your server URL (e.g. `http://localhost:8090`), paste the
   API token if you enabled one, and click *Test connection*.
2. Use **Add game**: give it a name, an executable pattern (e.g. `eldenring.exe`) —
   or click a running process to fill it in — and pick the folder holding the game's
   saves.
3. Start the game: the client detects the process and watches its save folder.
   Changed files are uploaded automatically when *Auto-sync* is on; you can always
   upload/restore individual files from the game's detail view.

## Notes & limitations (v1)

- Whole-file uploads only (no chunking / delta sync).
- One server, one user — designed for a personal box or LAN.
- Autodetection matches process names/executables against your patterns; it does not
  inspect store launchers (cracked games usually run without them anyway).

## License

[MIT](LICENSE)
