# 394 首次激活反例与只读参谋（2026-11-17）

- 事实：组合normal主仓1c104631/前端1301345，139个样本0采样错，5条加载期前台/style/z违规；首个同HWND可见样本已foreground=daw，之后约391ms恢复shell。nonce/ready后续成立不抵消此反例。原失败证据在候验分支codex/entry-final-validation 21d560ce `matrix_20261117_first/normal`。原Godot默认日志因先还原未保全，不能据此声称已验原始mounted/snapshot链；装置已补失败路径保存。
- 用户模型边界：常规上限Sol，Astra仅最疑难只读参谋。本问题由Sol/xhigh原实现owner与Astra/high只读参谋独立查证，无Astra实现或终验。
- 精确Godot4.6.1源码：console_wrapper_windows.cpp:138-149重建STARTUPINFO仅STARTF_USESTDHANDLES，丢首次显示设置；display_server_windows.cpp:1773起最大化分支显式SetForegroundWindow/SetFocus，先于no_focus；最小化分支仅ShowWindow(SW_SHOWMINIMIZED)。该CLI无--minimized。
- no_focus不能作捷径：_get_window_style:2424同时带WS_EX_TOPMOST；关闭该flag的style更新可能不带NOACTIVATE。mainwindow初show发生于main.cpp:3323 setup2，脚本/SceneTree稍后建立；提前在CREATE后restore会经WM_WINDOWPOSCHANGED:5849改变引擎内部minimized状态，反而使稍后show走激活分支。
- 候选A（待主管方案裁定/实现/实测）：直接同版本GUI sibling，STARTF_USESHOWWINDOW/SW_HIDE，专属feature override初mode1且no_focus=false；早期nonce/PID bootstrap ACK证明初show已过，再原生加临时NOACTIVATE、以非激活方式展开work-area大窗并放shell后。大窗满足用户裁定，无需激活式最大化。
- feature机制GODOT_EDITOR_CUSTOM_FEATURES仅TOOLS_ENABLED/editor binary运行项目，不假称export模板支持；特性未生效必须诚实fail-closed，不能静默回旧抢焦点路径。备选B初SW_SHOWMINNOACTIVE+本次主窗SHOW同步点，但归属更脆弱。LockSetForegroundWindow仅限制SFW，不作为所有显示/创建路径保证。
- 验收不变：无DAW前台事件，首个非最小化可见窗已NOACTIVATE且在shell后、无TOPMOST；bootstrap/显示/ready/release关联同nonce/PID；用户切外部app不拉回；终态清样式/线程/进程树。候选A先最多2次诊断运行，同断点先取证再修，不能原样凑通过；通过才继续完整矩阵。

官方来源：

- https://github.com/godotengine/godot/blob/4.6.1-stable/platform/windows/console_wrapper_windows.cpp#L138
- https://github.com/godotengine/godot/blob/4.6.1-stable/platform/windows/display_server_windows.cpp#L1773
- https://github.com/godotengine/godot/blob/4.6.1-stable/main/main.cpp#L3323
- https://learn.microsoft.com/en-us/windows/win32/api/winuser/nf-winuser-showwindow

上述为源码事实、文档约束及待证方案，未宣告已修复。原生候选隔离检出shell-first-focus-394；不改主检出既有状态。
