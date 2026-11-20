# STACKLEASE-OWNER-PARAM-1：ps1 lease 门 owner 参数化窄修

- 池序 442；P3 卫生；PC；发卡=lead（2026-11-21 凌晨，夜间 free 车道批）；验收负责人=lead。
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
