# GODOT-MAIN-HWND-OBSERVATION-1：精确主窗关联的观察器修订

- 池序404；P1；PC；发卡/验收=主管Codex `/root`，2026-11-17。派发确认已确认，用户授权主管自主拆卡/执行取证；必要性来自403 B2已证观测缺陷。
- 单目标：用Godot同nonce/PID就绪回执的native HWND精确绑定被测主窗口，读全PID窗口清单作反例，禁止首个无owner窗口启发式误指helper。仅修取证观察器，不改产品。
- owner=`/root/restore403`（Sol/high，PC），403已停止、显式转本卡；两执行worktree及branch沿用，主baseline=beb38025（runner f9792767），前端baseline=bb2a401，前端clean；主仅两已知隔离shader缓存untracked，不动不入库。共享main领取基线=c623b68b；本领取推main并重读owner后可开始。
- 唯一代码文件：前端`VitShell/tests/win32_noactivate_restore_probe.cpp`。runner无需改，三文件B候验组合可作基线但不合main。工件在主执行树`coord/runs/GODOT-MAIN-HWND-OBSERVATION-1/`。不改403失败证据，不触manager/生产GD/UI、用户工程/fixture。
- 观察契约：ready的nonce/PID/seq完整且PID=已创建child；从native_handle取HWND，IsWindow且GetWindowThreadProcessId exact child，无owner/主层级合理，read-only EnumWindows列出该child所有候选含class/owner/style/iconic/visible/geometry/SHOW匹配。错PID/无效HWND/缺rawSHOW必须明确拒绝，不能回退首个unowned。采样固定exact HWND，不在循环换成helper；ready读取至关联之间晚消息按原始native tick保留。
- 无窗验证：构建、静态/纯关联谓词反例覆盖①helper先枚举但engine主HWND有效②错PID③无效HWND④未ready/错nonce⑤缺show事件⑥主HWND重用/更换。不得以probe自建窗口假装真实Godot。必要自测试函数只在同probe.cpp内，不能默认启动窗口；不启动Godot/产品。
- 本轮只授权实现+无窗检查+提交短回执，不启任何新测试窗口/child。主管审核diff/反例/构建后，才会另登记新运行预算（最多两个条件）。403 B预算关闭，本卡不是原样重跑。
- 停止：需其他代码域/无法精确归属/丢原证据/运行资源冲突即上报；不启kernel/agent/完整工程，不复跑E/E2，不CUA/输入/截图/其他窗口控制。结尾提交hash、唯一文件域、命令exit、报告路径与待证边界，不自行合main或最终验收。

## 无窗复核与独立运行预算（15:17 +0800）

- 前端cdf1924仅1文件，主工件4641f6c3；主管亲读完整diff、23例原输出/构建与source/binary hash，亲跑显式无窗selftest23/23 exit0，未使用自建窗口充Godot。engine同nonce/PID/native handle唯一选择、CREATE/SHOW generation与每次固定身份核验符合卡意。
- 现在授权本卡新独立runtime：先RUNBOOK-404冻结，新`coord/runs/GODOT-MAIN-HWND-OBSERVATION-1/formal_01`，主runner f9792767内容、前端cdf1924保持，不准运行中改代码。最多synthetic/external各1个新Godot child，≤15min，无补跑；任一失败/观测不足/装置/环境异常即停止其余，不以部分正常续凑。
- 仅最小隔离Godot与单SW_SHOWNOACTIVATE路径；所有403阶段B成功标准原样维持，加exact engineHWND/PID/nonce/逐窗inventory/generation关联证据。safeiconic后hide+NOACTIVATE再restore，Win32 normal与engine mode0两秒一致、无childFG、保baseline。自然external缺失not_exercised，尊重他人前台。跨进程无WndProc/stdout独立capture缺口如实，不伪造完整性。
- 停止后精确区分：关联成立但candidate恢复/模式失败是新功能反例；身份不成立仍coverage不足，均保全原证据，不能继续改实验追跑。无生产产品/工程启动或修改，结果不直接放行391/395/用户手测。
- 资源登记成功重读后，每child前fetch owner和≤60秒净空；recents快照hash、子进程exact handle拆净、末态全零，原字节ZIP排cache/build，全部commit/push后短报。主管保留最终验收和资源释放。

## R1门禁装置失败与必要接缝扩域（15:23 +0800）

- R1在runner硬编码resource第4行403卡号时拒绝404，actual owner未变；runner1/窗口0。报告与原字节证据1c3df5fe保留，不归HWND或API失败。主管承认在换卡时未核此接缝。
- 现仅扩域原`scripts/vitshell_smoke.ps1`资源断言（本卡总2文件），修为显式预期卡号参数与exact owner验证、只允许403/404，记录实际匹配行；旧默认403兼容，404运行必须显式指定。必须同时要求首个当前资源记录=占用/预留、卡号与owner一致，不能历史任一行搜到owner就放行；空闲/其他owner/错卡/空记录均拒绝。
- 只无窗静态helper正反例/PS解析/代码diff，至少403预留、404预留通过，错owner/错card/释放/空闲/历史匹配/缺header拒绝；不调用top-level、fetch、进程或窗口。提交代码/精简报告后主管复核，再决定运行预算。现不授新窗口/Godot运行。
