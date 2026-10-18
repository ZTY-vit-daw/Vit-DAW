# RESULTS — 144 FIX-STACK-DRAFT-ROOT-ISOLATION-1 回执

- 执行：GLM ZCode PC 执行会话（2026-10-18 19:12–19:25 晚窗）
- 领取基线：e4d113f3（ff 后；原始领取 HEAD=d27f6dbc）；工作树本卡 diff=scripts/*.ps1 + 本目录工件，`git diff --stat` 已核对仅脚本/coord
- 栈腿：run=dev_agent_smoke_20261018_191332，exit 0；登记/释放见 PC-RUNTIME-STACK.md（19:12 占用→19:16 释放，均推 main）

## 交付（对应卡目标 1–4）

1. **审计表**：`audit_table.md`（107 个 ps1 全量：16 已隔离 / 3 等价隔离 / 1 中心部分隔离 / 26 未隔离 / 11 委托 / 6 kernel-only / 其余 N/A）。
2. **修复 26 个入口 + 中心**：
   - 新增 `scripts/vit_stack_draft_isolation.ps1`（`Enable-VitDraftRootIsolation` 外部 env 优先 + run 目录默认 / %TEMP% 兜底；`Assert-VitDraftRootIsolation` 断言 env 非空+路径存在+脚本自持根在隔离基内）；
   - `dev_agent_smoke.ps1` 中心修复：任何进程启动前默认设 env（场景=run 目录 `agent_drafts`；非场景=%TEMP% 每调用目录），`Assert-DraftRootIsolation` 于两处 agent 启动前断言，run 目录留 `draft_root_isolation.json` 痕；journey 原块不动（外部优先语义一致）；
   - 11 个直接起 agent 的 berth/脚本 + 12 个 Godot 链脚本 dot-source helper；h_g8 三脚本在 LOCALAPPDATA 等价隔离之上补显式 env；
   - **零业务断言改动**；零产品代码（agent/VitApp/前端未动）。
3. **真栈验证**（代表腿，不启 UI）：
   - 命令：`powershell -NoProfile -ExecutionPolicy Bypass -File ./scripts/dev_agent_smoke.ps1 -RepoRoot D:/Vit_DAW -StartKernel -RestartAgent -SkipBuild`
   - 退出码 0；被测 kernel SHA256=B6565DCF85…（stock）、agent SHA256=6C4443EA98…（-SkipBuild；被测面=脚本行为）
   - 真实根前后：`draft_root_before.json`（10185 @19:13:25）→ `draft_root_after.json`（10185 @19:14:45）→ `draft_root_delta.json` **zero_delta=true（零新增零删除）**
   - 隔离根证据：`draft_root_isolation_marker.json`（source=default）+ `isolated_root_listing.txt`（隔离根 1 个运行时 draft，19:13:35，含 .vit_agent/.vit_derived/.vit_history）
   - grep 证据：启动 VitAgent 的脚本中 `VIT_HISTORY_DRAFT_ROOT|vit_stack_draft_isolation` 缺失者=∅（见 audit_table §三）
4. **文档**：`scripts/SMOKE_TESTS.md` § Draft Root Isolation Contract + 本目录 `RUNBOOK.md`（契约摘要+验证方法+历史残留记录）。根 RUNBOOK.md 未动（守住「diff 仅脚本/coord」验收面，契约入 scripts 文档+本 RUNBOOK，可由决策侧提升）。

## 边界与如实记录

- 领取预检发现真实根 10181→10185（4 个 18:44–18:57 draft，非本腿产物，本腿零新增已证；未动，交决策侧——与 139 残留同模式的活跃泄漏，本卡修复落地后新入口不再产生此类写入）。
- `%TEMP%\fedraft1` 隔离区证据未删未动（139 残留，决策侧处置）。
- 本腿验证的是非场景模式（%TEMP% 默认路径）；场景模式 run 目录路径与 journey 原路径一致（同一 Join-Path 语义），未另跑场景腿（烟测面零业务断言改动，静态解析 94 脚本全过）。
- 全量解析校验：`scripts/*.ps1` 94 个 PowerShell parser 零错误。
