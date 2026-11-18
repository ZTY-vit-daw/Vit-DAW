# SHELL-ENTRY-REFRESH-1 执行计划（主管预写，2026-11-18 晚）

- 前置：410/411 已由主管验收并合入前端 main；412 已验收并合入主仓 main；两仓 main 已推送共享远端。
- 目标：让实际手测入口 `VitShell/build/Release/启动VitShell.cmd` 使用 409 产品路径 + 本批呈现/呈现源的新产物；只做构建/无窗验证，零真栈。
- 不提交源码；允许的未提交产物：frontend tracked `extension/bin/*.dll`（记录旧/新哈希）、`VitShell/build/**`、主仓 `agent/bin/VitAgent.exe`、`agent/webui/dist/**`。

## 步骤

1. 两仓 `git fetch origin main` → `git pull --ff-only`；记录 HEAD、`git status --short`。
2. VitShell Release：`cmake -S VitShell -B VitShell/build -G "Visual Studio 18 2026" -A x64 "-DVITSHELL_WEBVIEW2_DIR=D:/Godot/project/vit-daw-frontend/extension/third_party/webview2"`；`cmake --build VitShell/build --config Release`。记录 `VitShell/build/Release/VitShell.exe` + WebView2Loader.dll SHA256。
3. GDExtension（复用 413 预构建产物，若前端源码自预构建后无变化）：隔离目录 `%TEMP%/shell413-ext-<id>/`（extension/ + godot-cpp/ 拷贝）；`scons platform=windows target=template_debug arch=x86_64 build_library=no generate_bindings=no -j4`；产物 DLL 复制到主检出 `extension/bin/vit_extension.windows.template_debug.x86_64.dll`（旧/新 SHA 记录，不提交）。
4. VitAgent：`cd agent && go build -o ./bin/VitAgent.exe ./cmd/vitagent`（若 410/411/412 未改 agent 源码，仍是刷新入口产物）；记录 SHA。
5. WebUI dist：`cd agent/webui && npm run build`（412 已合入 395 呈现）；记录 `dist/index.html` SHA。
6. 无窗验证：
   - 隔离 headless 项目加载新 DLL：`EXTENSION_REGISTERED=true/METHODS_OK=true/FEATURE_ENABLED=true`（参考 413 预构建探针）。
   - `Godot_v4.6.1-stable_win64_console.exe --headless --import --check-only --path D:/Godot/project/vit-daw-frontend` exit 0。
   - 核对 `project.godot` 含 `window/size/mode.vit_shell_hidden_boot=1`；`app/startup/vitshell_window_entry.gd` 与 `app/shell/vitshell_launch_readiness.gd` 存在。
7. 证据：`coord/runs/SHELL-ENTRY-REFRESH-1/` 下 PLAN 执行记录、命令+exit、哈希表、`git status` 摘要、探针输出。

## 边界

不启 VitShell/Godot GUI/agent/内核栈；不改 tracked 源码；`启动VitShell.cmd` 当前参数仍适用则不改（如需改先报主管）。构建/探针同断点两次失败→停。
