import {fileURLToPath} from "node:url";
import react from "@vitejs/plugin-react";
import tailwindcss from "@tailwindcss/vite";
import {defineConfig} from "vite";

export default defineConfig({
  plugins: [react(), tailwindcss()],
  resolve: {
    alias: {
      "@": fileURLToPath(new URL("./src", import.meta.url)),
      cn: fileURLToPath(new URL("./src/ui/cn.ts", import.meta.url)),
    },
  },
  server: {
    host: "127.0.0.1",
    port: 28742,
    strictPort: true,
    hmr: {clientPort: 28741, path: "/__vite_hmr"},
  },
  build: {
    modulePreload: {polyfill: false},
    rollupOptions: {
      input: {
        panel: fileURLToPath(new URL("./index.html", import.meta.url)),
        verification: fileURLToPath(new URL("./verification.html", import.meta.url)),
      },
      output: {
        manualChunks: id => (id.includes("echarts") ? "charts" : undefined),
      },
    },
  },
});
