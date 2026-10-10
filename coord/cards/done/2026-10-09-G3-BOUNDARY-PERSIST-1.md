# G3-BOUNDARY-PERSIST-1：pull 边界持久化瞬态失败观察取证（单次）

- 池序 37（[G3 终裁遗留①](../../rulings/2026-10-09-G3-FINAL-ruling.md)；来源=G3-ATTRIB-1 异常分记）；目标仓库=D:\Vit_DAW
- 优先级 / 预估 / 依赖：P3 / 0.5h / 无；单测域
- 模型分级：L0 / 任意引擎
- 现象（ATTRIB-1 run 20261009_214138_harnessab，pull base r1）：run 边界持久化（continuation/边界提交面）失败一次，内部状态呈 completed/done 瞬态——单次、未复现、未影响 run 终局分类。
- 目标：定位失败点（日志+工件回读），判定=瞬态竞态（如文件锁/时序）还是确定性缺陷；单测复现或"不可单测复现"结论如实登记。修复非本卡义务（确定性缺陷另立卡）。
- 验收：失败点锚点（文件:行）+分类结论（瞬态/确定性）入回执；若可复现附最小反例。
- 领取：2026-10-10 闲时车道 / origin/main 33036cfb / owner=闲时任务·GLM-5.3-Flash（ZCode 主管会话 sess_1c9ec412 派发）·PC / 分支=无（只读取证卡，零代码改动）/ 领取提交=本 mv commit
- 回执：[coord/runs/G3-BOUNDARY-PERSIST-1/receipt.md](../../runs/G3-BOUNDARY-PERSIST-1/receipt.md)——**分类结论=瞬态竞态（跨进程 agent runtime state 租约竞争），非确定性缺陷**。失败点锚点：goalrunner_chat.go:2037-2042（发射）→ :2990/:3058（persistContinuationState）→ continuation_scheduler.go:832-843 → server.go:7123-7174（四出口，锁竞争出口采信）→ runtime_state_lock.go:50-71（文件租约 2min，owner 每进程唯一）。证据：pull 日志 :42-43（同秒 completed/done→failed）+ :19（同窗同函数锁竞争 WARN，owner=scheduler_75eac58790a1bfc0 expires 21:44:49）+ 全 run 唯一（pull 1/push 0）+ f1 于 21:43:51 同路径自愈。时间线=push 进程 21:42:49 后仍活、pull 21:42:50 启动共占同一 fixture 工程租约域，r1 的 500ms 重试窗落入外来持有期。非系统性数据丢失：全量快照下次 persist（30s 后）自愈闭合、无 durable continuation 在押。单测复现性：降级机制可复现（预写他主活租约→边界断言，最小反例描述入回执 §6）；自然竞态不可确定性复现（如实登记）。观察项两条：A=goal 边界 persist 失败无专用 WARN（观测面小缺口，归 hygiene 机会面）；B=harness push→pull 交接进程并存（场景脚本域卫生项）。
- 验收：**pass（2026-10-10 晚窗主管终审）**——[rulings/2026-10-10-EVENING-IDLE-FINAL-rulings.md](../../rulings/2026-10-10-EVENING-IDLE-FINAL-rulings.md) §1；时点妥协记录=闲时车道主管会话内执行+同会话验收，按 G3-ATTRIB-1 A1 先例以主管等强度直接复核（五环链+原始日志逐锚点亲读）替代独立复核腿。观察项处置：A→立卡 HYGIENE-GOALPERSIST-WARN-1（池序 48）；B→并入 FS-LARGEPROJECT-SMOKE-1 约束④（脚本清场检查）。G3 终裁遗留①销项。
