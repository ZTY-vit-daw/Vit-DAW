# FE-GUI-START-FEED-1：工程列表页改版——磁带工程流启动页（PC 原生形态，池序 104）

- 发卡：GLM 主管决策侧 / 2026-10-12 深夜；**修订同夜（用户设计同步纠正：不照搬移动端纵向滑动触屏逻辑，PC 形态+磁带造型为用户原创组合）**
- 派发确认：已确认（用户明示授权+设计口述）
- 验收负责人：GLM 主管决策流
- 池序 104；目标仓库=**前端仓（Godot DAW 前端工程）**
- 优先级 / 预估 / 依赖：P1 / 1-1.5 天 / 无（启动页独立面，与 FE-GUI-B/C/RACK-LIST 文件域不相交，可并行）
- 模型分级：L1 / GLM 前端执行会话（PC）
- 蓝本：[V5-手机端.html](../../runs/FE-GUI-SHRINK-ROUND1-SCOPED-RECON/prototypes/V5-手机端.html) 磁带机视觉语言（#tape-line 族：纸感色系+1.5px ink 描边+圆角+Consolas 时间字+红色 knob）——**只取视觉语汇，交互按本卡 PC 形态重新设计**；设计基线存档只读不改

## 用户设计口述（权威）

工程一屏；**工程图标=磁带造型**，磁带上带**动态气氛底图或 MV 封面页**；双击进入；启动页**默认展示磁带**；**可点击播放试听**；滚轮上下切换；**可点击切换至列表状态**选择工程；切换交互学 PC 抖音。

## 目标（v1/v2 分期）

1. **v1 必做——磁带工程流（默认视图）**：一工程一屏居中一盘**磁带造型卡**（盒带壳体+双带轮+窗口/标签区，V5 ink 描边纸感语言）；窗口/标签区承载**气氛底图 v1=按工程名哈希取色静态渐变**（动态气氛/MV 封面=v2 后置）；信息浮层=工程名+最近打开时间（现成数据不新造查询）；**双击进入**（Enter 等价）；新建工程卡固定流尾；空/单工程优雅退化。
2. **v1 试听腿**：磁带卡播放钮（挪用 V5 tape-line transport 语汇）——数据源=**工程既有导出/渲染音频文件直接播放**（Godot 音频流，不起内核栈）；无导出=按钮禁用+诚实提示「无导出可试听」；领取时实测导出文件可寻址性（最近 render 产物索引），不可寻址则试听腿降级为占位+上交数据缺口。
3. **v1 切换三通道（PC 抖音模式调研结论+滚轮补全）**：①方向键 ↑↓/PgUp·PgDn；②屏幕右缘上/下切换按钮；③**鼠标滚轮整翻**（一格=切一盘，节流防连跳，离散翻页非平滑滚动）——抖音 PC 端原生=键盘+右缘按钮不主打滚轮，滚轮为 PC 补全通道。
4. **v1 双视图**：磁带流 ⇄ 列表态右上切换钮；列表态保留现役列表布局为选择态（美化后议）。
5. **v2 后置登记（本卡不实施）**：动态气氛底图（L3/频谱烘焙按工程级聚合）与 MV 封面（用户设定/VIS-FILM 产物）——届时另立卡接数据源；in-kernel 工程内试听同后置。

## 文件域

前端仓启动页/工程列表面+其测试；不碰主工作区（FE-GUI-B/C 域）与右键列表（RACK-LIST 域）。

## 验收标准

- 前端仓构建/既有测试全绿；headless 探针断言：磁带卡节点/三通道切换/视图切换/新建卡/双击与 Enter 进入回归/试听按钮态（有导出可播/无导出禁用）；
- 用户目检（Godot 拉起）：磁带造型与 V5 纸感观感一致、切换跟手、双视图与试听符合口述设计。

## 停止条件

启动页在 Godot 侧结构与假设不符（如被平台分支拆分）→ 锚点上交。

- 领取：2026-10-12 21:20 +0800 / origin/main=d0b696b571b188833f171f2ef138d140026e935f / owner=GLM-5.3 · PC · ZCode 前端执行会话（用户 2026-10-12 晚窗口令指派领取本卡）/ 前端仓分支=port/fe-start-feed-1，基线=vit-daw-frontend **c56e38f586f10b8c154d538f7485c644b1d731aa**（FE-GUI-SHRINK-IMPL-1 Phase A ruling pass 活线 tip，rulings/2026-10-11-BATCH2；FE-GUI-RACK-LIST-1 7c3a7f2 done 待验收域不相交→平行分支不叠加；启动页锚点=app/startup/start_page.gd 单文件 1015 行+start_page.tscn，无平台拆分，boot 平台分支在函数内部不影响 UI 面——在该基线核实）/ 前端 worktree=D:/Godot/project/vit-daw-frontend-start-feed（独立 worktree）/ 领取提交=**3f602891044ae8d98bf6558be5fb52a03ec23a9a** / 领取时实测两数据缺口已定位：①recent_projects=内核仅回路径数组（global_project_config.json 无时间戳）→信息浮层时间字取工程文件 mtime（现成数据零新查询，口径回执注明）②导出产物不可寻址（start_render 路径调用方自选、无产物索引、磁盘工程旁无 wav）→试听腿按卡面降级占位+上交数据缺口
- 回执：实现 commit=vit-daw-frontend **76201ea3a6fcc3a2ebccdc712b242850cb5b2ff5**（port/fe-start-feed-1 已推前端仓 origin 新分支，main 未触碰；worktree 保留供复验）/ diff 域=卡内 9 文件（改 start_page.gd/.tscn；增 start_feed_view.gd 磁带流呈现层+tape_atmosphere.gd 哈希气氛纯函数+wav_audition_loader.gd WAV 装载纯函数+probe_start_feed_view.gd 探针+3 .uid）；vit_dock/**、rack/**、boot/IPC 链函数体零 diff / 探针=tools/probe_start_feed_view.gd **140 钉×2 连跑 PASS exit 0**（默认磁带流结构+空态仅新建卡+三通道切换[键盘/右缘按钮/滚轮 0.28s 节流与卡面转发]+端点钳制+busy/列表态守卫+磁带卡结构[壳/标签条/窗口气氛底图+双带轮/试听排]+recent 双视图同源+双击与 Enter 进入回归+新建卡双击/Enter+试听两态[无源诚实禁用 vs 夹具注入 24bit→16bit 可播/翻盘即停/坏源解码提示]）；回归=vitnote face_resolve+container+shrink phase A 三面 PASS+f5_startup_smoke PASS（主场景断言+进入工作区 535ms 零慢帧）+--import exit 0 零 SCRIPT ERROR / 试听数据源实测=**不可寻址**（内核遥测仅 job_id 无产物路径、无任何 render 索引、工程旁无 wav 实存——详录工件 RECEIPT.md）→按卡面降级占位：resolve_audition_source 候选集空=按钮恒禁用+「无导出可试听」，播放机构（RIFF 手解/24bit 降位/双态）在位受夹具验证；**数据缺口上交**：v2 建议 start_render 成功回执登记产物路径或导出弹窗固化默认目录，届时只填候选集函数 / 端测边界声明=Godot 前端无浏览器断言面 headless probe 即端侧门（RACK-LIST 同口径）；真实渲染视觉（纸感/磁带造型观感）/真实指针双击节奏与焦点流转/真实音频输出/内核 open_project 落链归用户手测（手测清单 6 条入 RECEIPT.md）/ 工件=D:/Vit_DAW/coord/runs/FE-GUI-START-FEED-1/（RECEIPT.md+探针/回归/构建七日志，留盘不入库）/ 停止条件未触发（启动页单文件结构在基线核实如卡面假设）/ [等待用户:目检排期]
- 验收：**pass（用户目检待排期）** / 2026-10-12 深夜（[rulings/2026-10-12-NIGHT-BATCH3-rulings.md](../../rulings/2026-10-12-NIGHT-BATCH3-rulings.md)④；探针 140 钉×2+回归四面+f5 启动烟测采信；试听数据缺口采信降级+登记；手测清单 6 条在 runs/FE-GUI-START-FEED-1/RECEIPT.md）

- **R2 手测返工（2026-10-18 晚窗，用户实机手测；卡重开 doing）**——用户反馈 6 条（1 通过，2–6 返工）：
  1) ✅ 调度视图（F2 台前调度）显示正常，通过。
  2) **命名**：这是**主启动页**，界面上不得出现「磁带流」这类称呼（去 toggle/标题等可见文案中的"磁带流"；品牌 Vit-DAW 保留）。
  3) **磁带卡太小/未响应式缩放**：应直接占据视窗中心、大小合适；按手机与 PC 显示器尺寸比对缩放——以视口尺寸推导卡尺寸并等比缩放、居中、resize 重排（PC 1920×1080/大屏与手机竖屏比例都要合适）。
  4) **缺新建工程入口**：上下翻页流没有可见的新建途径——在流中加「＋ 新建」入口（按钮/页），点击**滑出一盘空白磁带**，可**重命名**，点击创建后进入工程页；列表页也要有「＋新建」tile。
  5) **列表页**：现列表态仍是**旧启动页**（旧 ItemList/旧按钮布局）——旧启动页废弃；新列表页=**罗列各工程磁带 icon 的网格**，可直接点选；展示页 vs 列表页区别=单卡（可播放）vs 罗列（可播放+进入工程）。
  6) **Godot 控制台大量警报/报错**：修复时一并处理——捕获启动控制台输出，修复可处理项（SCRIPT ERROR/缺资源/uid/布局告警等），环境类（如 godot_cef dll）如实分类登记；回执给 before/after 计数+清单。
  - **设计裁定（Lead 暂定，用户可覆写）**：a) 可见文案页名不用"磁带流"；toggle 用「展示/全部工程」或仅图标；b) 新建=顶栏常驻「＋ 新建工程」按钮+空白磁带滑出卡（默认名"未命名工程"，LineEdit 可编辑，创建走现有 ersion.project_new IPC 链→进工程；取消滑回），翻页流保留/优化新建卡作为发现入口；c) 响应式卡=宽 clamp(视口宽*0.50, 560, 980)、高=宽*(340/560)、高≤视口高*0.62，等比统一缩放、居中、resize 重排；d) 列表页=磁带 icon 网格（响应式列数，item 支持选中/进入（双击或 Enter 或按钮）与 ▶试听，首/尾「＋新建」tile；「打开工程文件」「引擎设置」入口保留（顶栏/列表页），**不恢复旧 ItemList 布局**；e) 不启真栈；窗口态离屏渲染诊断按已采信边界（离屏/VIT_SKIP_DEV_AUTOSTART=1/无栈/用完退出）可作视觉证据；headless 探针按新设计更新（保留可迁移钉、旧布局钉更新并在回执列明）。
- 交付 R2 执行=exec-fe-gui-c（task-15；前端仓 worktree D:/Godot/project/vit-daw-frontend-startfeed-r2，分支 port/fe-start-feed-r2-1 自 frontend main=3ab4dfd；红线 9 文件零触碰；不 push main）。
- 验收 R2=返工提交 + 探针电池（start-feed 探针更新+Phase A/B/C/vitnote/FE-RACK 回归）+控制台错误 before/after 清单 + **用户复测 5 项**（命名/居中缩放/新建流程/新列表页/控制台干净）。