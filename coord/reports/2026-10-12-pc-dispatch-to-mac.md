# PC 调度件 → Mac 决策侧（2026-10-12 早窗）

- 发出：PC 主管决策流（GLM）/ 收件：Mac 决策侧会话
- 性质：用户直接裁定，非 PC 代排建议。

## 一、用户裁定（2026-10-12 早窗原话要旨）

**Mac 侧任务由 Mac 决策侧自行排序执行**——Mac 与 PC 并非同步执行，PC 不代排 Mac 线顺序；用户观察到 Mac 侧多张积累任务未开始，领取顺序、并行深度、会话分配**全由 Mac 决策侧按自身容量与卡面依赖自定**。

## 二、PC 侧当前在飞（仅供文件域互斥参考，非指令）

| 卡 | 文件域 | 状态 |
|---|---|---|
| M2-TIMEFX-1（71） | chat/capability seam+d1s1 域表+单测 | PC 执行会话今日领取 |
| LOUDNESS-A-CONFIG（89） | rlm 配置面/projectAgentRuntimeState | PC flash 会话今日领取 |
| M3-RECON-1（93，PC 主管立卡） | 纯只读勘察，写面仅 coord/runs/M3-RECON-1/ | 已排 PC 闲时车道 |

## 三、Mac 侧可用卡（卡池 todo 现状，排序权在你）

KEYFIELD(76，DISCLOSE 已合入解锁) / INJECT(77，并行窗开) / CONSUME(78，待 INJECT 合入) / PLUGIN-SHELL-DESIGN(80) / VIS-FILM-A1·B1·B2·C1（W1 四线，P0 已 pass 解锁；B4/C3/D1 后续） / GEN-VOCAL-SYNTH-1（SVS）。

纪律不变：领取记录 push 共享 main 后开工；文件域互斥（VIS-FILM 规格件 [runs/VIS-FILM-DESIGN-1/05-impl-card-specs.md](../runs/VIS-FILM-DESIGN-1/05-impl-card-specs.md) 内互斥表：B2 接线待 B1 建文件、playwright_discovery.mjs 与 C1 互斥）；MAC-RUNTIME-STACK 同一时刻单 owner。

## 四、验收流与用户待办（不变）

- Mac 交付后沿用双裁定先例（b14a7d0d）：Mac 决策侧可先行裁定+报告合流，PC 主管复核留档。
- 用户项两件：genapi 真源验证（有凭据后填 `~/.vit/genapi_providers.json`，零代码）；FE 手测 7 条待排期。
- PC 侧今日决策侧活（三勘察 ruling 补档 pass、M3-RECON 立卡）已入 main（809cab5d 及后续），无待 Mac 配合事项。
