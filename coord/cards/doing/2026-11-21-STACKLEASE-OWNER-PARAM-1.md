# STACKLEASE-OWNER-PARAM-1：ps1 lease 门 owner 参数化窄修

- 池序 442；P3 卫生；PC；发卡=lead（2026-11-21 凌晨，夜间 free 车道批）；验收负责人=lead。

## 领取

- 领取时间：2026-11-21；领取时本地 main HEAD=883a0db9b76028665b4ea5726465d4a409c526b7；origin/main=a6420951f975339bfa108b032959b3ef977caf19（本地领先 5 提交、无落后；协调提交由主管序列化，本卡只本地提交不推远端）。
- 执行 owner：leaseparam442（ZCode GLM-5.3-Flash 执行子代理，PC）；分支 pc442-lease；worktree D:/Vit_DAW_wt_pc442；文件域 scripts/*.ps1（零栈腿，不启真实栈）。
- main 工作树领取时存在其他卡未提交改动（CURRENT-STATE.md、Workspace 运行态、coord/runs 工件）：与本卡文件域不相交，不触碰。scripts/ 干净。
- 普查停止条件判定：ledger 内容解析门恰 5 处（entry388/restore403/zproduct/full/open_chain），未触发 >5 停卡；边界情况（5=5）已在 run 工件 PROGRESS.md 标注候主管复核。普查全文：coord/runs/STACKLEASE-OWNER-PARAM-1/PROGRESS.md。
- 背景（408 runtime 发现，POOL-INDEX backlog 登记）：ps1 装置的栈 lease 门**硬编码 owner 串**且与 PC-RUNTIME-STACK.md 资源记录格式耦合——曾致预检假拒+现场格式修正绕行。409 已收口，本窄修解锁。

## 目标

1. 普查 `scripts/*.ps1`（重点 dev_agent_smoke / fs_largeproject_smoke / run_yue_wire_smoke / run_legame_bind_smoke / g_runtime_readonly_smoke 等带 berth/lease 门的装置）：定位 owner 串硬编码与记录格式解析点。
2. 窄修：owner 参数化（`-RuntimeOwner` 或等价参数，缺省取当前用户/会话可识别串）；lease 门对 PC-RUNTIME-STACK.md 记录行的解析对**格式变化容错**（前后空白/字段序/时间戳格式），拒绝时给出可诊断原因而不是静默假拒。
3. 防回归：一个最小自测（可用 Pester 或内嵌断言模式，按仓库既有 ps1 测试惯例；无惯例则 dry-run 模式断言）。

## 约束

- 文件域：`scripts/*.ps1`（装置面）；不改 Go/C++/webui 产品代码；不改 PC-RUNTIME-STACK.md 语义（只改读写它的装置侧）。
- 停止条件：lease 门实现分散无共面（>5 处异构实现）→ 停卡上报普查结果，不强行统一。

## 验收（零栈）

- 改动脚本语法过（`powershell -NoProfile -Command` 解析/`-WhatIf` 或 dry-run 到 lease 检查处）；自测 exit 0；硬编码串清零（grep 亲证）。
