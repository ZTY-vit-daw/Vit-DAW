# SHELL-ENTRY-REFRESH-1：实际手测入口产物刷新

- 池序 413；P1；PC；发卡/验收=本会话 PC 主管（lead）；派发确认=已确认（用户 2026-11-18 晚批准方案 A）。
- 依赖：410/411 验收合入前端 main、412 验收合入主仓 main；执行时先 fetch 并快进两仓到协调 main。
- owner=`lead/entry413`（DeepSeek-V4.1-Flash 执行子代理，PC）；主检出执行（`D:/Godot/project/vit-daw-frontend` + `D:/Vit_DAW`）；不提交源码，只留未提交构建产物与哈希证据。
- 单目标：让 `VitShell/build/Release/启动VitShell.cmd` 实际指向 409 产品路径的新产物；不开真栈。

## 步骤

1. 两仓 `git pull --ff-only` 到协调 main；记录 `git rev-parse HEAD` 与 `git status --short` 摘要；前端 tracked 必须 clean 后再构建。
2. VitShell：`cmake -S VitShell -B VitShell/build -G "Visual Studio 18 2026" -A x64 "-DVITSHELL_WEBVIEW2_DIR=D:/Godot/project/vit-daw-frontend/extension/third_party/webview2"`；`cmake --build VitShell/build --config Release`；记录 `VitShell.exe` SHA256。
3. GDExtension：`cd extension && scons platform=windows target=template_debug arch=x86_64 build_library=no -j4`（godot-cpp 用仓内 `../godot-cpp` 4.2.2；环境缺失→停报）；确认 `extension/bin/vit_extension.windows.template_debug.x86_64.dll` 更新且 `dumpbin /exports` 含 `vit_extension_library_init`；记录 DLL SHA256。
4. 主仓：`cd agent && go build -o ./bin/VitAgent.exe ./cmd/vitagent`；`cd agent/webui && npm ci（如需）&& npm run build`；记录 VitAgent.exe 与 dist/index.html SHA256。
5. 无窗验证：隔离项目副本做 Godot 4.6.1 `--headless` 扩展加载探针（`EXTENSION_REGISTERED=true` / `METHODS_OK=true` / `FEATURE_ENABLED=true`），并核对主检出 `project.godot` 含 `window/size/mode.vit_shell_hidden_boot=1`。
6. 工件：`coord/runs/SHELL-ENTRY-REFRESH-1/` 记录命令、exit 码、产物哈希、两仓 HEAD 与 status 摘要、探针输出；留待主管提交。

## 边界与停止

不启动 VitShell/Godot GUI/agent/内核栈；不改 tracked 源码；不提交 `VitShell/build`、`extension/bin` DLL、agent 二进制/dist。`启动VitShell.cmd` 当前路径仍有效则不改；如需改先报主管。任一构建/探针同断点两次失败、或需要源码改动/越域→停报。

## 回执（短）

命令+exit、产物哈希、HEAD、探针输出、未提交产物清单、边界声明（零真栈、零 GUI）。

## 领取（2026-11-18 晚，主管）

- owner=`lead/entry413`（DeepSeek-V4.1-Flash 执行子代理，PC）；执行主检出；先做**隔离扩展 DLL 预构建**（临时目录、零源码改动、零真栈），待 410/411/412 合入并经主管放行后再刷新最终产物（VitShell Release / webui dist / 入口哈希）。

## 回执与验收（2026-11-18 晚）

- 主管放行后完成最终刷新：VitShell.exe `5D420DED…`（311,808 B）、extension DLL `B270362D…`（替换 tracked 旧 DLL，未提交）、VitAgent `27D09FA0…`、webui dist `0886058E…`；真实工程 import 两命令 exit 0；部署 DLL 探针三断言 true。
- 入口 `启动VitShell.cmd` 未变且路径有效。工件：[PRESTAGE](runs/SHELL-ENTRY-REFRESH-1/PRESTAGE.md)、[FINAL](runs/SHELL-ENTRY-REFRESH-1/FINAL.md)。
- 裁定：[rulings/2026-11-18-SHELL-ENTRY-REFRESH-1-pass.md](../../rulings/2026-11-18-SHELL-ENTRY-REFRESH-1-pass.md) PASS；真实栈呈现由 414 验证。
