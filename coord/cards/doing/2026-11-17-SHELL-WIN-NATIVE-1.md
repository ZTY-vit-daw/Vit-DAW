# SHELL-WIN-NATIVE-1：391-A 原生窗口焦点与层级

- 池序394；父卡391 SHELL-WIN-BEHAVIOR-1；P1；归属PC。
- 发卡/验收负责人：Codex PC主管 `/root`，2026-11-17。
- 派发确认：已确认，用户本聊天批准今日执行方案并指令开工；依据 `coord/decisions/2026-11-17-autonomous-subagent-execution.md`。
- 目标：VitShell打开Godot DAW时保持对话窗在前、DAW在后（不永久置顶）；仅装载窗口期维持焦点，完成后用户点谁谁在前。提供可经既有status面观察的真实状态以供395消费，禁止虚构焦点/进度。
- 文件域：前端仓 `VitShell/godot_manager.*`、`launcher_bridge.*`、`main.cpp`，最多5实现文件；若须status_server或Godot场景扩域先报告。允许同目录必要单测；不动webui、不动app/shell，后者归396。
- 并行：388主仓UI域不相交。与395的协议接缝先报主管确认；不得擅自启动真栈。
- 验收：原生构建exit0+有意义的窗口状态/生命周期验证；真实三件套按现有vitshell_full_smoke体系扩展，断言焦点与前后层级+装载完成后解除保持，exit0。代码自验不能替代真栈。脚本改动若跨到主仓先请求主管分卡/扩域。无computer-use、键鼠模拟或桌面操作，测试采用应用status+只读窗口查询。
- 停止：前提失效/域外缺口/缺少可靠加载终态→附锚点上报；同确定性断点连败2次停止原样重跑；真实栈未经主管放行不得启动。
- 领取：owner=`/root/window391`（GPT-6.1 Sol/xhigh，PC）；前端基线`d05043e11a79452b9155a89f3405668994eafdcb`，分支`codex/shell-win-native-394`；worktree=`C:/Users/timoz/.codex/worktrees/shell-window-391/vit-daw-frontend`，领取时tracked status为空。主仓领取基线`116f22155b1ba52c5c6e37076799791a3d1ee1a7`。正式开工以本卡领取提交成功同步共享main并核对远端owner为准。
- 回执/验收：待回填。

### 终态边界裁定（2026-11-17）
现launch-result ok来自start_page change_scene_to_packed受理，未证项目仓库加载完成；本卡原生阶段明确命名scene_accepted，以现有终态有限释放临时noactivate，不宣称workspace_ready。允许修复spawn后ResetEvent清掉快速回执的同域竞态。395只呈现实证步骤，不捏造ready；完整workspace_ready若需Godot装配+首次project_repository.apply应独立小卡。main只沿既有set_chain/detail暴露真实状态，不扩status_server生产文件。
