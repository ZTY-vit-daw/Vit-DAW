# 2026-10-14 批量六裁定（109 rerun6 双 exit 0：终验收+三账销账+108 销账+M2 体验场次解锁）

> 验收人：GLM 主管决策流（ZCode 主会话）。触发：用户"任务卡完成，可以检查"。
> 方法：回执亲读+工件亲核（console×2 尾行 D1-S1 PASS+报告 verdict=pass×2+user_judgment_pending 停泊证据×2+stale/TERMINAL 零命中+LLM 探针在场）；本腿零代码改动无需亲跑。

## ① 109 rerun6 —— **终验收 pass：两轮连过 exit 0（run12 20261014_211848 + run13 20261014_212916），本卡历史首次**

- **115 新锚点 2/2 真栈生效**：合法停泊 user_judgment_pending 存活过全部 2s 轮询窗口（run12 7 连/run13 5 连），全程零 project_revision_stale/零 FS-SETTLE-TERMINAL（console 亲扫）；对照 rerun5 停泊 1s 内被压过=修复生效实锤。
- 既有锚点在场：112（run9 形态未复发，标记 0 次=未触发）；113（run12 新文案在场+模型成功换视图）；腿 E（全字段+骨架教学，一次教学即收敛）。
- 模型口径行核过：deepseek-flash 思考档 config 全程未动，预探测 200/echo flash/reasoning 23（探针在卷）。
- 卡面验收标准全项达成（跨腿累计）：diff 亲读（B-i/C/D/E 四腿）+seam 设计依据+回归测试钉+95 包 0 FAIL（决策侧三次独立亲跑）+**D1 reverb exit 0×2**（工件含 console+summary+模型口径行）。**109 卡终验收 pass，移 done。**
- delay 加分腿可选未跑（exit 0 即停，卡面判据如此）——如实登记，不追补。

## ② 三笔挂账 —— **销账（补记入卡）**

- M2-TIMEFX-1（挂账 run 111841：API 额度恢复后复跑一轮 exit 0）——**销账**：rerun6 两轮 exit 0 同基线（deepseek 官流）达成且超额（两轮非一轮）。
- M2-TIMEFX-2（挂账 run 195448）——**销账**：同上证据。
- FIX-106（挂账：修复面真栈 exit 0）——**销账**：同上证据（FIX-106 修复面经 109 全系修复链后真栈两轮通过）。
- 销账证据锚：coord/runs/FIX-FS-SETTLE-DEADCHAIN-1/{console_d1_reverb_run12,run13}.txt + run_20261014_{211848,212916}/d1_smoke_report.json（verdict=pass×2）。

## ③ 108 TIMEFX-D1-EXIT0-DEEPSEEK-1 —— **blocked 解除销账转 done**

- 该卡 blocked 条件="收口腿 exit 0"——rerun6 双 exit 0 即其目标达成（D1 reverb 全链 deepseek 基线）；其发现的空态 settle 死链经 109 线（B-i+C+D+E+112+113+115 七腿）修复闭环。**复验采信=rerun6 证据，销账转 done。**

## ④ M2 体验场次 —— **解锁（等用户排期）**

- 解锁条件（10-04 约定：M2 exit 0 后同基线排）达成：基线=当前 main（含全系修复）+deepseek-flash 思考档。
- 场次内容：时域效果器控制第一个可感功能（混响衰减语义干预链）用户亲手体验——话术「给这条轨加一点混响（衰减短些）」（10-12 手测 B 话术修正裁定）+人工 A/B 试听（停泊 user_judgment_pending 即等该裁决）。手测入口=用户从 Godot 拉起前端进工程（AGENTS §5）。
- 体验场次中发现的断裂按 finding 开卡（既定口径）。

## 工件与收口

- 109 卡移 done；fsdc1r2 worktree 拆除（rerun6 分支 origin 留基线标记）；三挂账卡+108 卡销账补记随本批。
- 裁定文件=本文件；main=本批 commit。
