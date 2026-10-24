<script lang="ts">
  import { onMount, createEventDispatcher } from "svelte";
  import { listen } from "@tauri-apps/api/event";
  import { scanSaves, uploadSaves, restoreSave, deleteGame } from "../api";
  import type { Game, SaveStatus } from "../types";

  interface Props {
    gameId: string;
    games: Game[];
    running: boolean;
    onBack: () => void;
    onDeleted: (id: string) => void;
  }
  let { gameId, games, running, onBack, onDeleted }: Props = $props();

  const game = $derived(games.find((g) => g.id === gameId));
  let statuses = $state<SaveStatus[]>([]);
  let loading = $state(true);
  let error: string | null = $state(null);
  let busyFile: string | null = $state(null);

  const dirtyCount = $derived(statuses.filter((s) => s.state === "changed" || s.state === "local_only").length);
  const remoteOnlyCount = $derived(statuses.filter((s) => s.state === "remote_only").length);

  async function rescan() {
    loading = true;
    error = null;
    try {
      statuses = await scanSaves(gameId);
    } catch (err) {
      error = String(err);
    } finally {
      loading = false;
    }
  }

  onMount(() => {
    let unlisten: (() => void) | undefined;
    (async () => {
      try {
        unlisten = await listen<{ gameId: string }>("cs://save-changed", (e) => {
          if (e.payload.gameId === gameId) rescan();
        });
      } catch {
        /* ignore */
      }
      rescan();
    })();
    return () => unlisten?.();
  });

  async function syncAll() {
    busyFile = "__all__";
    try {
      const n = await uploadSaves(gameId, null);
      await rescan();
      if (n > 0) alert(`Synced ${n} save file${n === 1 ? "" : "s"}.`);
    } catch (err) {
      error = String(err);
    } finally {
      busyFile = null;
    }
  }

  async function syncOne(file: SaveStatus) {
    busyFile = file.filename;
    try {
      await uploadSaves(gameId, [file.filename]);
      await rescan();
    } catch (err) {
      error = String(err);
    } finally {
      busyFile = null;
    }
  }

  async function restoreOne(file: SaveStatus) {
    if (!confirm(`Overwrite local ${file.filename} with the server copy?`)) return;
    busyFile = file.filename;
    try {
      await restoreSave(gameId, file.filename);
      await rescan();
    } catch (err) {
      error = String(err);
    } finally {
      busyFile = null;
    }
  }

  async function removeGame() {
    if (!game) return;
    if (!confirm(`Remove "${game.name}" from tracking? Server-side saves are kept.`)) return;
    await deleteGame(gameId);
    onDeleted(gameId);
  }

  function stateLabel(s: SaveStatus) {
    switch (s.state) {
      case "synced":
        return { text: "in sync", cls: "ok" };
      case "changed":
      case "local_only":
        return { text: s.state === "local_only" ? "new save" : "changed", cls: "warn" };
      case "remote_only":
        return { text: "on server only", cls: "dim" };
    }
  }

  function fmtSize(n?: number) {
    if (n === undefined) return "—";
    if (n < 1024) return `${n} B`;
    if (n < 1024 * 1024) return `${(n / 1024).toFixed(1)} KB`;
    return `${(n / (1024 * 1024)).toFixed(1)} MB`;
  }

  function fmtTime(sec?: number) {
    if (!sec) return "—";
    return new Date(sec * 1000).toLocaleString();
  }
</script>

<div class="head">
  <button class="back" onclick={onBack}>← Games</button>
</div>

{#if game}
  <div class="titlebar">
    <div>
      <h1>{game.name}</h1>
      <p class="sub mono">{game.exe_pattern || "no executable pattern"} · {game.save_dir_hint || "no saves dir set"}</p>
    </div>
    <div class="title-actions">
      {#if running}<span class="badge run"><span class="dot"></span>running</span>{/if}
      <button class="primary" disabled={busyFile === "__all__" || dirtyCount === 0 || loading} onclick={syncAll}>
        {busyFile === "__all__" ? "Syncing…" : `Sync all${dirtyCount ? ` (${dirtyCount})` : ""}`}
      </button>
      <button class="danger" onclick={removeGame}>Remove</button>
    </div>
  </div>

  {#if error}
    <div class="banner">⚠ {error}</div>
  {/if}

  {#if loading}
    <p class="muted">Scanning saves…</p>
  {:else if statuses.length === 0}
    <div class="empty">
      <p>No save files found in the saves directory.</p>
      {#if remoteOnlyCount > 0}<p>Saves exist on the server — restore them below.</p>{/if}
    </div>
  {:else}
    <table>
      <thead>
        <tr>
          <th>File</th>
          <th>Size</th>
          <th>Last modified</th>
          <th>Status</th>
          <th></th>
        </tr>
      </thead>
      <tbody>
        {#each statuses as s (s.filename)}
          {@const st = stateLabel(s)}
          <tr>
            <td class="mono file">{s.filename}</td>
            <td class="muted">{fmtSize(s.local_size ?? s.remote_size)}</td>
            <td class="muted">{fmtTime(s.local_mtime ?? s.remote_updated_at)}</td>
            <td><span class="state {st.cls}">{st.text}</span></td>
            <td class="ops">
              {#if s.state === "changed" || s.state === "local_only"}
                <button disabled={busyFile !== null} onclick={() => syncOne(s)}>
                  {busyFile === s.filename ? "…" : "Sync"}
                </button>
              {:else if s.state === "remote_only"}
                <button disabled={busyFile !== null} onclick={() => restoreOne(s)}>
                  {busyFile === s.filename ? "…" : "Restore"}
                </button>
              {/if}
            </td>
          </tr>
        {/each}
      </tbody>
    </table>
  {/if}
{:else}
  <p class="muted">Game not found.</p>
{/if}

<style>
  .back {
    border: none;
    background: transparent;
    color: var(--text-dim);
    padding: 0;
    margin-bottom: 12px;
  }
  .back:hover {
    color: var(--text);
    background: transparent;
  }
  .titlebar {
    display: flex;
    justify-content: space-between;
    align-items: flex-start;
    gap: 16px;
    margin-bottom: 20px;
  }
  h1 {
    margin: 0 0 4px;
    font-size: 22px;
  }
  .sub {
    margin: 0;
    color: var(--text-dim);
  }
  .mono {
    font-family: ui-monospace, SFMono-Regular, Menlo, monospace;
    font-size: 12.5px;
  }
  .title-actions {
    display: flex;
    align-items: center;
    gap: 8px;
  }
  table {
    width: 100%;
    border-collapse: collapse;
    background: var(--bg-raised);
    border: 1px solid var(--border);
    border-radius: var(--radius);
    overflow: hidden;
  }
  th,
  td {
    text-align: left;
    padding: 10px 14px;
    border-bottom: 1px solid var(--border);
  }
  th {
    font-size: 12px;
    text-transform: uppercase;
    letter-spacing: 0.05em;
    color: var(--text-dim);
    font-weight: 600;
  }
  tbody tr:last-child td {
    border-bottom: none;
  }
  .muted {
    color: var(--text-dim);
  }
  .file {
    font-family: ui-monospace, monospace;
    font-size: 12.5px;
    word-break: break-all;
  }
  .state {
    font-size: 12px;
    padding: 3px 10px;
    border-radius: 999px;
    border: 1px solid var(--border);
  }
  .state.ok {
    color: var(--ok);
    border-color: rgba(61, 220, 132, 0.4);
  }
  .state.warn {
    color: var(--warn);
    border-color: rgba(255, 180, 84, 0.4);
  }
  .state.dim {
    color: var(--text-dim);
  }
  .ops {
    text-align: right;
  }
  .banner {
    border: 1px solid rgba(255, 180, 84, 0.5);
    background: rgba(255, 180, 84, 0.08);
    border-radius: var(--radius);
    padding: 10px 14px;
    margin-bottom: 16px;
  }
  .empty {
    text-align: center;
    padding: 48px 0;
    color: var(--text-dim);
  }
  .badge {
    display: inline-flex;
    align-items: center;
    gap: 7px;
    font-size: 12px;
    padding: 3px 10px;
    border-radius: 999px;
    border: 1px solid var(--border);
  }
  .badge.run {
    color: var(--ok);
    border-color: rgba(61, 220, 132, 0.4);
  }
  .dot {
    width: 8px;
    height: 8px;
    border-radius: 50%;
    background: currentColor;
  }
</style>
