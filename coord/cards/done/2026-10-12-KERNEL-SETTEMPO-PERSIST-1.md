# KERNEL-SETTEMPO-PERSIST-1：set_tempo open 形态静默跳写修复（变更注册补齐）

- 发卡：GLM 主管决策侧 / 2026-10-12 晚窗（依据=GEN-CTX-KEYFIELD-1 域外上交证据+Mac 裁定 3bcaa39a 采纳「P2 修复卡建议」；[PC 复核留档](../../rulings/2026-10-12-PC-COUNTERSIGN-MAC2.md)）
- 派发确认：已确认（主管依已验收卡的上交换证据发卡）
- 验收负责人：GLM 主管决策流
- 池序 101（原 95 与 GENSHELL 六卡块 95-100 撞号，后发者改号——Mac 决策侧 2026-10-12 协调修正，GENSHELL 块 e2d28836 在先）；目标仓库=D:\Vit_DAW
- 优先级 / 预估 / 依赖：P2 / 0.5 天 / 无（内核卫生卡）
- 模型分级：L1 / GLM 执行会话（PC，内核 C++）

## 背景（上交证据摘要）

KEYFIELD 迭代史 run2 定位：**裸 `setProperty` 写 edit state 不注册变更→`EditFileOperations` 静默跳写**（不落盘）；同款缺陷存在于 set_tempo 的 open 形态（有值写入路径已注册，open/清除形态未注册——领取时以实际代码为准）。

## 目标

1. **同模式命令全清单化**：grep CommandDispatcher 全部写 edit state 的命令，列出「裸 setProperty 未注册变更」的命令/形态清单（不止 set_tempo）。
2. **修复**：清单内全部补 `markAsChanged`/等价变更注册（KEYFIELD 5ea95f75 同款修法）。
3. **回归门**：真栈冒测——set_tempo open 形态写后 save/reload 往返一致（§11 三验形态：set_at 逐字节比对），受影响命令逐条正例。

## 文件域

VitApp/Source/Service/CommandDispatcher.cpp（+冒测脚本扩展 SMOKE_TESTS 登记）；领取时与在飞卡核对（M2-TIMEFX-2 探针注册同文件不同区段，PROTOCOL §2.1）。

## 验收标准

内核构建 exit 0+真栈冒测 exit 0（往返一致正例+清单在卷）；域外发现新缺陷→上交不扩域。

## 停止条件

markAsChanged 语义与 5ea95f75 修法不符（内核已重构）→ 锚点上交。

- 领取：2026-10-13 21:27 +0800 / origin/main=b0226b65 / GLM-5.3 PC 执行流（ZCode 会话，本会话）/ 分支 port/kernel-settempo-persist-1 / worktree D:/Vit_DAW_wt_settempo / 领取提交=3d5a5feb
- 回执：
  - **实现 commit**：cd261982@port/kernel-settempo-persist-1（CommandDispatcher.cpp +24 行四处 markAsChanged + 冒测三件套 settempo_persist_smoke.py / dev_agent_smoke -SetTempoPersistSmoke 腿 / SMOKE_TESTS.md 登记）。
  - **全清单（任务①）**——判定口径=「写 edit state 且变更注册不可靠（裸 ValueTree 写不经 undo 白名单监听 / 引擎 API 写经 UndoManager **异步** change message，headless 同步 persist 前 markAsChanged 未置位）」：
    1. `set_project_key` set 形态（state.setProperty）——KEYFIELD 5ea95f75 已修（markAsChanged 在 2745 行）✅不动作
    2. `set_project_key` clear 形态（state.removeProperty）——已修（2706）✅
    3. `set_project_style_note` set 形态——已修（2805）✅
    4. `set_project_style_note` clear 形态——已修（2779）✅
    5. `set_tempo`（tempo->setBpm，CachedValue 经 um 注册但 markAsChanged 靠异步消息；写后同步 runAutoPersist）——**本卡修复**（handleSetTempo）
    6. `clear_project`（clips removeFromParent + invalidateStoredLength；EditTreeWatcher::childAddedOrRemoved 只 restartPlayback 不置 hasChanged；写后 runAutoPersist）——**本卡修复**（handleClearProject）
    7. `undo`（um.undo 写 state，无自身 persist，后续显式 save_project 走同跳写门）——**本卡修复**（handleUndo）
    8. `redo`（同上）——**本卡修复**（handleRedo）
    - 登记不修（机制不同/无运行路径/域外）：`applyControlBindingsFromOutput`（CommandDispatcher.cpp:506 匿名副本）=死代码无调用路径（活动副本在 PluginRackControlService.cpp 自带真 UndoManager，域外）；clip 编辑辅助（applyCutOverlapPolicy/pruneTinyEditedClips/applyMicroFadeIfNeeded）与装载辅助（ensureTrackRackGraphForEdit 等）=命令无自身 persist 调用（ClipService 未注入 autoPersist），非「写后 save 假成功」直接形态，是否补注册由决策侧裁定；VitHeadlessService save 链内 vit_* 裸写（ipcSaveProjectToCurrentPath 等）=save 命令自身暴露面，域外上交。
  - **缺陷机制（锚点）**：runAutoPersist→ipcSaveProjectToCurrentPath→EditFileOperations::saveAs(path,true)，path==getEditFile()（open_project 形态）时内部转 save(true,**false**,false)——`hasChangedSinceSaved()==false` 则跳写主文件但返回 true（engine tracktion_EditFileOperations.cpp:250 门；saveTempVersion 只写 .tmp 不动主文件）。Edit::markAsChanged 的唯一异步触发链=UndoManager change message→UndoTransactionTimer；命令同步 persist 在消息 dispatch 之前。修法=写后显式 `edit->markAsChanged()`（5ea95f75 同款，语义相符，停止条件未触发）。
  - **run ID**：KERNEL-SETTEMPO-PERSIST-1/settempo_persist_smoke_{baseline_20261013, fixed_20261013}（coord/runs/，随实现 commit 入库）。两轮同一占用（登记 bb5401c2）：基线差分轮=主检出 VitApp.exe B6565DCF85D1DA86（10-06 构建，无本卡修复）**exit 1 于 file_written 哈希门——open 形态 set_tempo 后工程文件 sha256 不变=静默跳写实锤**（冒测腿区分度证据，非恒真断言）；修复轮=worktree 自建 VitApp.exe（21:42 构建，含四处修复）**PASS exit 0 八检**：set_tempo.open_form.file_written（哈希变化）+ roundtrip_96（open 形态写后 save/reload 往返精确 96——tempo 侧的 set_at 逐字节等价断言）/ clear_project precondition_clip_on_disk + roundtrip_zero_clips / undo roundtrip_96 / redo roundtrip_144；dev_agent_smoke 整体 "dev agent smoke completed" exit 0。栈释放记录见 coord/resources/PC-RUNTIME-STACK.md。
  - **验收证据链**：内核构建 exit 0（worktree Release 全量 21:27–21:42；C4819 警告=juce 第三方代码页固有不增量）+ 真栈冒测 exit 0（修复轮）。go test 不涉（纯内核卡，Go 面零改动；agent 由 dev_agent_smoke 从 worktree 源构建=基线等价）。
  - **端测边界声明**：① 冒测面=内核 ZMQ 命令面直连（dev_agent_smoke -StartKernel 栈 + py 脚本），未覆盖 webui 渲染面与用户旅程（本卡不触及渲染面）② 基线内核早于 music_context 披露，基线轮取证仅用文件哈希门（写盘发生性），值往返断言在修复轮（新内核）全过 ③ 冒测工程为密文 .vit（save_as 加密形态），逐字节比对以 sha256+重开披露值双断言实现 ④ undo/redo 正例以 set_tempo 事务为载体（tempo 值可精确断言），未逐个覆盖其他 undo 事务类型。
- 验收：（裁定文件 / 验收 commit）
