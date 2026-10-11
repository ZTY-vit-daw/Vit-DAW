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
- 回执：commit 92704452（port/midi-argspec-align-1，基线 55c997d3，分支已推 origin）；落点=catalog.go argHint（grep 实证与卡面锚点一致，:540）+catalog_test.go 新增锁定测试；**argspec 前后对照**——
  - BEFORE：`clip_id:string optional track_id:string time_unit:beats notes:{id|note_id:string optional pitch/start/length|duration/velocity}[]`
  - AFTER：`clip_id:string optional track_id:string time_unit:beats notes:{id:string required non-empty (by-id lookup; note_id accepted as alias key; region form not accepted) optional pitch/start/length|duration/velocity}[]`
- 内核核验（停止条件未触发）：MidiService.cpp handleMutateMidiNotes 现版每 note 经 readRequestedNoteId（`id`→`note_id` 双键）取目标，空 id fail-closed "each note requires a non-empty id"，无区域形式——卡面前提成立；agent 侧 handler（harness.go:9350 一带）把空 id 的 note_id 归一为 id，别名事实与内核一致，描述面如实保留别名注记。
- 同型漂移 grep（仅记录不扩域）：argHint 全表 `id|note_id` 型别名仅 mutate 一处（已修）；add_midi_notes/add_midi_notes_bulk `id?:string`=内核空 id 自生成 `note_<uuid>`（MidiService.cpp add 路径）无漂移；delete_midi_notes `note_ids:string[]` 与内核 :375 一致无漂移（卡面顺带核对完成）；apply_midi_note_patch `operations:object[]` 泛型形态不含同型误导（内核 op 级收 note_id/note_ids 或 start+length 区域，:457）。
- 测试：新增 TestMutateMidiNotesArgspecMatchesKernelByIDSemantics（四要素锁定+`id|note_id` 反门+delete note_ids 无漂移锁定）；无既有 argspec 快照需同步（grep 证实）。
- 验收命令：go build ./... exit 0；go test ./internal/tools -count=1 ok；gofmt blob 级净（工作树 `gofmt -l` 命中为 autocrlf CRLF 检出伪影——基线 blob 同样命中，去 CR 后净）。
- 端测边界声明：纯描述面变更（argHint 字符串+注释+单测），零 handler/内核/渲染面触碰，无需真栈烟测；唯一消费方=modelSummaryLine（LLM 工具摘要面）。
- 验收：**pass**（2026-10-11 晚夜：diff 亲核=仅 catalog.go argHint+注释+锁定测试（纯描述面）；tools 包测试亲跑 ok；内核核验链（readRequestedNoteId/by-id/空 id fail-closed）与停止条件核对在卷；同型漂移全表 grep 记录=add/bulk 自生成无漂移+delete 一致+patch 泛型不含误导——顺带核对项完成。argspec 前后对照在卷。裁定文件 rulings/2026-10-11-BATCH2-rulings.md）。cherry-pick 入 main=**ec963d7e**。
