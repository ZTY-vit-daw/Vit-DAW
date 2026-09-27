# FIX-STALE-SAMPLES-1：E8/E16 两处漏标活样本修复（现状锁定测试升级为修复验证）

- 优先级 / 预估 / 依赖：P1 / 0.4 天 / MAT-C 已入现状锁定测试（E8：rlm mergeSourceRows 字段级覆盖无 revision 比较 rlm/projection.go:170-192；E16：B1 pack 同 run 建后不失效 static_mix_gain_staging_context.go:21-23）；设计裁定"修复单开卡"（MATERIALIZATION §4 rlm 段）
- 模型分级：L2 / GLM 亲自或强督导（新鲜度语义修复，行为变化须钉死）
- **执行侧（mac 会话，夜池）**
- 目标：
  1. **E8 修复**：`mergeSourceRows` 合并加 revision 比较（三源 executed 记忆的新鲜度门——旧 revision 不得覆盖新 revision 的字段级合并）；锁定测试的升级断言启用（MAT-C 测试内已注释升级条件：修复后行为红→绿）。
  2. **E16 修复**：static_mix_gain_staging_context 的 pack 行在同 run 重建后失效（同 run 建后不失效=陈旧 pack 误用）；同样用锁定测试升级验证。
  3. **行为边界钉死**：两处修复都是"拒绝陈旧数据覆盖新数据"方向（宁缺勿旧），不做其他行为扩展；旧记录兼容（§11：无 revision 字段的旧记录=维持现状语义还是视为最旧？卡内裁定并写回执）。
  4. **门**：agent 全量 0 FAIL+两锁定测试升级后绿。
  5. 回执：红绿+diff stat+§11 裁定说明+HEAD。
- 约束：文件域 rlm+staticbalance（两文件族）；行为变化范围最小（只加新鲜度门）；提交显式列文件。
- 验收：两测试升级绿+全量 0 FAIL+§11 裁定在案
- 停止条件：revision 语义与消费方冲突（修复导致现役行为回退）→ 冲突实证上交
- 领取：2026-09-27 21:05（mac）/ origin/main=b9b8a702a4a1b6552e27cb5faf60372f482fa5d2 / 分支 port/fix-stale-samples-1（独立 worktree，主工作树另有并行流）
- 回执：
  - **实现 commit**：port/fix-stale-samples-1 @ `4362e83587b743297b6f1afabbbfe3fdd59b9c6b`（已推 origin）；diff stat：4 文件 +300/-70——`agent/internal/rlm/projection.go`（+124 核心）、`agent/internal/rlm/projection_test.go`、`agent/internal/agentloop/static_mix_gain_staging_context.go`、`agent/internal/agentloop/materialize_red_test.go`（两锁定测试升级）。领取时 HEAD 与 origin/main 一致，无他人 diff 混入。
  - **红→绿**：
    - E8 红（修复前，仅升级断言）：`go test ./internal/rlm -run TestRLMThreeSourceMergeRevisionMismatchIsVisible` FAIL@断言② `TrackFaderDB=0x14000098430（期望工程态 -6.0 不被旧观察覆盖）`——复现旧观察 -3.0 静默覆盖工程态 -6.0；修复后 PASS（TrackFaderDB=-6.0、RMSDBFS=-18.0 legacy 档维持、Limitations 带 `rlm_stale_source_fields_withheld`）。
    - E16 红（修复前，仅升级断言）：`go test ./internal/agentloop -run TestB1PackSurvivesProjectChangeInRun` FAIL `现状为静默短路（stopped=false status="" trace 1→1）`——复现 r1→r2 变更不触发失效；修复后 PASS（进入重建路径）。
    - 新增钉死单测：rlm `TestMergeSourceRowsRevisionGate`（异 revision 拒绝覆盖+补缺放行 / §11 无 revision 源只补缺 / 双 legacy 后写胜）与 agentloop `TestB1PackStalenessRevisionSemantics`（同 revision 不失效 / 异 revision 失效 / 双无 revision 维持现状 / pack 无注记而工程态出现 revision 失效），全绿。
  - **门**：`agent/` 下 `go test ./... -count=1` 退出码 **0**，86 包 ok + 13 无测试文件，零 FAIL 零 panic（本机日志 /tmp/fss1_fulltest.log；mac go1.24.1 CGO_ENABLED=1）。
  - **§11 裁定（卡内授权，行为边界钉死）**：
    - E8（mergeSourceRows）：revision 比较采用**相等才可覆盖**的不等式门（不做序关系推断——hash 型 revision 无序可推，对齐仓内 `stale_for_current_revision` 的 `!=` 惯例，task_runtime_trajectory.go:69）；**无 revision 字段的源对"带 revision 的既有字段"视为不可证明新鲜（视为最旧方向的保守化）——只补缺不覆盖**；两个无 revision 源之间维持现状（后写胜）。拒绝覆盖在投影 Limitations 带 `rlm_stale_source_fields_withheld` 可见性（不暴露具体被拒值）。
    - E16（B1 pack）：pack 建立时把可见的 project_revision 编入 trace 标记（`... default pack built project_revision=<rev>`）；当前工程 revision 与标记不等 ⇒ pack 失效重建。**pack 与当前工程态双端都无 revision 可见 ⇒ 维持现状不失效**；pack 无注记而工程态出现 revision ⇒ 视为可证明变化（失效，一次重建后收敛）。
  - **端测覆盖边界声明（AGENTS §5 渲染面/旅程门槛）**：改动为 agent 内部投影合并语义与 B1 preflight 门逻辑，不涉 webui 渲染面与用户旅程 UI；本卡按卡面门以单测升级+全量验证收口，真栈烟测未执行——是否需补由决策侧裁定。
  - 复验 worktree：`/Users/timozty/Documents/Vit-DAW-fss1`（验收后由决策侧清理，PROTOCOL §3）。
