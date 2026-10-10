# Ruling：2026-10-10 晚窗——两张闲时只读卡终审 pass（BOUNDARY-PERSIST-1 / G3-PROBE-METER-RECON-1）

- 裁定：两卡均 **pass**。时点妥协记录：两卡由闲时车道在主管会话内执行，验收同会话——按 G3-ATTRIB-1 A1 先例以**主管等强度直接复核**（执行全程逐锚点亲读：BOUNDARY 五环链+原始日志行、PROBE 八源锚点+工件反向确认）替代独立复核腿，如实记录，不构成豁免先例。

## 1. G3-BOUNDARY-PERSIST-1：pass

- 结论（瞬态竞态/跨进程租约竞争/非系统性数据丢失）采信：证据链五环在执行期逐环亲读（goalrunner_chat.go:2037-2042 → :2990 → continuation_scheduler.go:832-843 → server.go:7123-7174 四出口排除 → runtime_state_lock.go 租约机制+owner 每进程唯一）；pull 日志 :42-43 同秒双态 + :19 同窗同函数锁竞争 WARN + 全 run 唯一计数 + f1 自愈——闭环。
- 观察项处置：**A（goal 边界 persist 失败无专用 WARN）→ 立卡 HYGIENE-GOALPERSIST-WARN-1（池序 48）**；**B（交接进程并存）→ 并入 FS-LARGEPROJECT-SMOKE-1 已知约束④（脚本清场检查）**。
- G3 终裁遗留①销项。

## 2. G3-PROBE-METER-RECON-1：pass

- 报告三节齐+逐项锚点亲读（settleBatch 预声明 :411-430、budget.go 账户/执法/披露、l2_probe_batch.go:38 结构化 ElapsedMS、harness.go:879-892 墙钟、内核 K1/K2 日志形态、VSP 结构化面反向确认零命中）；三层口径分层不触发停止条件的判定复核成立。
- **G3 重开条件①的计量前提由"缺失"改判"可满足（有源）"**；层(a)落地卡=PULL-PROBE-METER-1（池序 44，已在池）。
