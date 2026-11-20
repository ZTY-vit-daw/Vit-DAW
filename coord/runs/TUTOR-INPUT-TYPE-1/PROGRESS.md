# PROGRESS — TUTOR-INPUT-TYPE-1（池序 444，产线 inputtype444）

任务卡：D:/Vit_DAW/coord/cards/todo/2026-11-21-TUTOR-INPUT-TYPE-1.md（零栈小卡：步骤卡输入型按 expect 自适应）

## 领取基线（2026-11-21）

- main HEAD：afe7ca6c2b5f82b79faa9a42fd47d439c2065e1a（origin/main=a6420951，本地领先，无远端新提交）
- 工作树状态：agent/、coord/cards/、coord/agents/ 路径干净；已有脏文件仅 CURRENT-STATE.md / VitApp Workspace / coord/runs 截图（领取前已有，本卡不触碰）
- 分支：pc444-input；worktree：D:/Vit_DAW_wt_pc444input
- 文件域：agent/internal/chat/tutor_sim.go(+test)、agent/webui/src/project/（TutorWidgets/projectData/projectService/ProjectSurface 类型面与接线）；与 398 scoreworkbench 域不相交（TutorWidgets.tsx 仅复用其 render import，不改其文件）

## 步骤

- [x] S1 读卡面+入场文件（tutor_sim.go / tutorsim.go Step.Expect / TutorWidgets.tsx / projectData.ts / projectService.ts / ProjectSurface.tsx / 三处既有测试约定）
- [x] S2 记录领取基线（本节）
- [x] S3 领取：卡 mv doing/ + 回填 + 只提交本卡 coord 文件到 main → commit 0a1e409b（显式 pathspec，未触碰 index 中 SCORE-GUI-FOLLOW-1 残留暂存）
- [x] S4 建 worktree D:/Vit_DAW_wt_pc444input + 分支 pc444-input（自 0a1e409b）
- [x] S5 服务端完成：tutorSimAnswerKind + workflow_data.answer_kind（start/chat/AlreadyCompleted 同源）；新增 TestTutorSimAnswerKindClassification / TestTutorV0CarriesNumericAnswerKind / TestTutorV1AnswerKindTextStep，定向 -run 五例（含 437 既有 v0/v1 例）全 PASS，go build ./... exit 0
- [ ] S6 UI：projectData.ts（TutorStepView.answerKind? + tutorAnswerKindFromWorkflowData + reducer answerKind 携带/清除/回退）、projectService.ts（TutorStartOutcome.answerKind + start 回执投影）、ProjectSurface.tsx（start/chat 两路接线）、TutorWidgets.tsx（TutorStepCard 按 answerKind 渲染 text/number 输入框）；vitest 用例新增（只加不减）
- [x] S7 四门验证 exit 0：go build（无输出）/ go test ./internal/chat -count=1（ok 122.973s）/ npm run test（68 文件 845 测试全绿）/ npm run build（✓ 2.58s）；日志 gate1..gate4 log 落本目录
- [x] S8 完成：实现 commit 7195a8b1（pc444-input）→ merge main b5f7b6ab（main 领先 4 coord 提交，ort 真合并无冲突）→ 卡 mv done/（执行完成信息回填）→ ROSTER inputtype444 入册 → receipt.md 落本目录

## 终态：DONE（2026-11-21）

- 四件套：
  1. 命令+退出码：见 receipt.md 四门节（全 exit 0）；
  2. 产物路径：代码=main b5f7b6ab（agent/internal/chat/tutor_sim.go + agent/webui/src/project/ 10 文件）；证据=本目录（PROGRESS.md/receipt.md/gate1..4 log）；
  3. 遇到的问题：worktree 无 node_modules（npm ci 4s 解决）；merge 时 main 被 443/351/398 coord 提交推进，--ff-only 失败改 ort 真合并（无冲突，无越域，未触碰他卡暂存）；
  4. 断了 0 次（未感知重派）。
- 遗留：真栈手测并入 443 窗口（卡面既定）；origin push 视网络在本会话末尝试，失败则本地提交为权威输入待主管同步。

## 长命令记录（S7）

```
cd agent/webui && npm ci   # S6.5 新 worktree 无 node_modules，先装依赖
cd agent && go build ./...
cd agent && go test ./internal/chat -count=1
cd agent/webui && npm run test
cd agent/webui && npm run build
```

- [x] S6 UI 完成：projectData.ts（TutorStepView.answerKind? + tutorAnswerKindFromWorkflowData + reducer text 落键/number 清键/缺省回退 + TutorStartOutcome.answerKind?）、projectService.ts（start 回执投影，仅 workflow_data 在场时带键）、ProjectSurface.tsx（start/chat 两路接线）、TutorWidgets.tsx（type/inputMode/placeholder/kind 属性按 answerKind 切换，提交链路不变）；vitest 新增 4 describe（TutorWidgets 3 例 / projectData 2 例 / projectService 1 例 / ProjectSurface.tutor 2 例），全部只加不减

## 设计要点（防重推）

- 服务端判定单点：tutorSimAnswerKind(expect)——TrimSpace 后逐 rune 判 0-9；空串→number（v0 兼容，Parse 校验 expect 必填，防御兜底）；非纯数字（含「完成」「模进」「-5」）→ text。
- workflow_data 单点增 key：tutorSimStepWorkflowData 内 data["answer_kind"]=...，start 与 chat（含 AlreadyCompleted）两路同源。
- UI 兼容面：view.answerKind 仅在 "text" 时落键（"number" 是缺省，不落键）——437 projectData.test.ts 的 toEqual 严格相等面不回归；tutorStartOutcomeFromResponse 仅 workflow_data 在场时投影 answerKind——436 v0 无 workflow_data 的 6 键严格映射面不回归。
- TutorStepCard：type/inputMode/placeholder/data-tutor-step-input-kind 按 answerKind 切换；提交链路（onSubmit→send）不变。
