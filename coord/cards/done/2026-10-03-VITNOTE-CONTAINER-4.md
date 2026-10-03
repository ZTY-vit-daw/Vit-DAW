# VITNOTE-CONTAINER-4：图钉回归视窗（大/半透明/固定/代表范围）+ 便签列表入左侧资料库列 + 底部图钉带退役（P2，用户裁定 2026-10-03 二号）

- 池序 7；目标仓库=D:\Godot\project\vit-daw-frontend；来源=[decisions/2026-10-03-note-pin-design-v2.md](../../decisions/2026-10-03-note-pin-design-v2.md)（修订 CONTAINER-3 设计段 B）；分支自 port/vitnote-container-3@82635c1 切出
- 优先级 / 预估 / 依赖：P2 / 0.5 天 / CONTAINER-3 已 conditional pass（把手/描边/管理器基线在位）
- 模型分级：L1 / GLM 或 flash 可接（纯 Godot 域，CONTAINER-3 先例充分）
- 已核实事实（用户裁定原文要点，详见 decision）：
  1. 用户肯定 CONTAINER-3 方向（"设计更好了"）：把手、面板拖动、作用域概念均保留。
  2. **推翻底部图钉带落点**：图钉应**留在视窗上**（代表其圈选范围处），可比旧小圆点更大，压内容上方时**半透明**，**不可移动**。
  3. 便签列表如需罗列：**左侧资料库区**专门一列（与 group/plugin 列表同级同模式）。
- 目标：
  1. **图钉回归视窗**：收起态图钉渲染于圈选范围处（overlay 顶层，轨道头之上结构性不遮）；尺寸可放大（≥旧 12px 的 1.5-2 倍，实测定值）；与内容重叠时半透明（alpha 阈值实锚）；固定不可拖（CONTAINER-3 断言面沿用）；单击重开/右键删除语义不变；悬停 peek 作用域描边沿用。
  2. **底部图钉带退役**：删除 pin tray；空带隐藏逻辑随之退役。
  3. **便签列（左侧资料库）**：资料库抽屉内新增便签列（group/plugin 列表同款交互模式）：条目=图钉 glyph+序号+摘要缩写；单击=重开对应 note；右键=删除；无便签时空态文案。若本卡取简可先做"列内只读罗列+单击重开"，新建仍走 Q 圈选（不入本卡）。
  4. 回归：CONTAINER-3 探针族迁移（pin tray 断言改视窗图钉断言：位置=范围处/不可拖/半透明 alpha/不被轨道头遮）+四回归+`--import` 零 SCRIPT ERROR；探针输出落 runs/。
  5. 用户手测复验（图钉在视窗/大而可辨/半透明/固定；资料库便签列；底部无停靠带），过则 CONTAINER-3+CONTAINER-4 同场转正（连带 CONTAINER-2/IMPL-D）。
- 文件域：Godot `vit_note_panel.gd`+`vit_note_manager.gd`（+资料库面板挂列如需，实锚后申报 ≤4 文件+工具）。
- 约束：不动 main；headless 探针不占端口；与 NOTESTREAM-2 Godot 腿串行（本卡先——同文件域）。
- 验收标准：目标 1-3 落地+探针/回归全绿+用户手测复验。
- 停止条件：资料库面板结构不容新列（需动 dock/抽屉架构）→ 上交裁定；图钉半透明与 overlay 渲染层耦合超预期 → 方案上交。
- 领取：2026-10-03 19:53 PC 执行侧（ZCode GLM-5.3）/ Vit-DAW 仓 origin/main=48ee12c28afc486b8a9a1a25289220e1a47a679a / Godot 仓 port/vitnote-container-4 自 port/vitnote-container-3@82635c1 切出
- 回执（2026-10-03 PC 执行侧，ZCode GLM-5.3）：
  - **commit**：Godot 仓 `port/vitnote-container-4` @ `a2930a7`（自 82635c1 切出+1；6 文件=4 产品+2 工具 +712/-182；已推 origin）。⚠ 署名申报：commit 尾误带一行 "Co-Authored-By: Claude Opus 5 (planning)"——本卡全程 ZCode GLM-5.3 执行、无 Opus 参与；分支已推送按纪律不做 amend，特此留档更正。
  - **实锚申报**（卡面"实锚后申报 ≤4 文件+工具"）：产品 4=`vit_note_manager.gd`（图钉回归视窗+tray 退役+便签列供给）+`vit_note_panel.gd`（头注释/tooltip 语义更新）+`left_library_dock.gd`（第四 tab+便签列）+`MediaPoolAdapter.tscn`（LibraryNotesTab 按钮节点）；工具 2=`probe_vitnote_container.gd`+`capture_vitnote_panel_layout.gd`。自由量实锚：PIN_SIZE=22px（旧 12px 锚点 1.83 倍，裁定 1.5-2x 窗内）；PIN_OVER_CONTENT_ALPHA=0.42；半透明判定节流 20 帧；图钉中心=圈选范围**左上角**（拟物钉住范围一角、避开范围中心内容——裁定未定角落归属，按"代表范围+不遮范围内容"取左上）；便签列=动态容器（TrackGroupListPanel 是 tscn 实例先例，本列无独立场景需求）；pin_rect_for/pin_overlaps_content 为**实例方法**非 static（-s 探针装载期编译不解析 autoload 标识符，本类 `_circle_hint_text` 引用 VitShortcutManager——CONTAINER-3 的 VitNotePanel 静态族无 autoload 引用故可 static，本类不具该条件；产品语义不变）。
  - **目标 1 图钉回归视窗**：收起→图钉落圈选范围左上角（pin_rect_for：中心-11px、viewport 边距钳制）；manager overlay 顶层直接子节点（主场景根末位子树，轨道头/clip 结构性遮不到；子序=圈选层首位→描边层→图钉→面板不变量保持）；22px 圆钉+自绘圆头斜针 glyph（无 emoji 依赖）；压内容半透明=四供给器 vit_face_rects 相交判定（任一面命中→modulate.a=0.42，悬停恢复不透明）；固定不可拖（无位移代码路径，建钉一次定位）；单击重开（位移<4px 判定）/右键即删/悬停 peek 沿用；tooltip=完整摘要。
  - **目标 2 底部图钉带退役**：_build_pin_tray/_pin_tray_root/_pin_tray/_pin_box/带内序/空带隐藏/PIN_TRAY_HEIGHT/TRAY_BG 全退役（探针字段+方法双侧断言锁定）。
  - **目标 3 资料库便签列**：Notes 第四 tab（同款 toggle+ButtonGroup）；列=glyph+「N1 · 轨道时间线 72%」缩写+收起/展开状态小字；单击重开（仅收起态——展开态 reopen 会 grab_focus 扰动输入，探针断言无操作）；右键删除（manager.remove_note 真实链）；空态文案「（无便签）按住 Q 并在视窗圈选即可创建便签」+「N 条」计数；manager 层 get_note_entries 数据源+vit_note_list_consumer 组广播（open/remove/collapse/reopen 四调用点全覆盖）；新建不入场（Q 圈选链不动）。
  - **探针结果**：语义迁移+新面 **141/141 EXIT=0**（run1-run3 修复两处测试侧问题后全绿：GDScript 不支持 Python 式推导；headless 64x64 几何下 pin 位置期望须与产品同源取 get_viewport_rect 而非探针 VP_SIZE——产品代码零返工）。新断言面：tray 退役双侧、PIN_SIZE 1.5-2x 窗、pin_rect_for 三例数值表（中心=范围左上/贴缘双角钳制）、pin_overlaps_content 三例（相交/远离/退化块）、pin=manager 直接子节点、pin rect==pin_rect_for 同源断言、半透明三态（注入伪造供给器→0.42；悬停→1.0；覆盖撤离→1.0）、单击重开/拖动零位移不重开/悬停 peek/再收起原位、余钉不受删除扰动、便签列 14 项（tab 在场入组/容器初始隐藏/消费者组/空态文案/计数/行三件套/tooltip/collapse 广播状态更新/行单击重开/展开态无操作/右键删/空态回归）。四回归全绿：face_resolve **127/127**+circle_state **52/52**+panel_summary **16/16**+input dock **EXIT=0 零 FAIL**；input legacy **EXIT=2 与基线同因**（headless 64x64 几何限制 FAIL setup 同款；输出与基线**非逐字一致**——差异三类：①本卡预期 diff（NotesTab/便签列在场、PinTray 消失、dock 行号平移）；②临时节点 id 差（基线先例已知）；③**环境耦合**（见泊位声明））。`--import` EXIT=0 零 SCRIPT ERROR。原始输出 7 份落 `coord/runs/VITNOTE-CONTAINER-4/`。
  - **前后截图**：`coord/runs/VITNOTE-CONTAINER-4/` 9 份——layout_before_{open,collapsed}.png（@82635c1 临时 detached worktree 取证，底部图钉带在画=阳性对照）+layout_after_{open,collapsed,notes}.png+crop_after_collapsed_pin.png（图钉 4x 放大件）+crop_after_notes_column.png（便签列裁剪件）+import/probe 工件。目检记录：collapsed 态底部带消失（退役✓）、图钉 22px 圆钉在圈选范围左上角在画、半透明（分析器读出 alpha~0.4）可辨（全幅分析器对 22px 图钉未辨——尺寸/透明度所致，CONTAINER-3 同款边界；裁剪放大件佐证+探针断言面锁定，观感归用户手测）；notes 态 Notes tab 激活、「便签|1 条」头部、行=图钉 glyph+N1·轨道时间线 72%+「收起」标记、搜索框隐藏。
  - **泊位声明**：本卡全程**未启动真实运行栈**（VitApp 内核/Go agent 未由本卡启动；Godot 仅 headless 探针+--import+窗口模式截图/诊断脚本，进程即起即退）——零主动真栈、零端口占用、零残留进程。**环境耦合如实申报**：取证期间用户手测真栈内核持续在场（ping 可达），窗口模式截图与 legacy input 探针所载场景的**内置 IPC 客户端被动连接**了该内核（只读 ping+拓扑/工程推送，无写操作；legacy 探针输出因此含真实工程轨道数据，before 截图时间线显示真实工程 bass/drums 等轨）——CONTAINER-3 基线跑时无内核故逐字对比不可复现，FAIL 行同因已核；本卡探针/截图均未向内核发送任何变更命令。工作树：独立 worktree `D:/Godot/project/vit-daw-frontend-c4` 开发（主工作树零触碰，用户手测占用不受影响）；before 取证用临时 detached worktree（82635c1），两 worktree 待决策侧验收后清理。Vit_DAW 仓领取提交曾误带入暂存区残留的 REPLY-GEN-TOOLGATE-1 staged rename（该卡未领取，权威位置=todo）——即知即改：追加修正提交移回 todo（c04b39d5），历史保持线性真实。
  - **自验清单（用户手测复验项，按裁定逐条）**：1) 收起便签→视窗**圈选范围左上角**出现圆形图钉（22px、压内容时半透明、悬停恢复）；2) 图钉**按住拖动无任何响应**（固定=代表范围）、单击重开（描边高亮渐隐指示位置）、右键即删（不可恢复，用测试 note）；3) 底部**无停靠带**（退役）；4) 左侧资料库 **Notes 第四 tab**：无便签时空态文案；有便签时「N 序号·首面摘要」行（收起/展开标记）——单击收起行重开、右键行删除；展开态行单击无反应（不抢焦点）；5) Q 圈选→胶囊→开便签链路照常（新建不入列交互）；6) 面板标题栏拖动/八把手拉伸照旧；7) IME 中文输入照常。
- 验收：**conditional pass（2026-10-03 决策会话）**——[rulings/2026-10-03-VITNOTE-CONTAINER-4-conditional.md](../../rulings/2026-10-03-VITNOTE-CONTAINER-4-conditional.md)；我方复跑探针全绿（container 141/141+127+52+16+input+import 零错，输出 runs/…/verify_pc/）；⚠可见性风险标注（决策侧两读未目辨图钉）——转正条件含用户目辨；NOTESTREAM-2 Godot 腿基线=a2930a7
