# Agent HUD Open 借鉴笔记（开放录音棚参考）

- 记录：GLM 主管决策流 / 2026-11-09（用户提供参照物：https://github.com/jazzenchen/agent-hud-open）
- 消费方：开放录音棚协作方案 v2 讨论（[v1 定稿](../decisions/2026-11-07-vit-open-studio-collaboration-v1.md) 已定调未实施）；成员适配器规格制定时
- 参照物性质：macOS 缺口/屏幕边 HUD（Swift 6，Apache-2.0）——17 家 coding agent 的统一观测面：配额/余额/Token 统计/审批等待/回复提醒；近端直连无中心服务；iPhone/Watch 伴侣（本地网络免费直读，远端走自有 iCloud 付费）

## 一、可借鉴五点（映射到 v1 定稿接缝）

### 1. Provider 抽象 + 能力矩阵 → 成员适配器规格（§3 成员适配器）

Agent HUD 用 per-agent provider 把 17 家异构 agent 归一（身份/配额/状态/审批/回复），且把**能力矩阵当一等数据**（每家 agent 支持哪些面：quota/live/approvals/questions/replies/return/turns——README 有现成矩阵表）。

**借鉴**：成员适配器规格应内置**成员能力矩阵**作为契约数据——每个成员 harness 支持哪些结构化接口（公共发言/工具请求/权限确认/状态披露/执行回执），不支持的面显式披露为缺失，不假设全量。开放录音棚裁定过的「成员 harness 须自带 typed 等价控制面」即矩阵中的一列（随 ADR-EFFECT-CONTROL-REDESIGN-V2 重议时改写该列语义）。

### 2. 审批等待面三件套 → Vit 权限与执行层的确认交互（§2.8/§3）

Agent HUD 的 approvals 设计：请求**带完整上下文等待**（命令/文件/编辑 diff/工具入参）→ 应答不切窗（Deny / Allow once / **Always allow=授权规则沉淀**）→ **显式 TTL 回落**（无人应答 10 分钟后回退给 agent 自己的 prompt，不静默吞）→ 明文原则「**Agent HUD never answers for you**」。

**借鉴**：studio 确认交互的四件套同款：上下文等待卡 / 三级应答+规则化授权（对应 PCA 准入思路的产品形态）/ 超时显式回落语义 / 不代答原则写入管理员行为规范（v1 §3 已有"管理员不替成员改写立场"，此为其交互面补全）。
**交叉链接**：这正是 327 卡（FIX-GOV-INTERACTION-TTL-1）在修的产品先例——10 分钟回落=治理交互 TTL 的成熟形态，327 验收时可引用。

### 3. 近端直连无中心服务 → 成员状态披露面（v1 基础设施未细化段）

「同网络免费直读：Mac 直接服务手机，无 Agent HUD 服务器」。

**借鉴**：studio 成员状态/请求的近端披露可走同款形态——workspace 本地服务、成员经本地网络读取、不引入中心服务器；与既有裁定精神一致（卡池不启用自动投递/watcher/定时调度，用户触发执行）。

### 4. 数据访问三律 → 成员入群信任契约（§4 成员读取成本/信任边界）

Agent HUD 明文：只读 agent 已有记录 / 向厂商读配额余额但**凭据永不存储** / 开源构建**零上传**。

**借鉴**：成员适配器的数据契约三律——优先读成员 harness 已有记录（telemetry/hooks）、不要求成员交出凭据、公共面零上传。作为成员入群的最低信任边界写入 v2。

### 5. 成本透明账本 → 唤醒记账的展示形态（§3「唤醒」定义段）

v1 已裁定「建立连接、保存收件、恢复空会话与启动推理应分别记账」；Agent HUD 的 per-agent/model/session/turn Token 统计+API 价估算（含 sub-agent 归属父会话、cache 命中率）正是这四类记账的成熟展示粒度。

**借鉴**：studio 公共现场的成本账本按此粒度设计；成员配额耗尽预警（90% 或撑不到重置时提前告警）对应成员「本轮能否完成」的就绪状态披露。

## 二、不迁移面（如实登记）

1. **单人驾驶舱 ≠ 多人现场**：HUD 是"我监督我的 agents"，studio 是管理员/用户+多成员（人+agent）协作——只借观测/审批的抽象与 UI 模式，不借整体形态；
2. **它只观测不控制**：与 v1 §2.9 结构化接口分流天然兼容，但 studio 还需要 HUD 没有的提案/回执/工程状态共享；
3. **逆向适配各家本地记录格式的维护成本**：studio 成员接入应优先结构化协议（VSP/A2P 裁定），HUD 式"读已有记录"仅作降级通道；
4. **Mac/Apple 生态特化**（notch/Watch/iCloud）：借模式不借实现；其分层法（Support/Core/Desktop 三包+DesktopApplication 宿主嵌入）值得成员适配器打包时参考。

## 三、顺带观察

ZCode 本身在 Agent HUD 支持矩阵中（approvals/questions 两面）——若成员侧用 ZCode 系 harness，观测面接近免费；成员适配器的首版可以 ZCode 系 + Claude Code 系两型起步（矩阵最全的两家）。

## 维护

- 本笔记为过程参考，studio v2 设计时消费；v2 落档后本文件转历史记录。
