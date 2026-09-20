# PORT-SMOKE-MAC-3：VSP 五件套 mac 化（三件套口径，hub 已就位）

- 优先级 / 预估 / 依赖：P1 / 1 天 / 依赖 VSPHUB-1 已验收（三件套起栈配方与冒测入口 `scripts/vsp_hub_three_piece_smoke_mac.sh` 在 main）；与 PORT-PS1-SYNC-2 跨机并行；本卡为唯一 Mac 栈使用者
- 模型分级：L2 / GLM-5.3（VSP 协议面端测移植，机械面为主 + hub 交互语义）
- 目标：现行套件 PC-only 的 VSP 五件在 mac 三件套栈（内核+vsphub+agent）各 exit 0：
  1. `vsp_hub_extension_smoke.ps1` → mac 等价
  2. `codex_vsp_readonly_probe.ps1` → mac 等价（VSPHUB-1 已做过其语义的 10 检查子集，可复用模式）
  3. `vsp_phase4_hub_http_asset_smoke.py` → 跨平台适配或薄 mac 包装（⑦ dad_probe 先例）
  4. `vsp_hub_websocket_smoke.ps1` → mac 等价（VSPHUB-1 的 ws 探针模式可复用）
  5. `run_vsp_hub_lifecycle_smoke.ps1` → mac 等价
  - 起栈统一三件套口径（hub 起停/注册链复用 VSPHUB-1 配方；vsphub 二进制已在 `agent/bin/`，如缺重新构建）；语义对照 ps1↔mac 入回执；SMOKE_TESTS.md 五条目
- §8 纪律：每件 ≤3 轮、同断点两败止损；LLM 依赖按 JOURNEY 先验（key 零入工件）
- 文件域：`scripts/`（新增 ≤5 个 mac 等价物 + SMOKE_TESTS.md）；ps1 原文零改动；`agent/`、`VitApp/`、前端仓零触碰
- 验收标准：①五件各权威 run exit 0（三件套栈、run ID/各段日志/端口归属/hub 注册证据）；②语义对照表；③SMOKE_TESTS 5 条目；④工件目录（§8 账目/双 bin+hub sha256/HEAD 与 dirty）
- 停止条件：某件语义与 mac 三件套现状不兼容 → 单件取证上交其余继续；vsphub 运行时在五件场景暴露缺陷 → 取证转 blocked（hub 侧修复另卡）；需改 agent/vsphub 源码 → 域外上报
- 领取：2026-09-20 18:12 / origin/main 4826248379367db4a9bb99b11178e2740a1fd27d / 分支 port/smoke-mac-3（主工作树；领取前已有 VitApp/Workspace/{Settings.xml,default_project.xml} 内核运行态改写〔SETTLE-1 回执在案的既有已知项，非本卡 diff〕；本卡为唯一 Mac 栈使用者，doing/ 中 SETTLE-1 为 PC 侧卡）
- 回执：2026-09-20 18:45 CST（Mac/GLM-5.3 执行侧自验完成，待决策验收）
  - **实现 commit**：port/smoke-mac-3 @ b246305（6 文件 +3729：五件 mac 脚本 + SMOKE_TESTS.md +179；ps1/py 原文零改动〔③ py 原样复用，sha256 入各 run_meta〕；agent/、VitApp/、前端仓零触碰）。coord 领卡 commit：main @ b15b35b。两提交均已推 origin。
  - **交付①-⑤（五件 mac 等价物，统一 VSPHUB-1 三件套配方栈：fake-root 内核 + agent 先起 required 注册〔重试窗实证〕+ hub 起停，端口归属 5555/5556/5557/7878/8787 断言 + teardown 排空断言 + gates.env + summary.json，env=3/functional=1 分类）**：
    1. `scripts/vsp_hub_extension_smoke_mac.sh`（①ps1 语义 1:1：extension 注册/在册/403 permission_denied 拒写/注销/除名；`--check-kernel-routes`=ps1 同名 switch，加 hello+state.snapshot；信封字段/client_id/schema/检查名逐一保留）
    2. `scripts/codex_vsp_readonly_probe_mac.sh`（②全参数面：`--client-id/--client-version/--scope/--timeout/--hub-only`；10 检查同名 `codex_*`；unregister 在 finally；project.timeline 内轨 1002-1006 零泄漏断言）
    3. `scripts/vsp_phase4_hub_http_asset_smoke_mac.sh`（③薄包装，⑦ dad_probe 先例：栈起后**原样调用**跨平台 py，断言其 exit 0 + v1 summary；PC 驱动的 timeout 映射 max(30,N)/max(60000,N*1000) 保留）
    4. `scripts/vsp_hub_websocket_smoke_mac.sh`（④ps1 语义 1:1 over `ws://…:8787/vsp/stream`：hello→hub.transport_binding、client 供 session_id 注册、status transport=vsp.hub.websocket、unregister payload.removed、除名；event.notification 跳过计数、40 消息窗、correlation_id=message_id 均保留。.NET ClientWebSocket → **内嵌 stdlib RFC6455 客户端**〔握手 Accept-key 校验/掩码帧/分片重组/ping-pong〕，mac python3 无 ws 库且不装依赖）
    5. `scripts/run_vsp_hub_lifecycle_smoke_mac.sh`（⑤lifecycle 全门移植：构建+sha256 证据〔含产品路径 agent/bin 双 bin 哈希记录、不写入〕→ VSPHUB-1 配方栈起〔**PC Godot start_page autostart 段的三件套等价**〕→ 端口归属+运行 exe 路径（inode 比对）+运行二进制 sha256 核对〔ps1 "Verify process ownership"+"binary hashes match" 对应〕→ /health+/vsp/status〔vsp.hub.http+vit.agent.official role=agent〕→ Godot headless ws 消费探针〔VSPHUB-1 配方，`--skip-ws-probe` 可跳〕→ 原样 asset py 集成〔`--skip-hub-http-asset-smoke`=ps1 -SkipHubHttpAssetSmoke〕→ teardown+summary）
  - **语义对照表（ps1↔mac 要点；逐件全表在脚本头注释）**：| PC | mac |—| HubUrl/StatusUrl/StreamUrl 参数 → --hub-http 派生；switch 参数 → 同义 flag（--check-kernel-routes/--hub-only）；Invoke-WebRequest/RestMethod → urllib（HTTPError 捕获保 403 语义）；ConvertTo-Json 信封 → 同字段 dict（schema/message_id 前缀/trace_id/created_at UTC ISO）；.NET ClientWebSocket → stdlib RFC6455；PC "栈由 lifecycle/dev smoke 外部起" → 脚本内 VSPHUB-1 配方栈（卡裁定三件套口径）；⑤ Godot autostart → 配方栈同端口同注册链；工件 PC 落 repo Workspace → 隔离 artifact root（AGENTS §10）。**未移植（超三件套口径，SMOKE_TESTS Notes 在案）**：⑤ 的 Godot start_page autostart 生命周期本体、-Reuse* 开关、60-track playback、GUI health/architecture 探针（需前端作栈主）。
  - **§8 账目（预声明：每件 ≤3 轮、单权威 run exit 0 全门绿即成）**：六 run 零失败轮——①`vsp_ext_smoke_mac_20260920-183232` **exit 0**（7 门/7 检查绿；首 run `…-182941` 同为全门 PASS，唯退出码被控制台管道吞〔执行侧取证失误非功能断点〕，补直采 run 即本权威 run）；②`codex_vsp_probe_mac_20260920-183022` **exit 0**（10 检查）；③`vsp_p4_asset_smoke_mac_20260920-183055` **exit 0**（py 9 检查：track 1010/import clip 1014/资产 no-big-json/清理）；④`vsp_ws_smoke_mac_20260920-183118` **exit 0**（5 检查含 ws hello transport_binding）；⑤`vsp_hub_lifecycle_mac_20260920-183135` **exit 0**（11 门含 port_ownership_and_hashes/ws_consumption_probe/hub_http_asset_smoke；归属证据 8787|hub|workdir vsphub@429bdedf…、7878|agent|workdir vitagent@445971f4…、5555+5556|kernel|repo VitApp@fd0d3d84…）。栈停记录均 SIGTERM:0:1（agent/hub）/SIGTERM:143:1（kernel，A1 在案 mac 平台特征）；收尾端口排空断言过、全五端口无监听复核过。
  - **二进制账目**：六 run 同内核 `fd0d3d84cc20…`（`VitApp/build/…/Debug/VitApp`，mtime 2026-09-18=VSPHUB-1 stage-2 同源；VitApp 源码 HEAD 0ea259a 2026-09-17 后零改动，--kernel-bin 复用合规〔§9：sha+mtime 入件〕）；agent `445971f4d39c…`/hub `429bdedf1997…` 各 run 自 HEAD=**b15b35b**（=origin/main）go build 新建；产品路径旁证 hub `agent/bin/vsphub`@`77fdffe9…`=VSPHUB-1 交付①原样未动、agent@`502a1aef…`（仅记录零写入）。run 时 dirty=2（VitApp/Workspace 两运行态文件，领取前已有项，非本卡 diff；本卡内核均跑 workdir fake root，未触主仓 Workspace）。
  - **端测覆盖边界声明（AGENTS §5）**：全部证据来自真实三进程栈（内核+vsphub+agent，每件独立起停）；⑤ 的 ws 消费走 Godot headless 探针（前端适配器数据通道）；编辑器 UI 面/渲染面未覆盖（本卡无 webui/渲染改动；手测入口归用户闸门）。LLM 零参与（纯协议/栈确定性断言，JOURNEY 先验不适用，无 key 入工件）。
  - **工件**：`~/Documents/vit-smoke-mac3-artifacts/<run_id>/` ×6（run_meta 含 HEAD+dirty+脚本 sha、双 bin sha256、gates.env、各探针 JSON、hub 状态/日志、lsof 证据、停栈记录、summary.json）；控制台日志 `/tmp/mac3_p{1..5}_run{1,2}.log`。SMOKE_TESTS.md 五条目（套件节+逐件 Purpose/Common commands/Notes）已入实现 commit。
  - 卡片处置：验收①②③④全达成（五件 exit 0+对照表+5 条目+工件账目），**mv done 待决策验收**。
- 验收：pass（2026-09-20，ruling [2026-09-20-SMOKEMAC3-pass.md](../../rulings/2026-09-20-SMOKEMAC3-pass.md)：五件全绿工件亲核（lifecycle summary=PASS 直读）、栈/二进制账目严格（端口归属+sha256 核对+§9 复用合规）、websocket stdlib 客户端实现采认、未移植边界显式声明；b246305 已 cherry-pick 入 main；**现行套件 PC-only 项清零，mac 全绿达 15 件**）
