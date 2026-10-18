# FIX-STACK-DRAFT-ROOT-ISOLATION-1：真栈/烟测脚本默认隔离 VIT_HISTORY_DRAFT_ROOT（139 腿发现真实 draft 根被写）

- 发卡：12:00-18:00 决策流 / 2026-10-18 晚窗（依据=FIX-TEST-DRAFT-ISOLATION-1(140) 定位窗口发现：139 真栈腿 agent 未设隔离 draft 根，向真实根写 4 个运行时 draft；Lead 已隔离到 `%TEMP%\fedraft1\stack_leg_orphan_drafts\`）
- 派发确认：**已确认（用户 2026-10-18 晚窗）**（P2 hygiene；不阻塞功能）
- 验收负责人：GLM 主管决策流（或当班决策流）
- 池序 144；目标仓库=D:\Vit_DAW；P2 / 0.5 天 / 无依赖
- 模型分级：L1/L2 / PC 执行流

## 问题

- 真栈/烟测脚本对 draft 根的隔离不一致：118/134/137 等腿在 run 目录内设了 `VIT_HISTORY_DRAFT_ROOT`，但 139 腿（kernel+vsphub+agent berth）未设，agent 启动/激活把运行时 draft 写进**真实** `C:\Users\timoz\AppData\Local\Vit\ProjectHistory\drafts`（本次 4 个：18:04:25/18:04:26/18:05:47/18:07:58，含 .vit_agent/.vit_history；已由 Lead 移入 `%TEMP%\fedraft1\stack_leg_orphan_drafts\` 未删；真实根 10185→10181）。
- 风险：污染用户真实 draft 根/干扰手测现场；违反 AGENTS §10「测试运行时状态不得落共享真实目录」同精神。

## 目标

1. **审计**全部真栈/烟测脚本（`scripts/*.ps1`、`coord/runs/**/run_*.ps1` berth 等）是否设置 `VIT_HISTORY_DRAFT_ROOT`（或等价隔离），输出脚本→隔离状态表。
2. **修复**未隔离脚本：默认在 run 目录建隔离 draft 根并注入 env（外部显式 env 优先），启动前断言 env 生效/路径在 run 目录内；不改变各腿业务断言。
3. **验证**：至少 1 条代表腿（kernel+agent，不启 UI）run 前后真实 draft 根目录数不变（附前后清单+时间戳）；grep 证据=无未隔离真栈入口。
4. 文档：RUNBOOK/README 注明隔离契约；真实根历史残留已隔离（记录）。

## 文件域

`scripts/*.ps1`（真栈/烟测脚本）、`coord/runs/**/*.ps1`（berth/诊断脚本）；**零产品代码**（agent/VitApp/前端不改）。越域先上报。

## 验收标准

- 审计表（脚本→是否隔离→证据）；未隔离脚本全部补默认隔离（外部 env 优先）；
- 代表腿前后真实 draft 根零新增（前后清单+时间戳）；`git diff --stat` 仅脚本/coord 工件；
- 真栈验证腿需 Lead 登记窗口+GO（改脚本可先静态做）。

## 停止条件

- 需改产品代码/内核协议 → 先上报；不得删除隔离区证据（由决策侧处置）。

## 关联

- 发现卡：FIX-TEST-DRAFT-ISOLATION-1（140，done）；139 TRACK-LIST-GAP-1 run 窗口残留
- 纪律：真实根残留事件已记录 BATCH9 §二

## 领取记录（2026-10-18 19:12 +0800）

- owner=GLM ZCode PC 执行会话（本会话，L1/L2）；仓库主检出 D:\Vit_DAW（脚本域直接改主检出，无独立 worktree——文件域仅 scripts/*.ps1 + coord 工件，与 doing 其余卡零冲突）
- 领取基线：HEAD=e4d113f3（ff 同步 origin/main 后）；领取前工作树既有改动=VitApp/Workspace 两 xml（既存运行时状态，非本卡）+ coord 未跟踪工件
- 真栈验证腿占用登记见 coord/resources/PC-RUNTIME-STACK.md（19:12 窗口）

## 验收（2026-10-18 晚窗 GLM 主管决策流）：pass

- diff 亲读：helper（外部 env 优先/run 目录默认/%TEMP% 兜底/断言三查含路径包含）+dev_agent_smoke 中心修复（任何进程启动前置 env、全脚本生命周期保持供晚到 restart 继承、两处 agent 启动断言、run 目录 draft_root_isolation.json 标记）+berth 接入模式（dot-source+Enable+Assert）抽查 restart_agent.ps1 干净；零业务断言改动零产品代码核实。
- 决策侧独立复验：全部 scripts/*.ps1 PowerShell Parser 亲跑 **0 错误**（94+ 文件）；工件亲核=zero_delta=true（10185→10185，19:13:25→19:14:45 包夹 run 窗）+isolation marker（source=default，%TEMP% 隔离基）+isolated_root_listing（隔离根 1 运行时 draft）。
- **处置裁定（执行侧上交的 10181→10185 四个 18:44–18:57 draft）**：时间窗=用户 R2 手测时段（含「新建工程」流程项），判定=**用户合法手测产物非泄漏**（真实使用写真实根为正确行为），保留不动、不入隔离区；后续用户手测如需干净根可自行清理。
- **流程瑕疵登记**：执行侧 release 提交（bea42f83）只含工件漏脚本本体——交付未入库；验收侧补救提交（scripts 28 文件 +366/−19）随本裁定入 main。执行侧后续卡须在 release 提交核对交付物完整性。
- 卡终态=done（随本裁定移池）。
