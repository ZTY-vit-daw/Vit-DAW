# 406 隔离装配：构建及真实PS5静态检查（未运行窗口）

授权e9c5167d卡末节读取后实施；owner/root/restore403，主管/root验收。测试树仍codex/window-transition-406；主工件树codex/win32-restore-forensics-403。没有Godot、DLL、GUI/test-window或真实栈启动。原403/404/405证据和4个shadercache不动。

## 本段提交与范围

前端1a4ff1c：从已验cdf1924复制win32_noactivate_restore_probe.cpp（复制时源/目标SHA均85F2CBAB4A785D5D52328AAB2A24225A863818F26116D10EF17CF25F87AD2AED），只追加显式--godot-observer装配和自然停止协议。CMake新增probe target，原5个test和observer policy test保留。window_transition_observer.h仅追加纯StopHandshake；policy_test.cpp加11个停止协议反例/正例。**observer.cpp核心与5ec88a9完全相同**，未改subclass/固定ring/安全点drain实现。

主仓2d7eb8f2：仅现scripts/vitshell_smoke.ps1显式TestTransitionObserver，须GodotRestoreForensics+ForensicsCard406同时指定；406缺observer开关也拒绝，防止误走默认产品路径。ForensicsCard允许403/404/406且report.card动态；旧默认403，未选新开关时原路径及原测试行为保留。没有生产源码改动。

## 装配契约

新可选腿从run内官方ABI header依赖构建测试DLL，按SHA复制到隔离project。生成test.gdextension显式入口vit_window_transition_test_init、compatibility_minimum4.6、debug/release同一run内DLL、reloadable=false；不生成autoload/插件/生产extension列表。生成GDScript在自身_ready先设置该nonce和绝对native输出env，再调用自身GDExtensionManager.load_extension；是Godot官方loader，不是父进程远程加载或注入。OS环境仅在隔离child内改变。

父probe用--godot-observer识别此腿；先完成原404 exact engine HWND/generation/nonce/PID关联，再要求observer safe_armed记录ok/installed/failures0，并核exact nonce/PID/所属TID/HWND/generation。安全点drain状态在恢复前、每个恢复观察循环中及末尾核failures/dropped/io_failed；未arm或observer异常不执行候选。仍只原404 hide/NOACTIVATE+单次SW_SHOWNOACTIVATE，没有405新geometry/z策略。

完成/失败后父只用CREATE_NEW写本nonce到隔离stop.request，不覆盖/复用旧文件；fixture_process读取未经trim的正文与nonce精确匹配，仅处理一次。在Godot UI线程调用unload_extension，确认is_extension_loaded=false，读取本nonce/PID/HWND的removed_verified/shutdown，写独立unload_ack.json、记录engine事件，再get_tree().quit(0或2)。native观察文件没有闭环则fixture退出2。

父等待actual child自然exit0，核ack nonce/PID/HWND/nonce_matches/unload_result0/notloaded和native removed_verified/shutdown exactTID与零fail/drop，再认成功。固定5秒超时或缺ack/remove/shutdown、非零/提前退出均失败；只失败时exact handle TerminateProcess兜底。forced0不能变成功，旧--godot非observer路径保持原收尾。policy的纯数据StopHandshake直接被父使用，包含请求重复/复用、nonce、deadline、退出码/forced与闭环条件。

## 无窗验证与证据

- VS18 x64 configure/build exit0：DLL、pure policy test、copied probe均可构建。DLL核心无warning；复制的旧probe保留已知C4244宽转窄nonce warning，没有错误。
- CTest只选^window_transition_observer_policy$：34check/0failure，exit0；原23条策略加11条请求nonce不符/旧文件复用/双请求/timeout/ack nonce/missing remove/shutdown/engine ack/forced exit/非零退出/自然0闭环。未运行目录中其他窗口test。
- copied probe只执行--identity-selftest：23/23 exit0，无窗。未执行--godot/--godot-observer。
- ctest -N注册6项，原loading_window_policy、window_status_serialization、launch_result_correlation、launcher_presentation、hidden_bootstrap均在；仅列清单，不把未运行的5项标通过。
- 真实Windows PowerShell5.1.26100.9818/ACP936 AST ParseFile+helper：403/404兼容与406预留正例通过，错卡/owner/释放/历史/空header等9case全通过。实际原生git-show资源顶部为404释放，本段无runtime预留；对406**应拒绝且实际拒绝**，actual_lease_check.json明确不是实际406放行证明。
- 真实PS5仅AST抽取门控和资源生成片段，不调用脚本top-level：5条option门控（旧默认、缺Godot、错card、缺observer、406显式）全通过。最终生成project.godot/main.tscn/observer.gd/test.gdextension均UTF8无BOM；native日志split使用单个GDScript换行escape，stop正文精确不trim；源脚本ASCII兼容无BOM。
- DLL只用dumpbin /exports静态检查：出口vit_window_transition_test_init存在，exit0，没有LoadLibrary/Godot执行。

check_ps51_assembly.ps1、check_ps51_gates.ps1为run内验收工件，不是新增脚本框架。初版静态fixture及v2/final均保留，未覆盖旧字节；新增v2/final仅收紧stop正文精确匹配与卸载后observer_loaded读数，最终source/loader/配置匹配在static_fixture_final。所有GDScript检查只是生成文本/入口/路径/SHA/编码检查，**未声称Godot parser或实际loader已通过**。

## 命令与哈希

commands.txt和各configure/build/ctest/PS5/dumpbin原始输出记录实际退出码；final_hashes.json对应最终源码+DLL/probe/test产物。copy_provenance.json记录cdf1924来源，native observer核心差异检查exit0。二进制/build/cache不提交；fixture中的DLL也排除，只留loader与文本/SHA。

未来如主管授权运行，调用必须显式：
powershell -NoProfile -ExecutionPolicy Bypass -File scripts/vitshell_smoke.ps1 -GodotRestoreForensics -TestTransitionObserver -ForensicsCard 406 -FrontendWorktree C:/Users/timoz/.codex/worktrees/shell-window-transition-406/vit-daw-frontend -RunRoot <全新406run目录>
完整运行卡仍须冻结完整命令；当前没有运行授权，不能直接执行此模板。

实际loader/SCENE安装、nested消息配对/drain、自然unload/remove/shutdown及Godot自然exit0仍未验证。运行前必须新资源406预留+精确owner、即时净空/recents快照、固定次数/失败止损、真实原始工件及最终全零。现阶段不新增生产/产品结论，不放用户手测；主管审装配后另裁定。
