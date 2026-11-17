# WINDOW-TRANSITION-OBSERVER-1：测试专用目标线程转换记录

- 池序406；P1；PC；发卡/验收=Codex主管/root；派发确认=已确认（用户“继续直到手测”及405主管裁定）。单目标=补同进程UI线程User32转换因果证据，非产品窗口控制实现。
- owner=`/root/restore403`（GPT-6.1 Sol/high，PC），405只读停写后转本卡；主worktree沿用`C:/Users/timoz/.codex/worktrees/win32-restore-403/Vit_DAW`，branch codex/win32-restore-forensics-403，HEAD9e8fe2dd，tracked clean/4已知cache保留。前端新独立worktree=`C:/Users/timoz/.codex/worktrees/shell-window-transition-406/vit-daw-frontend`，branch=`codex/window-transition-406`，base575dd713，初status/diff空。共享main领取基线d9b8f7db。领取记录推main并重读owner后才实施。
- 唯一前端域：VitShell/tests/ 下测试专用`window_transition_observer.h/.cpp`、`window_transition_observer_policy_test.cpp`、既有`CMakeLists.txt`；至多必要第五测试文件，先报告。主仓仅`coord/runs/WINDOW-TRANSITION-OBSERVER-1/`工件。本段不改现ps1或生产extension/manager/GD/UI，不改E/E2旧树/EXR。
- 实现路径：独立原生DLL按官方GDExtension C ABI加载（只需官方gdextension_interface.h，不依赖godot-cpp编译/生产WebView）。DLL只有显式测试env nonce+输出路径且self PID/TID/HWND验证通过才启用。加载由被测Godot自己的GDExtensionManager/隔离.gdextension资源完成，绝不LoadLibrary远程注入。没有授权时零副作用。
- DLL使用合法同线程SetWindowSubclass/DefSubclassProc/RemoveWindowSubclass观察自身唯一Engine主窗。只能读geometry/z/style/focus，不改geometry/不吞消息/不补激活。native callback入口/DefSubclassProc后、所有嵌套depth、WM_WINDOWPOSCHANGING/CHANGED（复制WINDOWPOS）、SHOW/SIZE/STYLE/DPI/ACTIVATE/FOCUS/DESTROY/NCDESTROY必须保原始seq/time/TID/generation，固定ring溢出/错线程/未配对等fail-closed。不得把宿主自己的WndProc伪称Godot记录。
- 回调不同步IO/HTTP/GDScript/分配大对象/泵消息；安全点可用本进程message-only测试窗的deferred消息批量写原始JSONL，记录drain/overflow/dispatch边界，卸载前depth0/Remove后GetWindowSubclass无项、refdata生命周期安全。输出阶段nonce/PID/HWND/TID/DPI上下文足以与engine/passive父hook关联。无初始化前主窗事件覆盖的虚假承诺；safe-iconic前提仍由完整前段观测证明。
- 首段只授权实现/构建/纯数据无窗策略测试；纯测试覆盖nested配对、overflow、wrongTID、destroy/reuse、缺CHANGING/NOSENDCHANGING、remove失败/退出闭环等必要反例。构建精确记录所用官方ABI header来源/hash，可将header放run依赖目录且只读include，不擅自克隆整个引擎/改公共SDK。
- 不授权任何Godot/测试窗口/真实栈启动；完成短回执提交/文件域/退出码/工件/边界。主管审代码后单独放行自有窗口或最小Godot，明确次数/止损，不自动执行产品。遇事实否定方案或需生产域即停报，不自扩域。用户手测门仍是完整真栈/渲染/旅程通过+实际入口产物刷新。
