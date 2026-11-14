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
- 领取提交：06f333cc（coord-only，todo→doing，已推 main）

### 回执（执行侧自验通过，2026-11-14，候主管验收）

- **清单**：recents 存储=`%APPDATA%\Vit-DAW\global_project_config.json`（内核 VitGlobalProjectConfig.cpp，max 10，无时间戳）——卡面线索 Settings.xml 证伪（其未提交改动为无关运行时权威输入，零触碰留证）。清理前 10 条**全部**为装置/烟测 fixture：govttl 327 装置 ×4（FIX-GOV-INTERACTION-TTL-1，%TEMP%）+ waveshell 309 stackleg ×6（wt_309recon）；**用户真实工程条目 0 条=已被 11-09 晚 fixture 洪泛挤出**（附带损害上交，projects/ 目录五项候选重建候用户裁定）。清单：port 分支 `coord/runs/RECENTS-HYGIENE-1/forensics/recents_inventory.json`
- **泄漏路径**：VitHeadlessService.cpp 三处（open/save/save_as 成功后）无条件 `prependRecentProject`，无调用方区分；脚本侧实证 gov_ttl_replay_driver.py（save_as+open_project）与 waveshell_stackleg_smoke.py（save_as）。339/340 还原=会话窗口净差零（哈希亲证），还原基准快照已含 11-09 晚泄漏→「装置净差零」与「用户面持续污染」并存；当前文件 sha256==339 before 快照==340 主腿 before 快照（c3543f45）——当前污染全部为 339 机制建立前的泄漏。报告：port 分支 `coord/runs/RECENTS-HYGIENE-1/forensics/FORENSICS.md`
- **清理亲证**：cleanup_recents.py 清单精确匹配删除（ABORT 防线防误删用户路径；副本演练→进程即时复核→原子写），removed=10/added=0/kept=0，活文件终态 `{"recent_projects": []}`（内核 juce 风格）；零残留 grep（govttl|waveshell|stackleg）=0 命中；live sha256==post 快照。备份字节级保留（pre sha256 c3543f45，可回滚）。亲证：port 分支 `coord/runs/RECENTS-HYGIENE-1/diff/ZERO_TOUCH_PROOF.md` + `diff/cleanup_diff.json`
- **用户真实工程零触碰**：recents 面用户条目前后均 0（无新增改写）；projects/ 用户文件 mtime 全早于本 run 窗口；Settings.xml sha256 零漂移（b773c82d 前后一致）；VitApp/Workspace git status 与领取基线一致（既有 2 个 ` M` 未 stage 未 commit）
- **隔离机制建议书**：port 分支 `coord/runs/RECENTS-HYGIENE-1/ISOLATION_PROPOSAL.md`——a=独立用户配置目录（env `VIT_DAW_USER_CONFIG_DIR` 覆盖，结构性根除，需另卡+真栈验收）；b=后置 recents 还原常态化（339 先例升格装置纪律条款，零改动即刻生效）。**建议双轨：b 即刻+a 另卡收口，b 为老内核兜底**；候主管裁定
- **工件**：port 分支 `port/recents-hygiene-1` 提交 `1a502bfd`（coord/runs/RECENTS-HYGIENE-1/ 7 文件）；磁盘全量（备份/快照/PROGRESS.md，不入 git）=主检出 `D:/Vit_DAW/coord/runs/RECENTS-HYGIENE-1/`
- **端测覆盖边界声明（AGENTS §5）**：本卡为零栈产品状态域任务（不占真栈），验收面=存储文件取证/diff/哈希对账/清理亲证，无渲染面与用户旅程改动；启动面 recents 呈现效果建议用户下次手测时目检（非本卡门槛）。内核未运行（全程三轮进程核查全零），清理写入不构成启动影响（recents 仅被 get_recent_projects 读出呈现，空数组解析路径已核）
- **停止条件核对**：存储格式已明、文件可写、清理不影响内核启动/工程状态加载——三条均未触发
- **遗留候裁定项**：①隔离方案 a/b 取舍；②被挤出用户 recents 是否重建（候选五项，精确原序不可恢复）

## 验收（主管 2026-11-14 晚窗）

- 裁定文件：[rulings/2026-11-14-RECENTS-HYGIENE-1-pass.md](../../rulings/2026-11-14-RECENTS-HYGIENE-1-pass.md)（**pass**；主管亲证：活状态终态空数组 / VitHeadlessService.cpp:1207/1353/1475 锚点亲 grep / Settings.xml b773c82d 零漂移重算 / cleanup_report removed=10）
- 裁定要点：①隔离双轨——b 案即刻生效（AGENTS §10 新装置纪律条款）+ a 案另卡 REC-ISOLATION-CONFIGDIR-1（池序 351，env=VIT_DAW_USER_CONFIG_DIR，fail-open，P3 候栈）；②用户 recents 重建**候用户裁定**（不擅自写用户数据），候选五项在 `$APPDATA/Vit-DAW/projects/`
- **用户裁定（2026-11-14 晚，主管会话转达）**：recents 重建选 **b——留空自然重建**，不做一次性写入。本卡遗留项②关闭。
- 验收 commit：工件自 port/recents-hygiene-1 合入 + 本回填同批（见 git log，coord-only）

