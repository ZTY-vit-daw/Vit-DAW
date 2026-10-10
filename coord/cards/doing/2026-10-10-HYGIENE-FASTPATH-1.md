# HYGIENE-FASTPATH-1：fastpath 三副本漂移守卫 + newFastPathRouter 轮中 panic 时机清偿（L1-5-IMPL-C 两挂账）

- 发卡：GLM 主管决策侧（/morning 会话）/ 2026-10-10
- 派发确认：已确认（用户 2026-10-10 /morning 裁定标准日+机会面清理（gate 建议序②））
- 验收负责人：GLM 主管决策流
- 池序 38（来源=[L1-5-IMPL-C pass ruling](../../rulings/2026-10-09-L1-5-IMPL-C-pass.md) 挂账①②）；目标仓库=D:\Vit_DAW
- 优先级 / 预估 / 依赖：P2 / 1h / 无（main a28eb1b0）
- 模型分级：L1 / **flash 可接**（测试为主+单点小改；先例=IMPL-A 复核腿机械验证）

## 挂账①：三副本机械等价守卫

- 现状：`internal/fastpath/gain_staging_ref.go` 含三个**逐字副本**声明（卡内注释自证）——`GainStagingExplicitRequest`（原件 `agentloop/static_mix_gain_staging_context.go:1746-1768`）、`StaticBalanceIntentReference`（:1770-1775）、`GainStagingStrictReferenceIntent`（:1777-1798）；原件原地保留（agentloop 继续消费原件），副本仅供 fastpath 平移函数依赖。**无任何机械守卫**：原件改动后副本静默漂移。
- 目标：新增测试（建议 `internal/fastpath/gain_staging_ref_drift_test.go`），测试期逐字对照断言——
  1. **按声明名提取，不按行号**（行号锚点是 IMPL-C 时点快照，原件任何插入都会漂移）：从两文件源码中按类型/函数名定位声明体（go/parser 或行扫描均可，须声明提取口径）。
  2. 归一化后逐字节比对：归一仅限 IMPL-C 登记的引用改名+空白，其余任何差异=失败。
  3. 失败消息须打印双方差异定位（fail-visible，对齐 IMPL-C 完整性契约风格）。

## 挂账②：newFastPathRouter panic 构造时机

- 现状（2026-10-10 实读锚点，领取时回查）：`internal/agentloop/message_loop.go:430` `newFastPathRouter()`；:443-445 `slices.Equal(names, fastpath.DefaultEntryNames)` 不等即 `panic`；**消费点 `loop():459`**——每次 message loop 运行都构造，即 **panic 落在轮中路径**。L1-5-IMPL-C ruling 挂账②明文：启动期 panic 可接受，**轮中 panic 不可**。
- 目标：漂移守卫语义从"进程崩溃"改为"显式 run 失败"——`newFastPathRouter` 改返回 `(*Router, error)`（或 loop() 内联判定），漂移时走 `r.fail(state, …)` + trace 事件（建议 Kind 沿用 final_gate 或新增 fastpath_drift，措辞保留"注册面漂移"关键词与词条清单）；`fastpath.DefaultEntryNames` 完整性契约与注册序契约**零变化**（fail-visible 不弱化为日志静默）。
- 测试：构造漂移 router（错误词条序/缺项）断言 run 失败路径返回显式错误而非 panic（`recover` 探针或错误返回值断言）。

## 边界与红线

- 文件域：`internal/fastpath/gain_staging_ref.go`（只加注释如需）+ 新测试文件；`internal/agentloop/message_loop.go` + 新增/扩展测试。**不碰** `router.go` 注册逻辑、不碰任何 preflight handler 行为、fastpath 包其余文件零改动。
- 行为红线：正常路径（注册面无漂移）行为零变化——既有 agentloop/fastpath 全部测试零改动全绿是验收面；panic→fail 改动仅激活漂移分支。

## 验收标准

- `go build ./...` exit 0；`go test ./internal/fastpath ./internal/agentloop -count=1` 全绿；全量 `go test ./... -count=1` 0 FAIL；触碰文件 blob 级 gofmt 净。
- 回执附：副本对照测试的三组声明提取锚点（文件:行）+ panic→fail 改动 diff 位置 + 漂移分支测试名。

## 停止条件

- 副本与原件已发生漂移（对照测试写不绿）→ 停止上报漂移清单（等价性已被破坏，修复归主管裁定，不在本卡私改原件或副本）。
- `loop():459` 消费时机与上述实读不符（如构造已移出轮路径）→ 锚点+形态上交。

## 并行与资源

- 并行域 A；与 HYGIENE-GOFMT-1 / REFSCHEMA-M4/M5 文件域不相交（已核：本卡文件均不在 gofmt 债 51 清单、不在 capabilitycontext/chat 回执族）。不需要真栈（纯单测域）；无资源占用。

- 领取：2026-10-10 10:16（Asia/Shanghai） / origin/main=204f0cc8 / owner=GLM-5.3-flash 执行会话（PC / ZCode flash / HYGIENE-FASTPATH-1 实现） / 分支 port/hygiene-fastpath-1 / worktree D:/Vit_DAW_wt_hygiene_fp1 / 领取提交=本条状态提交（coord/hygiene-fastpath-1-claim，推 main 后以远端 log 核对）；独立 worktree 领取前 status/diff 均为空
- 回执：（commit hash / 报告链接 / 端测边界声明——本卡纯单测域，真栈覆盖=无（行为零变化面），如实声明）
- 验收：（裁定文件 / 验收 commit）
