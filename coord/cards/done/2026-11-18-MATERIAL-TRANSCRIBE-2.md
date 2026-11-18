# MATERIAL-TRANSCRIBE-2：教材谱例材料转写流水（批 2–4，剩余 planned_transcription）

- 池序 420；P2；PC；发卡=本会话 PC 主管（lead）；派发确认=已确认（用户 2026-11-18 晚同意并行推进展演线）。
- owner=`lead/materials420`（DeepSeek-V4.1-Flash，PC）；分支 `codex/material-transcribe-420`；worktree=`C:/Users/timoz/.codex/worktrees/pc420-materials/Vit_DAW`（主仓）；基线=`origin/main`。
- 依据：`coord/cards/todo/2026-11-16-MATERIAL-TRANSCRIBE-1.md`（批1 已完成 5 条 pitch-*）；`coord/lesson-designs/music-creation-unit2/assets/catalog.yaml` 中所有 `status: planned_transcription`、`file: null` 的条目；`TEXTBOOK-ALIGNMENT-AND-MATERIALS.md`；批1 工件 `coord/runs/MATERIAL-TRANSCRIBE-1/batch1/`（可读，不修改）。
- 目标：把剩余 planned_transcription 条目全部推进到 `transcribed_unverified`（批 2–4，每批≤5 条），不冒充 checked / 不冒充可加载。

## 每条流程（与批1同构）

1. 读 `source_image`（source/pages/ 教材页缓存）核对 `excerpt` 实际小节范围，回填 catalog `excerpt`；
2. 对照原谱转写音高/节奏/休止/调号/拍号 → MIDI 落 `assets/midi/<材料id>.mid`，catalog 回填 `file: midi/<材料id>.mid`；未给 BPM/调号处按批1惯例单独记录演示设定（不改原谱事实）；
3. 用仓库现成 MIDI 解析/回读工具（批1 `transcribe_batch1.py` 可复制到本卡 run 目录复用/扩展）回读 MIDI，核对音高序列/时值/拍号与转写意图一致；每条留回读记录；
4. status 改 `transcribed_unverified`；`uncertain` 音符/小节逐条记录到 verification，不猜不编；
5. 工件落 `coord/runs/MATERIAL-TRANSCRIBE-2/batch<N>/`（catalog before 快照、diff、回读记录、uncertain 清单）。

## 可行性与停止

- 第一步必须先用 `read_image` 打开一张 source page 并确认可读；若工具/模型无法辨识五线谱或图像不可用 → **停手上报**（附证据），不得凭记忆/猜测伪造转写。
- 同一条目连续两次回读不一致→停该条上报；不降标准、不把 uncertain 写成确定。

## 边界

- 只改 `coord/lesson-designs/music-creation-unit2/assets/`（catalog.yaml + midi/ 新增）与本卡 run 工件；不动 `source/pages`、不动生产代码、不动 `assets/midi/` 既有两条 generated_symbolic_checked、不新增材料条目、不改 POOL-INDEX。

## 验收（主管抽核）

- 全部剩余 planned_transcription 条目 → `transcribed_unverified` + MIDI 文件在场 + 回读记录在卷；
- catalog diff 与 run 工件一致；uncertain 清单如实（有则接受、无则明确零）；
- 主管抽 3 条目检（谱面对照+回读记录+catalog diff 一致）；
- 回执：commit、文件清单、逐条清单、命令+exit、uncertain 汇总、边界（零真栈/零 LLM 说明）。

## 回执与验收（2026-11-18 晚）

- commit `44ad301c`；主管独立复跑 validate_catalog/final_check exit 0、文件域 181/181 在域、crop 抽检自洽；合入 main `25b58b8a`。
- 结果：15/15 → `transcribed_unverified`；22 条 = 20 unverified + 2 checked；uncertain 61 条逐项记录；未听核不冒充 checked。
- 裁定：[rulings/2026-11-18-MATERIAL-TRANSCRIBE-2-pass.md](../../rulings/2026-11-18-MATERIAL-TRANSCRIBE-2-pass.md) PASS（零栈）。
- 后续：W01/W19 才把材料接入运行时；听核升级另节点。
