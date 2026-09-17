# 决定：PC 侧用户级调度规则快照入仓（2026-09-17）

- 背景：用户直令——把 PC 的 agents.md 发一份到仓给 Mac 看，让 Mac 侧也遵守 PC 侧开发习惯，便于双端同步。此前各端本地用户级 AGENTS.md 按 BOOTSTRAP.md 设计「各自维护、不入仓」，Mac 无法看到 PC 侧调度规则。

## 裁定内容

1. PC 用户级全局调度规则（`C:\Users\timoz\.zcode\AGENTS.md`）以**快照**形式入仓：`coord/PC-AGENTS-SNAPSHOT.md`。
2. Mac 遵守口径：**纪律 / 节奏 / 任务流原则**按快照执行（一任务一会话、探索用搜索工具少 spawn 子 agent、review 看 diff 不看全文件、长文件交摘要引擎、消耗止损即降级、进度驱动）；**资源与路由**（引擎名单、积分额度、`C:\` 路径、queue 位置）是 PC 机器事实，Mac 按自身可用引擎对号入座，不逐字照搬。
3. 口令权威不变：BOOTSTRAP.md 仍是双端口令与流程唯一权威；各端本地工作区级 AGENTS.md「各自维护、不入仓」的设计不变——本快照是用户直令下的参考副本，不构成第二权威。
4. 时效：PC 原文更新不自动同步，由决策侧按需发新快照替换并更新日期。

## 修订落点

- 新增 `coord/PC-AGENTS-SNAPSHOT.md`（原文快照 + 适用口径头注）
- [BOOTSTRAP-MAC.md](../BOOTSTRAP-MAC.md)：注记挂链；备用开流提示词阅读清单纳入快照
