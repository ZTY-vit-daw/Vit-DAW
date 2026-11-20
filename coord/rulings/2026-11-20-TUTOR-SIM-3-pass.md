# 裁定：TUTOR-SIM-3 控件链真栈 PASS

- 日期：2026-11-20 18:30 +0800
- 卡：437 TUTOR-SIM-3（状态/进度卡 + 步骤输入卡）
- 裁定：**真栈浏览器级 DOM 手测 PASS**（lead 亲跑，用户批准「lead 亲跑」）。
- 代码：553c187c → main b0c83c85（11-19 已合入）；本轮零代码改动，被测=agent `1739E82C…` + webui dist `d118136B…`（437 后重建）。
- berth：coord/runs/TUTOR-SIM-3/berth_20261120_1810（快照/哈希/截图/teardown 全卷）。

## 手测事实（全链 DOM 断言）

- 打开 `D:/Vit_DAW_handtest/TUTOR-SIM-1/live.vit`（经 `/agent/invoke project.open confirmed=true`；浏览器直连下对话式打开因 Vit 桥不可用诚实降级，符合 UI-BRIDGE 设计）→ `?surface=project` 工程面。
- **自动开场**：ProjectSurface 自动调 start（无命令词），LLM 人设开场进对话流；TutorProgressCard=「第 1/3 步」、TutorStepCard=问题文本+数字输入框+提交键（空输入禁用）。
- **错输入不推进**：输入 0 → 回复 YAML retry 文本（逐字一致），进度卡保持 1/3，输入清空、提交回禁用。
- **7 → 2/3；42 → 3/3**：pass 文本+进度卡同步推进；42 腿经 dom_cua 真实鼠标点击提交成功。
- **123 → completed**：进度卡「第 3/3 步 · 已完成」、步骤卡「实验已完成」、输入框与提交键双禁用，附「导师实验已完成，无需继续输入」。
- **普通工程零 UI 变化**：打开 `W02-A1-v0/live.vit`（无 tutor_sim.yaml）→ 工程面无进度卡/步骤卡/原始状态文本。
- 截图：shot_1of3_initial.png / shot_completed.png。

## 边界与备注

- 提交按钮 Playwright 定位器 click 两次被 actionability 检查拦截；dom_cua 真实点击成功且 Enter 路径可用 → **产品面可点性成立，判定为测试面伪象**（不构成缺陷；E2E-WEBUI-1 装置建设时复核）。
- agent stdout 无 [tutor_sim] 行（logx 无 path、verbose off）；状态机证据=服务端判定驱动的 UI 收据（retry/pass 文本逐字来自 YAML、进度 fail-closed 渲染），435 已有服务器端 smoke PASS 在卷。
- 导师会话仍内存态；实验 schema 仍 vit.tutor_sim.v0。
- 本轮首次以浏览器级 DOM 断言覆盖 437 渲染面（436 裁定遗留的「无浏览器级 DOM 烟测」缺口在本卡补上；仍非可重复脚本，E2E-WEBUI-1 装置化留卡）。

## 下一步

- 437 done；TUTOR-SIM-4 已停卡（premise_falsified，见 coord/cards/blocked/…438…）：音频资产不存在+生产 done_when 非文本，待主管拆卡裁决。
- 音乐控件（谱面/Tap/播放器）随 SIM 线后续卡。
