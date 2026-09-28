# Ruling: REFSCHEMA-D2 — pass（G1 ruling D 类迁移全量收官）（2026-09-28 决策侧）

## 裁定

**pass**。COM paired 消费方双形态升级+内核 D2 三处切换验收通过，实现合入 main（cherry-pick 0b595c8a）。**G1 ruling D 类内核迁移就此 5/5 全量完成**（D1/D3/D4/D5 于 REFSCHEMA-D1，D2 于本卡）。

## 决策侧复跑与亲核

- **ctest 双配置复跑**：主树 `cmake-build-pcverify1-tests`（执行侧重配置），`-C Release` 与 `-C Debug` 均 **10/10**；测试二进制时间戳 21:03/21:05 与实现提交 0b595c8a（21:06）时间线吻合（§9 二进制对应关系）。
- **go 门隔离复跑**（branch worktree）：build 0、双形态测试 `TestValidatePairedEvidenceReceipt/ArtifactAcceptsBothEvidenceRefShapes` 复跑 PASS、全量 **87 包 0 FAIL**、vet com 清、四触碰文件 blob 级 gofmt 全清——回执"双级清"声明属实（DOSE 轮教训已被执行侧采纳）。
- 红先行采信：vit:// 接受用例双 FAIL（evidence_ref_invalid/pair_identity_invalid）实测红形态在回执；覆盖含错 kind/错 PairID（双形态各一）/malformed 缺 #- 段/opaque。
- 三处切换样例（:534/:964/:243 漂移后实锚）输出同构 `vit://dad.compressor_dual_tap/track:<t>/t=<win>@<pair>#-`，与 T5 黄金样例一致。
- **builder 前提漂移实勘采信**：makeCompressorDualTapRef 在 D1 未落地（T5 经 formatEvidenceRef 直测锁语义），本卡补齐 builder+T5b 黄金样例（builder 输出==T5 串）+T6 宽限期语义——领取栏先行记录，处置正确。
- 泊位声明合规：未启真栈；并如实上报非本卡启动的 Godot 进程（PID 30936，疑用户自有会话，未处置）——正确。

## 上交处置（决策侧验收时代修）

`scripts/compressor_dual_tap_smoke.py:133` legacy 硬等值断言（域外上交，§11 正确）：内核切换后该烟测必炸，属活破坏点。决策侧验收中代修——`valid_dual_tap_evidence_ref` 双形态助手（镜像 com 侧语义：legacy 字面等值或 vit:// kind 精确+snapshot==pair_id+hash 段必须在，G1 ruling #3 口径），六形态自检全对（legacy 过/vit:// 过/缺 #- 拒/错 pair 拒/错 kind 拒/legacy 错 pair 拒）。随验收 commit 入库。

## 遗留

- 主 VitApp 全链接构建验证与真栈烟测腿（与 REFSCHEMA-D1 同项）：归下次内核腿/真栈轮统一验证。
- G1 ruling D 类迁移收官；L1-1 段就此完全闭合。
