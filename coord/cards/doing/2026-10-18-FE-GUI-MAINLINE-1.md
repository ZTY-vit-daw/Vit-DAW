# FE-GUI-MAINLINE-1：前端仓主整合——GUI 改版/启动页/机架列表移植回原 GUI（frontend main 归一）

- 发卡：12:00-18:00 决策流 / 2026-10-18 午窗（依据=用户「需要把这版 GUI 移植回原 GUI 上」+ 2026-10-14 前端仓 main 归一挂账项到期）
- 派发确认：**已确认（用户 2026-10-18 选定=全部合入：C 线 GUI 改版 + 启动页 start-feed + 机架列表 rack-list）**
- 验收负责人：GLM 主管决策流（或当班决策流）
- 池序 142；目标仓库=**前端仓** `D:/Godot/project/vit-daw-frontend`；**P1** / 预估 0.5 天 / 依赖=**MIXER-LIVE-REFRESH-1 修复先合入 C 线**
- 模型分级：L1 / PC 前端执行流

## 目标

1. 在独立 worktree 把前端仓 **main fast-forward 到 C 线最终 commit**（现状：main=5280822 是 `port/fe-gui-shrink-c-floating` 的祖先，落后 18 commits；已预检可 FF）。
2. 按用户确认范围合入（merge-tree 预检与 main 无冲突，仍须合并树验证）：
   - `port/fe-start-feed-1@76201ea` = FE-GUI-START-FEED-1 启动页磁带工程流（done/pass，用户目检待排期）；
   - `port/fe-rack-list-1@7c3a7f2` = FE-GUI-RACK-LIST-1 机架右键插件列表分组（done/pass，用户目检待排期）。
3. 合并树验证：全探针电池（Phase A 69 / Phase B 119 / Phase C 319 / vitnote 六 / FE-RACK 76 + start-feed 140 钉 + rack-list 109 钉）全 exit 0；`--import` 零 SCRIPT ERROR；**红线 9 文件零命中**（git 门）。
4. **用户主检出 `D:/Godot/project/vit-daw-frontend` 切到 main**（现状：`port/fe-gui-shrink-1`，tracked 无本地改动、仅 untracked 运行时目录；切换前须用户确认并按需备份 untracked 状态，**不得 git reset/clean**）。
5. 用户目检清单：新 GUI（抽屉/F2/浮卡/真窗/底栏/动效）+ 启动页磁带流与切换/试听态 + 机架右键插件分组 + 进入工程（目标工程非新 draft）。

## 文件域

前端仓 main/相关分支与独立 worktree（合并操作在独立 worktree 完成、push main；**用户主检出只在第 4 步经用户确认后切换**）；主仓 `coord/runs/FE-GUI-MAINLINE-1/` 工件。不改主仓生产代码；不动前端红线 9 文件。

## 验收标准

- frontend main 含目标分支内容（C 线 + 用户确认的其它分支），远端一致；
- 合并树探针电池全 exit 0 + 红线门零命中 + `--import` 零 SCRIPT ERROR；
- 用户主检出切 main 后：构建/启动正常（Godot 拉起不报 SCRIPT ERROR）、新 GUI/启动页/机架列表按范围可见；
- 用户目检通过（或如实记录未过项与返工卡）。

## 停止条件

- 合并冲突/探针失败/红线命中 → 停并上报（不得强推/改写历史）；
- 用户确认范围外分支一律不合；
- 用户检出切换前必须用户确认，不擅自 switch/reset 用户工作树；
- 不在本卡修 mixer 功能（由 MIXER-LIVE-REFRESH-1 先在 C 线修好）。

## 关联

- 前置：MIXER-LIVE-REFRESH-1（141，P1）修复合入 C 线
- 内容：92 GUI 改版（done/pass，C 线）；FE-GUI-START-FEED-1（done/pass）；FE-GUI-RACK-LIST-1（done/pass）
- 长期挂账项：2026-10-14-BATCH1「前端仓 main 归一」由本卡收口

- 领取：2026-10-18 14:55 +0800 / 前端仓 origin/main=5280822（frontend main，C 分支祖先）/ owner=DSH PC 执行流 exec-fe-gui-c（模型+机器+会话）/ 合并 worktree=D:/Godot/project/vit-daw-frontend-mainline（新建）/ 目标分支=frontend main（fast-forward + merge 三支）/ 领取提交=待建；task-11（blocked_by task-10 MIXER 修复）；用户 2026-10-18 确认范围=全部合入
