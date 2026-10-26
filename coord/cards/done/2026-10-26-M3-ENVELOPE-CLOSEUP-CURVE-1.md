# M3-ENVELOPE-CLOSEUP-CURVE-1：包络视窗曲率编辑+精细面（C4 二段，D3A）
- 发卡：GLM 主管决策流 / 2026-10-26 晚（依据=用户 D3=A 裁定：段中点单柄+右键预设）
- 派发确认：已确认（用户裁定链）
- 验收负责人：GLM 主管决策流
- 池序 195；目标仓库=**前端仓 D:\Godot\project\vit-daw-frontend**；P3 / 0.5 天 / **依赖=194 合入**（canvas 件就位后领取）
- 模型分级：L1 / PC 执行会话；真栈腿同 §2.2 三纪律。

## 目标
1. **曲率句柄（D3A）**：段中点单柄——拖动=连续改 c 值（-1..1），右键=预设菜单（线性/快入/快出/S 型）；句柄命中入视窗局部手势机。
2. **曲线渲染**：画布折线改按 c 值采样曲线路径（绘制式样同段）；c 值透传写回（set_points points[].c，190 已透传实证）。
3. **精细面**：横向缩放（复用 194 时间轴缩放件）；值/时间吸附开关（勘察定形态：可选网格吸附，默认关）。
4. **回退**：c=0 默认（线性）零行为变化；194 点位编辑零回退。

## 文件域
194 的 envelope_closeup_canvas.gd 邻域+页签件（如标记位需要）+探针。越域即停。

## 验收标准
独立探针全绿（句柄拖动 c 读回/预设四档/曲线路径采样/吸附开关/回退）+既有探针零回退+真栈腿 exit 0（写点带 c→内核 get_curve 读回 c 值一致→预设切换→读回）；工件入 coord/runs/M3-ENVELOPE-CLOSEUP-CURVE-1/；端测边界声明。

## 停止条件
- 内核 get_curve 回执不携带 c 值（读回校验面缺失）→锚点上交（如实记录，不虚构读回断言）；
- 曲线采样性能超帧预算（重绘节流失效）→上交取证。

- 领取：2026-10-26 22:15 晚窗 / 主仓 origin/main=c23f3606 / 前端仓领取基线 main=ef3fc29（worktree envcurve1 新建后 git status --short 干净亲核）/ owner=ZCode GLM-5.3 PC 执行会话（M3-ENVELOPE-CLOSEUP-CURVE-1 卡 195 专属）/ 前端分支 port/m3-envelope-closeup-curve-1 / worktree D:/Godot/project/vit-daw-frontend-envcurve1 / 领取提交=e610c2b1；领取前核对：卡在 todo+派发已确认（用户裁定链，fetch 亲读 origin/main=c23f3606）、依赖 194 已验收合入（前端 main=ef3fc29=194 merge 提交）、doing 仅 GEN-SUNO-WEB-DESIGN-1（域不相交）、PC-RUNTIME-STACK 当前占用=空（最新释放 194@21:38）。
- 回执：2026-10-26 晚窗自验完成，交主管验收（详 [runs/M3-ENVELOPE-CLOSEUP-CURVE-1/envelope_curve_20261026_2310/RESULTS.md](../../runs/M3-ENVELOPE-CLOSEUP-CURVE-1/envelope_curve_20261026_2310/RESULTS.md)，工件随 port/m3-envelope-closeup-curve-1-stack 入库）——
  - **commit**：前端仓 `port/m3-envelope-closeup-curve-1@a4fef23`（5 文件 1520 行：envelope_closeup_canvas.gd 曲率句柄+曲线采样渲染+吸附精细面；独立探针 probe_envelope_curve.gd；真栈探针 stack_probe_envelope_curve.gd/.tscn；194 探针 G5 点击位坐标适配。**文件域遵守：仅 canvas 件+探针，页签件/dock/registry/车道零触**）；主仓 `port/m3-envelope-closeup-curve-1-stack`（真栈工件+RESULTS.md）；均已推远端。
  - **设计裁定遵守（D3A）**：段中点单柄——柄位=段中点 t 处曲线采样值（随曲线走），左键拖=连续改 c∈[-1,1]（竖直位移映射，向上=快入向 c 减小），右键=PopupMenu 预设四档=勘察预览 HTML 值（线性 c=0/快入 c=-1/快出 c=+1/S 曲 c=±0.5，S 曲符号沿用段当前 c、c>=0 取 +0.5——执行侧对勘察「±」记号的落定，供验收复核）；句柄命中入 194 局部手势机（DRAG_CURVE 新模式，零仲裁器改动）。
  - **探针断言面**：独立探针 probe_envelope_curve **55/55**（S 曲线采样 6——内核 tracktion_Bezier/AutomationCurve 公式独立参考实现逐值对照+平台方向+c=0 共线退化+199 段性能守帧 0.75ms<50ms 预算/H 句柄手势 14——柄位锚定+拖动方向/clamp±1/手势零发包/释放恰一笔 region c 透传+他点不动/零位移零写回/回显门/仅改 c/P 预设 10——四档值+幂等零写回+error 回滚+右键命中面/N 精细面 6——吸附默认关+T/V toggle 命中+时间网格/值 1/8 量化+关吸附连续+Ctrl 滚轮复用/R 回退 3——加点默认 c=0+点编辑语义保持/W 写回链 5——曲率拖拽态外部忽略+释放合并 echo wins+queue_failed 回滚）；既有探针零回退：**closeup 79/79**（G5 两处坐标构造适配曲线几何——点击位取采样曲线上值+按下即建 t 断言 8.5，**断言面不变**，曲线化后 (9,1.0) 直线旧坐标偏离路径=卡面目标②直接后果）/lane_ui 39/lane_auto 41/badge 33/datachan 50 全 PASS。
  - **真栈 run**：**envelope_curve_20261026_2310**（登记 c0be628a/释放 ad033bb5 三纪律全执行）——ps1 最小腿 exit 0；stack_probe_envelope_curve **R2/R3 连续 24/24 exit 0**（R1 探针期望缺陷轮留证：C4 参考 t 坐标写错段+GDScript lambda 值捕获，生产面 C1/C3 首轮即绿）；腿=装载 ValhallaSupermassive→C1 外部写带非零 c 三点→呈现+内核 get_curve 逐点一致（**读回校验面实证，停止条件①不触发**）→C4 柄位锚定→C2 句柄拖动释放恰一笔→内核读回 c 一致→C3 预设四档逐档内核读回一致；kernel=192 二进制只读复用（本卡零内核改动亲核）；VIT_PROJECT_XML 隔离亲证 947E4D00→9BB0AD2A+主检出未动。
  - **端测边界声明**：本卡为 Godot 画布件——自动探针以画布内部状态+局部手势机注入+生产传输链内核对照覆盖（独立 55+真栈 24）；**PopupMenu 实际弹窗呈现/菜单点击交互与拖动手感（视觉面）未在自动探针覆盖**，PopupMenu 在探针中以 headless 守卫短路（命中面+菜单项读回+apply 生产函数已断言）——弹窗视觉与手感面留给用户手测节（时域效果器控制首个可感功能落地时体验的既有裁定覆盖）。
- 验收：（裁定文件 / 验收 commit）
