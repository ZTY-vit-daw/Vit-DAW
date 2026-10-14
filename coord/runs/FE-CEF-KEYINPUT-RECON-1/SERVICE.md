# FE-CEF-KEYINPUT-RECON-1 探针服务方式

## 构建产物与源码

- 源码：`agent/webui/input_probe/`（主仓分支 `port/fe-cef-keyinput-recon-1`，独立 vite 子项目，
  依赖复用上级 `agent/webui/node_modules`，本目录无需 npm install）
- 构建产物：`agent/webui/input_probe/dist/`（构建命令在 `agent/webui/input_probe/` 下执行
  `../node_modules/.bin/vite build`；**outDir 相对 input_probe 目录，永不触碰产品
  `agent/webui/dist`**）
- 探针构建产物快照哈希（run 时）：
  - `dist/index.html` + `dist/assets/index-BwOspHAG.js`（见 dist_probe_snapshot/ 目录内文件）

## 服务方式（独立端口 7879，避开 7878=agent、7899/5199=已占用模式）

```bash
# 启动（本 run 已用 nohup 常驻，PID 见下）：
cd /Users/timozty/Documents/Vit-DAW-fecef1/agent/webui/input_probe
nohup ../node_modules/.bin/vite preview --host 127.0.0.1 --port 7879 --strictPort \
  > /tmp/vit_input_probe_preview.log 2>&1 &

# 健康检查：
curl -s -o /dev/null -w '%{http_code}\n' http://127.0.0.1:7879/   # 期望 200

# 若会话重启后 7879 空了（lsof -nP -iTCP:7879 -sTCP:LISTEN 为空），按上面命令重启。
```

- 地址：`http://127.0.0.1:7879/`（构建产物静态服务，vite preview）
- dev 模式备用：`../node_modules/.bin/vite --host 127.0.0.1 --port 5179`（本 run 未使用）
- 本 run 服务进程：`node .../vite preview`，监听 `127.0.0.1:7879`（启动于 2026-10-14 18:3x +0800，
  日志 `/tmp/vit_input_probe_preview.log`）

## 产品 dist 防护核对

- 产品部署目录：`/Users/timozty/Documents/Vit-DAW/agent/webui/dist`（主工作树，10-13 修复版）
- 前后 mtime+sha256 清单：`dist_mtime_before.txt` / `dist_sha_before.txt` /
  `dist_mtime_after.txt` / `dist_sha_after.txt`——本卡全流程零触碰。
