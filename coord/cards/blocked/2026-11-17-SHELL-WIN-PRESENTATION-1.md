# SHELL-WIN-PRESENTATION-1：391-B 真实装载反馈与窗口状态呈现

- 2026-11-18 主管更新：409 已对当前 Windows 产品开发入口的 shell/external 两条件验收合入（前端 `bc8daf6`、主工具 `83bd5e77`），详见 [409 回执](../../runs/SHELL-WIN-ZPRODUCT-1/RULING.md)。本卡既有实现、分支、owner 与证据保留；下一步先核对旧 394-D 事件消费面与 409 新基线的兼容，再决定组合真栈。此更新不自动领取、不授予启栈权、不恢复旧 394/E2 未验分支，也不把本卡视为最终通过。

- 池序395；父卡391；P1；PC；发卡/验收=Codex主管 `/root`。
- 派发确认：目标已确认（2026-11-17用户开工授权）；实现范围待394勘察与388合入后锚定，不提前领取。
- 依赖：388验收合入；394真实状态协议确认。
- 目标：新壳内呈现真实装载步骤/失败态、DAW在前时轻量焦点反馈、侧栏收展动效；尊重reduced-motion。禁止假进度和跨窗联动动画。
- 文件域：主仓webui新壳/样式及现有宿主事件消费面，最多5实现文件，领取前列定；不与388/389并改同文件。
- 验收：渲染DOM正反断言+现有ps1真实三件套事件到UI旅程exit0。缺事件如实声明，不以定时器代进度。
- 停止：可靠事件缺失或需超5文件跨域→锚点上报再拆；真栈单owner登记与卫生规则照旧。
- 领取/回执/验收：待回填。

## 2026-11-17 正式领取
- 主管/验收=/root；388/401已主管PASS并合入be43a2a2。
- owner=/root/window391（Sol/xhigh，PC）；branch=codex/shell-presentation-395；worktree=C:/Users/timoz/.codex/worktrees/shell-presentation-395/Vit_DAW。
- 本领取提交同步main并重读owner后开始；先将干净checkout ff到本协调main，记录HEAD/status/diff。真栈未放行，271验证先排；只零栈实现/测试，不启动Godot/agent/kernel/VitShell。

### 395固定实现与协议（主管确认）

5生产文件=App.tsx、launcher/ConvergedLauncher.tsx、project/ProjectSurface.tsx、新project/loading/windowPresentation.tsx/.css。394-D协议前端组合7bd812f1：vit_launch_progress vitshell.launch_progress.v1含request_id/全bridge单调seq/真实phase/detail/可选stage；ask_vit_shell/window_presentation_ready非空subscription_id先订阅再握手重放fresh window_state与末progress，replay不自增seq。没有观测保持unknown；window焦点独立，不用release_reason当本请求终态。真实既有阶段按请求关联呈现，progress=null，无百分比。仅workspace_ready表示装配+首份快照，scene_accepted边界如实。旧五type不改。

复用LoadingCard；侧栏只现有recents收展与可复用slot框架，393列表未实现不假装已落地；尊重reduced-motion。不得改gallery/render-smoke/旧LauncherSurface/ui_entry_smoke（归389），必要独立render扩展需生产App装配正反断言而非自建件。最终组合真栈由主管统一排队。

## 2026-11-17 本轮止损与状态
blocked：391normal旧版首次激活FAIL，E/E2两个候选均bootstrap/恢复失败，最后E2 after仍iconic；按预设停止第三normal/产品修与依赖矩阵。具体代码/日志/范围见coord/reports/2026-11-17-pc-development-progress.md与coord/runs/SHELL-WIN-BEHAVIOR-1/stopped-native-candidates/。本卡既有局部测试/呈现成果保留，不等于最终端侧通过；不合未验候选main、不放手测。恢复前先403独立取证，未自动领取。真实栈全零/哈希还原，owner无在飞运行，分支/worktree保留。
