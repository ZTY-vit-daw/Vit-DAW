# VSTI-WRITEGUARD-1：agent 通用写面 de-esser 守卫对 VSTi 的误拦分类修（298 转卡）

- 发卡：GLM 主管决策流 / 2026-11-07 晚窗（依据=[298 回执确定性发现](../done/2026-11-07-VSTI-PARAM-PROBE-1.md)——agent 通用写面 `set_plugin_param` 对 Grand Rhapsody 被 de-esser 守卫 fail-closed 拒绝（插件级 `unresolved_de_esser_surface`→默认分支拦截，goalrunner_chat.go:1425-1455），Surge XT 同面通过；内核 ZMQ 面两件全绿（apply→readback→restore 零 mismatch）——缺陷仅在 agent 通用写面的守卫分类）
- 派发确认：已确认（主管发卡）；验收负责人：GLM 主管决策流
- 池序 302；目标仓库=主仓 agent（goalrunner_chat.go 守卫分类面）；P2 / 0.5 天 / 依赖=298 工件在库（probe1_20261107_1920 参数面+拒绝回执为夹具）；L2 / PC；零栈先行，真栈回归搭后续 run

## 目标

1. **守卫分类修**：de-esser 守卫默认分支对 instrument 族插件的正确分类（不落 `unresolved_de_esser_surface`——按 298 采证的 template_role=instrument 通道或插件族判据分流；非 VSTi 插件行为零变化）；
   **【2026-11-07 晚窗主管扩域】+300 S0 取证同族面一并修**：typed 守卫对 H-Delay/Plates 类的 `unresolved_gate_expander_surface`/`typed_limiter_surface_unavailable` 误拒（goalrunner_chat.go:1302-1340/1356-1394，300 s0 run2 T3+run5/6 T4 forensics 为负锚）——同分类缺陷族，同一张卡收口；
2. 配对单测：instrument 写面通过+效果器原守卫行为不变+未知族维持 fail-closed 三方向钉死（298 工件拒绝回执为负锚蓝本）；
3. 回归：`go test ./internal/chat -count=1` 全绿+build 0；真栈回归腿（Grand Rhapsody agent 面写通+快照恢复）搭 296-R2 或后续栈 run 顺带，不单占。

## 约束

- 文件域=goalrunner_chat.go 守卫分类分支±测试；不触 PCA 门/装载链/内核；**非 instrument 插件的守卫语义零改动红线**；
- 禁止 git reset --hard / git clean；不动主工作树；零真栈（本腿）。

## 停止条件

发现守卫分类存在在案裁定约束（如 de-esser 面曾有明确 fail-closed 裁定）→锚点取证上交转裁定。

## 领取/回执/验收

- 领取：2026-11-07 21:07 +0800 / origin/main=`db9bd088fecaf3959065cb22b864b8eaf58895e4` / owner=ZCode GLM-5.3 L2-worker PC 执行子代理（会话 302/304 同批 runner；与 304 DELAY-APPLY-MS-FIX-1 主管合并派发，实现合并进同一 port 分支） / 分支=`port/vsti-writeguard-1` / worktree=`D:/Vit_DAW_wt_302wg` / 领取基线：HEAD=`db9bd088`（fresh worktree 自 origin/main），`git status --short`=空、`git diff --stat`=空（全净）。前置核对：doing 四卡（GEN-YUE-DIALECT=genapi 域、ARRLOOP-AUTHBRIDGE=harness 授权面、CLOSURE-ROUTE-ADAPT=chat 路由/closure 预算面、B3-ABLATION=canary 钩）文件域与本卡（goalrunner_chat.go 守卫分类段 1220-1460±测试）不相交；305 仍在 todo/（其 typed 接管面=goalrunner_chat.go:260-307，与本卡守卫面不同段，push 冲突按 §2.1.3 重放）。
- 回执（2026-11-07 晚窗，执行侧。**接手注明**：前代理 21:17 限额定格于领取后零实现（worktree 基线 db9bd088 全净、无半成品 diff、无 PROGRESS 留档），本会话续占同 worktree/分支完成）：
  - **diff**（impl commit `df12a3fe` @ `port/vsti-writeguard-1`，已推远端）：`agent/internal/chat/goalrunner_chat.go`（+61：`genericWriteGuardFamilyExemption` helper + de-esser/gate-expander/limiter 三守卫 default 分支仅在 unresolved 码上单点分流）+ 测试两件（`plugin_writeguard_family_test.go` 配对单测 + `writeguard_family_fixtures_test.go` 298/300 工件真实面复放夹具）。守卫干净码路径（not_*/unsupported_*）、owned 拦截（typed_*_control_required）、transient-shaper/compressor 守卫零改动。
  - **机理**：三守卫对 `readLive*ControlSurface` 的 `unresolved_*_surface` 码一律硬拦（cannot prove outside → fail-closed）。取证钉死该码在两类非动力学面误伤：①音源（298：Grand Rhapsody 被 de-esser 守卫拒、Surge XT 同面过）——kernel `template_role=instrument` 面；②时序效果器（300 s0：H-Delay 被 gate 守卫拒 run2/6 T4、Plates 被 limiter 守卫拒 run1 T3）——timing/output 命名重叠触发 dynamics 候选但 delay/reverb 识别器可证拓扑。修=unresolved 码上新增族豁免通道（instrument role 或可证 delay/reverb 拓扑 → 放行通用写）；未知族、读失败、其余默认码维持原拦（fail-closed 红线保持）。
  - **测试命令+退出码**：`go test ./internal/chat/ -run "TestGenericWriteGuard" -count=1 -v` → **exit 0**（新增 5 件+既有 3 件守卫测试全绿）；回归 `go test ./internal/chat -count=1` → **exit 0**（ok 114.98s）；`go build ./...` → **exit 0**。
  - **负锚复放**：夹具=298 `grand_rhapsody_parameters`（41 参+template_role=instrument）+ 300 s0 `T4_before`/`T3_before` 真实面（注：捕获件缺 `host_controllable` 键，缺失时候选全灭得 not_*——已按内核实报 true 补真并在夹具头注记）。测试先复钉三面分别产出 `unresolved_de_esser/gate_expander/limiter_surface`（复现 298 拒绝与 300 s0 拒绝的原分类形态），再钉修复后三守卫放行；未知族夹具（Plates 去 Plate Selector/Damper/Predelay 时序证明参数）三守卫全拦 `typed_*_surface_unavailable` 原样+读失败拦。
  - **端测边界声明（零栈）**：本腿纯单测+编译，零真栈（卡面约定）；真栈回归腿（Grand Rhapsody agent 面写通+快照恢复）按卡面挂 296-R2 或后续栈 run 顺带，不单占。
- （主管回填）
- 验收（2026-11-07 深夜，主管）：**pass**——单点族豁免方案合格（unresolved 码上按 instrument role/可证拓扑放行+未知族维持 fail-closed=治理语义零放松）；三件真实面复放夹具（298+300 工件蓝本）+夹具坑（host_controllable 缺失补真）处置正确；主管交叉复跑定向+chat 全包 118.4s 绿。实现 df12a3fe 合入 main。真栈回归腿挂后续栈 run 顺带。L1 台账 48h 窗口。
