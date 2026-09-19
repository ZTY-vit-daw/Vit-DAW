# PORT-SMOKE-MAC-1：产品路径端测组 mac 化（7 件，两件套栈，达"PC 类似实验端测"基线）

- 优先级 / 预估 / 依赖：P1 / 1.5-2 天 / 无（与 PORT-VSPHUB-1 并行；两件套栈即可，不等 hub）
- 模型分级：L2 / GLM-5.3（脚本移植机械面为主 + 个别脚本平台语义判断）
- 目标：SMOKE_TESTS.md 现行套件中 **PC-only 产品路径组 7 件**在 mac 两件套真实栈各 exit 0，mac 等价脚本落 `scripts/`（命名对齐 `_mac.sh` 惯例），SMOKE_TESTS.md 逐件补条目：
  1. `run_ab_result_smoke.ps1` → mac 等价（**健康检查 §6 核心件**，MOM/mixboard/ABResult 全模式——优先做，作为组的验收锚）
  2. `run_project_audio_settings_preflight_smoke.ps1`
  3. `run_project_stems_import_smoke.ps1`
  4. `run_mix_single_tick_e2e.ps1`
  5. `run_vit_product_path_smoke.ps1`
  6. `run_live_material_observation_smoke.ps1`
  7. `run_observation_v1_acceptance_smoke.ps1`
- 移植口径：语义对照 PC 原件（ps1 段→mac 段对照表入回执，JOURNEY-1-MAC 同款）；起栈复用 A5/C2/JOURNEY 既有 mac 模式（真实内核+agent、端口归属、隔离临时目录、AGENTS §10）；机器态差异（如 Windows 专属 fixture 路径）按 JOURNEY 前例做 mac 适配并逐项声明；**不另起测试机制**（AGENTS §5）
- **§8 纪律（每脚本预声明）**：各脚本 ≤3 有效运行；成功=单轮 exit 0 且该脚本断言全过；失败分类分记（环境中断须原始证据）；同一脚本同断点两败止损，先取证/补锚点再跑；LLM 依赖脚本按 JOURNEY-1-MAC 先验模式（config 已就位，key 零入工件）
- 文件域：`scripts/`（新增 ≤7 个 `_mac.sh`）+ `scripts/SMOKE_TESTS.md`（补条目）；不改 ps1 原文；`agent/`、`VitApp/` 零触碰（接口缺口→域外上报）
- 验收标准：①7 件各权威 run exit 0 + 断言证据入工件；②§8 运行账目（逐脚本轮次/分类/止损状态）；③语义对照表；④SMOKE_TESTS.md 7 条目；⑤工件目录（run ID/日志/各段 JSON/双 bin sha256/HEAD 与 dirty）
- 停止条件：某脚本语义与 mac 栈接口现状不兼容 → 单件上交证据（其余件不受阻继续）；同断点两败止损；发现需改 agent/内核 → 域外上报；PC 侧脚本本身已腐化（跑不动/与现实现不符）→ 记录上交裁定
- 领取：
- 回执：
- 验收：
