# RUNBOOK — 144 FIX-STACK-DRAFT-ROOT-ISOLATION-1：真栈脚本 draft 根隔离

- 卡：`coord/cards/todo/2026-10-18-FIX-STACK-DRAFT-ROOT-ISOLATION-1.md`
- 审计表（脚本→状态全量清单）：本目录 `audit_table.md`
- 隔离契约正文：`scripts/SMOKE_TESTS.md` § "Draft Root Isolation Contract (Card 144)"

## 隔离契约（摘要）

1. 任何启动 VitAgent 的真栈/烟测脚本（直接启动 / kernel+agent berth / Godot 链），必须在**首个进程启动前**设置 `VIT_HISTORY_DRAFT_ROOT`，使 agent 会话 draft 不落用户真实根 `%LOCALAPPDATA%\Vit\ProjectHistory\drafts`。
2. 优先级：**外部显式 env 永远优先**；否则默认 run 目录 `<RunDir>\agent_drafts`；无 run 目录的腿用 `%TEMP%\VitStackDrafts\<label>_<时间戳>\agent_drafts`。
3. 启动前断言：env 非空、路径存在、脚本自持根必须在隔离基内（helper `scripts\vit_stack_draft_isolation.ps1` 的 `Enable-VitDraftRootIsolation` + `Assert-VitDraftRootIsolation`；中心脚本 `dev_agent_smoke.ps1` 内联同契约 + `Assert-DraftRootIsolation`）。
4. kernel-only berth（不起 agent）无需 draft 根隔离；构建/静态/HTTP-only 脚本不在契约内。
5. 新增真栈脚本时必须 dot-source helper 或内联同契约；评审红线=启动 VitAgent 的脚本没有隔离即不合规。

## 验证方法（代表腿，本次采用）

- 前置：登记 `coord/resources/PC-RUNTIME-STACK.md`（窗口+owner=本卡），确认 143 栈腿不在跑。
- 代表腿：`dev_agent_smoke.ps1 -StartKernel -RestartAgent -SkipBuild`（kernel+agent，不启 UI；非场景模式=修复新增的 %TEMP% 默认路径）。
- 前后断言：run 前后各一次真实根 `C:\Users\timoz\AppData\Local\Vit\ProjectHistory\drafts` 目录清单快照（名称+时间戳），**目录数不变、零新增**；断言 run 产生的 draft 落在 %TEMP% 隔离目录。
- 通过标准：脚本退出码 0 + 前后快照零差集。

## 历史残留记录（不删，决策侧处置）

- 139 腿（TRACK-LIST-GAP-1）2026-10-18 18:04-18:07 写入真实根的 4 个运行时 draft，已由 Lead 移入 `%TEMP%\fedraft1\stack_leg_orphan_drafts\`；真实根 10185→10181（BATCH9 §二）。
