import { defineConfig } from "vite";
import react from "@vitejs/plugin-react";

export default defineConfig({
  base: "/app/",
  plugins: [react()],
  server: {
    port: 5178,
    strictPort: false,
    proxy: {
      "/agent": "http://127.0.0.1:7878",
      "/health": "http://127.0.0.1:7878"
    }
  }
});
