# FE-GUI-START-FEED-1 R2 工件索引（task-15 / 用户手测 6 条返工）

## R2.1 返工（Lead R2 预检驳回 #3 居中/缩放 → 已修，commit **bb30af1**）

- **缺陷根因**：卡在未父化时用 `PRESET_CENTER`，偏移按自身尺寸计算成 0 → 加入满视口宿主后矩形从视口中心向右下展开（1920 下视觉中心偏移、竖屏右下裁切）；翻转动画又把 scale 回写为 1.0。
- **修复**：`_apply_responsive_layout` 显式布局——anchors=top-left、size=560×340、pivot=卡中心、position=(viewport-CARD_SIZE)/2（pivot=中心时缩放后卡中心=视口中心）；翻转动画保持响应式 scale 目标；空白磁带卡同法居中。索引标签移至左下安全区（左 0–52%）、状态标签移至右下（锚定/右对齐/clip）→ 不再重叠。
- **探针新增 16 钉**（共 **186 钉 PASS**）：1920×1080 / 1366×768 / 900×1600（含 resize 重排）三尺寸下 `_cards_host` 满视口、卡视觉中心==视口中心 ±1px、卡 rect 全在视口内（安全边距≥8px）、缩放尺寸在界内、索引/状态矩形不相交。
- **视觉证据重拍**（`visual_evidence/`，截图含红色视口中心十字+青色卡视觉矩形+顶部量测文字）：`r2_display_1920x1080.png`、`r2_list_1920x1080.png`、`r2_new_tape_overlay_1920x1080.png`、`r2_display_1366x768.png`、`r2_display_900x1600.png`、`r2_list_900x1600.png`；shot 日志实测三尺寸 `delta=(0.00,0.00)`（`visual_evidence/shot_console_r2fix.log`）。
- **列表页整理**：页头「全部工程」+计数/操作提示；网格列数按最大 tile 宽推导（2–6）+ tile 横向撑满（96px 磁带 icon、200px 最小高）→ 不再是左上单行孤岛。
- **回归电池**（`regression_battery_r2fix/`）：Phase C 342 / B 119 / A 69 / vitnote 六 / rack 109 全 exit 0；红线门 47 变更文件、9 红线零命中。
- 前端修复 commit **bb30af1**（port/fe-start-feed-r2-1，已推）；探针工件 commit **a57a5a40**。

## 初版 R2（244946d）内容

- owner=DSH PC 执行流 exec-fe-gui-c；前端仓 worktree `D:/Godot/project/vit-daw-frontend-startfeed-r2`，分支 **port/fe-start-feed-r2-1**，自 frontend main=3ab4dfd；**未 push main**、未动用户主检出。
- 实现 commit：**244946d**（4 文件 +775/−361：start_feed_view.gd / start_page.gd / start_page.tscn / tools/probe_start_feed_view.gd）。
- 全程未占真栈（headless 探针 + 窗口态离屏诊断）；无栈端口占用，窗口进程用完退出。

## 用户 6 条反馈 → 处置

1) 调度视图（F2）✅ 正常 → 未动。
2) 「磁带流」称呼：可见文案全部去除（切换钮=「展示」「全部工程」；内部命名不限）；探针含可见文案扫描钉（Label/Button 文本 + tscn）。
3) 磁带卡太小/不缩放：卡保持设计尺寸 560×340，整体等比 scale：宽=clamp(viewport_w*0.50,560,980)、高=宽*(340/560)、高≤viewport_h*0.62；居中、resize 重排。实测 scale：1920×1080→1.714（宽 960）、1366×768→1.220、900×1600 手机竖屏→1.0（宽 560）。视觉证据 `visual_evidence/r2_display_1920x1080.png`、`r2_display_portrait_900x1600.png`。
4) 新建入口：顶栏常驻「＋ 新建工程」pill + 展示流尾新建卡（点击/双击/Enter 同语义）→ 空白磁带卡滑出（UiMotion 120–220ms；headless 立即终态）；LineEdit 默认「未命名工程」；创建→既有 `version.project_new` IPC 链（`_create_new_project`，payload 透传 project_name 前向兼容；**内核当前不消费自定义名=已登记边界**）→ 进入工程页；取消/点遮罩滑回。视觉证据 `r2_new_tape_overlay.png`。
5) 旧启动页废弃：`MarginRoot`/ItemList/BtnNewProject/BtnOpenProject/BtnEngineSettings 从 tscn+start_page 移除（boot/dev runtime/IPC 逻辑保留）；新「全部工程」页=工程磁带 icon 网格（响应式列数，首/尾「＋新建」tile，选中/双击或「进入」、▶试听同源；路径缺失诚实标注）；「打开工程…」「引擎设置」保留在顶栏。视觉证据 `r2_list_1920x1080.png`、`r2_list_portrait_900x1600.png`。
6) 控制台：同机同法 before（main 工作树，import 后窗口态启动）vs after（R2 工作树，import 后窗口态启动）：
   - **SCRIPT ERROR：0 → 0**（R2 首跑未 import 时存在 1 条 telemetry_manager 的 `VitWaveformReader` 陈旧类缓存 parse error，import 后消失；该文件为红线保护，未触碰）。
   - ERROR：环境类 4 条 = `addons/godot_cef/godot_cef.gdextension` dll 缺失（构建产物不入库，Phase A/B/C 同源边界）；WARNING：1 条 = ObjectDB/resources at exit（引擎退出类，既有）。无新增可修脚本/uid/布局告警。日志 `console_before/`、`console_after/`。

## 探针/回归

- `tools/probe_start_feed_view.gd` 按新设计重写：**170 钉 PASS**（原 140；保留可迁移钉：气氛纯函数/试听 loader/entry builder/wheel 节流/导航三通道/busy/试听两态）。日志 `probe_start_feed_view_r2.log`。
- 回归电池（对 R2 合并树实跑，`regression_battery_final/`）：Phase C **342** / Phase B **119** / Phase A **69** / vitnote 六 / rack-list probe **109** —— 全 exit 0；红线 git 门 924318d..worktree：47 变更文件、9 红线文件零命中。
- 探针脚本边界：headless 无渲染/真实指针/真实内核 IPC 回拉；真实视觉/手感/启动链由用户真栈复测。

## 需用户复测（R2 worktree 打开 Godot）

1) 启动页无「磁带流」字样；「展示/全部工程」切换正常；2) PC 大屏与手机竖屏比例下磁带卡居中且大小合适，改窗口尺寸后重排；3) 顶栏「＋ 新建工程」/列表页「＋新建」tile → 空白磁带滑出 → 改名 → 创建进入工程页；取消滑回；4) 「全部工程」页为磁带 icon 网格，可直接点选/双击进入/试听（无导出时诚实禁用）；旧列表布局不再出现；「打开工程文件」「引擎设置」仍在；5) 控制台无新增 SCRIPT ERROR/布局告警（godot_cef dll 缺失属环境类）。
