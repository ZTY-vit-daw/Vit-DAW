# RECENTS-HYGIENE-1 工件索引

- 执行：freeflash（GLM-5.3-Flash 免费车道子代理）·PC·2026-11-14；分支 port/recents-hygiene-1（本目录即分支内副本）
- 磁盘全量工件（含不入 git 的备份/快照）：主检出 `D:/Vit_DAW/coord/runs/RECENTS-HYGIENE-1/`（backup/ snapshots/ diff/ forensics/ PROGRESS.md）

## 本分支内（入库，验收评审面）

- `forensics/FORENSICS.md` — 取证报告：存储定位（%APPDATA%\Vit-DAW\global_project_config.json）、清单分类、泄漏路径（VitHeadlessService 三处无条件 prependRecentProject+两装置脚本实证）、11-09 晚泄漏时间线哈希对账、339 还原覆盖范围并存结论
- `forensics/recents_inventory.json` — 清理前 10 条全量清单+来源分类（govttl 327 ×4、waveshell 309 ×6、用户 0）
- `diff/cleanup_diff.json` — 清理 diff（removed=10/added=0/kept=0，前后 sha256，用户工程目录 mtime 证据）
- `diff/ZERO_TOUCH_PROOF.md` — 零残留+用户真实工程零触碰+Settings.xml 零漂移亲证
- `ISOLATION_PROPOSAL.md` — 隔离机制建议书（a 独立配置目录 / b 后置还原常态化；建议双轨，候主管裁定）
- `cleanup_recents.py` — 清理脚本（清单精确匹配删除、ABORT 防线、原子写、幂等；副本演练后执行）

## 仅磁盘（按派发词不入 git）

- `backup/global_project_config.pre_cleanup_20261114_181919.json`（sha256 c3543f45…，字节级可回滚）
- `backup/Settings.xml.pre_evidence_20261114_181919.xml`（既有未提交运行时改动留证，本卡零触碰）
- `snapshots/global_project_config.post_cleanup_20261114_182304.json` + `snapshots/cleanup_report_20261114_182304.json`
- `PROGRESS.md`（断点续跑日志，全程 7 里程碑）
