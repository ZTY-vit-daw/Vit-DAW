# RECENTS-HYGIENE-1 PROGRESS（freeflash 免费车道子代理·PC）

- 卡面：coord/cards/doing/2026-11-10-RECENTS-HYGIENE-1.md（池序 342，P2）
- 分支：port/recents-hygiene-1；worktree：D:/Vit_DAW_wt_recents1
- 注意：本 run 目录在主检出的 coord/ 下——快照/备份/清单留此目录不入 git；取证脚本与建议书入 port 分支。

## 步骤日志

- [x] M1 领取（2026-11-14）
  - fetch 复核 origin/main=a4759235（与卡面一致）；卡在 todo/、已确认、零依赖；与 doing 三卡域不相交
  - worktree add D:/Vit_DAW_wt_recents1 -b port/recents-hygiene-1 origin/main，工作树干净
  - 卡面回填领取段 → git mv todo→doing → add 新路径 → coord-only 提交
  - 首推遇网络中断（kex_exchange_identification abort，重试 3 次均 non-fast-forward——远端已被 5e9f2766 UI-FOUNDATION-1 领取推进）
  - 按 PROTOCOL §2.1(3) 重读远端：同卡仍无 owner → rebase 自己未推送的领取提交到新 origin/main → 推送成功
  - 领取提交 hash：06f333cc（已推 main，远端卡面 owner 核对通过）
  - 下一步：M2 取证——内核 recents 存储定位
- [x] M2 取证（2026-11-14）
  - 存储定位：NOT Settings.xml → %APPDATA%/Vit-DAW/global_project_config.json（VitGlobalProjectConfig.cpp，max 10，prepend+尾删，无时间戳）
  - 泄漏路径：VitHeadlessService.cpp 三处 open/save/save_as 成功后无条件 prependRecentProject；脚本侧实证 gov_ttl_replay_driver.py（save_as+open）与 waveshell_stackleg_smoke.py（save_as）
  - 清单：当前 10 条全 fixture（govttl 327 ×4 + waveshell 309 ×6），用户真实工程 0 条=已被挤出（附带损害，候裁定）
  - 339 覆盖范围：还原=会话窗口净差零，基准快照已含 11-09 晚泄漏→原样保留并存；当前文件 sha256==339/340 before 快照（c3543f45）
  - 产物：forensics/recents_inventory.json + forensics/FORENSICS.md；backup/global_project_config.pre_cleanup_*.json（字节一致亲证）+ Settings.xml 证据快照
  - 下一步：M3 清理（写前再核进程为零）
- [x] M3 清理（2026-11-14 18:23:04）
  - cleanup_recents.py（清单精确匹配，非模式匹配；ABORT 防线防误删用户工程路径）
  - 先副本演练（10 删 0 留 exit 0）→ 进程即时复核全零 → 活文件原子写（tmp+os.replace）
  - 结果：removed=10 / kept=0 / added=0，活文件={"recent_projects": []}（内核 juce 风格）
  - pre 备份字节级保留（sha256 c3543f45，可回滚）
- [x] M4 亲证（2026-11-14）
  - 零残留：grep govttl|waveshell|stackleg = 0 命中；live sha256==post 快照
  - 零触碰：recents 用户条目前后均 0（无新增改写）；projects/ 用户文件 mtime 全早于 run 窗口；Settings.xml sha256 零漂移；workspace git status 与领取基线一致
  - 产物：diff/cleanup_diff.json + diff/ZERO_TOUCH_PROOF.md
  - 下一步：M5 隔离机制建议书
- [x] M5 隔离机制建议书（2026-11-14）
  - ISOLATION_PROPOSAL.md：a=独立用户配置目录（env 覆盖，结构性，需另卡）；b=后置还原常态化（零改动，纪律依赖+崩溃窗口+用户竞态+不清历史）
  - 建议：b 即刻升格装置纪律 + a 另卡收口（VIT_DAW_USER_CONFIG_DIR）；用户 recents 重建与否候用户裁定
  - 下一步：M6 取证脚本/清单/建议书入库 port 分支
- [x] M6 工件入库 port 分支（2026-11-14）
  - port/recents-hygiene-1 提交 1a502bfd（7 文件：forensics×2+diff×2+建议书+脚本+README）+ 0ce6ade5（时间戳笔误修正）
  - main 回执路径：协调分支（基于 origin/main a52c1c07）仅重放卡片 doing→done，e76ed57d 推 main（工件不随回执进 main，候主管验收 cherry-pick）
  - 备份/快照/PROGRESS 留 run 目录不入 git（按派发词）；PENDING 命名副本改名 precopy_second_20261114_182019
- [x] M7 回执（2026-11-14）
  - 卡面回填领取段+回执段（清单/清理亲证/零触碰/建议书链接/覆盖边界声明/停止条件核对/遗留裁定项）
  - git mv doing→done → add 新路径 → coord-only 提交；远端 done/ 卡面回执核对通过
- DONE（2026-11-14）。四件套见会话报告；遗留候主管裁定：隔离 a/b 取舍+用户 recents 是否重建。
