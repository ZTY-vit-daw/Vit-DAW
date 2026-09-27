# MAT-D4 取证工件：timing-carried 登记落地+真栈分轮断言就位；收敛轮仍 timing 涨——停止条件触发，取证上交

- 执行卡：coord/cards/todo/2026-09-29-MAT-D4.md
- 日期：2026-09-27
- 结论状态：**实现面完成并经真栈验证（变更轮 timing 登记✓）；收敛轮在静默窗后
  仍 timing 涨（两轮独立复现、hash 对恒定）——卡面停止条件触发，G2-D 未闭环，
  根因证据上交决策侧。**

## 一、本卡已完成并验证的部分（实现面闭环）

1. **物化侧 timing-carried 登记**（与 measurement-carried 同款：排除+单列计数，不静默）：
   - `materialize/timing_carried.go`：`timingCarriedSignature`——对"会判分歧"的 dom 行用
     当前 deps 同路径重放（`BuildDOMRowForTrack`，与 `DOMAdapter.Build` 严格同链）：
     重放==物化行 而 观察行异 ⇒ 两侧输入世代差 ⇒ 排除+计数
     `Metrics.ShadowTimingCarriedExcluded`；重放缺行/不符 ⇒ 判定不可靠 ⇒ 宁计分歧
     不误排除（宁少勿滥）。
   - `materialize/recompute.go`：`ReconcileShadowWithTiming(fresh, deps)`（旧
     `ReconcileShadow`/`ReconcileShadowDetailed` 委托新实现，行为不变）；
     `ShadowReconcileResult.TimingDetails` 携带被排除行两侧 hash（红必须可解释）。
   - `materialize/metrics.go`：`ShadowTimingCarriedExcluded` 单列计数。
   - `harness/materialize_shadow.go`：影子轮切换 `ReconcileShadowWithTiming`；timing
     排除打 INFO（含 fresh/stored hash）；shadow_round metrics 行加
     `timing_carried_excluded=%d`。
2. **S2 烟测分轮改造**（`scripts/materialize_shadow_smoke.ps1`）：
   - 变更轮（set_volume 后观察）：断言 `timing_carried_excluded` delta ≥1（分歧登记不静默）；
   - 静默收敛窗：≥8s 地板 + `[materialize]` 事件面行数稳定（3 次 2s 轮询；活栈有
     authority/chat 周期日志，总行数永不稳定——判据限定物化事件面），预算 40s；
   - 收敛轮（第三轮观察）：断言 新分歧==0 且 新 timing 排除==0 且 reconcile_rows
     delta≥1（非空洞）——排除计数不涨才叫收敛，防 timing 排除把热窗口静默吞掉。
   - 基线非空洞改为"dom 行到达对账闸门（timing/measurement/reconcile 任一面>0）"
     （恒排除形态下 reconcile_rows 恒 0，单独要求它结构性不可能）。
3. **门**：`go build ./...` + `go test ./...` 全量 0 FAIL（86 包 ok）；红先行
   `timing_carried_test.go`（4 用例：世代差排除/不可靠计分歧/收敛零分歧/stored-missing
   计分歧）+ harness 级 `TestMaterializeShadowTimingCarriedRound`（时序两态→登记；
   同世代收敛轮→零分歧+排除不涨+真比过）。

## 二、真栈运行记录（MAT-D4-1）

| run | 栈 | 静默窗 | 基线 timing | 变更轮 timing（delta） | 收敛轮 timing（delta） | 未分类分歧 | 结果 |
|---|---|---|---|---|---|---|---|
| 20260927_210524 | 全新起栈 | 40s 预算耗尽（总行数判据，后修） | 1 | 2（+1 ✅） | 3（+1 ❌） | 0 | FAIL（S2c） |
| 20260927_210904 | -ReuseStack | — | — | — | — | — | 环境（S1：活栈周期日志把 mode=shadow 启动行冲出 800 行环形缓冲；复用形态限制，非功能失败） |
| 20260927_210942 | 拆栈后全新起栈 | 8s 稳定达标 ✅ | 1 | 2（+1 ✅） | 3（+1 ❌） | 0 | FAIL（S2c） |

两次有效运行（run2 静默窗真稳定）同一签名：**每轮 timing 排除恰好 +1，未分类分歧恒 0，
hash 对轮内恒定**：

- run1（21:05:38→21:06:23，跨 45s）：fresh=sha256:dfb873ae7b8602a6 /
  stored=sha256:e30e995aca62cfba，三轮不变；
- run2（21:09:57→21:10:11）：fresh=sha256:d213b3d7610e4037 /
  stored=sha256:020ea6f6650887fa，三轮不变。

## 三、停止条件的证据链（收敛轮为何必红）

收敛轮（run2，静默窗 8s 稳定后）关键读数：

1. `dom:rec` 停在 2（收敛轮未重算——kind 干净，无新失效）；
2. timing 判定成立（重放==物化行）⇒ `build(deps_now) == build(deps_变更轮)` ⇒
   **deps 侧 feature snapshot 在整个窗口内零变化**；
3. fresh hash 三轮恒定 ⇒ 观察侧投影同样确定性、其输入同样稳定；
4. fresh≠stored 每轮成立 ⇒ **两侧输入在同一墙钟时刻就是不同的视图**——不是遥测
   时序窗（时序窗在静默后应闭合，45s/8s 两档都试过），是**结构性的输入面差异**。

机械定位（接 MAT-D3 取证 §3.3/3.4）：观察侧 finalize 消费的 feature snapshot 与
尾挂 `obs.GlobalSummary["feature_snapshot"]`（deps 消费的同一 map）不同源——
候选机制仍是 compactFeatureSnapshot 浅引用穿透/紧凑化丢键（time_segments 等，
MAT-D3 §3.2 实测尾挂行 time_segments 缺失、keys=30 vs 44）或观察链 finalize 后续
步骤原地改写。**物化侧内部自洽（stored==replay 恒成立），无漏标证据——G2 的失效
模式未被触发；红的是观察侧与物化侧的输入奇偶性，属 OBS-IMMUTABILITY-RECON-1
（共享引用穿透）域。**

判定语义说明：timing 签名（物化行==当前输入重放 而观察行异）在本证据下排除的是
"观察侧旧世代采样"与"观察侧异源视图"两种形态——对 G2（漏标检测）两者都不是物化
侧失效，排除语义正确；但收敛断言（timing 不涨）要求输入面真正齐偶，结构性差异下
永远红——正确行为（不静默吞掉热窗口）。

## 四、G2-D 状态声明

**G2-D 未闭环（停止条件触发，如实上交）。** 本卡交付：

- timing-carried 登记全链路（物化侧判定+计数+日志+烟测分轮断言），变更轮登记断言
  两轮真栈 PASS（登记机制真实工作）；
- 收敛轮红的根因证据：**世代差窗口与遥测静默无关，是观察链投影输入与物化
  DepInputs 的结构性视图差异（每轮恒定 hash 对）**——比 MAT-D3"时序两态"假设更强
  的否定证据：静默窗（8s 稳定/40s 预算两档）闭合不了它。

交决策侧裁定的点：G2-D 收敛态闭环的前置是否归 OBS-IMMUTABILITY-RECON-1（输入面
奇偶/穿透修复）——修复方向候选已在 MAT-D3 取证 §四（A：finalize 时深拷贝快照；
B：对账基准换尾挂态重算；C：DepInputs 扩轴——本卡证据再次否定 C：扩轴解决不了
"观察投影输入≠尾挂输入"的结构差）。

## 五、复现命令

```powershell
powershell -NoProfile -ExecutionPolicy Bypass -File .\scripts\materialize_shadow_smoke.ps1 -RepoRoot D:\Vit_DAW
```

（run2 的 agent 二进制=本卡工作树构建；kernel=pcverify1 增量构建，sha256 见
run_report.json。）
