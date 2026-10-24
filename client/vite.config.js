import { defineConfig } from "vite";
import { svelte } from "@sveltejs/vite-plugin-svelte";

// Tauri expects a fixed dev port and no HMR websocket issues in the webview.
export default defineConfig({
  plugins: [svelte()],
  clearScreen: false,
  server: {
    port: 1420,
    strictPort: true,
  },
  envPrefix: ["VITE_", "TAURI_ENV_"],
  build: {
    target: "esnext",
    outDir: "dist",
  },
});
