# GODOT-MAIN-HWND-OBSERVATION-1：精确主窗关联的观察器修订

- 池序404；P1；PC；发卡/验收=主管Codex `/root`，2026-11-17。派发确认已确认，用户授权主管自主拆卡/执行取证；必要性来自403 B2已证观测缺陷。
- 单目标：用Godot同nonce/PID就绪回执的native HWND精确绑定被测主窗口，读全PID窗口清单作反例，禁止首个无owner窗口启发式误指helper。仅修取证观察器，不改产品。
- owner=`/root/restore403`（Sol/high，PC），403已停止、显式转本卡；两执行worktree及branch沿用，主baseline=beb38025（runner f9792767），前端baseline=bb2a401，前端clean；主仅两已知隔离shader缓存untracked，不动不入库。共享main领取基线=c623b68b；本领取推main并重读owner后可开始。
- 唯一代码文件：前端`VitShell/tests/win32_noactivate_restore_probe.cpp`。runner无需改，三文件B候验组合可作基线但不合main。工件在主执行树`coord/runs/GODOT-MAIN-HWND-OBSERVATION-1/`。不改403失败证据，不触manager/生产GD/UI、用户工程/fixture。
- 观察契约：ready的nonce/PID/seq完整且PID=已创建child；从native_handle取HWND，IsWindow且GetWindowThreadProcessId exact child，无owner/主层级合理，read-only EnumWindows列出该child所有候选含class/owner/style/iconic/visible/geometry/SHOW匹配。错PID/无效HWND/缺rawSHOW必须明确拒绝，不能回退首个unowned。采样固定exact HWND，不在循环换成helper；ready读取至关联之间晚消息按原始native tick保留。
- 无窗验证：构建、静态/纯关联谓词反例覆盖①helper先枚举但engine主HWND有效②错PID③无效HWND④未ready/错nonce⑤缺show事件⑥主HWND重用/更换。不得以probe自建窗口假装真实Godot。必要自测试函数只在同probe.cpp内，不能默认启动窗口；不启动Godot/产品。
- 本轮只授权实现+无窗检查+提交短回执，不启任何新测试窗口/child。主管审核diff/反例/构建后，才会另登记新运行预算（最多两个条件）。403 B预算关闭，本卡不是原样重跑。
- 停止：需其他代码域/无法精确归属/丢原证据/运行资源冲突即上报；不启kernel/agent/完整工程，不复跑E/E2，不CUA/输入/截图/其他窗口控制。结尾提交hash、唯一文件域、命令exit、报告路径与待证边界，不自行合main或最终验收。
