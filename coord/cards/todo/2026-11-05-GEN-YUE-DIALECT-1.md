# GEN-YUE-DIALECT-1：YuE2 生成接入——genapi yue-local dialect 直连（生成首张 / 注册链③）

- 发卡：GLM 主管决策流 / 2026-11-05（依据=[259 报告](../../reports/2026-11-05-gen-recon-1.md) + [generation-mcp-pivot](../../decisions/2026-11-05-generation-mcp-pivot.md) §四-5）
- 派发确认：已确认（用户 2026-11-05 批准入池）
- 验收负责人：GLM 主管决策流
- 池序 278；**注册链第 ③ 位（前置=277 已验收提交）**；目标=genapi 包 dialect 扩展+注册；P1 / 1.5-2 天；L1-L2 / PC；真栈烟测腿一次（agent 侧），YuE2 引擎腿独立进程

## 目标

1. **yue-local dialect**：genapi 增加 YuE2 本地 REST（127.0.0.1:8087）dialect——输入（ABC 方言谱+歌词/风格参数）→ 生成任务 → 产物捕获（artifact 五件套：catalog spec / policy / harness case / 实现包 / artifact 捕获，对照 browser_fetch 先例 catalog.go:141/:462 模式）。
2. **注册**：catalog+policy 行级（若 genapi 命令族已注册则仅扩 dialect 参数面，注册面最小化）。
3. **复验条款执行（259 验收挂账）**：重跑"仅人声模式"探测（a cappella+Ins 全休止谱→生成→转谱回读无器乐断言）——单次实证转正式断言。
4. **真栈烟测腿**：chat 驱动"用 YuE2 把这段谱唱出来"→ dialect 调用→产物落 artifact→回读确认；YuE2 引擎独立进程启停（不占 VitApp 栈窗口时段）。

## 约束

- 文件域=agent 侧 genapi 包+catalog/policy 行级+测试+烟测脚本；零内核；YuE2 侧零改动（纯客户端）；
- 生成产物落 artifact 体系（不入源码树）；60s 歌墙钟 21-40s，烟测预算含两次生成；
- 显存预检（≤6.3GB 峰值实测值，超限→环境中断上交）。

## 验收标准

- 真栈烟测 exit 0（含复验断言：仅人声回读无器乐）；既有 genapi 测试零回退；
- 回执附：dialect 调用形态、两次生成产物路径、复验断言输出、commit hash。

## 停止条件

- YuE2 引擎不可达/显存不足→环境中断取证（不重跑原样超两次）；MCP 形态不在本卡（归 285）。

## 领取

- （执行侧回填）

## 回执

- （执行侧回填）

## 验收

- （主管回填）
