# MIDI-ARGSPEC-ALIGN-1：mutate_midi_notes argspec 对齐内核 by-id 语义

- 发卡：GLM 主管决策侧 / 2026-10-11 晚窗（依据=GEN-MAC-BASELINE-1 验收随卡裁定，缺口源=其 report.md §二.2）
- 派发确认：已确认（主管设计裁定：对齐 argspec 到内核，不动内核 fail-closed 行为）
- 验收负责人：GLM 主管决策流
- 池序 69；目标仓库=D:\Vit_DAW
- 优先级 / 预估 / 依赖：P2 / 0.25 天 / 无（与 AIGC-DRUM-GEN-1 文件域不相交；是其质量前置但不阻塞领取）
- 模型分级：L0 / flash 或闲时

## 目标

`agent/internal/tools`（或 catalog.go 实际所在包）中 `mutate_midi_notes` 的 argspec：notes 数组每元素的 id 由"可选（id|note_id 别名暗示区域形式可用）"改为**必填非空 id**，与内核校验一致（MidiService.cpp:1125-1144 实测 "each note requires a non-empty id"，GEN-MAC-BASELINE-1 run2 排障实录+summary 探针佐证）。只改描述面（argspec JSON/注释），**不改内核、不改 handler 行为、不加别名宽容**。

- 锚点：catalog argspec 现位置 catalog.go:540 一带（发卡侧未逐行亲验，领取时以 grep `mutate_midi_notes` 实际落点为准，漂移则按实际执行并回注）。
- 顺带核对：`delete_midi_notes`（GEN-MAC 勘测结论=只收 note_ids 与 catalog 一致无漂移）——确认即可，无漂移不动。

## 文件域

argspec 定义单点文件+对应测试（若有 argspec 快照测试同步）；越域上报。

## 验收标准

- argspec 变更后：单测/编译绿（`go build ./...`+相关包 `go test -count=1`）；grep 证实无其他命令族同型漂移（仅记录，不扩域修）。
- 回执附变更前后 argspec JSON 对照。

## 停止条件

发现内核近期已改收区域形式（行为漂移）→ 停手上交锚点。

- 领取：2026-10-11 21:47 / origin/main 8e66e674 / owner=GLM-5.3-Flash（ZCode PC flash 执行流会话，用户指派双卡）/ 分支 port/midi-argspec-align-1 / worktree D:/Vit_DAW_wt_midi_argspec_1 / 领取提交=coord-claim-20261011-flash-l0
- 回执：（commit / argspec 对照）
- 验收：（裁定文件）
