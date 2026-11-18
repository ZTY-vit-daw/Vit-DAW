# SHELL-WIN-BEHAVIOR-1：窗口行为包——DAW 不抢焦点+对话窗前台保持+窗口内动效

- 2026-11-18 主管更新：409 新产品窗口路径已独立验收并合入（shell06/external07 script/wrapper/host 均 exit0；前端 `bc8daf6`、主工具 `83bd5e77`），范围与原始证据见 [回执](../../runs/SHELL-WIN-ZPRODUCT-1/RULING.md)。本父卡仍 blocked：395 装载呈现兼容、组合真栈及实际产品入口刷新尚待完成；本轮用户只配合自动前台设置，未放行整体手测。旧 394/E2 失败工件保留，不重启原失败矩阵。

- 发卡：主管 2026-11-17 凌晨（[裁定档](../../decisions/2026-11-17-window-and-score-design-ruling.md) §一；用户「没问题，可以 gate 了」确认）
- 池序 391；P1；依赖=343 UI-HOST（同 HWND 机制）+371 live 装载事件裁定项（动效数据源）；真栈腿候栈位
- 目标：
  1. **DAW 不抢焦点打开**：VitShell 拉起 Godot 工程窗时以 no-activate 方式（Win32 `SWP_NOACTIVATE`/等效 show 方式）在对话窗**身后**出现；装载期间对话窗保持前台，之后标准 z 序（用户点谁谁前面）。
  2. **空间关系**：DAW 大窗在底（最大化或大窗），对话窗浮于其上（现状位置策略核对，保证可拖动不被强制压底）。
  3. **窗口内动效包**（跨窗联动动效已裁定不做）：装载/切换分步动效（对话流内进度——数据源接 live 装载分步事件，371 遗留裁定项一并落地）+焦点状态视觉响应（DAW 前台时对话窗标头轻量态/「工程已在前台」提示，回焦恢复）+侧栏收展动画（工程列表↔工程态，若 393 未合入则先做占位动画框架）。
  4. 真栈烟测腿：打开工程→DAW 在后出现+对话窗前台保持亲证（自动化断言焦点序）+装载动效分步事件到 UI 的链路。
- 边界：不动 Godot 窗口内部；不合窗（双窗架构保持）；跨窗联动动画不做（裁定）。
- 文件域：VitShell 宿主窗口管理（前端仓 `VitShell/`）+webui 对话窗动效面+371 live 事件链如需开启；领取时锚定。
- 验收：真栈腿 exit 0（焦点行为+动效链路断言）+用户手测节点（打开工程时对话窗不退后、装载动效可见）。

## 领取/回执/验收：待回填

## 2026-11-17 拆卡
本卡改为父卡，不直接领取；执行子卡394 SHELL-WIN-NATIVE-1与395 SHELL-WIN-PRESENTATION-1。先394原生层，395候388验收与394协议确定，父卡需两子卡+综合真栈证据后收口。

## 父391组合验收装置（2026-11-17）

/root/entry388在自身主仓执行分支准备现有vitshell_full_smoke/cdp/wrapper三脚本WindowBehaviorChecks默认off；owner与CardId参数化、沿早退卫生包装；原生焦点/ready与395消费按独立run可复核，不能用388稳定前端run代替。402-B白名单失败注入已显式交接，C++不可扩域。当前271占栈，仅静态准备。

## 组合真栈排栈（2026-11-17）
ROOT entry388为本轮验证owner，组合主仓1c104631/前端1301345；normal后按已冻结矩阵，389实际LegC另退出码；未pass不合入main。主管零栈组合focused57/build与最终11渲染报告均exit0，原compact190px真回退留证，CSS修min0后绿，断言未降。用户已改手测入口为VitShell产品cmd，裁定86ae9c87。

## 2026-11-17 本轮止损与状态
blocked：391normal旧版首次激活FAIL，E/E2两个候选均bootstrap/恢复失败，最后E2 after仍iconic；按预设停止第三normal/产品修与依赖矩阵。具体代码/日志/范围见coord/reports/2026-11-17-pc-development-progress.md与coord/runs/SHELL-WIN-BEHAVIOR-1/stopped-native-candidates/。本卡既有局部测试/呈现成果保留，不等于最终端侧通过；不合未验候选main、不放手测。恢复前先403独立取证，未自动领取。真实栈全零/哈希还原，owner无在飞运行，分支/worktree保留。
