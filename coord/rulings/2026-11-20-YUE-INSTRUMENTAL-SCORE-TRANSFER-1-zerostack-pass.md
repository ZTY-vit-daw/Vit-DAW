# 裁定：YUE-INSTRUMENTAL-SCORE-TRANSFER-1(440) 腿A+B 零栈 PASS（腿C 真栈留窗）

- 日期：2026-11-21 00:20 +0800
- 卡：440（scorexfer440，free 车道）
- 裁定：**腿A+腿B 零栈 PASS**；腿C（真栈 1–2 次生成+用户人耳）候窗。
- 代码：cab1cadb（腿A）+9354bc99（腿B）→ merge 05be3d6f；主管复跑 main `go build` 0、genapi 5.9s ok、harness 19.0s ok。

## 收口事实

- **腿A 金样对账字节级达成**：迁移输出与金样 instrumental.abc 逐字节一致；单测钉死 197→0/78→275 守恒+逐音 pitch/onset/duration+和弦/拍号/速度（yue_transfer_test.go 408 行，断言未放宽）。
- **腿B 两段式落地**：pass1 生成→replay sidecar 取谱（缺失=response_parse_error 分类）→Go 迁移→pass2 固定谱 verbatim 重渲染（214808 口径）；收据入 `two_stage`（双 job id+守恒表+seed+final=pass2）；S6 披露=两段式/双倍成本/结构级抑制/pass1 仍可能唱/用户听核门；381 反超卖断言硬化（禁 single-stage engine-level suppression 等措辞）。
- TestGenJobsLifecycleYueLocal fixture 按两段式更新，覆盖扩大未放宽。

## 边界

- pass1 生成谱偏离 native dialect → 两段式分类失败=概率类（372 §8），不重跑。
- 腿C 成功判据=用户人耳零人声/零星；PASS 后 390 fallback 口径升级勘误。
