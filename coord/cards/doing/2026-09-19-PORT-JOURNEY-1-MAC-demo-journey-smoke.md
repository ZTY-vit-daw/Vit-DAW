# PORT-JOURNEY-1-MAC：demo 关键旅程冒测 mac 化（工程打开→权限→实验→装载→A/B，移植收官卡）

- 优先级 / 预估 / 依赖：P1 / 1-1.5 天 / 依赖 B4B 已验收+真栈手测 PASS（内核/agent/前端三件套在位与起栈链路已实证）
- 模型分级：L3 / GLM-5.3（千行级旅程脚本移植 + LLM 概率性运行的判定面；止损触发或语义冲突时找参谋决策模型讨论）
- 目标：PC `scripts/journey1_demo_journey_smoke.ps1`（1062 行，demo 关键旅程权威驱动）的**旅程语义**在 mac 真实栈可执行：五段全链——**工程打开→权限→实验→装载→A/B 试听**——驱动脚本 exit 0。产出 `scripts/journey1_demo_journey_smoke_mac.sh`（对齐 mac 冒测惯例：g_runtime_readonly_smoke_mac.sh / pca_calibration_chain_mac.sh 的结构、起栈复用 A5/C2 模式、运行态独立临时目录）；回执附 ps1↔mac 段级语义对照表；`scripts/SMOKE_TESTS.md` 补条目
- **§8 概率纪律（卡面预声明，执行不得倒推）**：最多 **3 轮有效运行**；成功条件=单轮 exit 0 且五段各有显式断言通过（工程/权限/实验/装载/A/B 逐段判定，非仅日志阶段名出现）；失败分类按 AGENTS §8 分开记录（no_candidate_found / capability_blocked / 断言失败 / 崩溃 / 环境中断——环境中断须有原始日志/退出证据方可排除出有效轮次）；**止损线**：同一确定性断点连续两败后禁止原样重跑，先补代码锚点/缩小假设或转取证卡
- **LLM 前置（先验项）**：旅程含真实 LLM 参与（ps1 内 prompt/nudge 为英文 A1 词汇表口径）；开工先验 agent LLM 运行环境就位性（key/模型配置，**key 不得入仓入工件入日志**）；环境缺失 → blocked 上交，不得以桩/伪造应答替代
- 文件域：`scripts/journey1_demo_journey_smoke_mac.sh`（新增）+ `scripts/SMOKE_TESTS.md`（补条目）；不改 ps1 原文；`agent/`、`VitApp/` 零触碰（接口缺口→域外上报）
- 验收标准：①mac 真栈（内核+agent 真实起停、ZMQ/HTTP 面）权威 run exit 0；②五段显式断言证据（各段 JSON/断言输出入工件）；③§8 运行账目（有效轮次/失败分类/止损状态如实）；④语义对照表+SMOKE_TESTS 条目；⑤工件目录（run ID/全量日志/各段 JSON/双 bin sha256/HEAD 与 dirty 记录）
- 停止条件：LLM 环境缺失→blocked；止损线触发→停转卡；PC 旅程语义与 mac 栈接口现状不兼容→证据上交由决策侧裁定扩域或调口径；GUI 面不在本卡（用户手测已覆盖渲染面，本卡走 agent 面）
- 领取：
- 回执：
- 验收：
