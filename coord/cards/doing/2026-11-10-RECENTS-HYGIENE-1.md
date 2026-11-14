# RECENTS-HYGIENE-1：用户 recents 被烟测装置工程污染（排查+清理+隔离机制）

- 发卡：GLM 主管决策流 / 2026-11-10 晚（依据=手测 v2 首验截图：启动面 recents 列表含 `govttl_round_20261109_200213`、`waveshell_stackleg_fixture_render_direct_surge_xt` 等装置/烟测临时工程——用户「进入了不知道哪个的工程」的直接污染源之一）
- 派发确认：已确认（主管发卡；P2——341 修复精选渲染后影响面缩小，但卫生问题独立存在）
- 验收负责人：GLM 主管决策流
- 池序 342；P2 / 0.5 天 / 依赖=零；L1 / PC；零栈为主（产品状态域）

## 目标

1. **取证**：recents 全量清单化（内核 recents 存储位置+条目来源分类：用户真实工程 vs 各代烟测/装置工程——309 八 run/339-340 复跑/govttl 装置等）；泄漏路径定位（装置 open 的工程为何入用户 recents——339 曾做「recents 原样还原哈希亲证」，查它还原的覆盖范围与更早泄漏的并存）；
2. **清理**：装置条目清出（先备份 recents 存储；用户真实工程条目零触碰）；
3. **隔离机制候裁定**：装置工程 recents 隔离方案（a. 烟测装置统一走独立用户配置目录/隔离 workspace——AGENTS §10 精神推广到 recents 面；b. 装置腿后置 recents 还原常态化=339 先例推广为装置纪律条款）——出建议候主管裁定，不自行改内核。

## 文件域

产品状态域（内核 recents 存储，先备份）+coord/runs/RECENTS-HYGIENE-1/ 工件+卡面；零内核代码（隔离机制若需内核改动→裁定后另卡）。

## 验收标准

清单+清理 diff（装置条目零残留亲证）+用户真实工程零触碰+隔离机制建议书。

## 停止条件

recents 存储格式未知/写保护→取证上交候裁定。

## 领取/回执/验收

### 领取（2026-11-14）

- 领取时间：2026-11-14（免费车道窗口）
- 领取时 origin/main：`a4759235136725229d48406659dccf37ed9ed8f7`（fetch 复核一致）
- 执行 owner：freeflash（GLM-5.3-Flash 免费车道子代理）·PC·ZCode 主管会话派发
- 执行分支：`port/recents-hygiene-1`
- worktree：`D:/Vit_DAW_wt_recents1`（领取时工作树干净，`git status --short` 空）
- 文件域核对：与 doing 三卡（CLOSURE-ROUTE-ADAPT-1 / ARRLOOP-AUTHBRIDGE-1 / WAVESHELL-LOADING-FORENSICS-1，域均在 agent/chat/harness/渲染面）不相交；本卡域=产品状态（内核 recents 存储，主检出活文件）+coord/runs/RECENTS-HYGIENE-1/ 工件+卡面，零内核代码
- 领取提交：本提交（coord-only，todo→doing）
