# TUTOR-INPUT-TYPE-1：步骤卡输入型按 expect 类型自适应

- 池序 444；P2 打磨；PC；发卡=lead（2026-11-21 凌晨）；验收负责人=lead。
- 背景：438 真栈手测观察项——步骤卡恒为数字输入框（type=number），词答案（「模进」「已装载」「完成」）被拒，用户被迫改走对话 composer（438 裁定档观察节在卷）。

## 目标（零栈小卡）

1. TutorStepCard 输入型自适应：服务端在 start/chat 的 workflow_data 当前步增加 `answer_kind`（`number`|`text`，服务端按 expect 是否纯数字判定，默认 number 兼容 v0）；UI 按 answer_kind 渲染数字框或文本框（提交链路不变）。
2. v0 兼容：workflow_data 无 answer_kind → 数字框（现行为）；437/438 用例不回归，新增 number/text 两态用例。
3. 文件域：`agent/internal/chat/tutor_sim.go`(+test)、`agent/webui/src/project/`（TutorWidgets/projectService 类型面）；与 398（scoreworkbench 域）不相交。

## 验收

- go test ./internal/chat 定向+vitest+build exit 0（回执附原始输出）；真栈手测并入 443 的窗口。

## 领取记录（2026-11-21 inputtype444）

- 领取时间：2026-11-21；领取时基线：main afe7ca6c2b5f82b79faa9a42fd47d439c2065e1a（origin/main=a6420951，本地领先 10+ 提交、远端无新提交；本机 git 环境推送视网络另行记录）。
- 执行 owner：inputtype444（GLM-5.3-Flash freeflash 免费车道执行子代理，PC）。
- 分支：pc444-input；worktree：D:/Vit_DAW_wt_pc444input（自 afe7ca6c 建立实现）。
- 领取提交：仅本卡 coord 文件（todo→doing 改名+回填），显式 pathspec 提交，不触碰 index 中其他卡（SCORE-GUI-FOLLOW-1 todo 副本清理）的暂存状态。
- 文件域：agent/internal/chat/tutor_sim.go(+test)、agent/webui/src/project/{TutorWidgets.tsx,projectData.ts,projectService.ts,ProjectSurface.tsx} 及同目录测试；与 398（scoreworkbench 域）不相交。
