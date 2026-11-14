import { resolve } from "node:path";
import { defineConfig } from "vite";
import react from "@vitejs/plugin-react";

export default defineConfig({
  base: "/app/",
  plugins: [react()],
  build: {
    rollupOptions: {
      // UI-FOUNDATION-1 主管授权域扩展：仅新增 gallery.html 多页入口（2026-11-14）。
      input: {
        main: resolve(__dirname, "index.html"),
        gallery: resolve(__dirname, "gallery.html")
      }
    }
  },
  server: {
    port: 5178,
    strictPort: false,
    proxy: {
      "/agent": process.env.VITE_AGENT_PROXY ?? "http://127.0.0.1:7878",
      "/health": process.env.VITE_AGENT_PROXY ?? "http://127.0.0.1:7878"
    }
  }
});
