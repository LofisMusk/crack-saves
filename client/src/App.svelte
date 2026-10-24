<script lang="ts">
  import { onMount } from "svelte";
  import { listen } from "@tauri-apps/api/event";
  import {
    getSettings,
    fetchGames,
    checkConnection,
    uploadSaves,
    getRunningMatches,
  } from "./lib/api";
  import type { Game, Settings } from "./lib/types";
  import Dashboard from "./lib/components/Dashboard.svelte";
  import GameDetail from "./lib/components/GameDetail.svelte";
  import AddGameWizard from "./lib/components/AddGameWizard.svelte";
  import SettingsView from "./lib/components/SettingsView.svelte";

  type View =
    | { kind: "dashboard" }
    | { kind: "game"; gameId: string }
    | { kind: "wizard"; prefillExe?: string }
    | { kind: "settings" };

  let settings: Settings | null = null;
  let games: Game[] = [];
  let connected = false;
  let loading = true;
  let loadError: string | null = null;
  let view: View = { kind: "dashboard" };
  let toasts: { id: number; message: string; tone: "info" | "ok" | "error" }[] = [];
  let toastId = 0;

  let runningMatches: Map<string, boolean> = new Map(); // gameId -> running
  let knownRunningGames = new Set<string>(); // for "game detected" toasts

  const cleanup: (() => void)[] = [];

  function toast(message: string, tone: "info" | "ok" | "error" = "info") {
    const id = ++toastId;
    toasts = [...toasts, { id, message, tone }];
    setTimeout(() => {
      toasts = toasts.filter((t) => t.id !== id);
    }, 6000);
  }

  async function reloadGames() {
    loadError = null;
    try {
      games = await fetchGames();
    } catch (err) {
      loadError = String(err);
    }
  }

  onMount(async () => {
    settings = await getSettings();
    if (settings.server_url) {
      connected = await checkConnection().catch(() => false);
    }
    await reloadGames();
    loading = false;

    // Watcher events from Rust: save files changed on disk.
    const unlisten = await listen<{ gameId: string }>("cs://save-changed", async (event) => {
      const gameId = event.payload.gameId;
      if (settings?.auto_sync) {
        try {
          const n = await uploadSaves(gameId, null);
          if (n > 0) toast(`Synced ${n} save${n === 1 ? "" : "s"}`, "ok");
        } catch {
          /* offline — will sync next time */
        }
      }
    });

    // Poll running processes every 5s for the "running" badge + detection toasts.
    const interval = setInterval(async () => {
      try {
        const matches = await getRunningMatches();
        const nowRunning = new Set<string>();
        for (const m of matches) {
          if (!m.game_id) continue;
          nowRunning.add(m.game_id);
          if (!knownRunningGames.has(m.game_id)) {
            knownRunningGames.add(m.game_id);
            toast(`Game detected: ${m.game_name}`, "info");
          }
        }
        runningMatches = new Map(matches.filter((m) => m.game_id).map((m) => [m.game_id!, true]));
      } catch {
        /* ignore transient errors */
      }
    }, 5000);

    // Connection status poll.
    const connInterval = setInterval(async () => {
      if (!settings?.server_url) return;
      connected = await checkConnection().catch(() => false);
    }, 30000);

    cleanup.push(() => unlisten(), () => clearInterval(interval), () => clearInterval(connInterval));
  });
  $: if (!loading && settings && !connected) loadError = loadError ?? "Cannot reach server";

  async function onSettingsSaved(next: Settings) {
    settings = next;
    connected = await checkConnection().catch(() => false);
    await reloadGames();
  }

  async function afterGameCreated() {
    view = { kind: "dashboard" };
    await reloadGames();
  }

  async function afterGameDeleted(gameId: string) {
    if (view.kind === "game" && view.gameId === gameId) view = { kind: "dashboard" };
    await reloadGames();
  }
</script>

<div class="shell">
  <aside class="sidebar">
    <div class="brand">
      <span class="logo">CS</span>
      <span>Cracked Saves</span>
    </div>
    <nav>
      <button class="navlink" class:active={view.kind === "dashboard"} onclick={() => (view = { kind: "dashboard" })}>
        Games
      </button>
      <button class="navlink" class:active={view.kind === "settings"} onclick={() => (view = { kind: "settings" })}>
        Settings
      </button>
    </nav>
    <div class="conn" class:online={connected} class:offline={!connected && !!settings?.server_url}>
      <span class="dot"></span>
      {connected ? "Connected" : settings?.server_url ? "Offline" : "No server set"}
    </div>
  </aside>

  <main class="content">
    {#if loading}
      <p class="muted">Loading…</p>
    {:else if view.kind === "dashboard"}
      <Dashboard
        games={games}
        loadError={loadError}
        running={runningMatches}
        onRefresh={reloadGames}
        onOpenGame={(id) => (view = { kind: "game", gameId: id })}
        onAddGame={() => (view = { kind: "wizard" })}
      />
    {:else if view.kind === "game"}
      <GameDetail
        gameId={view.gameId}
        games={games}
        running={runningMatches.has(view.gameId)}
        onBack={() => (view = { kind: "dashboard" })}
        onDeleted={(id) => afterGameDeleted(id)}
      />
    {:else if view.kind === "wizard"}
      <AddGameWizard
        prefillExe={view.prefillExe ?? ""}
        onCancel={() => (view = { kind: "dashboard" })}
        onDone={afterGameCreated}
        {toast}
      />
    {:else if view.kind === "settings"}
      <SettingsView settings={settings} onSave={onSettingsSaved} {toast} />
    {/if}
  </main>
</div>

{#each toasts as t (t.id)}
  <div class="toast" class:ok={t.tone === "ok"} class:error={t.tone === "error"}>{t.message}</div>
{/each}

<style>
  .shell {
    display: flex;
    height: 100vh;
  }
  .sidebar {
    width: 200px;
    min-width: 200px;
    background: var(--bg-raised);
    border-right: 1px solid var(--border);
    display: flex;
    flex-direction: column;
    padding: 16px 12px;
    gap: 18px;
  }
  .brand {
    font-weight: 700;
    font-size: 15px;
    display: flex;
    align-items: center;
    gap: 10px;
  }
  .logo {
    background: var(--accent);
    color: #fff;
    border-radius: 8px;
    width: 30px;
    height: 30px;
    display: inline-flex;
    align-items: center;
    justify-content: center;
    font-size: 12px;
  }
  nav {
    display: flex;
    flex-direction: column;
    gap: 4px;
  }
  .navlink {
    text-align: left;
    border: none;
    background: transparent;
    color: var(--text-dim);
    padding: 8px 10px;
    border-radius: 8px;
  }
  .navlink:hover {
    background: var(--bg-hover);
    color: var(--text);
  }
  .navlink.active {
    background: var(--bg-hover);
    color: var(--text);
    font-weight: 600;
  }
  .conn {
    margin-top: auto;
    display: flex;
    align-items: center;
    gap: 8px;
    color: var(--text-dim);
    font-size: 12.5px;
  }
  .dot {
    width: 9px;
    height: 9px;
    border-radius: 50%;
    background: var(--warn);
  }
  .conn.online .dot {
    background: var(--ok);
  }
  .conn.offline .dot {
    background: var(--error);
  }
  .content {
    flex: 1;
    overflow-y: auto;
    padding: 24px 32px;
  }
  .muted {
    color: var(--text-dim);
  }
  .toast {
    position: fixed;
    right: 20px;
    bottom: 20px;
    background: var(--bg-raised);
    border: 1px solid var(--border);
    border-radius: var(--radius);
    padding: 10px 16px;
    margin-top: 8px;
    box-shadow: 0 6px 24px rgba(0, 0, 0, 0.4);
    animation: slide-in 0.15s ease;
  }
  .toast.ok {
    border-color: rgba(61, 220, 132, 0.5);
  }
  .toast.error {
    border-color: rgba(255, 107, 107, 0.5);
  }
  @keyframes slide-in {
    from {
      transform: translateY(8px);
      opacity: 0;
    }
    to {
      transform: none;
      opacity: 1;
    }
  }
</style>
