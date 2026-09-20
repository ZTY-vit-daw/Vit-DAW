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
- 领取：
- 回执：
- 验收：
