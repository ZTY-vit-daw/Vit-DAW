# FE-SEND-LEVEL-CONTROL-1：send 发送量参数口不可交互（场次⑤手测：紫色小点无法点击）

- 发卡：GLM 主管决策流 / 2026-10-23 凌晨（依据=场次⑤用户手测：send 对卡展开后「一边的紫色小点」=send_level 参数口渲染在场但**无法点击交互**——171 终验断言到 DOM 存在未断言可交互=覆盖缺口如实记录）
- 派发确认：已确认（用户手测现场反馈）
- 验收负责人：GLM 主管决策流
- 池序 178；目标仓库=**前端仓 D:\Godot\project\vit-daw-frontend**；P1（主旅程可感面：发送量调节是 send 路由的核心操作） / 0.25-0.5 天 / 依赖=无（与 176 文件域同族 rack 邻域——**串行约定：与 176 协调领取**，若同 worktree 顺序执行注明先后）；模型分级：L1/L2 / PC 前端执行流

## 现象

展开 send 合一 icon 后对卡上的 send_level 参数口（紫色小点=ParamPortRow/参数口形态）渲染在场但点击无响应——用户无法调节发送量。混音台 SEND 槽位（167 面）调量是否可用一并核对（K3 实现=行构建+调量 payload，若同样不可交互同修）。

## 范围

①取证：参数口控件的交互语义——K3 控件族（十词表 knob/slider）可调 vs 参数口=挂宏的口不可直调的既有设计边界；用户预期=**可直接调节发送量**（点击弹值/拖拽调值，对齐普通控件交互）；②实现可交互（或按取证结论给最小可感路径：点击弹参数面板/直接拖拽，给用户可用的调节面）；③探针补交互断言（模拟点击/值变更 payload 发出——补 171 断言只到 DOM 的缺口）；④窗口态手感=用户手测目检项。

## 文件域

以勘察固定（预期 `app/rack/` 参数口/控件族邻域+探针件）。越域即停。

## 验收标准

探针交互断言+真栈腿（调节 send_level→track.send.list 读回值变化）+既有探针零回退；端测边界声明；工件入主仓 coord/runs/FE-SEND-LEVEL-CONTROL-1/。

## 停止条件

参数口直调与 K3 控件语义模型冲突（需 UX 重设计）→ 锚点上交用户裁定。

- 领取：2026-10-23 10:2x 本会话机器钟 / origin/main=d0fb88d8（领取前 fetch 亲核=当前 main HEAD） / owner=GLM-5.3 L2 PC 前端执行会话（ZCode，176/178 卡会话，机器=PC） / 分支=port/fe-send-level-control-1（基=176 领取与完成态 7c82260——串行约定：与 176 同 worktree 顺序执行，176 先 178 后，K3 探针文件同域叠加） / worktree=D:/Godot/project/vit-daw-frontend-sendchain1（同一 worktree；协调分支 coord/fe-send-chain-position-1-claim@D:/Vit_DAW_wt_sendchain1_coord 续用） / 领取提交=本 commit
- 回执：**2026-10-23 10:5x 本会话机器钟执行侧自验完成，done 待主管验收**
  - **交互语义结论（范围①取证）**：参数口本义=控制层绑线的目标端——紫色小点=ColorRect 口标记（`mouse_filter=IGNORE` 零交互），口行唯一交互=「＋控件」创建控制卡按钮，而 send_level 口=always_visible 硬接线形态**连该按钮都不渲染**=点击无响应的根因（171 只断 DOM 在场的覆盖缺口如实确认）；既有调值面=控制卡绑线（K3 L5 macro→send amount 联动）/参数面板 get_params/set_param/167 混音台 SEND 槽位 SpinBox（**核对结论=代码在场可用**：SpinBox value_changed→track.send.set_amount payload 接线完整，K3 mixer167 断言面已及 handler→payload，无需同修）。
  - **实现（范围②，点击弹值路径）**：send_level 口行文本→平钮 `SendLevelAdjustButton`（口本义不变，紫色小点仍为口标记）；点击弹 `PopupPanel` 值面板（SpinBox -96..+6 dB 步进 0.5 与 167 槽位同范围，初值=拓扑 amount_db，就近弹出）；改值实时 `track_send_set_amount_payload`（与 167 槽位同一 payload 面），回执进既有 track.send.* 拓扑刷新链→卡标题/SendAmountLabel 自动更新；悬空路由/无源轨=fail-closed 用户提示零发包；侧链发送不挂入口（构建面 is_sidechain 守卫）。合一 icon 保持无调节入口（操作面=展开态，快速调量面=混音台槽位——域边界声明）。
  - **commit**：前端 port/fe-send-level-control-1@**513d26f**（3 文件 +199/-2，基=176 完成态 7c82260）；主仓工件 port/fe-send-level-control-1-stack@**c5fa2cd4**（run 目录）。
  - **探针断言面（范围③，补 171 只断 DOM 的交互缺口）**：K3 **309→318**（+9 lvl178：SendOut 卡调节入口在场/合一 icon 无入口/点击→弹窗+SpinBox 在场/初值=拓扑 -6.0/set_value(-9.5)→track.send.set_amount payload 发出且 source/fx/amount 三字段正确/悬空路由点击零发包/混音台槽位 SpinBox **真实信号链** set_value→payload）；popup **111/111**、shrink a/b/c PASS、start_feed **244/244** 零回退。
  - **真栈 run**：**lvlctrl_gui_20261023_1046**（同 176 模式，被测=worktree@513d26f，kernel=主检出 K3 722A79D7 只读）**脚本 exit 0，stack probe 60/60**（SUMMARY_JSON checks_failed=0；176 后基线 56+L10 腿 4 断言：调节入口在场→点击弹窗带 SpinBox→初值 -6.0=拓扑→set_value(-9.5)→**repository 拓扑读回收敛到 -9.5**=GUI payload→内核→读回端到端）。占用登记 8161fafb（10:4x）→拆栈手拆 VitAgent 16432/VitApp 15280→复核进程=0、六端口 tcp+udp=0。
  - **端测边界声明**：断言到按钮/弹窗节点、SpinBox 值、payload 字段与零发包计数、repository 拓扑读回值；**不覆盖**真实鼠标点击/拖拽手感、弹窗定位观感与 SpinBox 步进交互手感=**用户手测目检项**（入口=Godot 前端拉起，建议目检点：展开 send 对卡点「发送量」弹窗就近弹出/箭头步进与直接键入/调节后卡上数值刷新/混音台槽位同步）。§8：本卡断言面 LLM=0（脚本自带 chat/confirm 冒测面非本卡断言参与）。
- 验收：（裁定文件 / 验收 commit）
