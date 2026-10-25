# FE-SEND-LEVEL-FADER-1：发送量推子直调（用户裁定形态——滑动推子替代点击弹窗）

- 发卡：GLM 主管决策流 / 2026-10-24（依据=用户 182 复验反馈：弹窗输入已生效且听感确认，但期望形态=可直接拖动的推子控件）
- 派发确认：已确认（用户现场表达）
- 验收负责人：GLM 主管决策流
- 池序 183；目标仓库=**前端仓 D:\Godot/project/vit-daw-frontend**；P1（主旅程可感面·用户裁定形态） / 0.5 天 / 依赖=182 已合入（ee8fa0e）；模型分级：L1/L2 / PC 前端执行流

## 现状与目标

- 现状（182 后）：SendOut 对卡「发送量」口行平钮→点击弹 PanelContainer 叠层（SpinBox 步进/键入）→实时 track.send.set_amount。功能闭环，用户复验通过。
- 目标（用户裁定）：**卡上直接可拖的推子**（slider/fader）调节发送量——拖动即时回显、无需先点开弹窗。

## 范围

①交互实现：SendOut 对卡发送量行加横向推子（HSlider 或仓内既有 slider 先例形态，勘察 K3 控件族十词表 slider 件与 167 混音台邻域后择型）；范围 -96..+6 dB；映射设计注意宽跨度下常用区（约 -30..0）可拖精度（线性步进 vs taper 由勘察定，拖动粒度≥0.5dB 对齐 SpinBox 步进）；口行平钮弹窗保留为精调键入面（双入口并存，弹窗关闭语义沿用 182）。②发包节流：拖动中间帧本地回显、释放/防抖才落 track.send.set_amount——**复用仓内控件值提交节流先例**（Vit_Graph_Rack.gd:335 邻域「drag 中间帧本地回显，释放才落」机制），不得每帧直发 payload。③混音台 SEND 槽位（167）是否同升推子=勘察项写结论（改/不改给理由，改则一并实现，用户裁定面）。④探针断言：真实事件管线（push_input 拖动序列 motion→press→motion…→release）→payload 节奏断言（拖动中零发包/释放后恰一笔+值正确）+repository 读回收敛；禁止 emit 直调（182 通则）。⑤窗口态手感=用户手测复验。

## 文件域

以勘察固定（预期 `app/rack/Vit_Graph_Rack.gd` 发送量行+节流复用件+探针件；若 167 同升则含混音台邻域）。越域即停。

## 验收标准

真实拖动管线断言（拖中零发包/释放落值/读回收敛）+既有探针零回退（K3 313/popup 111/realinput 15/shrink abc/start_feed 244）+真栈腿（SendRouteGuiSmoke 模式，§2.2 登记）；工件入主仓 coord/runs/FE-SEND-LEVEL-FADER-1/；用户手测手感复验。

## 停止条件

推子与对卡尺寸/GraphEdit 拖拽手势冲突不可调和（卡内拖动被画布手势抢）→ 锚点上交再裁定。

- 领取：（2026-10-24 18:09 / 主仓 origin/main=4c7736c9 / 前端仓 main=ee8fa0e / owner=ZCode GLM-5.3 PC 前端执行会话（FE-SEND-LEVEL-FADER-1 专属） / 分支 port/fe-send-level-fader-1 / worktree D:/Godot/project/vit-daw-frontend-lvlfader1 / 领取提交=本提交）
- 回执：（2026-10-24 19:2x 自验完成，交主管验收）
  - **择型结论**：HSlider+数值回显 label（`[定宽 52px value label + EXPAND_FILL slider]` 行，MacroValueSlider 先例同款形态，加在发送量口行上方）；映射=**分段线性 taper**（slider 内部 0..1 step 0.001：低段 25% 行程覆盖 -96..-30 粗调、高段 75% 覆盖 -30..+6 细调，120px 卡内推子下高段≈0.45dB/px——常用区 -30..0 拖动粒度优于 0.5dB ✓；两端点/分段连续/往返在探针数学面钉死）；**落值量化 0.5dB**（round(db*2)/2，与 178 弹窗/167 SpinBox step 同粒度，读回一致）；共享静态件=rack_send_route_rules.gd（对卡与槽位同源）。节流=卡面②严于 macro 先例：value_changed 仅本地回显（value label+SendAmountLabel）**零发包**，drag_ended 悬空/未绑定 fail-closed 守卫后恰一笔 track.send.set_amount；拖动中拓扑回执不抢跳（vit_fader_dragging meta 门）；侧链卡不挂推子（与平钮同款构建守卫）。**平钮弹窗保留**（双入口：推子=直调拖动面/弹窗=精调键入面，弹窗改值经回执同步刷新推子位）。
  - **167 勘察结论：同升（改）**。理由：①用户裁定面=「发送量可直接拖动调节」，167 SEND 槽位卡定位即「发送量主控面」，两面形态一致符合裁定精神；②strip 主音量推子本有「拖动节流+drag_ended 补发」先例（_VOL_IPC_INTERVAL_MS），SEND 槽位推子同升与仓内形态一致；③卡面文件域已预留「若 167 同升则含混音台邻域」。实现=槽位行 SpinBox 下加同款 HSlider（同 taper 同节流：拖中零本地动作/释放恰一笔），**SpinBox 键入面零行为改变**（仅补 name=SendSlotAmountSpinBox 可寻址性），回执到达后 _refresh_send_slots 重建行刷新两面。
  - **commit**：前端仓 `port/fe-send-level-fader-1@060a5e9`（5 文件 +774：Vit_Graph_Rack.gd 推子行+preview/commit/回执同步、rack_send_route_rules.gd taper 三函数、vit_mixer_channel_strip.gd 槽位推子、tools/probe_rack_sendlevel_fader.gd 新独立腿、tools/stack_probe_sendroute_macro.gd L11 腿）；已推远端。
  - **探针断言面**：独立腿 `probe_rack_sendlevel_fader.gd` **35/35 PASS**（headless）——taper 数学 8（端点/分段点/连续/往返/量化/钳制）/对卡 20（推子+值 label+平钮在场、初始 unit 映射 -6.0、侧链构建面无推子、**真实拖动管线 push_input motion→press→motion…→release**：拖中零发包+双 label 回显、拖中回执不抢跳、释放恰一笔+payload 三字段量化、读回收敛+推子位同步、悬空 fail-closed 零发包、弹窗双入口共存真实点击）/strip 槽位腿 7（SpinBox 在场 167 零改变+推子在场+初始映射+真实拖动拖中零发包/释放恰一笔+payload）。**零回退**：K3 313 / popup 111 / realinput 15 / shrink a 87 / shrink b 119 / start_feed 244 全绿；shrink c=**基线既有不稳定失败（179 域非本卡引入）**——主检出纯基线 ee8fa0e 对照复跑同族 mixersync 失败 1/2 浮动，179 卡（PROBE-SHRINKC-MIXERSYNC-1）已 done 待合入修复（前端 c55b3cd）。**探针方法学发现**（如实）：同帧连发 motion 仅首个生效须 process_frame 分帧；新建 worktree 缺 .godot/{extension_list.cfg,global_script_class_cache.cfg} 致 GDExtension 类缺失与 `extends "res://..."` 动态子类解析失败（复制+--import 重建缓存解决）。
  - **真栈 run**：`coord/runs/FE-SEND-LEVEL-FADER-1/sendroute_gui_20261024_191155`（SendRouteGuiSmoke 脚本 exit 0，stack probe **72/72**=182 基线 64+L11 拖动腿 8；§2.2 登记 0a6dd0bf/释放 752e3528，工件随 `port/fe-send-level-fader-1-stack@c9570847`）。首轮 190938=67/72 留档：5 失败全为**主检出运行时工程预存 send 边 1032→1037@+6.0 污染**（SendRouteGuiSmoke 非 journey、内核按 exe 目录加载主检出 default_project.xml；182 当时无此边故全绿；本卡零内核/脚本/工程改动）——未触碰主检出状态，VIT_PROJECT_XML 指 git 干净版隔离副本复跑一次过全绿。
  - **端测边界声明**：①本卡交互面全部经真实事件管线断言（push_input 拖动序列，零 emit 直调）；②**窗口态手感**（taper 拖动感受/常用区灵敏度）=用户手测复验面，验收后由主管安排；③167 槽位腿覆盖 `_make_send_slot_row` 直构面（动态子类覆写 _ready 绕开场景骨架依赖）——**未覆盖**完整 strip .tscn 装配态（@onready 场景绑定）与混音台整装渲染面，该面由既有混音台探针（shrink_c mixersync 族=179 在修）承载，本卡对 SpinBox 行为零改动；④栈探针 L11 读回断言=repository 拓扑收敛面，内核 audio 湿分量渲染不在本卡断言面（与 178/182 同界）。
- 验收：**整卡通过**（GLM 主管决策流 2026-10-24 晚窗）。①diff 亲读全过：taper 数学分段连续/量化 0.5dB（rack_send_route_rules 共享件）、对卡拖中零发包（value_changed 仅本地回显）+drag_ended 恰一笔+悬空/未绑定 fail-closed+拖中 vit_fader_dragging 门防回执抢跳、167 槽位同升且 SpinBox 零行为改变、探针零 emit 直调（红线扫描 clean）+L11 全程 push_input 分帧管线、35 断言=37 处 _check 减 2 处 null 守卫分支与回执吻合；②工件亲核：SUMMARY_JSON 72/72 零失败+L11 八断言逐条在场，首轮 67/72 污染归因链完整（主检出运行时工程预存边=用户场次⑥手测写入，非本卡域，处置正确未触碰权威输入）；③主管复跑（合并态 5da6360，§2.2=dda3ea5f）：SendRouteGuiSmoke 隔离副本一次过 72/72 exit 0+活栈探针族 shrink_c×2/a/b/fader 35/popup 111/realinput 15/start_feed 244 全绿（supv_rerun_20261024_1955）。前端合入 3e9d00d→**5da6360** 已推；工件入主仓 coord/runs/FE-SEND-LEVEL-FADER-1/。**收口面=用户窗口态手感复验（taper 常用区灵敏度）后置安排，非阻塞——已闭环：用户 2026-10-25 复验确认「没有问题，完全可用状态」，183 全链收口**。
