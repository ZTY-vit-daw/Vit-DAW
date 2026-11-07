# MIXBOARD-TRACKKEY-1：mix.read 逐轨观察键路由修（274 相邻勘察②转卡）

- 发卡：GLM 主管决策流 / 2026-11-07 早窗（依据=[274 回执相邻勘察②](../done/2026-11-07-D1D3-DIGEST-WIRING-1.md)+[274 裁定](../../rulings/2026-11-07-D1D3-DIGEST-WIRING-1-pass.md)④——逐轨 stereo mix.read 7/7 missing vs feature snapshot 19 行 ready：`readObservationKey` 取观察包自身 TargetRef.ID（full_project 观察=`"current"`，agent/internal/mixboard/catalog.go:469/:570），d1d3 请求携带的真实 track id 键恒走 default→`catalog_key_not_found`；feature snapshot 的 `stereo_relation_summaries` 在 mix.read 键面无消费路径=第二处消费错位）
- 派发确认：已确认（主管发卡）；验收负责人：GLM 主管决策流
- 池序 292；目标仓库=主仓 agent（mixboard 键路由）；P1 / 0.5 天 / 依赖=274 已验收合入（digest 视图在 main，逐轨面回归基准就位）；L2 / PC；零栈先行（真栈回归门搭后续 run）

## 目标

1. **键路由修**：track-keyed 观察读取对 full_project 观察包的正确解析（TargetRef 语义路由——track id 请求应命中 full_project 观察内的对应 track 面，而非按观察包自身 ref 键恒 miss）；`stereo_relation_summaries` 的 mix.read 消费路径打通或如实记录不可消费边界；
2. 单测反钉：265 实盘形状夹具（真实 track id 键→ready 读数 7/7；修复前恒 missing 的负锚）；回归 `go test ./internal/mixboard ./internal/mom ./internal/chat -count=1` 全绿+build 0；
3. 勘察顺带：catalog 键面是否存在同型第三处消费错位（只勘察，发现即记录上交不扩修）。

## 约束

- 文件域=agent/internal/mixboard（catalog.go 键路由±同包消费面）+测试；不触 chat/d1d3 消费面（274 已收口）、不触在飞卡域（273/275/277/281）；
- 观察包铸造面（TargetRef 语义本体）零改动——若修复必须改铸造面→跨域上交转裁定；
- 零真栈：真栈回归门=265 D2 门断言搭后续 run。

## 停止条件

修复发现 full_project/track 观察的键语义存在在案裁定约束→锚点取证上交；断言失败取证不重跑。

## 领取/回执/验收

- 领取：2026-11-07 早窗 / origin/main=acc17453f45c9ba1da17eb1043d2b3d307017823 / owner=GLM 主会话 L2-worker 子代理（ZCode/PC）/ 分支=port/mixboard-trackkey-1 / worktree=D:/Vit_DAW_wt_292impl / 领取基线：HEAD=acc17453（worktree 自 origin/main 新建）、status --short 空、diff --stat 空（零领取前 diff）；在飞卡文件域核对：293=coord/runs 取证零源码、273=harness/chat（已停候栈）、275/281=py+edu+server、277=capability_pack+catalog 消费面（与 mixboard 键路由不相交，卡面红线条目在案）
- 回执：（执行侧回填）
- 验收：（主管回填）
