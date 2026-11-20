import { fileURLToPath } from "node:url";
import react from "@vitejs/plugin-react";
import tailwindcss from "@tailwindcss/vite";
import { defineConfig } from "vite";

export default defineConfig({
  plugins: [react(), tailwindcss()],
  resolve: { alias: { cn: fileURLToPath(new URL("./src/ui/cn.ts", import.meta.url)) } },
  build: { modulePreload: {polyfill: false}, rollupOptions: { input: {
    panel: fileURLToPath(new URL("./index.html", import.meta.url)),
    verification: fileURLToPath(new URL("./verification.html", import.meta.url)),
  } } },
});
