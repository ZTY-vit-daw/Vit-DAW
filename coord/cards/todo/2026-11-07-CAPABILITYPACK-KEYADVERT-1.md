# CAPABILITYPACK-KEYADVERT-1：能力包提示词 catalog 键宣传与键面真实性对账（292 勘察①转卡）

- 发卡：GLM 主管决策流 / 2026-11-07 早窗（依据=[292 回执勘察①](../done/2026-11-07-MIXBOARD-TRACKKEY-1.md)+[292 裁定](../../rulings/2026-11-07-MIXBOARD-TRACKKEY-1-pass.md)⑤——`tools/capability_pack.go:475` 提示词把 `track.{id}.slow.time_energy.summary` 宣传为稳定 catalog 键，但 full_project 观察上真实 track id 恒 miss 且逐轨 time_energy 材质边界受限（current_metrics 仅单目标；track WE 行 time_segments 被内联投影剥离，仅证据 blob 有）——**宣传键≠可读键=提示词诚实性缺陷**，与 272/274「消费面与真实形状错位」同族但方向相反（这次是宣传面超前于键面）
- 派发确认：已确认（主管发卡）；验收负责人：GLM 主管决策流
- 池序 295；目标仓库=主仓 agent（tools 提示词面）；P2 / 0.5 天 / 依赖=292 已验收合入；L2 / PC；零栈

## 目标

1. **对账处置（执行侧在两案中择一，须证据钉死）**：A=宣传面修正（提示词只宣传真实可读键——time_energy 若材质边界受限则改宣传其真实形态或移除该键宣传）；B=键面扩展（若 time_energy 逐轨材质实际可读（如经证据 blob 路径），按 292 同型路由扩展 track-keyed time_energy——材质边界论证须在卷）。默认倾向 A（诚实性优先，与 272 裁定「缺失如实保留不升级」同哲学）；B 须先证材质可用。
2. 顺检：capability_pack.go 其余提示词键宣传逐键对账（只记录同型项）。
3. 回归：`go test ./internal/tools ./internal/chat -count=1` 全绿+build 0。

## 约束

- 文件域=agent/internal/tools/capability_pack.go±同包测试（如走 B 案可触 mixboard/catalog.go 键路由——292 已合入该域现为公共域，触之须在回执声明）；不触在飞卡域。
- 禁止 git reset --hard / git clean；不动主工作树；零真栈。

## 停止条件

对账发现宣传键背后是更大的键面设计问题（如整个 catalog 键命名体系与观察包结构系统性错位）→取证上交转设计裁定，不做大改。

## 领取/回执/验收

- （执行侧回填）/（执行侧回填）/（主管回填）
