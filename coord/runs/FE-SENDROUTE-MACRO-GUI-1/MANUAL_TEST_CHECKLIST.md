# FE-SENDROUTE-MACRO-GUI-1（K3/130）手测清单【等待用户】

手测入口（用户裁定 2026-09-12，AGENTS §5）：**从 Godot 拉起 DAW 前端，再从前端启动页进入目标工程**。
需要环境变量时在拉起 Godot 的同一 shell 先设（Godot 拉起的 agent 进程继承）。本清单无需环境变量。

被测：前端仓 worktree `D:/Godot/project-vit-daw-frontend-sendmacro`（port/fe-sendroute-macro-gui-1）+
内核须含 K1（本腿自建 kernel SHA256 722A79D7…，或任何 main 基线构建；**stock Export/staging B6565DCF 不含 K1，send 命令会报 unknown command**）。

## A. send 创建与听感（干湿）

1. Godot 拉起前端（或直接用运行中的栈）→ 进任一有素材的工程 → 选一条有 clips 的轨 → 打开该轨机架画布。
2. 画布空白处右键 → 应出现三入口菜单（装载插件…/新建发送…/新建宏）→ 点「新建发送…」。
3. 目标选择器：确认分组（FX 轨/普通轨）+检索框+底部「＋新建 FX 轨」；点「＋新建 FX 轨」→ 命名行展开（默认 Reverb Bus）→ 回车。
4. 选定即连：画布出现乐观 pending 芯片（「SEND ▸ … 连接中」）→ 内核确认后消失，画布出现 **SEND ▸ Reverb Bus -6.0 dB** 节点与 **◂ SEND** 节点（串联占位，与效果器同构可连线）。
5. 在 FX 轨机架装一个混响（如 ZamVerb）→ 播放：源轨应有湿声（干+湿混在源轨）；**SendIn 拖到不同链位=汇入点变化（重连电缆）**，听感应随 tap 点变化（pre/post 语义）。
6. 双击 SendOut 节点 → 跳转 FX 轨画布；FX 轨画布顶部应有「SEND 来源（只读 N）」面板（源轨名@量，点击跳回源轨）。
7. 右键 SendOut/SendIn → 「移除发送（成对）」→ 两节点随状态刷新消失，播放回到纯干声。

## B. SEND 槽位（通道条主控面）

1. 混音台（Mixer）→ 源轨通道条：Z 按钮下方有「SEND (N)」按钮 → 点开。
2. 每行=FX 名+量 SpinBox+「断开」：调量（如 -18 dB）→ 播放中湿声变小；断开 → 湿声消失。
3. 轨列表：FX 轨标题行应出现紫色「FX」徽标。

## C. 宏控制层（两层模型）

1. 机架画布空白右键 →「新建宏」→ 出现紫色竖条宏卡（默认 1 槽旋钮 macro_1）。
2. 画布左上「● 信号」「◆ 控制」层 chip：默认双显；单点「● 信号」→ 仅控制层+信号层压淡；再点回双显；单点「◆ 控制」→ 仅信号层（控制卡隐藏）。
3. 绑定手势：点宏卡输出锚「●」拖出**紫虚线** → 悬停插件卡 → 展开「⌗ params」参数口（或 SendOut 卡自带「发送量」口）→ 松手在参数口上 → 绑定菜单（控件类型下拉+curve 四选+min/max）→ 确认 → 紫虚线常驻（中点菱形把手）。
4. 转宏卡旋钮 → 目标参数应实时跟随；SendOut 口绑定时，转动旋钮 = 发送量变化（听感湿声大小）。
5. 线中点菱形点击 → 编辑绑定（curve/min/max）；控制线右键 → 断开绑定。
6. 参数口行「＋控件」→ 建议控件类型菜单（确定性建议，可下拉改）→ 确认 → 新控制卡+自动绑定。
7. xypad/range 控件（类型下拉可改）：双输出锚 out_x/out_y，可各绑一个参数，拖动双参数联动。

## D. 侧面板 MacroPanel（agent 会话视图）

1. 让 agent 生成宏面板（或已有 artifact）→ 侧面板渲染器应显示十类控件（knob/slider/numberbox/xypad/toggle/button/range+时变壳）；旧 artifact 的 switch/checkbox/trigger 正常渲染。
2. 控件施加（有 node_id 锚的）→ 走内核 control_* 命令面（机架控制层同源，agent 会话不再双轨）。

## 验收口径

- 全程无 SCRIPT ERROR（Godot 日志）；send/宏操作后刷新状态一致（重进工程后 send 节点/宏卡/绑定线恢复）。
- 听感干湿/宏联动为主观确认项——不符合预期请记录现象（哪一步、期望 vs 实际）。
