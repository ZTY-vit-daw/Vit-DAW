# FE-GUI-START-FEED-1 回执工件（留盘不入库，按 2026-09-29 政策）

- 执行：GLM-5.3 · PC · ZCode 前端执行会话
- 前端仓 worktree：D:/Godot/project/vit-daw-frontend-start-feed（分支 port/fe-start-feed-1，基线 c56e38f）
- 日期：2026-10-12 晚窗

## 验证记录（本目录日志）

| 门 | 命令 | 结果 |
|---|---|---|
| 新探针 ×2 | `"D:\Godot\Godot_v4.6.1-stable_win64_console.exe" --headless --path . -s tools/probe_start_feed_view.gd` | run1/run2 均 **PASS exit 0，140 钉 0 失败** |
| 回归 vitnote face_resolve | 同上 `-s tools/probe_vitnote_face_resolve.gd` | PASS exit 0 |
| 回归 vitnote container | `-s tools/probe_vitnote_container.gd` | PASS exit 0 |
| 回归 shrink Phase A | `-s tools/probe_fe_gui_shrink_phase_a.gd` | PASS exit 0 |
| 回归 f5 启动冒测 | `-s tools/diagnostics/f5_startup_smoke.gd` | PASS exit 0（主场景断言+启动页 6 子节点+进入工作区 535ms 零慢帧） |
| 构建门 | `--headless --path . --import` | exit 0，SCRIPT ERROR 计数=0 |

## 试听数据源实测（领取时，卡面要求）

- 内核 `get_recent_projects`（VitGlobalProjectConfig.cpp）只回**路径字符串数组**，`%APPDATA%/Vit-DAW/global_project_config.json` 无时间戳字段 → 信息浮层「最近时间」取**工程文件 mtime**（现成数据零新查询；口径=最近保存时间，非最近打开——现无更准的现成字段）。
- 导出产物**不可寻址**：`start_render` 的 file_path 由调用方自选（render_export_popup 是空 LineEdit+浏览）；内核遥测只有 job_id 进度事件无产物路径；无任何 render 产物索引（agent/user:// /工程目录均查证）；磁盘工程文件旁无 .wav 导出实存。
- 处置（按卡面）：试听腿**降级占位**——`resolve_audition_source()` 候选集为空 → 按钮恒诚实禁用+提示「无导出可试听」；播放机构（RIFF 手解、24bit→16bit 降位、双态按钮、翻盘即停）完整在位，探针以夹具注入验证「有导出可播/无导出禁用」两态。
- **上交数据缺口（v2 建议）**：内核 start_render 成功回执登记产物路径（global_project_config 或独立 render 索引 json），或导出弹窗固化默认目录约定；届时只填候选集函数即可点亮试听。

## 端测边界声明

Godot 前端无浏览器断言面，headless probe 即端侧门（RACK-LIST 同口径）。本探针断言到**呈现结构与命令构造面**（信号发射/按钮态/纯函数/接线）；不覆盖：真实渲染视觉（纸感/磁带造型观感）、真实指针双击节奏与焦点流转、真实音频输出、内核 open_project 落链——归用户手测。

## 手测清单（用户目检，手测入口=Godot 拉起前端，AGENTS §5 裁定）

1. 启动页默认=磁带流：磁带造型卡（盒带壳+双带轮+窗口区哈希渐变）与 V5 纸感观感一致；工程名+最近时间浮层正确。
2. 切换三通道跟手：↑↓/PgUp·PgDn 键盘、屏幕右缘 ▲▼ 按钮、滚轮一格一盘（连续滚动不连跳）。
3. 双击磁带卡与 Enter 均能进入工程（走既有引导链）；翻到流尾新建卡，双击/Enter 触发新建。
4. 右上「磁带流 ⇄ 列表」切换：列表态保留原布局；recent 列表与磁带卡同源同序。
5. 试听按钮：当前应恒为禁用态+悬停提示「无导出可试听」（诚实态，数据缺口见上）。
6. 空工程/单工程退化：仅新建卡时导航禁用；路径缺失工程的磁带卡显示「文件不可寻址」诚实标注。

## diff 域

- 改：`app/startup/start_page.gd`（+157：接线/双视图/键盘通道）、`app/startup/start_page.tscn`（FeedLayer 节点+StatusLabel 全局化+MarginRoot 默认隐藏）
- 增：`app/startup/start_feed_view.gd`（磁带流呈现层）、`app/startup/tape_atmosphere.gd`（哈希气氛纯函数）、`app/startup/wav_audition_loader.gd`（WAV 装载纯函数）、`tools/probe_start_feed_view.gd`（探针）+3 个 .uid
- 未触碰：vit_dock/**（FE-GUI-B/C 域）、rack/**（RACK-LIST 域）、boot/IPC 链函数体
