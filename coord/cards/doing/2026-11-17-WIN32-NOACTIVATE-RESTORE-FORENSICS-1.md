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

### R1装置失败复核与R2独立预算（14:25 +0800）

- R1=`formal_01`，runner exit1，正式窗口场景0/6：PS5 Stop把fetch原生stderr提升异常，configure/build/Start-Process均未到达。原fetch实际退出码未被采集，不追认exit0；原始failure与finally failure均保留。不是API失败，不作Godot结论。
- 执行提交主仓`8cca7d9cf0429499ada9bf01ec9a1e56036351d3`、前端`ac3866cf38e4e6c87a4c36fd1e759e7890c3df3f`均远端亲核。主管已读完整3文件diff、R1 report/RUNBOOK、构建与静态工件；亲跑仅helper的stderr正/负对照得到0/7、父Stop保持，PS解析与diffcheck exit0。只认可装置修正，未认可恢复语义。
- 本次单独授R2：先落新的`RUNBOOK-R2.md`，新目录`formal_02`，同三API×两前台条件≤6场景、每场景新进程1次，最长20分钟；无环境/装置补跑，不修改既有判据，不覆盖R1。沿原candidate/coverage/probe/environment/not_exercised分类；两candidate失败停止，任何装置/观测/控制异常立即停止。此次不是复跑E/E2，仍不启动Godot或产品栈。
- R2只运行上述已审代码，若须改probe/runner先停止报告；前端baseline=ac3866c、主仓baseline=8cca7d9c，启动前fetch远端owner与即时净空。主管已再次核进程/全部端口零、recents维持807226F4…13E353。
- 收尾先提交报告与原始工件（排除build目录/二进制/缓存；按具体路径加入），保留所有失败，再短回执。协调卡/资源由主管统一更新；本授权成功同步main并重读后可执行R2，Godot仍需之后证据关口。

### 阶段A验收与阶段B隔离Godot授权（14:37 +0800）

- 自有窗口6/6通过，主管全6原始序列定向重解析+激活消息交叉核查+哈希与末态亲证；裁定`coord/rulings/2026-11-17-WIN32-NOACTIVATE-RESTORE-FORENSICS-1-phase-a-pass.md`。自窗三文件已合main，不纳失败E/E2产品代码。403仍doing。
- 阶段B只隔离Godot恢复/模式同步，仍非真实工程/产品栈。owner、两worktree/branch不变；允许仅扩原probe.cpp与原vitshell_smoke.ps1取证腿（CMake必要接线可用），不触生产manager/GD/UI。最小Godot project/scene/观察脚本只生成在全新run隔离目录，不加载前端生产工程/autoload/插件/extension，不起kernel/agent/HTTP/用户工程。
- 采用同版本实际GUI Godot binary，记录来源/版本证据/SHA；不能用console wrapper。先静态核候选E2使用的具体路径/启动设置与Godot源码锚点，保留差异。子进程用显式CreateProcess/STARTUPINFO记录；不凭启动参数宣称首次hidden，原生hook须在spawn前安装，按exact child PID/HWND归属观测，ready关联nonce/PID证明初show之后。禁止注入/跨进程subclass/全局输入hook。
- 本阶段唯一恢复候选=ShowWindow(SW_SHOWNOACTIVATE)，不做SW_RESTORE产品修法、不追加placement矩阵。最多2个正式Godot子进程，synthetic守卫前台和自然external各1次，独立run；无环境/装置补跑。自然前提缺失not_exercised不强造；首个失败/coverage缺口/崩溃/前台丢失立即停剩余场景，不改代码追跑。准备及运行≤30分钟，先落RUNBOOK-B冻结完整命令/成功条件/分类/止损，再配置构建。
- 前置最小项目初始mode=minimized、no_focus=false、不启TOPMOST，参数/生成项目入工件；ready后先亲证safe iconic且本child从未FG，再受控hide、临时NOACTIVATE/自身层级准备、候选API一次恢复。若startup抢FG，归startup_path_fail并停止，不把它误归API恢复失败或再纠正冲销。首次show前后native事实与Godot观察分开记录。
- 观测：原始child相关WinEvent全区间与连续性证据、Win32 visible/iconic/style/placement/geometry/FG，以及Godot自身Window/DisplayServer mode、focus通知与resize/readiness日志同nonce/PID时序。跨进程无WndProc原始WM通道须显式声明，不能伪称自窗WM已覆盖Godot。Godot脚本只观察、不runtime改mode或做恢复，以隔离引擎后续同步。
- 候选通过须恢复可见normal/non-iconic/nonTOPMOST/正几何，零child FG，守卫或外部FG保持，Godot mode跟随normal且至少连续2秒不改回minimized；原生与引擎读数冲突/时序缺口如实失败或coverage不足。只可证最小Godot路径，不证完整产品的工程打开/大窗z/ready/395UI。根据结果再发生产修订，不能直接修manager。
- 同一资源独占；启动前fetch重读owner及≤60秒即时净空，长build后再查；启动/清理仅本次exact child handle，结束全部Godot/probe/Vit进程和端口全零；用户recents启动前快照保全、finally核哈希（不启动kernel、原则不产生写入），fixture零使用。
- 回执仍短；提交两repo代码与原始工件，完整日志留盘。补阶段A原始字节ZIP/哈希（排除build/二进制，不删展开件），避免Git换行规范化丢原字节。若实现不能在原3文件域安全完成，先回只读方案与阻塞，不扩域。
