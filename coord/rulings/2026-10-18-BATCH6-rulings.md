# 2026-10-18 午窗 批量六裁定（92 终验 pass + GUI 实时链路/主整合立项）

- 验收会话：12:00-18:00 决策流（DSH PC，本机）；裁定来源=用户 2026-10-18 午窗 R2 复测「没问题了 / GUI 改版可以验收」+ 新增观察与主整合要求。
- 基线：origin/main=**b9f26258**（本轮探针工件 cherry-pick 后）；本批合入后 main=__MAIN_AFTER__。
- 执行侧：DSH exec-fe-gui-c（92-R1/R2）；协调面由 12:00-18:00 决策流单写。

## 一、FE-GUI-C-FLOATING（92）——**终验 pass（GUI 改版范围）**

- 用户复测：R1（抽屉层级 / F2 面板定位 / 过渡动效）通过；R2（mixer 双窗越界裁剪 / 窄槽可用性）通过；用户结论「没问题了」。
- 交付：前端仓 **port/fe-gui-shrink-c-floating@51a9882**（R2 最终）；主仓探针 **port/fe-gui-shrink-c-probe-1@74110cfe**；R1/R2 电池与视觉证据已 cherry-pick 入 main（6effecc9/b9f26258）。
- 验证（历次亲核汇总）：Phase C 探针 319 钉 PASS（R1 167 全保留）、回归电池 A 69/B 119/vitnote 六/FE-RACK 76 全 exit 0、红线门 924318d..51a9882 零命中、双窗/四窗视觉证据亲看（抽屉浮于台前、F2 在走带栏下、mixer 裁剪、浮卡落主区）。
- **边界（如实登记）**：①本卡验的是前端 GUI 改版；**GUI↔内核实时数据链路不在本卡验收面**（见 §二）；②真实栈 GUI 腿（`-StartUI`）历史未覆盖（91/92 同款：内核先绑端口致 UI 启动被跳过）——主整合后由用户目检 + 实时链路卡覆盖；③窗口态离屏诊断 2 次/轮按已采信边界（离屏、skip dev autostart、Dummy、未占栈、已退出）。
- 卡 92：**done / 验收 pass**。

## 二、新发现→**MIXER-LIVE-REFRESH-1（池序 141，P1，已立卡并派发）**

- 用户观察：新建轨道后 mixer 推子不刷新。
- 事实核验：**不是"GUI 未连内核"**——用户 R2 手测期间主检出 kernel `VitHeadlessServer2026-10-18_14-43-23.log`、`agent_last.log`(14:43:42)、`vsp_hub_last.log`(14:43:37)、baker 日志均在卷，GUI/内核/agent/hub 链路在跑。
- 代码锚点（Lead 核）：`vit_dock/views/mixer_dock_adapter.gd` `_bind_project_events()` 仅连 `project_loaded` / `track_focus_changed`；`_rebuild_strips()` 触发点无"轨道增删/重命名"事件；`_bind_ipc()` 未在 track 命令完成后重建 strips → 新建轨道后 strips/推子不同步。
- 处置：立卡 `MIXER-LIVE-REFRESH-1`（P1，0.5 天，前端仓 C 线先修；随后主整合带上）。派发=exec-fe-gui-c（task-10），要求事件链取证→最小事件订阅修复→探针补钉（R1/R2 319 钉不回退）+回归电池+红线。**不在 92 卡内扩域、不阻塞 92 GUI 改版验收**。

## 三、**FE-GUI-MAINLINE-1 预立（主整合/移植回原 GUI，P1；范围待用户确认）**

- 用户要求：「把这版 GUI 移植回原 GUI 上」。可行性已核：
  - 前端仓 `main`=5280822 **是** `port/fe-gui-shrink-c-floating@51a9882` 的祖先（落后 18 commits）→ **可 fast-forward**；
  - 启动页改版 `port/fe-start-feed-1@76201ea`（FE-GUI-START-FEED-1 done/pass，用户目检待排期）与机架插件列表 `port/fe-rack-list-1@7c3a7f2`（FE-GUI-RACK-LIST-1 done/pass，用户目检待排期）均为 main 后 15 commits 的独立分支，`git merge-tree` 预检：与 main 合并**无冲突**。
- 拟定流程：①先在 C 线完成 MIXER-LIVE-REFRESH 修复；②main FF→C；③按用户确认范围合入 start-feed / rack-list；④合并树全探针+回归电池+红线门；⑤用户主检出 `D:/Godot/project/vit-daw-frontend` 切到 main（现状：该检出在 port/fe-gui-shrink-1，tracked 无本地改动，仅 untracked 运行时目录）；⑥用户目检（新 GUI/启动页/机架列表）。
- **范围待用户确认**：A) 仅 C（GUI 改版）；B) C + 启动页 start-feed；C) C + 启动页 + 机架列表（推荐：三项均已验收，一次归一减少碎片）。

## 四、启动页状态（答用户问）

- **启动页改版已做且已验收**：FE-GUI-START-FEED-1（磁带工程流启动页，2026-10-12 done/pass，用户目检待排期），分支 **port/fe-start-feed-1@76201ea**——**但尚未合入前端仓 main/用户主检出，所以现 GUI 里看不到**；将在主整合（§三）中合入。
- 启动页「进入工程」bug：今天已修（FE-START-EMPTY-DRAFT-1(118) 最小修复 pass，复用空壳 draft 消除双 draft）；根因卡 **137 FIX-START-DRAFT-SIDE-EFFECT-1（P1）** 与 138/139/140 均在池待用户确认。

## 五、剩余任务池（建议优先级，供用户排序）

- **立即**：MIXER-LIVE-REFRESH-1（已派）→ FE-GUI-MAINLINE-1（待范围确认后派）。
- **待用户确认派发**：137 读路径物化 draft+reader（P1）、138 42MB 启动恢复（P1 候选）、139 track.list 0 轨取证（P2）、140 测试隔离（P2）、126 KERNEL-L3-TIMEENERGY-1（P2 已确认）、136 CAP-BOUNDARY-1（P2 已确认，须排池序 54 后串行）。
- **依赖/设计面**：129 FE-RACK-MACRO-LAYER-DESIGN（决策侧迭代领取中）→ 130 FE-SENDROUTE-MACRO-GUI（依赖 129 定稿）。
- **其它池（需逐卡核前提/用户确认）**：VIS-FILM 系列（A1/B1/B2/B4/C1/C3/D1）、GENSHELL 系列（GUI/AUDITION）、GEN-VOCAL-SYNTH、AIGC-DRUM-GEN、DEFENSE-SHOW-1、FE-MOJIBAKE（Mac 线）、ARTIST-AUDITION 等。
- **今日已收口**：90/118/132/133/134/135/92（pass）与 133 纪律闭环。
