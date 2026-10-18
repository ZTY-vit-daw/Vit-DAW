# FE-GUI-MAINLINE-1 工件索引（池序 142 / task-11）

- owner=DSH PC 执行流 exec-fe-gui-c；用户 2026-10-18 确认范围=**全部合入**（C 线 GUI 改版 + start-feed 启动页 + rack-list 机架插件列表）。
- 独立 worktree：`D:/Godot/project/vit-daw-frontend-mainline`（自前端仓 main=5280822 建，未动用户主检出 D:/Godot/project/vit-daw-frontend）。
- 合并后 main：**3ab4dfdb01191462ad4e208db364b3d8b235fe37**（已 fast-forward push origin/main，见回执）。

## 合并序列

| 步骤 | 来源 | 结果 |
|---|---|---|
| 0 | origin/main=**5280822**（FE-GUI-SHRINK Phase A 之后 main 基线） | worktree 检出 main |
| 1 | `merge --ff-only port/fe-gui-shrink-c-floating @ **be6da5d**`（含 Phase A/B/C + 92-R1/R2 + MIXER-LIVE-REFRESH-1） | fast-forward，main=be6da5d |
| 2 | `merge --no-ff port/fe-start-feed-1 @ **76201ea**`（启动页磁带流/大气层/试听 loader + probe 140） | **clean merge**（9 文件 +1738/−9），merge commit 7cf8da9 |
| 3 | `merge --no-ff port/fe-rack-list-1 @ **7c3a7f2**`（机架插件列表分组 + probe 109） | **clean merge**（2 文件 +480/−105），merge commit 3ab4dfd |

## 合并树验证（全部在本 worktree 实跑）

- `--import`（headless）：exit 0，**SCRIPT ERROR=0**（唯一 ERROR=addons/godot_cef 构建产物 dll 缺失的环境提示，与本合并无关，Phase A/B/C 同源边界）。
- **红线 git 门**：`git diff --name-only 924318d..3ab4dfd` = 47 文件，9 红线文件**零命中**。
- 全探针电池（`battery/`，C ps1 wrapper 对合并树实跑）：Phase C **342** / Phase B **119** / Phase A **69** / vitnote 六（52/141/143/取证型/26/16）/ rack-list probe **109**（合并后版本）——**全部 exit 0**。
- start-feed 探针（`start_feed_probe/`）：`tools/probe_start_feed_view.gd` **140 钉 PASS exit 0**。
- rack-list 探针独立复跑（`rack_list_probe/`）：**109 钉 PASS exit 0**（与电池内同脚本一致）。

## 端测边界

- headless 探针=端侧门（AGENTS §5，FE-RACK/91/92 先例口径）；真实视觉/交互（新 GUI 四台前、浮卡真窗、启动页磁带流/大气动效、机架列表分组观感）归用户目检。
- 本次未占真栈（用户/其它腿优先）；启动页磁带流真实音频试听依赖用户真栈/手测环境。

## 需用户目检

① 用户主检出切到 main 后（由 Lead/用户执行）：进工程 → 新 GUI（四位台前/F2/抽屉/浮卡真窗/底栏状态栏）与手测清单一致；②启动页：磁带流/大气层/工程预览悬停试听观感；③机架：插件搜索/列表分组/右键菜单无回归；④新建轨道 mixer 即时刷新（MIXER-LIVE-REFRESH-1 随本次合入）。
