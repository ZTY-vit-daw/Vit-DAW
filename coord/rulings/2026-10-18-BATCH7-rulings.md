# 2026-10-18 晚窗 批量七裁定（142 前端仓主整合 pass + 交付后步骤）

- 验收会话：12:00-18:00 决策流（DSH PC，本机）；裁定来源=用户 2026-10-18「把这版 GUI 移植回原 GUI 上」+ 范围确认（全部合入）+ 后续手测安排。
- 基线：origin/main=**7ed94235**（栈登记提交后）；本批合入后 main=8c643a49。
- 执行侧：DSH exec-fe-gui-c（task-11）；协调面由 12:00-18:00 决策流单写。

## 一、FE-GUI-MAINLINE-1（142）——**pass（前端仓 main 归一完成）**

- 交付：前端仓 **main=3ab4dfdb**（纯 fast-forward push `5280822..3ab4dfd`；未 force、未改写历史；独立 worktree `D:/Godot/project/vit-daw-frontend-mainline`，**未动用户主检出**）。
- 合并序列（冲突=0）：
  1. `merge --ff-only port/fe-gui-shrink-c-floating@be6da5d` → main=be6da5d（Phase A/B/C + 92-R1/R2 + MIXER-LIVE-REFRESH-1）；
  2. `merge --no-ff port/fe-start-feed-1@76201ea` → clean merge，9 文件 +1738/−9，merge commit **7cf8da9**（启动页磁带流改版）；
  3. `merge --no-ff port/fe-rack-list-1@7c3a7f2` → clean merge，2 文件 +480/−105，merge commit **3ab4dfd**（机架插件列表分组）。
- 决策侧亲核：远端 main hash 亲核（`3ab4dfdb…` 与回执一致）；merge commit 链（3ab4dfd←7cf8da9←be6da5d←5280822）；`--import` log **SCRIPT ERROR=0**（唯一 ERROR=godot_cef dll 环境提示，Phase A/B/C 同源边界）；合并树电池日志尾亲读（Phase C **342** / Phase B **119** / Phase A **69** / vitnote 六 / rack-list **109** 全 PASS）；start-feed probe **140 PASS**、rack-list probe **109 PASS**；红线门 `924318d..3ab4dfd` 47 文件 **9 红线零命中**（执行侧 git 门，采信+抽查）。
- 工件：`coord/runs/FE-GUI-MAINLINE-1/20261018_145452/`（README/import log/battery/两探针日志）；探针工件 commit **90069383**（已 cherry-pick 入主仓 main，连同 MIXER 探针工件 cf2f39a7）。
- 边界：headless 探针=端侧门（AGENTS §5）；真实视觉/交互（新 GUI 四台前/浮卡真窗/启动页磁带流试听/机架列表分组观感）归用户目检；主整合未占真栈。
- **主整合归一挂账（2026-10-14-BATCH1「前端仓 main 归一」）解除**；卡 142 归 done。

## 二、交付后步骤（用户侧）

1. **用户主检出切换**：`D:/Godot/project/vit-daw-frontend`（现 `port/fe-gui-shrink-1@c56e38f`，tracked 无本地改动）→ 切到 **main@3ab4dfd**（`git switch main && git pull --ff-only`；untracked 运行工程/缓存由用户决定保留）。Lead 可代执行（不在本卡内改用户工作树）。
2. Godot「项目 → 重新加载当前项目」/重开工程 → F5 → 启动页 → 进工程。
3. **目检四项**：①新 GUI（四台前/F2/抽屉/浮卡真窗/底栏/动效）；②启动页磁带流/大气层/切换与试听态；③机架右键插件列表分组；④新建轨道 → mixer 即时刷新（MIXER-LIVE-REFRESH-1 随本次合入）。
4. 目检发现项 → 另卡返工（92/MIXER 的卡面验收以用户真栈复测为准，见 §三）。

## 三、真栈腿窗口与纪律（2026-10-18 晚窗）

- 已登记 PC-RUNTIME-STACK **17:50–18:20**：**137 FIX-START-DRAFT-SIDE-EFFECT-1 真栈腿先跑**（四断言 A/B/C/D；LLM=0；等待回执）→ 拆栈复核=0 后发 **139 TRACK-LIST-GAP-1 正式同帧对照 run** GO（两腿串行、不得并行）。
- **139 owner 14:58 未登记未 GO 启动事故（自报）**：因 `-PreflightOnly` 字符串替换 bug 意外完整起栈 ~75–80s（只杀自起 kernel/vsphub/agent；拆净复核=0；用户栈未在跑）；该 run=harness 干跑、不作证据；整改已落地（75f3caf2：`-PreflightOnly` 实测只预检不启栈 + 比较口径按 user-track 修正）。已入 PC-RUNTIME-STACK 纪律记录。
- **137 测试隔离偏差（如实登记）**：未隔离首轮测试向真实 draft 根写 18 个测试工件 → 已移入 `%TEMP%\fedraft1\startside_test_quarantine\`（未删；并入 140 处置）；harness.go 注释损坏已 checkout 恢复后用 edit 重放，最终 diff 以 b8f0bb9e 为准。

## 四、待办/下一批

- **140 测试隔离**：137 验收后解除 blocked（task-14），exec-start-draft 串行执行。
- **139 正式腿**：137 拆栈后 GO；结果用于 118 §2.5 取证收口 + 另立修复卡（`FIX-TRACKLIST-SHADOW-FRESHNESS-1` 草案：工具面以本命令内核回包为准/shadow_stale 显式披露 + 未知新轨 delta 去抖触发权威刷新）。
- **138 42MB 启动恢复**：待用户确认语义（不恢复/最近工程/显式）后派。
- **126 KERNEL-L3-TIMEENERGY-1 / 136 CAP-BOUNDARY-1**：已确认待派（136 须排池序 54 后串行）。
- **129 宏控制层设计**（决策侧迭代）→ 130 依赖 129。
- 用户目检反馈为 92/141/142 三卡的最终收口依据。
