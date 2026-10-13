# Ruling: KERNEL-SETTEMPO-PERSIST-1 —— pass

- 裁定人：GLM 主管决策流（ZCode 会话）/ 2026-10-13 晚
- 依据：用户"任务卡都已完成，可以检查"授权；执行侧回执 82f9d2c1（实现 cd261982 @ port/kernel-settempo-persist-1）
- 合入：merge 95b4d693（impl+工件+冒测三件套）

## 独立复核记录

1. **内核 diff 亲读**：四处 `edit->markAsChanged()`（handleSetTempo/handleClearProject/handleUndo/handleRedo），各附机制注释（undo 变更经 UndoManager 异步 change message、同步 runAutoPersist 先于消息 dispatch → 非强制 save 静默跳写）——与 KEYFIELD 5ea95f75 修法语义一致，停止条件未触发。
2. **全清单亲核（任务①）**：八形态判定口径在卷（KEYFIELD 已修×4 + 本卡修复×4）；「登记不修」三组的机制差异论证成立（applyControlBindingsFromOutput 匿名副本=死代码；clip/load 辅助无自身 persist 调用=无现行跳写窗口；save 链内 vit_* 裸写=save 命令自身暴露面）。
3. **差分证据亲核（工件在 main）**：baseline_20261013（主检出旧内核）exit 1 于 file_written 哈希门——"set_tempo open-form must write the project file (hash unchanged)"=静默跳写实锤，冒测腿具区分度非恒真断言；fixed_20261013（worktree 自建含修复内核）PASS exit 0 八检（哈希 814ea87f→6c4f5dd8 变化+往返 96/96+clear 1→0+undo 96+redo 144）。Go 面零改动，go test 不涉（口径正确）。
4. 栈纪律：登记 bb5401c2→释放 bc216f81，四端口四进程核净；两轮间手动换内核如实记录。

## 裁定与登记

**pass**。无挂账。

1. **「登记不修」三组采信不立卡**：死代码不动；clip/load 辅助在 ClipService 将来注入 autoPersist 时按本卡机制补注册（登记为先决条件，非现行缺陷）；save 链 vit_* 裸写维持域外。
2. **部署注记**：主检出 VitApp.exe 仍为 10-06 构建（不含本卡修复）——后续占栈卡若内核行为依赖此修复，需重建主检出内核或用 worktree 构建二进制（-KernelExe 先例）。
3. worktree D:/Vit_DAW_wt_settempo 验收后拆除（决策侧执行）。
