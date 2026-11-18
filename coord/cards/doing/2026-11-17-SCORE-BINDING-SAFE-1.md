# SCORE-BINDING-SAFE-1：392-A 轨道绑定与安全切换

- 池序397；P1；父392；PC；发卡/验收Codex主管`/root`；派发已确认（本聊天用户批准392拆卡并开工）。
- 依据：`coord/reports/2026-11-17-score-binding-recon.md`；374/382已验收合入。
- 目标：左侧轨道列表明确当前编辑身份，只列可编辑MIDI clip；单clip自动装载、多clip显式选择；空轨/音频轨/读取失败诚实显示。成功加载目标身份与buffer绑定，dirty切换需显式放弃/取消，busy期间拒切或token隔离；加载失败、跨工程、clip消失、过期回包均不得把旧buffer写向新目标。零自动apply。
- 固定实现域（≤5）：新`scoreworkbench/ScoreTrackWorkbench.tsx`、新`scoreworkbench/binding.ts`、`ScoreWorkbenchPanel.tsx`、`scoreworkbench.css`、`workbench/Workbench.tsx`（均agent/webui/src/下）；不动记谱model/render/derive/edit/kernel核心、App/launcher/project、agent后端、Godot。必要测试可加，既有score_workbench_smoke.mjs控件断言可迁但J1-J3不删。
- 边界：398承接GUI跟随、399时间小条、400草稿存储设计；本卡不顺手实现它们。
- 验收：meaningful vitest反例=dirty A→B取消/放弃、B reload失败、apply中切换、工程切换后延迟回包、删除/音频/空轨/多MIDI；vitest/build/render exit0；生产装配真实栈选轨→edit→apply→目标回读与J1-J3 exit0，真栈候主管放行。单测可先行，不能记最终通过。
- 并行：388 App/launcher/project、394前端VitShell均不相交；脚本共享改动须经主管排队。
- 停止：真实数据无clip type或项目身份无法锚定/需要>5生产文件或越域→锚点上报；禁止降断言、伪造GUI选择、主机GUI操作。
- 领取：owner=`/root/score392_recon`（勘察后转本卡执行，Sol/high，PC）；分支`codex/score-binding-397`；worktree=`C:/Users/timoz/.codex/worktrees/score-binding-397/Vit_DAW`；创建基线779477ea，初始status/diff空。正式实现须本领取提交成功同步main并重读owner。
- 回执/验收：待回填。

## 2026-11-17 零栈交付

- c6991e1872de690b9c8976ec465366ff7c3cb1c0 @codex/score-binding-397 已推，26文件957+/149-（5生产文件）。715项全量vitest+最终focused11反例+build+真实App→Workbench装配11断言执行侧exit0，日志/报告在coord/runs/SCORE-BINDING-SAFE-1/local-397。
- 不将局部自验视为最终通过；真实栈及J1-J3欠证，主管独立审diff/复跑待执行。生产path身份非CAS，前端检查到写入间并发切工程边界如实。

## 真实栈批次准备（2026-11-18 晚，PC 主管 lead 放行）

- owner=`lead/score397stack`（DeepSeek-V4.1-Flash，PC；续用既有 worktree `C:/Users/timoz/.codex/worktrees/score-binding-397/Vit_DAW`，分支 `codex/score-binding-397` @ c6991e18）；本段只做**零栈独立复核 + 真栈 runbook 准备**，不启真栈、不改生产代码。
- 前置：397 零栈交付 c6991e18（26 文件 957+/149−，5 生产文件）已推；主管尚未独立验收/合入，真栈前必须先由主管复核 diff 并合入当前 main。
- 本段任务：①在 worktree 独立复跑 vitest/build/必要 render 段并核对 5 生产文件与断言未弱化；②按真实栈验收要求（生产路径选轨→edit→apply→目标回读 + J1–J3）列出 smoke 覆盖缺口；③以 `coord/runs/SCORE-DG1A-COMPLETE-1/RUNBOOK.md` 为模板写本卡真栈 RUNBOOK（kerl+agent berth、webui 从本分支构建、kernel 二进制借用、插件数据库前置、运行次数/失败分类/止损、工件契约）；④提交零栈证据与 runbook 草案后停手，候主管复核合入与真栈授权。
- 真栈纪律：单 owner；跑前 PC-RUNTIME-STACK 登记+推送+重读；新 run 目录；recents 快照/finally 还原哈希亲证；零 LLM 死端点；断言失败不原样重跑。
