import { defineConfig } from "vite";
import { svelte } from "@sveltejs/vite-plugin-svelte";

// https://vite.dev/config/
export default defineConfig(({ command }) => ({
  plugins: [svelte()],
  base: command === "serve" ? "/" : "finicky-assets://local/",
  server: { host: "127.0.0.1", port: 5178, strictPort: true },
  build: {
    assetsDir: "assets",
    rollupOptions: {
      output: {
        assetFileNames: "[name][extname]",
        chunkFileNames: "[name].js",
        entryFileNames: "[name].js",
      },
    },
  },
}));
