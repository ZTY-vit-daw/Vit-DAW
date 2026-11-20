# PROGRESS — STACKLEASE-OWNER-PARAM-1（leaseparam442，池序 442，P3 卫生）

任务卡：coord/cards/（2026-11-21-STACKLEASE-OWNER-PARAM-1.md）；产线 leaseparam442。
工件目录：D:/Vit_DAW/coord/runs/STACKLEASE-OWNER-PARAM-1/

## 基线

- 领取基线：本地 main HEAD=883a0db9b76028665b4ea5726465d4a409c526b7（branch main）；origin/main=a6420951f975339bfa108b032959b3ef977caf19（本地领先 5 提交，无落后；协调提交由主管序列化，本卡按派发只做本地提交不推远端）。
- main 工作树有其他卡未提交改动（CURRENT-STATE.md、VitApp Workspace 运行态、coord/runs 各卡工件）：不在本卡文件域，一律不碰。scripts/ 干净。
- 基线全文见 claim_baseline_git.txt。

## 普查结论（scripts/*.ps1 全量，2026-11-20 执行）

引用 coord/resources/PC-RUNTIME-STACK.md 的脚本共 17 个。按门实现分类：

### A 类：解析 ledger 内容做 owner/格式门（本卡窄修对象，5 处）
1. run_entry_management_acceptance.ps1（L20-28）：`owner=Codex /root/entry388` 硬编码 + 卡号 `CONVERGE-ENTRY-1(388)` 硬编码 + 只读 origin/main 前 4 行（head 窗口）+ 占用标记用 char 码拼装。拒绝信息单一（'current remote stack owner is not entry388'），无原因分解。
2. vitshell_smoke.ps1（L74-78，Get-ForensicsResourceReservation）：owner 硬编码 `/root/restore403`（要求反引号引用格式）+ 卡号正则要求 `\*\*\d+`（数字紧跟 **，与现行 ledger 的 `**CARDNAME(441)` 格式不兼容——408 类假拒活体）+ 文件头逐字校验。卡号来自 -ForensicsCard（ValidateSet 403/404）。
3. vitshell_zproduct_smoke.ps1（L47-62，Assert-Lease）：owner 已参数化（-LeaseOwner 必填），但格式强耦合：事件头时间戳正则严格（秒/时区可选但格式死板）、owner 尾随分隔符类匹配（子串匹配有误授风险）、owner= 恰一个；拒绝只抛一条通用消息（'latest remote resource event does not grant this exact 409 runtime owner'），无原因分解。
4. vitshell_full_smoke.ps1（L391）：head 4/6 行窗口（origin/main 或工作树二选一）+ 卡号子串匹配（未 Escape）；无 owner 检查。
5. vitshell_open_chain_smoke.ps1（L157）：head 6 行窗口 + 硬编码卡号 'SHELL-2'；无 owner 检查。

### B 类：RuntimeOwner 参数门（已达标，无需修，4 处，同构）
run_legame_bind_smoke.ps1 / run_w02_a1c_smoke.ps1 / run_conv_cross_combo_smoke.ps1 / run_tutor_sim_smoke.ps1：-RuntimeOwner 非空校验，不解析 ledger。

### C 类：端口/进程归属守卫（不解析 ledger，无 owner 硬编码，不在窄修域）
dev_agent_smoke.ps1（Assert-StackPortOwnership + -AllowForeignStack）；ui_entry/ui_entry2/ui_os/ui_bridge/ui_host；run_yue_wire_smoke / run_yue_dialect_smoke；fs_largeproject_smoke（Phase 0 清场预检）。

### D 类：无门
g_runtime_readonly_smoke.ps1（只读白名单 GET 探针，设计如此）。

### 停止条件判定
「lease 门实现分散无共面（>5 处异构实现）→停卡」：**ledger 内容解析门恰 5 处（A 类），未超 >5**；且窄修为逐脚本局部修改，不要求统一实现，不触发强行统一。判定：继续执行（边界情况 5=5 已在卡面报告标注，由主管复核）。另：全脚本 grep 硬编码 owner 串仅 A1/A2 两处（'owner=Codex /root/entry388'、'/root/restore403'）。

## 实施计划

- 步骤 1：基线快照 + 卡片 mv doing + 领取节提交 main（只加本卡 coord 文件）→ claim_baseline_git.txt
- 步骤 2：worktree D:/Vit_DAW_wt_pc442 + 分支 pc442-lease
- 步骤 3：A1/A2/A3 修：新增纯函数 Get-StackLeaseVerdict（容错解析+原因分解），owner 参数化（空值 fail-closed 可诊断），删除硬编码 owner 串
- 步骤 4：A4/A5 修：head 窗口→最新事件行解析（tolerance）+ 可诊断拒绝；open_chain 卡号参数化（缺省 'SHELL-2' 保持兼容）
- 步骤 5：新增 scripts/stack_lease_gate_selftest.ps1（AST 抽取三份 Get-StackLeaseVerdict 同步断言 + 合成 ledger 用例表 + 解析检查 + 硬编码 grep 断言；运行态只写 $env:TEMP）
- 步骤 6：验收三件（解析/dry-run exit 0、自测 exit 0、grep 硬编码清零）→ evidence 文件落本目录
- 步骤 7：合入 main + 卡片 done + ROSTER leaseparam442 行 + 证据提交

## 步骤日志

- [2026-11-20] 步骤 0（普查+基线）完成：普查结论如上；claim_baseline_git.txt 落盘；origin/main 已 fetch。下一步：卡片领取提交。
- [2026-11-21] 步骤 1（领取）完成：卡片 mv doing + 领取节，提交 main d170d36a（仅本卡 coord 文件）。
- [2026-11-21] 步骤 2（worktree）完成：D:/Vit_DAW_wt_pc442 @ 分支 pc442-lease（基线 d170d36a）。
- [2026-11-21] 步骤 3-4（窄修）完成：5 装置脚本收口（3 owner 门 Get-StackLeaseVerdict 同形纯函数；2 head-window 门 Get-LatestOccupancyEvent 定位器）；entry388/restore403 硬编码清零；owner 空参 fail-closed 逐条件诊断。发现并处理两个坑：(a) entry_management/vitshell_smoke 原无 BOM，新加 CJK 字面量会 PS5.1 ANSI 误读 → 补 UTF-8 BOM（与 zproduct/full/open_chain/legame_bind 惯例一致）；(b) 裸 owner 值内嵌反引号 session id（owner=Name `<id>`）→ 取引号内 id 为规范可比 owner（原 403 门契约）。
- [2026-11-21] 步骤 5（自测）完成：scripts/stack_lease_gate_selftest.ps1（AST 逐字抽取 3 副本 Get-StackLeaseVerdict × 18 案例表 + 2 副本 Get-LatestOccupancyEvent × 5 案例 + 5 脚本解析检查 + 硬编码扫描排除自身 fixture）。首跑抓出 3 处（2 处为自测数据错误：双引号吞反引号、release 用例行序违反最新在前；1 处为扫描自引用）→ 修复后 **exit 0 全绿**。运行态仅写 %TEMP%（C:/Users/timoz/AppData/Local/Temp/stack_lease_gate_selftest_20261121_001056），报告副本落本目录 selftest_report.json。
- [2026-11-21] 步骤 6（验收三件）：解析 5 脚本零错（自测 step1 亲证）；自测 exit 0；grep 亲证硬编码清零（hardcode_grep_evidence.txt：device 脚本零命中；自测仅 fixture/needle 数据行）。「dry-run 到 lease 检查处」以自测逐字执行门函数实现（零栈验收，未启真实栈）。
- [2026-11-21] 步骤 7：分支提交 b71a37df；合入 main（merge --no-ff c6ce90bb，无冲突）；main 上复跑自测 exit 0；卡片 done + ROSTER + 证据提交。

## 终态

DONE — 执行侧自验完成，待主管验收（验收负责人=lead）。
- 四件套：命令+退出码见 acceptance_evidence.txt；产物=6 个 scripts/*.ps1（5 改 1 增）；问题与坑见步骤日志 3-5（BOM/反引号契约/自测自引用，均已修复）；断点=0 次（单会话一次跑通，无重派）。
- 遗留：调用方契约变化（entry388/403 取证重跑须显式传 owner 参数）与停止条件边界判定（5=5）见 acceptance_evidence.txt 遗留节。
