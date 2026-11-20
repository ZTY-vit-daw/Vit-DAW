# receipt — TUTOR-INPUT-TYPE-1（池序 444，产线 inputtype444）

- 卡：coord/cards/done/2026-11-21-TUTOR-INPUT-TYPE-1.md（done 待验收）
- 领取基线：main afe7ca6c（领取提交 0a1e409b）→ 实现 commit pc444-input 7195a8b1 → merge main b5f7b6ab
- worktree：D:/Vit_DAW_wt_pc444input，分支 pc444-input
- 被测代码版本：HEAD 7195a8b1 的 agent/ 工作树（merge 后 main 内容与该树在 agent/ 域逐字节一致，merge 无冲突）

## 四门原始输出（末 5 行逐字）

1. `cd D:/Vit_DAW_wt_pc444input/agent && go build ./...` — exit 0（无输出）
2. `cd D:/Vit_DAW_wt_pc444input/agent && go test ./internal/chat -count=1` — exit 0

```
ok  	vit-daw-agent/internal/chat	122.973s
```

3. `cd D:/Vit_DAW_wt_pc444input/agent/webui && npm run test` — exit 0

```
 ✓ src/project/ProjectSurface.tutor.test.tsx (18 tests) 372ms
 ✓ src/scoreworkbench/ScoreWorkbenchPanel.test.tsx (17 tests) 1102ms

 Test Files  68 passed (68)
      Tests  845 passed (845)
   Start at  00:24:21
```

4. `cd D:/Vit_DAW_wt_pc444input/agent/webui && npm run build` — exit 0

```
(!) Some chunks are larger than 500 kB after minification. Consider:
- Using dynamic import() to code-split the application
- Use build.rollupOptions.output.manualChunks to improve chunking: https://rollupjs.org/configuration-options/#output-manualchunks
- Adjust chunk size limit to this warning via build.chunkSizeWarningLimit.
✓ built in 2.58s
```

（chunk 大小警告为既有现象，非本卡引入；完整日志 gate1_go_build.log / gate2_go_test_chat.log / gate3_npm_test.log / gate4_npm_build.log）

## answer_kind 判定规则

- 服务端单点 `tutorSimAnswerKind(expect)`（agent/internal/chat/tutor_sim.go）：expect TrimSpace 后逐 rune 全为 '0'-'9' → `number`；含任一非数字 rune（「完成」「模进」「已装载」「-5」「12a」等）→ `text`；空/全空白 → `number`（v0 兼容缺省，Parse 校验 expect 必填，此为防御面）。
- 投影单点 `tutorSimStepWorkflowData`：`workflow_data.answer_kind`，start 与 chat（含 AlreadyCompleted，verdict.StepIndex 保持末步）同源。
- UI 兼容面：`tutorAnswerKindFromWorkflowData` 只认显式 `"text"`，其余一律 number；视图/接缝仅在 text 时落 `answerKind` 键——v0（无 answer_kind / 无 workflow_data）恒为数字框=现行为；显式 number 清除旧 text（换步不粘带）。
- TutorStepCard：`type`/`inputMode`/`placeholder`/`data-tutor-step-input-kind` 按 answerKind 切换；提交仍走 onSubmit(文本)，服务端精确匹配判定权威。

## 测试卫生

- 437/438 既有用例零删减：5 个测试文件删除行亲证 = 0（git diff '^-' 计数）。
- 新增：Go 3 例（判定表 / v0 start+chat number / v1 text 步+AlreadyCompleted）+ vitest 8 例（TutorWidgets 3 / projectData 2 / projectService 1 / ProjectSurface.tutor 2）。
- 运行时状态零落盘：测试均走 httptest/t.TempDir（既有 harness）；本卡未启动内核/Godot/agent 真实栈（零栈卡面约束）。

## 断/重派记录

- 未感知重派；单会话一次完成。
- merge 时 main 领先 4 个 coord 提交（443/351 领取、398 盖章），--ff-only 不可用，按 ort 真合并 b5f7b6ab，无冲突无越域。
- 推送 origin：见 PROGRESS.md 终态记录（本机网络视情况）。

## 遗留

- 真栈手测并入 443 窗口（卡面既定）；本卡验收负责人=lead。
