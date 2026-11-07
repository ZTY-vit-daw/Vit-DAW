# DELAY-APPLY-MS-FIX-1：apply_delay_controls 毫秒反演修（300 S0 取证转卡）

- 发卡：GLM 主管决策流 / 2026-11-07 晚窗（依据=[300 回执域外取证②](../done/2026-11-07-PLUGIN-CONTROL-EVAL-RUNNER-1.md)——`apply_delay_controls` value_ms=250 实落 norm 0.35065=**352ms 却报 apply_status=exact**；锚点 agent/internal/chat/plugin_compressor_apply.go:175-262+plugin_delay_apply.go:263-283；工件=300 S0 run4/5 T4 write_faces。注意：P 轮 T4 模型走 typed 路径落 249ms 正确，与该缺陷并存——缺陷在另一写面函数，非产品主路径）
- 派发确认：已确认（主管发卡）；验收负责人：GLM 主管决策流
- 池序 304；目标仓库=主仓 agent（delay apply 面）；P2 / 0.5 天 / 依赖=300 工件在库（write_faces 为夹具）；L2 / PC；零栈

## 目标

1. **反演修**：ms→normalized 换算链修（对 250ms 类目标落点正确； suspected：norm→ms 正演与反演不对称或 domain 形态误读——以 300 工件实盘蓝本钉真实机理再修）；
2. 配对单测：250ms→落点 249-251ms±宽容+apply_status 诚实（exact 仅在真精确时）+修复前 352ms 形态负锚；
3. 回归：`go test ./internal/chat -count=1` 全绿+build 0。

## 约束

- 文件域=plugin_delay_apply.go±plugin_compressor_apply.go 同链±测试；不触 typed 主路径/在飞卡域；
- 禁止 git reset --hard / git clean；零真栈。

## 停止条件

发现换算链存在在案裁定约束→锚点取证上交。

## 领取/回执/验收

- 领取：2026-11-07 21:08 +0800 / origin/main=`9bd1ce90f7dc0be3bf6407816699aaa128668565`（含本批 302 领取提交） / owner=ZCode GLM-5.3 L2-worker PC 执行子代理（会话 302/304 同批 runner；与 302 VSTI-WRITEGUARD-1 主管合并派发，实现合并进同一 port 分支 `port/vsti-writeguard-1`） / worktree=`D:/Vit_DAW_wt_302wg` / 领取基线：实现基线=origin/main=`db9bd088`（port 分支起点，worktree 全净 status/diff 均空）；协调面自 302 领取后的 main 连续建立。前置核对：doing 卡文件域（genapi/harness 授权/chat 路由/canary 钩+302 守卫段）与本卡（plugin_delay_apply.go+plugin_compressor_apply.go 同链±测试）不相交。
- 回执（2026-11-07 晚窗，执行侧；与 302 同会话接手续做，实现随 `port/vsti-writeguard-1` 同分支）：
  - **diff**（impl commit `03fd0fd7` @ `port/vsti-writeguard-1`，已推远端，基于 302 的 `df12a3fe` 之上）：`agent/internal/chat/plugin_compressor_apply.go`（+94：`compressorCurveFitTracksSamples` + `compressorCurveFitTrustSpanShare=5%` + `compressorCurveFitResiduals` + `compressorCurveSegmentInverse` + `compressorPhysicalTarget` curve 分支拟合质量门控回退）+ `agent/internal/chat/plugin_delay_apply.go`（apply 结果循环改 `delayWriteLandingStatus` 物理回读核对 + `delayPhysicalTolerance`）+ 测试 `plugin_delay_apply_ms_test.go`。typed 主路径（goalrunner 接管面）与 EQ 反演面（`plugin_eq_control.go`）零改动。
  - **机理**（以 300 s0 run4/5 T4 write_faces 蓝本数值钉死，suspected 两条均非——是全局幂拟合反演失真+物理显示无核对）：`apply_delay_controls value_ms=250` → `compressorPhysicalTarget` curve 分支 → `eqNormalizedFromCurve` 对 H-Delay Delay Sec 实测样 1/251/501/1540/3500ms 做全局幂拟合（指数 2.5218、span 相对残差 ~11%）并按幂模型反演 250ms → norm 0.35065 → 内核显示 352ms；而 delay 角色（time/sync）不在 `isEQPhysicalRole`，事务机只验 normalized 落点（1e-4 过）不验物理显示 → 谎报 `apply_status=exact`。修=双层：①反演——拟合最优残差 >5% span 时改实测折线分段线性反演（样点精确通过，250ms→norm≈0.249；平滑 log/线性面拟合优、路径不变）；②诚实——apply 结果循环按内核 `value_text` 物理回读核对（time/sync 容差=release 族 max(0.05, 1%)），超差降级 `status=approximate`；quantized 优先级与无物理声明写 exact 语义不变。
  - **测试命令+退出码**：`go test ./internal/chat/ -run "TestCompressorPhysicalTargetHDelayCurveInversion|TestApplyDelayControlsMsTargetLandsWithinTolerance|TestApplyDelayControlsHonestStatusOnPhysicalMiss|TestDelayWriteLandingStatusPrecedence" -count=1` → **exit 0**；回归 `go test ./internal/chat -count=1` → **exit 0**（ok 114.98s）；`go build ./...` → **exit 0**。
  - **负锚复放**：`eqNormalizedFromCurve(250, 实测样)` 复钉 ∈[0.3500,0.3515]（=352ms 修复前缺陷形态）+ `compressorCurveFitTracksSamples` 钉不信任（11%>5%）+ `compressorPhysicalTarget` 修复后落 0.248-0.250 + apply 级 250ms→display 249ms（248.5-251.5 窗）status=exact；诚实面=几何真值 fake（样点间指数内插、样点本身不变，模拟厂商曲线偏离折线）：落点 ~245ms → per-control/overall 均 `approximate`（事务仍过、normalized 落点精确）；`352 ms` 回读对 250ms 请求的单测直钉 approximate。
  - **端测边界声明（零栈）**：纯单测+编译，零真栈（卡面约定）；真栈 H-Delay 250ms 实盘落点验证挂后续栈 run 顺带。
- （主管回填）
