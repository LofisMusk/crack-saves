<script lang="ts">
  import { saveSettings } from "../api";
  import type { Settings } from "../types";

  interface Props {
    settings: Settings | null;
    onSave: (s: Settings) => Promise<void>;
    toast: (message: string, tone?: "info" | "ok" | "error") => void;
  }
  let { settings, onSave, toast }: Props = $props();

  let serverUrl = $state(settings?.server_url ?? "http://localhost:8090");
  let apiToken = $state(settings?.api_token ?? "");
  let autoSync = $state(settings?.auto_sync ?? true);
  let saving = $state(false);
  let testing = $state(false);

  async function test() {
    if (!serverUrl.trim()) return toast("Enter a server URL first", "error");
    testing = true;
    try {
      const res = await fetch(serverUrl.replace(/\/+$/, "") + "/api/health");
      if (res.ok) toast("Server reachable ✓", "ok");
      else toast(`Server responded with ${res.status}`, "error");
    } catch (err) {
      toast(`Could not reach server: ${String(err)}`, "error");
    } finally {
      testing = false;
    }
  }

  async function save() {
    saving = true;
    try {
      const next: Settings = {
        server_url: serverUrl.trim(),
        api_token: apiToken,
        auto_sync: autoSync,
      };
      await saveSettings(next);
      await onSave(next);
      toast("Settings saved", "ok");
    } catch (err) {
      toast(String(err), "error");
    } finally {
      saving = false;
    }
  }
</script>

<div class="head">
  <h1>Settings</h1>
  <p class="sub">Connection to your self-hosted server and sync behaviour.</p>
</div>

<form class="form" onsubmit={(e) => { e.preventDefault(); save(); }}>
  <label>
    Server URL
    <input bind:value={serverUrl} placeholder="http://localhost:8090" />
  </label>

  <label>
    API token
    <input bind:value={apiToken} type="password" placeholder="leave empty if auth is disabled on the server" autocomplete="off" />
    <span class="hint">Must match CS_API_TOKEN on the server. Empty = trusted LAN mode.</span>
  </label>

  <label class="row">
    <input type="checkbox" bind:checked={autoSync} />
    Auto-sync when saves change (watched folders)
  </label>

  {#if testing}
    <p class="muted">Testing connection…</p>
  {/if}

  <div class="actions">
    <button type="button" onclick={test} disabled={testing}>{testing ? "Testing…" : "Test connection"}</button>
    <button type="submit" class="primary" disabled={saving}>{saving ? "Saving…" : "Save settings"}</button>
  </div>
</form>

<div class="docs">
  <h2>Self-hosting quick start</h2>
  <pre>cd docker
docker compose up -d</pre>
  <p class="muted">
    The server listens on port 8090 by default and stores everything under a Docker volume
    (SQLite metadata + raw save files). Set <code>CS_API_TOKEN</code> in the environment to enable auth.
  </p>
</div>

<style>
  h1 {
    margin: 0 0 4px;
    font-size: 22px;
  }
  .sub {
    margin: 0 0 24px;
    color: var(--text-dim);
  }
  .form {
    display: flex;
    flex-direction: column;
    gap: 18px;
    max-width: 560px;
  }
  label {
    display: flex;
    flex-direction: column;
    gap: 7px;
    font-size: 13px;
    color: var(--text-dim);
  }
  .row {
    flex-direction: row;
    align-items: center;
    gap: 10px;
    color: var(--text);
    font-size: 14px;
  }
  .hint {
    font-size: 12px;
    color: var(--text-dim);
  }
  .muted {
    color: var(--text-dim);
  }
  .actions {
    display: flex;
    justify-content: flex-end;
    gap: 8px;
  }
  .docs {
    margin-top: 40px;
    padding-top: 24px;
    border-top: 1px solid var(--border);
    max-width: 560px;
  }
  .docs h2 {
    font-size: 16px;
    margin: 0 0 12px;
  }
  pre {
    background: var(--bg-raised);
    border: 1px solid var(--border);
    border-radius: 8px;
    padding: 10px 14px;
    overflow-x: auto;
  }
  code {
    font-family: ui-monospace, monospace;
  }
</style>
