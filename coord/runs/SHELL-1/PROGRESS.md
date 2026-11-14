# SHELL-1 执行进度（断点续跑协议）

- 卡：coord/cards/doing/2026-11-15-SHELL-1.md（领取提交见卡面）
- owner：GLM-5.3-Flash · PC · ZCode（freeflash 量产执行流）
- worktree：前端仓 `D:/Godot/project/vit-daw-frontend_wt_shell1`（port/shell-1 @ 83390f5）；主仓 `D:/Vit_DAW_wt_shell1`（port/shell-1 @ 90bf9527）
- 断点恢复指引：先读卡面，再读本文件最后一条「下一步」；按文件存在性判断产物，不重做已完成段。

## 勘察结论（M0 前置，已完成）

1. **停止条件判定：未触发**——`extension/src/vit_webview_host.cpp`（1923 行@83390f5）的 godot_cpp 耦合仅：头文件 GDCLASS/String/Dictionary/Variant 签名、`godot_to_wide/wide_to_godot` 转换、`emit_signal`、`DisplayServer` 父窗回退。Win32/WebView2 主体（窗类注册/窗口过程/WebView2Loader 动态加载/environment+controller 创建/事件回调/通知桥/topmost+最小化+关闭语义）全部纯 C++，可直接脱离 GDExtension 编译进独立 exe。
2. **agent 启动实证链**（343/355 证据：`app/startup/start_page.gd` L846-976/L1059-1080 + `scripts/dev_agent_smoke.ps1` L2013-2035）：
   - exe：`<dev_root>/agent/bin/VitAgent.exe`（freshness 排序候选链）；参数 `-http 127.0.0.1:7878 -last-log-path <log> -keep-last-log-lines 800`
   - env：`VIT_DAW_DEV_ROOT`/`VIT_DAW_ROOT`=<dev_root>；`VIT_AGENT_LAST_LOG_PATH`、`VIT_AGENT_LLM_TELEMETRY_PATH`、`VIT_AGENT_MESSAGE_LOOP_DEBUG_PATH`、`VIT_VSP_HUB_LAST_LOG_PATH`（empty-set）；`VIT_AGENT_VSP_HUB_URL=http://127.0.0.1:8787/vsp`
   - cwd：dev_agent_smoke 用 `$AgentDir`（=agent 目录）。**关键**：agent `findWebUIRootFrom`（internal/chat/server.go L7191 区）从 cwd 与 exe 目录向上找 `agent/webui/dist`——cwd 决定 React 是否被服务。**「API 连不上」头号嫌疑=cwd 缺失导致 /app/ 无根**；VitShell 将显式设 agent cwd=agent 目录，本卡验收面复现或排除。
   - 健康轮询面：`GET /agent/tools`（start_page `_AGENT_TOOLS_URL`）；装载真数据面：`GET /agent/launcher/greeting`（launcher_flow.go L555，无内核依赖，有 recents 兜底文案）
   - agent 无内核/无 VspHub 可独立启动并服务 /app/ + greeting（烟测 agent-only 复用腿先例）
3. **WebView2 SDK**：`extension/third_party/webview2/include/WebView2.h` + `WebView2Loader.dll`（头+Loader 模式，无导入库依赖——宿主代码 GetProcAddress 动态解析 `CreateCoreWebView2EnvironmentWithOptions`）。VitShell 按相对路径复用同一目录，不复制。
4. **复用机制裁定**（候主管验收复核）：抽取=`VitShell/shared/vit_webview_host_core.*` 独立编译单元（detached-window 最小面 + 通知桥 + 关闭/最小化语义），extension 源**零 diff**；provenance 逐函数标注源行号；分歧风险（extension 与 VitShell 双份宿主核心）记回执候 SHELL-2 裁定单源化。
5. doing 在飞卡文件域核对（90bf9527 时点）：五卡全在主仓 agent/webui/harness/coord 域，与前端 C++ 不相交。
6. godot-cpp：`D:/Godot/project/godot-cpp` 不存在（extension 构建链需 `../godot-cpp`）；本卡 extension 零 diff → **无需 extension 构建验证，无需建 junction**（消除了 §3 junction 纪律的触发面）。

## 里程碑

- [x] M0 领取（本提交）：双 worktree + 卡 todo→doing + PROGRESS 建立
- [ ] M1 VitShell exe 骨架+WebView2 主窗（共享核心+构建脚本+编译 exit 0）
- [ ] M2 agent 进程树（拉起/健康轮询/退出全树终止）
- [ ] M3 装载态/错误态 + status 端点
- [ ] M4 vitshell_smoke.ps1 + 真栈轻腿（§2.2 登记→≤60s 复核→run→拆栈→释放）
- [ ] 回执回填 + doing→done

## 下一步

M1：在前端仓 worktree 建 VitShell/ 工程（shared 核心 + main + CMake），编译 exit 0。
