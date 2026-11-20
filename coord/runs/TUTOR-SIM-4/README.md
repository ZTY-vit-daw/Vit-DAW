# TUTOR-SIM-4 勘察停卡证据（2026-11-20）

- 卡：`coord/cards/blocked/2026-11-20-TUTOR-SIM-4-PROD-NODES.md`（自 todo 移入 blocked）
- owner：tutorprod438（GLM-5.3 flash 执行子代理）；入场勘察停卡——未建 worktree、未领取实现、产品文件域零改动
- 领取基线（main）：`git rev-parse HEAD` = `a6420951f975339bfa108b032959b3ef977caf19`
- 失败分类：`premise_falsified`（依赖缺失）+ `mapping_blocked`；非执行失败、非环境中断

## 结论

1. **卡前提「既有材料音频资产（VIT_EDU_LEVELS_DIR 装置）可复用」不成立**：全部落点核实为零音频资产，仅 MIDI（kind=midi_reference）。卡约束「不自造新音频」下，生产派生 tutor YAML 的 `materials={kind:audio,...}` 无法真实填充。
2. **生产节点 done_when 全部非文本**，无法映射 tutor 步进模型的 trim 精确匹配 expect；卡验收第 4 条「停手上报，不硬塞」触发。

## 勘察命令与结果（全部只读，零写入源码树）

| 命令 | 结果 |
|---|---|
| `git rev-parse HEAD` / `git status --short` / `git branch --show-current` | a6420951… / 既有改动与 438 无关 / main |
| `ls coord/levels/` | 仅 README.md + first-run-twinkle（单关卡） |
| `diff ~/.vit/levels/first-run-twinkle/level.yaml coord/levels/first-run-twinkle/level.yaml` | 字节一致（IDENTICAL） |
| `ls coord/levels/first-run-twinkle/assets/` | theme_twinkle_v0.mid、theme_twinkle_corrupted_v0.mid、MANIFEST.yaml（无音频） |
| `git ls-files` 过滤 wav/mp3/ogg/flac/m4a/aac | 仅 7 个校准/测试 wav（calibration*、test_100hz_10s.wav、test_target_3s.wav） |
| 全仓 find 音频（排除 runs/node_modules/.git） | 其余全部为运行工件（coord/runs、artifacts、.vit_agent）或 JUCE 子模块 demo |
| `D:/Godot/project/vit-daw-frontend` 音频 find | 全部为 .vit_agent 运行工件 + baked_tiles/test_target_3s.wav（测试渲染） |
| `find coord/lesson-designs` 音频 | 0 个（assets/midi/ 仅 21 个 MIDI） |
| `Paper Crown.mp3` | `.git/info/exclude:10` 排除的用户个人文件，非教学资产 |

## 锚点

- `coord/levels/first-run-twinkle/level.yaml:34-50` — assets 仅 2×midi_reference（.mid）
- `coord/levels/first-run-twinkle/level.yaml:139-233` — 七节点 done_when 全非文本（n0 clip_exists+human_replied；n1/n3 note_sequence_match；n2 user_confirmed+tempo_set；n4 event user_chose；n5 user_feedback_given+user_confirmed；n6 export_completed）
- `coord/levels/first-run-twinkle/assets/MANIFEST.yaml:6-7` — license 注记「不使用现有录音」；`:34-37` deferred 仅谱面 PDF
- `agent/internal/edu/lesson_card.go:692` — 生产资产 kind 词表=midi_reference，无 audio 位
- `coord/experiments/tutor-sim-1/tutor_sim.yaml` — v0 现状装置（未改动，v0 兼容面不受本停卡影响）
- 卡面停止条件原文与验收第 4 条（见卡文件）

## 未动面

产品文件域零改动：`agent/internal/chat/tutor_sim.go`、`agent/internal/tutorsim/`、`agent/webui/src/project/`、`agent/webui/src/widgets/`、`scripts/run_tutor_sim_smoke.ps1`、`coord/levels/`、`coord/experiments/tutor-sim-1/`、冻结 production design schema。本目录为本次唯一新增工件。

## 仍可执行面（供主管拆卡参考）

schema v1（brief/materials 字段 + v0 缺省空兼容）、tutor_sim.go workflow_data 携带、UI brief 折叠 / AudioPlayer 卡 / 无材料不渲染（fixture src 测试，同 gallery `WIDGETS_FIXTURE_AUDIO` 模式）、smoke 断言扩展——均不被上述证据阻塞；仅「生产派生 YAML 携带真实音频材料」内容腿不可执行。处置选项（拆卡 / 另开音频资产卡 / 指认遗漏资产）见卡面停卡节，由主管裁定。
