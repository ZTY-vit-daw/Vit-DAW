# 仓库优化 TODO（Token 经济学 + 答辩基线）

日期：2026-08-22
背景：本清单来自一次 token 消费习惯审计。核心原则：**把"每开一个会话都要重付一次的成本"变成"一次性写入仓库的资产"。**
用法：每个任务开一个全新会话执行，开场白只需一句："读取 docs/REPO_OPTIMIZATION_TODO_2026-08-22.md，严格执行任务 TN，遵守验收标准，不做范围外修改。"
执行顺序：T1 → T8 依次执行；T9 及以后为答辩后项目。

---

## T1 删除 webui 影子备份文件（5 分钟，零风险）

git 历史就是备份，以下文件是纯负资产（每个会话的读取/索引税）：

- `agent/webui/src/App.tsx.fixed-preview`
- `agent/webui/src/App.tsx.fixed-preview2`
- `agent/webui/src/App.tsx.fixed-preview3`
- `agent/webui/src/App.tsx.mojibake-backup-20260617`

验收：`ls agent/webui/src/` 无备份残留；webui 测试（参考 G0 基线记录的 messageLifecycle 测试方式）通过；提交。

## T2 补全 .gitignore 并清理工作树垃圾（10 分钟）

当前未跟踪的垃圾目录会被 agent 会话 glob/读取/误判：

- 在 `.gitignore` 增加：`.gocache/`、`.playwright-cli/`、`agent/backups/`、`**/.vit_history/`
- 检查 `git status --porcelain` 中其余 `??` 条目，逐项判断：该 ignore 的 ignore，该提交的留给 T4，确认是运行时垃圾的删除（删除前先看一眼内容）

验收：`git status --porcelain` 不再出现上述目录；提交 .gitignore 变更。

## T3 修复 .vit_history 测试泄漏（30–60 分钟，小型代码修复）

`agent/internal/chat/.vit_history/` 出现在源码树内，说明有测试把运行时状态写进了相对路径。

- 定位写入方（重点排查 `agent/internal/chat/` 与 `agent/internal/history/` 中使用相对路径或固定路径的测试）
- 改为 `t.TempDir()` 或测试专属工作区
- 若同时发现同类泄漏（其他目录下的运行时残留），一并修复

验收：删除残留目录后运行嫌疑测试包，目录不再重新出现；`git status` 保持干净；提交。

## T4 收拢工作树并打答辩基线（1–2 个会话，本清单最重要的一项）

当前 55 个脏条目意味着每个新会话都要付"考古费"。**纪律：禁止 `git reset` / `git clean` / 丢弃任何现有改动**（G0 基线记录已声明当前工作树包含不可丢弃的实现工作）。

- 清点全部修改/未跟踪条目，按主题分组（自由态运行时修复 / 脚本 / 文档 / 测试）
- 跑 G0 验证命令 + `go test ./...`（在 `agent/` 下）确认绿
- 按主题分多个 coherent commit 提交；打 tag `defense-baseline-2026-08`

验收：`git status` 干净；tag 存在；测试全绿。此后到预答辩前只做实验与修 bug，不再开新战线。

## T5 写 AGENTS.md（1 个会话，杠杆最大的一次性资产）

位置：仓库根目录。必含六节：

1. **常用命令**：Go 构建/测试（在 `agent/` 下）、webui 测试、release 脚本入口
2. **目录地图**：三层架构（VitApp 内核 / Go agent / webui）+ `agent/internal` 重点包一览
3. **缩写词典**：CCB / COM / DOM / MOM / TIM / TOM / DAD / VSP / PCA / RLM / EPM / FXM 等
4. **文档白名单**：指向 CURRENT-STATE.md（T6），声明"只许读标记为现行的文档"
5. **已知陷阱**：Windows 路径在 bash 中的引号、测试不得写源码树、封存测试集纪律（不硬编码轨道名、失败只记类型）等
6. **健康检查命令**：从 G0 基线记录摘录那五条验证命令

验收：开一个全新会话，只给它 AGENTS.md，要求"跑健康检查"——它能独立完成。

## T6 写 CURRENT-STATE.md 并归档废弃文档（1–2 个会话）

- 为 `docs/` 全部 108 份文档建立索引，三态标注：**现行** / **已废弃** / **历史记录**（基线、验证记录类保留原位）
- 已废弃的用 `git mv` 迁至 `docs/archive/`（保留历史）
- 判断标准：内容已被实现超越且会误导 agent 的 → 废弃；纯记录性 → 历史记录

验收：索引覆盖全部文档，无未标注项；链接有效；提交。

## T7 写 RUNBOOK.md（1 个会话）

面向"不熟悉本项目的人"（包括未来的你、答辩委员会、潜在合作者、换了工具的新 AI 会话）：

- 如何构建：Go agent / VitApp（CMake）/ webui 三者各自步骤
- 如何运行：三进程启动顺序与依赖
- 如何测试：分层的测试入口（单元 / 契约 / 回放 / 真模型验收）
- 常见故障：至少收录当前已知的三五个坑及处置

验收：一个从未接触本仓库的人仅凭此文档能完成构建 + 启动 + 跑通健康检查。

## T8 写 TASK_TEMPLATE.md（15 分钟）

位置：仓库根目录。四个必填字段：

```text
目标：<一句话>
涉及文件/范围：<路径列表；明确不做什么>
验收命令：<一条你自己会运行且看得懂输出的命令>
参考：<相关现行文档章节>
```

用途：sol→terra 交接物 + 所有 AI 会话的任务开场。验收：用它在下一个真实任务上开一次会话，跑通。

---

## 答辩后（现在不做）

- **T9 拆分 App.tsx**（12k 行 → 按功能模块化；决定之后所有 UI 任务的入场费）
- **T10 FS 状态机收敛**（以 audioclosure 为宿主统一 FS0–FS9，消灭四层状态机漂移；见 FREE_STATE_MINIMUM_IMPROVEMENT_WORKFLOW_UPDATE_2026-08-22.md 的修订意见）
- **T11 跨 run 诊断记忆**（ledger 从 loop 级提升为 project 级）

## 长期习惯（不进仓库，进习惯）

- 单议题单会话；新任务新开
- 非平凡任务先要计划再动手
- 你知道位置就直接指路（file:line），别让 agent 搜索
- 粘贴日志先裁剪；纠正 AI 引用最小片段
- 机械任务 → 免费/便宜模型；强模型只留判断与疑难
- 每个里程碑后用自己的话写一页系统笔记（复述检查）
