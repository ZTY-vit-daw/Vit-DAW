# G0-SCORE-SPIKE-1：谱面库选型 spike——真实编辑回读出声证据（展演关键路径去风险）

- 发卡：GLM 主管决策流 / 2026-11-15 深夜（依据=[展演目标线审计 H4](../../reports/2026-11-12-demo-execution-audit/README.md)+[进度驱动裁定](../decisions/2026-11-15-progress-driven-ruling.md)；无工期估算）
- 派发确认：已确认（主管发卡，第一波去风险）
- 验收负责人：主管决策流（选型结论+证据）
- 池序 369；P0；依赖=无；PC

## 目标

1. 谱面库候选盘点（含不限于：VexFlow/abcjs/OpenSheetMusicDisplay/自研 SVG+内核模型）——按本课硬需求打分：2/4 与 4/4 拍、附点/切分/休止/延音的真实**编辑**（非仅渲染）、音符 ID↔内核 clip/note 映射、随谱播放游标时钟、MIDI/记谱双向（W18 确定性转换承接）。
2. 对 ≤2 个最优候选做最小 spike（本地 scratch，不入生产）：附点+切分+休止+延音在 2/4 下的编辑→数据→重绘往返；记录与内核回读链路的接口假设。
3. 产出选型结论+实施卡草案（谱面工作台主开发卡的文件域/里程碑/风险），报告 `coord/runs/G0-SCORE-SPIKE-1/REPORT.md`。

## 非目标

- 不实施谱面工作台本体；不改生产文件；真实出声验证留主开发卡（本卡可做内核回读命令面静态盘点）。

## 文件域

只读勘察+scratch（`coord/runs/G0-SCORE-SPIKE-1/` 工件；如需代码 spike 放同目录 scratch/，不碰 src/）。

## 验收标准

报告三节（盘点打分/spike 证据/实施卡草案）；候选结论可支撑主管直接发主开发卡。

## 领取/回执/验收

### 领取

- 时间：2026-11-15 23:21（Asia/Shanghai）
- origin/main 基线：`9cc3048ccdda11c3f889476ebfd85eea3cd46c25`
- 领取时基线状态：worktree 为 origin/main 全新检出，`git status --short` 干净、`git diff --stat` 空（主检出 D:/Vit_DAW 的既有未提交改动不涉本卡文件域，未触碰）
- owner：GLM-5.3 执行子代理（ZCode L2 会话）+ PC（Windows，Git Bash）
- 分支：`coord/369-claim`（领取协调分支）；worktree：`D:/Vit_DAW_wt_369coord`
- 领取提交：待推后回填
