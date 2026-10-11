# VIS-FILM-A1-ENVELOPE：音频特征面（VitReel 实施线 A1）

- 发卡：GLM 主管决策侧 / 2026-10-11 晚夜（依据=设计件五 卡2）
- 派发确认：已确认（用户 2026-10-11 VitReel 归 Mac 裁定）
- 验收负责人：GLM 主管决策流
- 池序 82；**规格全文=[设计件五 卡2](../../runs/VIS-FILM-DESIGN-1/05-impl-card-specs.md)（权威）**
- 归属：Mac 生成线（VitReel）；P1 / ~1 周 / 无（与 B/C 线并行）
- 模型分级：L1 / Mac 执行流（新包 agent/internal/film/，无真栈需求）

## 要点摘录

Render 封装（start_render→WaitRender→sha256）+WAV 解码→能量包络（hop 50ms/RMS+peak+6 频段）→energy_envelope.json（vit.film.energy_envelope.v1）+v1 段落切分；验收=fixture 黄金值比对+analyseL2ProbeFile 同口径对拍 ±0.5dB。

- 领取：（时间 / origin/main hash / owner / 分支 / worktree / 领取提交）
- 回执：（commit / 单测清单）
- 验收：（裁定文件 / 验收 commit）
