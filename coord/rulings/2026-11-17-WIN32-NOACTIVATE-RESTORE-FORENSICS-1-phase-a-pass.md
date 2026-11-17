# 403阶段A：自有Win32恢复语义取证通过

- 主管/root，PC，2026-11-17；只验收系统API对照，不验收Godot或产品双窗。
- 代码：主8cca7d9c→main45f08186，前端ac3866c→main4243d57；工件2c1d7e82→main20697fa7。代码3文件全读，独立测试exe未接产品。R1 formal_01装置exit1/窗口0场景保留；主管亲跑helper stderr正负0/7、PS解析/diff0，未把原fetch未知exit改成0。
- R2 formal_02 runner/wrapper0、6个新PID/6场景：SW_RESTORE在synthetic/external各恢复并激活；SW_SHOWNOACTIVATE及同showCmd placement在两条件各恢复normal且保前台。单次每条件，不宣称稳定性比例。
- 主管直接重解析全部6个原始JSONL：seq连续、hidden/iconic前提、尾hook_removed，恢复观察2000/2016ms、64样本、双marker、maxgap16ms；四候选无WM_ACTIVATE/WM_ACTIVATEAPP/WM_NCACTIVATE正事件或SETFOCUS，两个control分别4/5个正激活消息与rawFG。并核原commands/exit、source哈希与在盘源码一致。
- 首次显式ShowWindow请求4后实证visible1，虽然startup_flags1/show0；仅可证该自窗已执行首次show，不能从launcher标记推导首次隐藏，更不能推导Godot。候选不是WS_EX_NOACTIVATE产品精确复刻；placement不获得更强保证。
- 观测边界：WinEvent成功hook、marker和有限pump间隔加原生WndProc交叉；本地seq连续不代表OS绝无丢事件。synthetic守卫不是真VitShell；自然external分支只读，未写他人窗口。
- 卫生：主管独立核Vit/probe进程与TCP6口/UDP4445全零；用户recents807226F4…13E353与before一致；无kernel/agent/Godot/工程/fixture操作。R1/R2原始字节仍在执行树；Git text=auto有换行规范化边界，下一阶段补原字节归档而不删展开件。
- 裁定：阶段A通过，可进入独立Godot-only恢复/模式同步取证；403尚未整卡收口。391/394/395/402与用户手测保持原阻塞，不合失败产品候选，不刷新用户入口产物。
