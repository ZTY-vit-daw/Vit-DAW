# FIX-C2-TIMEFX-VIEWS-1：c2VerificationViews 补 reverb/delay 验证视图 case

- 发卡：GLM 主管决策侧 / 2026-10-13 晚（依据=FIX-TIMEFX-AXISFILTER-1 执行侧域外上交，[2026-10-13 裁定 §三](../../rulings/2026-10-13-FIX-TIMEFX-AXISFILTER-1-pass.md)采纳）
- 派发确认：已确认（主管依已核实的上交锚点发卡：`agent/internal/chat/c2_dynamic_control_runtime.go` c2VerificationViews :903 default 返回 nil）
- 验收负责人：GLM 主管决策流
- 池序 107；目标仓库=D:\Vit_DAW
- 优先级 / 预估 / 依赖：P2 / 0.5 天 / 无（FIX-106 已合入 main 2c241ca3）
- 模型分级：L1 / GLM 或 flash 执行会话

## 背景与缺陷画像

TIMEFX v2（reverb/delay）已注册为受治理动态族（registry/CoverageProofs/v2 认证/白名单全在，FIX-106 已收 D1 链上七处消费面漏接）。C2 能力会话面（capability_runtime_v1 动态控制）的 `c2VerificationViews(family)` 是"每族用什么观察视图验证干预效果"的对照表：compressor→track.time_dynamics、limiter→track.peak_structure、gate→activity_structure/time_dynamics、de-esser→frequency_time_events、transient→transient_structure、multiband→band_dynamics——**唯独缺 reverb/delay**。若 NL 请求经 C2 路径控制混响/延迟，验证步拿不到视图（nil）→C2 在受治理边界停下（fail-safe 停，不误写；与 D1 链不同路径，故 FIX-106 两轮复现未触发）。

## 任务

1. **取证前置（视图选型）**：对照 CCB view catalog 与既有六族选型语义，裁定 reverb/delay 各自应返回的观察视图（候选方向：时域效果的可感验证面——如 FXM A/B 变换观察、wet/dry 相关性类视图；**选型须有依据注记**，不得随手占位）。
2. **补 case + 测试**：c2VerificationViews 加两族 case；单测断言两族返回非 nil 且与选型一致；确认 C2 会话面到该函数的可达路径（若当前无可达触发路径，如实登记端测边界）。
3. `go test ./... -count=1` 0 FAIL + gofmt 净（LF blob 口径）。

## 文件域

agent/internal/chat/c2_dynamic_control_runtime.go + 同包新测试文件。

## 验收标准

diff 亲读+选型依据在卷+测试全绿；真栈腿可选（无可达路径时按 §5 显式声明端测边界即可）。

## 停止条件

视图选型需要新观察投影/view catalog 契约变更（超出补 case 范围）→ 带选型方案上交决策侧定契约。

- 领取：2026-10-13 21:27 +0800 / origin/main b0226b659c770a736fdc897bd5817efd5b579715 / GLM-5.3-Flash（PC，ZCode 执行会话） / port/fix-c2-timefx-views-1 / worktree D:/Vit_DAW_wt_c2views1 / 领取提交=本提交（coord/c2views1-claim）。领取基线：HEAD=b0226b65、status 净、diff 空。文件域与 SETTEMPO-101（D:/Vit_DAW_wt_settempo，VitApp/CommandDispatcher.cpp）不相交，并行合规。
- 回执：
  - 实现 commit：c5ef2e60（port/fix-c2-timefx-views-1，基线 b0226b65；diff=runtime +9 行 / test +19 行，2 文件）。
  - 选型依据：reverb/delay → `["track.time_dynamics","track.stereo_space"]`。①六族语义=「视图观察该族干预所作用的声学维度」，且与 registry `ObservationViews` 六族逐一互为镜像；②registry.go:198-201 注释**自认**两族行 `ObservationViews=["mix.multitrack_relationship"]` 为占位（"keeps the historical project relation view — dedicated paired-diff surface is M2-TIMEFX-2 scope"），且该 view 作用域为 project/track_group，与 C2 后验证恒为 `target_kind=track` 的请求形状不符——不镜像占位；③两族 registry CoverageVocabulary 轴恰覆盖两个可感维度：时间域（decay/size/pre_delay/echo_time/sync/feedback）→`track.time_dynamics`（"How does energy evolve over time?"，COM+bounded time-energy summaries——混响尾/回声重复即能量时间演化）；空间域（width/damping/reverb_mode=spatial_mode/ping_pong/delay_mode+wet_balance）→`track.stereo_space`（"How wide or correlated is the target?"，即卡面候选"wet/dry 相关性类"的目录内现有面）；④卡面候选方向"FXM A/B 变换观察"：FXM 投影存在但 CCB 目录无 fxm.\* view（OBSERVATION_PROJECTION_MANIFEST.md CCB 表 ：29、:72 显式"CCB v1 目录不含 FXM view"）——选它=目录契约变更=本卡停止条件，故不选；本选型全用现行目录 view，零契约变更，未触发停止条件。新增 `TestC2VerificationViewsStayInsideTheCCBCatalog` 把「选型 ⊆ CCB catalog」锁为回归守卫（防未来目录漂移使后验证静默 fail-safe）。
  - 端测边界声明：纯单测卡未占真栈。**可达性取证=消费链在、族上游被挡**：消费链=单卡 server.go:4397/4407 →completeC2PostAction→c2PostActionCCBObservation(:883)→c2VerificationViews(:903)，batch 路 c2_dynamic_batch.go:603 同汇此函数；但 reverb/delay 今日**无可达触发路径**——同文件上游两道族门不含两族：`c2SupportedFamilies`(:265)（legacy 发现/续跑校验与 batch 路 vocabulary/options/allowedFamilies 全派生自它）、`c2ProcessorTypeForFamily`(:573)（装载适配，两族返回 ""→unsupported_dynamic_family 边界）。任何 C2 会话无法冻结 reverb/delay 假设走到后验证，新 case 的 E2E 行为不可端测；按卡面任务③如实登记，不再深测。
  - 域外上交（AGENTS §11，待决策侧扩域或开卡）：上列 `c2SupportedFamilies`/`c2ProcessorTypeForFamily` 两道门是 C2 到 reverb/delay 的真正第一/第二 blocker；两处均在卡面文件域（同文件）内、但超出卡面任务①②清单（仅指 c2VerificationViews），未擅自扩域。
  - 文件域注记：卡面写"同包新测试文件"，实际复用既有同包测试 `c2_dynamic_control_runtime_test.go`——该文件即本函数契约测试（`TestC2VerificationViewsMatchTheExecutedFamily` 表驱动）所在，两族 case 入表 + 新守卫测试同文件；另立新文件会割裂同一契约表。
  - 验收门：`go test ./... -count=1` EXIT=0（95 包 ok、0 FAIL，日志 /tmp/c2views_gotest.log）；gofmt 按 LF blob 口径两改动文件 `gofmt -d`=0（工作树 CRLF 系 autocrlf checkout 所致，未触碰文件同被 `gofmt -l` 报出，属已知假阳性，不采信）。
- 验收：（裁定文件 / 验收 commit）
