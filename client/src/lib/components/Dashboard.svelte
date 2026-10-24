<script lang="ts">
  import type { Game } from "../types";

  interface Props {
    games: Game[];
    loadError: string | null;
    running: Map<string, unknown>;
    onRefresh: () => Promise<void>;
    onOpenGame: (id: string) => void;
    onAddGame: () => void;
  }
  let { games, loadError, running, onRefresh, onOpenGame, onAddGame }: Props = $props();

  let busy = $state(false);

  async function refresh() {
    busy = true;
    await onRefresh();
    busy = false;
  }

  function fmtPath(p: string) {
    if (!p) return "—";
    return p.length > 60 ? "…" + p.slice(-58) : p;
  }
</script>

<div class="head">
  <div>
    <h1>Games</h1>
    <p class="sub">Every game you track and the state of its saves.</p>
  </div>
  <div class="actions">
    <button onclick={refresh} disabled={busy}>{busy ? "Refreshing…" : "Refresh"}</button>
    <button class="primary" onclick={onAddGame}>+ Add game</button>
  </div>
</div>

{#if loadError && games.length === 0}
  <div class="banner error">
    Could not reach the server: <code>{loadError}</code>. Check your settings, then refresh.
  </div>
{:else if loadError}
  <div class="banner warn">Offline — showing last known games.</div>
{/if}

{#if games.length === 0 && !loadError}
  <div class="empty">
    <p>No games yet.</p>
    <p class="sub">Add a game to start tracking its saves, or launch a cracked build and let detection find it.</p>
  </div>
{:else}
  <table>
    <thead>
      <tr>
        <th>Name</th>
        <th>Executable</th>
        <th>Saves directory</th>
        <th>Status</th>
      </tr>
    </thead>
    <tbody>
      {#each games as game (game.id)}
        <tr class="row" onclick={() => onOpenGame(game.id)}>
          <td class="name">{game.name}</td>
          <td class="muted mono">{game.exe_pattern || "—"}</td>
          <td class="muted mono" title={game.save_dir_hint}>{fmtPath(game.save_dir_hint)}</td>
          <td>
            {#if running.has(game.id)}
              <span class="badge run"><span class="dot"></span>running</span>
            {:else}
              <span class="badge idle">idle</span>
            {/if}
          </td>
        </tr>
      {/each}
    </tbody>
  </table>
{/if}

<style>
  .head {
    display: flex;
    align-items: flex-start;
    justify-content: space-between;
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
  .actions {
    display: flex;
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
    padding: 12px 16px;
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
  .row {
    cursor: pointer;
    transition: background 0.1s ease;
  }
  .row:hover {
    background: var(--bg-hover);
  }
  .name {
    font-weight: 600;
  }
  .muted {
    color: var(--text-dim);
  }
  .mono {
    font-family: ui-monospace, SFMono-Regular, Menlo, monospace;
    font-size: 12.5px;
  }
  .banner {
    padding: 10px 14px;
    border-radius: var(--radius);
    margin-bottom: 16px;
    border: 1px solid var(--border);
  }
  .banner.error {
    border-color: rgba(255, 107, 107, 0.5);
    background: rgba(255, 107, 107, 0.08);
  }
  .banner.warn {
    border-color: rgba(255, 180, 84, 0.5);
    background: rgba(255, 180, 84, 0.08);
  }
  code {
    font-family: ui-monospace, monospace;
    font-size: 12px;
  }
  .empty {
    text-align: center;
    padding: 60px 0;
    color: var(--text);
  }
  .badge {
    display: inline-flex;
    align-items: center;
    gap: 7px;
    font-size: 12px;
    padding: 3px 10px;
    border-radius: 999px;
    border: 1px solid var(--border);
    color: var(--text-dim);
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
