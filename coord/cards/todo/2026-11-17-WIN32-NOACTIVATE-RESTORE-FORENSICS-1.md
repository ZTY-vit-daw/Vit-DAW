# WIN32-NOACTIVATE-RESTORE-FORENSICS-1：最小化状态恢复语义取证

- 池序403；P1；PC；发卡/验收=Codex主管`/root`；目标来自391首次显示阻塞，本卡为下轮候选，不自动领取/启栈。
- 前置：394-E2最后诊断FAIL已封存；本轮原生修改和第三normal运行已停止。原实现/工件分支保留，不覆盖失败。
- 待证假设：清WS_MINIMIZE+SetWindowPos FRAMECHANGED并未完成Win32最小化状态恢复（575dd713真实after hidden/iconic）。SW_RESTORE文档明确激活，不作产品修法；SW_SHOWNOACTIVATE有不激活文档依据，但它是否在首次show已消费后的隐藏最小化窗中恢复状态，以及Godot后续是否重改模式，须分别实测。SetWindowPlacement不自带更强整体保证。
- 单目标：产出“最小化→隐藏→非激活恢复”的精确API/消息/foreground对照证据，先隔离系统语义和Godot行为，禁止直接把新API塞产品再反复整栈试。
- 实现域候选≤3：前端仓VitShell/tests下一个自有窗口probe.cpp、同测试CMake小接线；主仓现有ps1烟测体系的可选取证腿（领取时锚定唯一脚本，不另建框架）。不改生产manager/GD/feature/395UI，不操作他人窗口，不键鼠/桌面控制。
- 取证步骤：①每API变体新自有测试进程，首次show明确已消费，复现minimized→hidden；②SW_RESTORE作已知激活对照，SW_SHOWNOACTIVATE与同showCmd placement作候选，记录IsIconic/visible/style/geometry/原FG事件与消息时序；③仅文档与自窗对照支持的路径再测独立Godot恢复，不接工程成功结论；④分别记录shell前台/外部前台，未覆盖如实not_exercised。
- 跑前冻结次数/成功条件/失败分类/止损；任一激活/恢复失败/事件缺口立即失败或覆盖不足，同断点两次停止；不得用放宽窗口标准凑通过。
- 真实栈/自有窗口测试均需主管下轮明确放行；独立worktree、fixture/log卫生与资源owner登记照旧。当前仅建卡，未授权本轮继续实验。
- 回执：精确命令/exit/API与原始消息/事件、source与binary哈希、可证/不可证边界、是否值得另发产品修订卡。
- 领取/回执/验收：待回填。
