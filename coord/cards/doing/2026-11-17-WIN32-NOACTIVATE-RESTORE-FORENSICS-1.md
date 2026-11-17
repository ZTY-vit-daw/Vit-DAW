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
- 领取/回执/验收：见下方本轮授权；原“下轮候选”文字为发卡历史，不再作为本轮阻断。

## 2026-11-17 接任主管授权与首轮冻结

- 派发确认=已确认。用户在本次接任提示词明确授权“在读档、核对owner/文件域后领取403并开展其明确范围的取证”；主管本轮仅放行自有Win32窗口对照，不放行Godot或完整工程启动，不复跑E/E2。
- 执行owner=`/root/restore403`，PC，GPT-6.1 Sol/high；主管最终验收=`/root`。这是403新owner，不接管394/402旧owner或候选生产文件。
- 主仓领取基线=`09be28a8abd035d6f52c8a48c7c4f4f36c03718e`，branch=`codex/win32-restore-forensics-403`，worktree=`C:/Users/timoz/.codex/worktrees/win32-restore-403/Vit_DAW`；前端基线=`d05043e11a79452b9155a89f3405668994eafdcb`，同名branch，worktree=`C:/Users/timoz/.codex/worktrees/win32-restore-403-fe/vit-daw-frontend`。两树领取时status/diff均空。领取记录推共享main且重读owner后方可实施。
- 唯一实现文件域：前端`VitShell/tests/win32_noactivate_restore_probe.cpp`、独立`VitShell/tests/CMakeLists.txt`；主仓既有`scripts/vitshell_smoke.ps1`新增显式早返回的可选取证腿，默认原烟测不变，不启动产品栈。工件只在主仓执行树`coord/runs/WIN32-NOACTIVATE-RESTORE-FORENSICS-1/`。若现有脚本不能安全容纳，先报告，不擅自另建框架。禁止生产manager/GD/UI改动。
- 运行上限：三变体`ShowWindow(SW_RESTORE)`已知激活对照、`ShowWindow(SW_SHOWNOACTIVATE)`候选、`SetWindowPlacement(showCmd=SW_SHOWNOACTIVATE)`候选；每变体每前台条件至多1个全新目标进程，共至多6个正式场景。条件为①自有守卫窗处于前台（明确是synthetic，不冒称真实VitShell）；②自然已有外部前台（只观察，绝不操纵他人窗口）。前提不具备记not_exercised，不强造外部前台。无环境补跑预算，装置问题留证后上报。
- 每目标进程必须先消费首次ShowWindow，再复现minimized→hidden，确认samePID/HWND、IsIconic=true、visible=false后才执行唯一恢复API；记录调用返回/last-error适用性、placement/style/exstyle/rect/client rect、WM_SHOWWINDOW/WM_SIZE/WM_WINDOWPOSCHANGING/CHANGED/WM_ACTIVATE等原始消息以及全区间foreground事件。记录source与binary SHA256、HEAD/status、完整命令、run ID、退出码。
- 跑前RUNBOOK必须落盘：候选成功=恢复normal/non-iconic且可见、非TOPMOST、目标无foreground/activation事件、守卫或外部前台保持、事件全区间可证；恢复后至少2秒观察，不能把一次即时快照当稳定。对照单列预期激活，不把对照的预期激活算候选PASS。观测缺口=coverage_insufficient；API/窗口前提不成立=probe_defect；环境/崩溃各自分类；候选激活或未恢复即该场景FAIL，不降判据。
- 止损：总计≤30分钟/6正式场景，不原样重跑；同确定性断点累计两次后停止相关路径。任一越域、资源冲突或不能证事件连续性立即停止并短报阻塞。结果仅支持API语义取证，不能解锁391/395或用户手测。
- 资源：主管已核Vit进程/TCP5555,5556,7878,7879,8787,9333/UDP4445全零；资源记录预留给403自有probe，启动前仍须fetch重读owner并≤60秒即时复核。不得启动kernel/agent/Godot；不触工程/fixture/用户配置，用户recents前后哈希亲证，若意外启动内核按快照/finally还原纪律处理并报告。
- 短回执：两个repo commit、文件范围、每命令退出码、报告/原始工件路径、可证/不可证结论、是否值得进入独立Godot取证及阻塞。原始日志留盘，最终验收由主管定向抽核。
