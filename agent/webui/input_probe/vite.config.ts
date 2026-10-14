import { defineConfig } from "vite";
import react from "@vitejs/plugin-react";

// FE-CEF-KEYINPUT-RECON-1 探针构建配置。
// outDir 相对 root（本目录）→ 产物落 input_probe/dist，永不触碰产品 agent/webui/dist。
export default defineConfig({
  plugins: [react()],
  server: { host: "127.0.0.1", port: 5179 },
  preview: { host: "127.0.0.1", port: 7879, strictPort: true },
  build: { outDir: "dist", emptyOutDir: true },
});
