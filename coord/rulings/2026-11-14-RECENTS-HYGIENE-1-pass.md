# RECENTS-HYGIENE-1 · 主管裁定：pass

- 日期：2026-11-14 晚窗
- 裁定人：GLM 主管决策流（ZCode 会话）
- 执行：freeflash 免费车道子代理（领取 06f333cc / 回执 e76ed57d / 工件 port/recents-hygiene-1 @ 1a502bfd+0ce6ade5）

## 证据核验（主管亲证，非转抄）

1. 活状态：`%APPDATA%\Vit-DAW\global_project_config.json` 终态 `{"recent_projects": []}`；备份/快照/清单在 `coord/runs/RECENTS-HYGIENE-1/`（磁盘全量）
2. 泄漏锚点亲 grep：`VitApp/Source/Service/VitHeadlessService.cpp:1207/1353/1475` 三处无条件 `prependRecentProject` ✓
3. 零触碰亲证：`VitApp/Workspace/Settings/Settings.xml` sha256 前缀 `b773c82d` 清理前后一致（主管重算）；Workspace 既有未提交改动未被 stage/commit
4. 清理报告：removed_count=10 全 fixture（govttl 327 ×4 + waveshell 309 stackleg ×6），pre_hash `c3543f45` 与 339/340 快照链对账一致
5. 卡面线索证伪处理正确：真实存储=`%APPDATA%\Vit-DAW\global_project_config.json`（Settings.xml 为无关运行时输入）

## 裁定

1. **验收 pass 收口**。取证-清理-建议书三段全达标；装置条目零残留、用户真实工程零触碰均有哈希级亲证。
2. **隔离机制双轨采纳**：
   - **b 案即刻生效**：升格为装置纪律条款，已写入 `AGENTS.md` §10（凡启动真实内核的装置腿=启动前快照+teardown 哈希亲证还原，还原失败非零退出按环境中断分类）。
   - **a 案另卡**：新卡 `REC-ISOLATION-CONFIGDIR-1`（池序 351，已发）——env 定名 `VIT_DAW_USER_CONFIG_DIR`，语义 **fail-open**（未设时维持现行为）；装置 launcher 注入 per-run 临时目录；真栈烟测验收；P3（栈空闲窗口执行，不占展演前主线）。a 落地后 b 保留为老内核兜底，不撤销。
3. **附带损害上交用户**：用户真实工程已被 11-09 晚洪泛挤出 recents（清理前 10 条全 fixture、用户条目 0），`$APPDATA/Vit-DAW/projects/` 五项候选可重建但精确原序不可恢复。**不擅自重建**（用户数据写入）；候用户裁定：一次性重建 / 留空自然重建。
4. 339/340 还原机制结论（净差零与历史污染并存）接受为在案事实，339 机制本身无罪、继续按 b 条款执行。

## 合入

- 工件自 port/recents-hygiene-1 合入 main（coord/runs/RECENTS-HYGIENE-1/ 评审面 7 文件）；本裁定与卡面验收回填同批提交。
