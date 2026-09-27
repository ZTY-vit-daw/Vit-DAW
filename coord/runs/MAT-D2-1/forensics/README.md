# MAT-D2 取证工件：S2 分歧真实根因=适配器合成包缺 TargetRef.Label（卡面前提被推翻）

- 执行卡：coord/cards/todo/2026-09-29-MAT-D2.md（登记型处置）
- 真栈 S2 重跑 run：coord/runs/MAT-D-1/materialize_shadow_smoke_20260927_173924（verdict=FAIL，exit 1）
- 日期：2026-09-27

## 结论（三段证据链）

1. **登记判定本身可靠且不触发**：真栈两轮观察（obs_20260927T093936 / obs_20260927T093938）
   `measurement_carried_excluded=0`——`mixboard.DOMMeasurementFallbackActive`（与
   `domInputFromObservation` 共用同一回退链与匹配谓词）判定 **MixPackage 测量回退轴未激活**：
   snapshot 供给了两侧行的 waveform/band（MAT-D 探针的
   `snapshot_waveform_envelope=true mix_metrics_waveform=true` 是"两图皆非空"，
   不是"回退发生"——顶层键经 targetNeedsMatchingRow 匹配检查后被保留，metrics 未被消费）。
2. **分歧是常量输入差**：两次 run 内 fresh/stored hash 各自跨轮恒定（set_volume 前后不变），
   fresh/stored 行 payload 逐字段相等——分歧在进 hash 不进 payload 的字段。
3. **机械定位（label_diff_minimal_repro_test.go.txt）**：复刻两侧行化输入差对拍——
   - hash(观察侧形态, label="Track 1") = sha256:a1acc8fc23a86c8c
   - hash(适配器侧形态, 无 label)   = sha256:0fb5d0992872fd97（≠）
   - hash(观察侧形态, label 去除)   = sha256:0fb5d0992872fd97（== 适配器侧）
   - 逐字段 JSON 差分：唯一非 volatile 差异键 = `target_ref`（label）；TimeRuler/MixPackage
     在场但未被消费时不影响 hash。
   - 真栈 divergence 明细：ref=vit://dom/track:1007/t=all@current
     fresh=sha256:a17b69101bbc9570 stored=sha256:c80e049d65016e58（payload 相等）。

## 根因

`agent/internal/materialize/adapters.go` `DOMAdapter.Build` 的合成包
`mixboard.TargetRef{Kind: "track", ID: trackID}` **不带 Label**；真实观察包的
TargetRef 带 label（"Track 1"，来自内核轨标签），label 进入 dom.Projection.target_ref
且**不在 volatileContentKeys 剔除表**→内容身份 hash 恒不等→每轮 track 定向观察 S2 恒红。
观察路径零缺陷；这是物化适配器合成包与真实观察包的输入奇偶性缺口。

## 与 MAT-D 验收裁定的关系

- MAT-D 取证的机制假设（"band 单数键空→metrics 回退被保留→两侧 band 不同源"）在真实
  S2 数据上不成立：回退链复演判定 metrics 未被消费。
- 裁定②（登记型）已按卡面完整落地并测试（本分支 port/mat-d2）；但 S2 红的分歧行**不是**
  measurement-carried 形态，闸门排除不（也不应）触发→"分歧行已入排除计数→S2==0"的闭环
  路径在当前数据形态下不存在。
- label 缺口是**新发现的第四种处置方向**（不在已裁定的 ①②③ 内；①=DepInputs 扩轴携带
  MixPackage/TimeRuler 已否决；③=回退序语义独立卡）。候选修法（供决策侧裁定，未实施）：
  DOMAdapter.Build 从 deps.ProjectState（shadow 工程快照 tracks 的 track_name）取轨标签
  填入合成包 TargetRef.Label——同文件域（adapters.go）、纯物化侧，但需裁定
  "shadow summary track_name == 内核 track label" 的奇偶性语义。

## 保留的登记实现价值

measurement-carried 登记（探针+注记+闸门排除+单列计数+合成双向测试）本身完整且判定
可靠：一旦真实数据形态使回退轴激活（如 snapshot 目标键空的轨），该形态会被正确登记、
排除并计数（合成测试 TestDOMRowFromObservationRegistersMeasurementCarried 证明触发方向；
TestDOMRowFromObservationSnapshotSourcedNotMarked 证明反例不过触发）。MAT-C 合成盲区
（MixPackage 恒空）已由本卡补上带测量的用例。
