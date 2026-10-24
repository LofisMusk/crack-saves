<script lang="ts">
  import { onMount } from "svelte";
  import { open } from "@tauri-apps/plugin-dialog";
  import { createGame, listProcesses } from "../api";
  import type { Game } from "../types";

  interface Props {
    prefillExe: string;
    onCancel: () => void;
    onDone: (game: Game) => Promise<void>;
    toast: (message: string, tone?: "info" | "ok" | "error") => void;
  }
  let { prefillExe, onCancel, onDone, toast }: Props = $props();

  let name = $state("");
  let exePattern = $state(prefillExe);
  let saveDir = $state("");
  let notes = $state("");
  let saving = $state(false);
  let error: string | null = $state(null);

  let processes = $state<{ name: string; count: number }[]>([]);
  const SKIP_PREFIXES = [
    "cracked-saves",
    "tauri",
    "explorer",
    "finder",
    "dwm",
    "kernel",
    "windowsservices",
    "systemui",
  ];

  onMount(async () => {
    try {
      const all = await listProcesses();
      processes = all
        .filter((p) => !SKIP_PREFIXES.some((s) => p.name.toLowerCase().startsWith(s)))
        .slice(0, 40);
    } catch {
      /* non-fatal */
    }
  });

  function pick(p: { name: string }) {
    if (!exePattern) exePattern = p.name.replace(/\.[a-z0-9]+$/i, "");
  }

  async function chooseDir() {
    const dir = await open({ directory: true, multiple: false, title: "Select the folder where this game stores its saves" });
    if (typeof dir === "string") saveDir = dir;
  }

  const valid = $derived(name.trim().length > 0 && exePattern.trim().length > 0 && saveDir.length > 0);

  async function submit() {
    saving = true;
    error = null;
    try {
      const game = await createGame({
        name: name.trim(),
        exe_pattern: exePattern.trim(),
        save_dir_hint: saveDir,
        notes: notes.trim(),
      });
      toast(`Added "${game.name}"`, "ok");
      await onDone(game);
    } catch (err) {
      error = String(err);
    } finally {
      saving = false;
    }
  }
</script>

<div class="head">
  <h1>Add game</h1>
  <p class="sub">Point Cracked Saves at the executable and the folder holding its saves. The watcher does the rest.</p>
</div>

<form onsubmit={(e) => { e.preventDefault(); submit(); }} class="form">
  <label>
    Game name *
    <input bind:value={name} placeholder="Elden Ring (cracked)" />
  </label>

  <div class="two-col">
    <label>
      Executable pattern *
      <input
        bind:value={exePattern}
        placeholder="eldenring.exe"
        list="procs"
      />
    </label>
    <div class="proc-hint">
      Running processes (click to use):
      {#each processes as p (p.name)}
        <button type="button" class="chip" onclick={() => pick(p)}>{p.name}</button>
      {:else}
        <span class="muted">none listed</span>
      {/each}
    </div>
  </div>

  <label>
    Saves directory *
    <div class="dir-row">
      <input value={saveDir} readonly placeholder="Pick the folder containing the game's saves…" />
      <button type="button" onclick={chooseDir}>Browse…</button>
    </div>
  </label>

  <label>
    Notes (optional)
    <textarea rows="2" bind:value={notes} placeholder="e.g. GOTY build v1.09, saves in UTF-8"></textarea>
  </label>

  {#if error}
    <div class="banner">⚠ {error}</div>
  {/if}

  <div class="actions">
    <button type="button" onclick={onCancel}>Cancel</button>
    <button type="submit" class="primary" disabled={!valid || saving}>
      {saving ? "Adding…" : "Add game"}
    </button>
  </div>
</form>

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
    max-width: 640px;
  }
  label {
    display: flex;
    flex-direction: column;
    gap: 7px;
    font-size: 13px;
    color: var(--text-dim);
  }
  textarea {
    font: inherit;
    background: var(--bg);
    color: var(--text);
    border: 1px solid var(--border);
    border-radius: 8px;
    padding: 7px 10px;
    resize: vertical;
  }
  .two-col {
    display: grid;
    grid-template-columns: 260px 1fr;
    gap: 16px;
  }
  .proc-hint {
    display: flex;
    flex-wrap: wrap;
    align-items: center;
    gap: 6px;
    font-size: 12.5px;
    color: var(--text-dim);
    border: 1px dashed var(--border);
    border-radius: 8px;
    padding: 10px;
    max-height: 74px;
    overflow-y: auto;
  }
  .chip {
    font-size: 12px;
    padding: 2px 9px;
    border-radius: 999px;
    font-family: ui-monospace, monospace;
  }
  .dir-row {
    display: flex;
    gap: 8px;
  }
  .dir-row input {
    flex: 1;
  }
  .muted {
    color: var(--text-dim);
  }
  .banner {
    border: 1px solid rgba(255, 180, 84, 0.5);
    background: rgba(255, 180, 84, 0.08);
    border-radius: var(--radius);
    padding: 10px 14px;
  }
  .actions {
    display: flex;
    justify-content: flex-end;
    gap: 8px;
  }
</style>
