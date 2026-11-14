# RECENTS-HYGIENE-1 清理亲证（执行侧自验）

- 清理执行：2026-11-14 18:23:04（cleanup_recents.py，exit 0；写前进程核查 VitApp/VitAgent/godot 全零）
- 数据三件套：pre=backup/global_project_config.pre_cleanup_20261114_182019.json、post=snapshots/global_project_config.post_cleanup_20261114_182304.json、diff=diff/cleanup_diff.json

## 1. 装置条目零残留亲证

- 清理前 10 条（govttl×4 + waveshell×6）→ 清理后 0 条；`{"recent_projects": []}`
- 残留 grep（govttl|waveshell|stackleg，-i）命中数 = **0**
- diff：removed=10（全部为清单精确匹配条目）/ added=0 / kept=0
- 活文件 sha256 == post 快照 sha256（清理后无其他写入者）
- 活文件格式 = 内核 juce 风格单行 JSON（与清理前同风格），`loadRecentPathsFromDisk` 解析空数组返回空（VitGlobalProjectConfig.cpp L44-55），`get_recent_projects` 将回空列表=启动面 recents 干净

## 2. 用户真实工程零触碰亲证

- recents 面：清理前用户真实工程条目 = 0（已被 11-09 fixture 洪泛挤出，见 FORENSICS.md §4）；清理后 = 0；diff 中 added=0/kept=0——本 run 对 recents 的写仅删除 10 条已分类 fixture，无任何新增或改写
- 删除机制：cleanup_recents.py 按 inventory 精确路径匹配删除（非模式匹配）；脚本内建防线——若清单或保留集中出现 `Vit-DAW\projects` 标记的用户工程路径即 ABORT（返回码 3），本次未触发
- 用户工程文件面：`%APPDATA%\Vit-DAW\projects\` 下 手测·小星星乐局.vit、实验·ProQ3 裸控 r1/r2/r3.vit、_320_run8_默认态存档_20261109 等 mtime 全部早于本 run 窗口（2026-11-08/09），本 run 零写入（证据：diff/cleanup_diff.json 的 user_projects_dir_evidence_at_verification 字段）
- 主检出运行时权威文件零触碰：Settings.xml sha256 写前=b773c82d… =清后=b773c82d…（零漂移）；VitApp/Workspace git status 清理前后同为 ` M Settings.xml`+` M default_project.xml`（既有未提交改动，本 run 未 stage/未 commit/未修改）
- 回滚能力：pre 备份字节级保留（sha256 c3543f45…），如需还原全部 10 条可整文件复制回写（是否回滚=主管裁定）

## 3. 边界声明

- 本卡清理仅覆盖 recents 存储单文件；fixture 工程文件本体（%TEMP%\govttl_327_projects\、wt_309recon run 目录）不在本卡文件域，未触碰
- 精确恢复被挤出的用户 recents 清单不可行（无早于 11-09 23:19 快照）；重建候选=projects/ 目录五项，置顶顺序与是否重建候主管/用户裁定
