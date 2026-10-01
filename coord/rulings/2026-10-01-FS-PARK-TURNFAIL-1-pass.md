# Ruling：FS-PARK-TURNFAIL-1 — pass（2026-10-01 决策会话）

- 实现：2b4953e7+6b3624d1@port/fs-park-turnfail-1 → 合并 main 63ea54c8/f2b51443（cherry-pick）；卡片状态变更 48d849a3 已由执行侧合规直推 main。全量烟测工件（agent log+完整报告+console）随验收补入 coord/runs/FS-PARK-TURNFAIL-1/（绿红两目录；worktree 内 876MB×2 完整工程副本不入库，摘要 digest 在案可回指）。
- 亲核项：
  1. **取证报告直读**（69 行）：三拒链事件级回放完整（拒①settled 轮门=正防线/拒②G 门 gap=陈旧调度器副本致 judgment boundary 失效+计数锁终局/拒③终局锁拒绝 LLM 按引导给出的 needs_observation）——与 M8 野外证据逐 seq 对上；三根因 RC1 路由语义缺失（主）/RC2 陈旧副本/RC3 终局强制定性成立；停止条件核验正确（路由有定义、缺采纳结算，用户裁定已定案）。
  2. **diff 直读**（judgment_park_continuation.go 159 新+server 拦截 8 行+goalrunner 挂点 1 行）：Part A 裁定忠实落地——`judgmentParkPendingRound` 精确谓词（park 边界∧当前轮零 UserJudgmentEvidence=显式 POST 通道优先）；全部失败路径 return false 且持久态零触碰（不引入新失败模式）；采纳结算链 DecideRound(adopted_by_continuation)→EventTaskSettled（清 pending interaction）→Settle→loop completed→audition 快照 adoption_status（AB-JUDGMENT-CARD-1 联动位）→CompleteGoal；封闭模板 summary 无域内容。Part B 只重写 FreeStateTerminalFallback/GateRejected 两停因+park 存活的投影，真执行失败仍诚实失败。
  3. **诚实边界核可**：receipt 级 disposition=adopted_by_continuation+human_ab=skipped_by_continuation；采纳 receipt 禁 claim human_confirmed 与 net_outcome=improved（有专钉）；trajectory EvaluationAdoptedByContinuation 入 Valid 白名单（旧值兼容）。**人耳判断与继续采纳在证据链上严格可分**——论文口径边界（取证报告 §5）达标。
  4. **我方独立复跑**：go build + go test ./... -count=1 于分支态——**87 包 0 FAIL**（与回执两轮一致）。
  5. **烟测工件亲读**：Run1 红=真实缺陷两处（探针断言面+trajectory 校验门缺枚举），补锚修复后重跑=§8 纪律正确执行（非原样重跑）；**Run2 绿 exit 0**——park 同形态前置校验（fs7/waiting_continue/judgment_park=true）+五族断言（无 turn.failed/全新 goal-run id/回复落盘/采纳结算落 task 语义+事件流且零伪造人耳判断/revision 不动）；红轮=M8 野外取证（seq47）构成红绿对照；二进制 sha256 per run 记录+-RestartAgent 安装=known-issues #28 纪律达标。
  6. **泊位声明合规**：栈已拆除未移交（netstat 复核 7878/5555 无监听）。
- **新发现（独立复跑暴露）→ 开卡**：TestMixboardSnapshotConcurrentDualWriteNoUnannotatedFork（internal/harness，5s 截止超时）在我方全量复跑中失败——**同族第二次出现**（09-30 VITNOTE-IMPL-4 ruling 首录，条款"重复出现即开修复卡"触发）：隔离复跑 3/3 绿+整包复跑绿+与本卡 diff 零文件交集+失败类型同首录（5s 超时）→ flake 归因成立、与本卡无关联；开 FIX-MIXBOARD-FLAKE-2（P3）。
- 端测覆盖边界（AGENTS §5）：agent HTTP/事件面+内核栈覆盖；webui 渲染面归 AB-JUDGMENT-CARD-1（其卡面已收裁定 2：新输入后卡 settle 为默认采纳终态=本卡 audition 快照 adoption_status 已供给）与 WEBUI-MSG-ORDER-1（已另卡 pass）。
- 联动解锁：**M1 复验第三轮前置齐**（FS-PARK+WEBUI 双修复合入 main f2b51443 态）——按 gate 09-30 排期可与 M8 手测同场。
- worktree 清理：D:/Vit_DAW_worktrees/fs-park-turnfail-1（工件已提取）随本验收移除；webui-msg-order-1/l1-4-design-1/l14d-claimrebase 一并清理（port 分支留远端作审计轨迹）。
