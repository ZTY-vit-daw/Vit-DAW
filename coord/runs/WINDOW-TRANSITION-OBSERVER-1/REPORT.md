# 406：测试专用目标线程观察器——构建及纯无窗自验

执行/root/restore403，主管/root验收。共享main1a968420的406卡与405末节读取后实施。新前端tree shell-window-transition-406，branch codex/window-transition-406，领取base575dd713初始clean；主树旧9e8fe2dd和4个shader缓存保留。本轮未改变任何生产extension/manager/GD/UI或旧403/404/405工件，也未启Godot/测试窗口/真实栈/GUI。

## 代码与文件域

前端仅4文件：VitShell/tests/window_transition_observer.h/.cpp、window_transition_observer_policy_test.cpp、既有CMakeLists.txt。提交d88a881 + 5ec88a9（最终）；两个commit已push至独立codex/window-transition-406，未合main。第二commit仅收紧“SetWindowSubclass成功但GetWindowSubclass核对失败”时仍保留installed所有权，确保deinit尝试移除或保留module引用，不把核验失败当已卸载；arm也拒绝任何same-PID前台窗而不只Engine HWND。

CMake默认仅增加纯策略test；只有显式VIT_TEST_GDEXTENSION_INCLUDE指向run内官方ABI header目录时才构建DLL。DLL链接user32/comctl32，不依赖godot-cpp、引擎库、生产WebView或ClassDB。导出vit_window_transition_test_init使用官方GDExtension初始化C ABI、SCENE级initialize/deinitialize回调；没有控制窗口的导出API。下一阶段必须由Godot自己的官方loader加载隔离测试.gdextension，不是父进程LoadLibrary/注入/跨进程subclass。

## 显式门控与记录机制（本轮只静态实现，未执行DLL）

- 只有VIT_TEST_TRANSITION_NONCE为16–64个ASCII字母/数字/下划线/短横字符，且VIT_TEST_TRANSITION_OUTPUT为绝对盘符路径时尝试启用。缺省/无效门控不创建输出、窗或subclass；CREATE_NEW禁止覆盖旧证据。没有实际加载验证，不能把此静态路径描述成已跑的零副作用测试。
- SCENE回调内EnumWindows只接受本PID、class Engine、无owner的唯一顶层主窗；TID必须等于当前调用TID。要求IsWindow/PID/TID一致、iconic、非TOPMOST、samePID不在前台。记录safe_armed与nonce/PID/TID/HWND/generation/DPI字段；所有生命周期行明确preinit_coverage=false，既往FG历史仍由启动前的外部原始hook证明。
- 同所属UI线程SetWindowSubclass/DefSubclassProc；只观察，不改geometry/z/style、不吞消息、不补激活。原生subclass记录每条消息（包含WINDOWPOS、SHOW、SIZE、STYLE、DPI、ACTIVATE/APP/NC、FOCUS、DESTROY/NCDESTROY）的entry和DefSubclassProc返回后数据，所有嵌套depth保留。WINDOWPOS/STYLESTRUCT/DPI建议rect在回调内复制，包含原始seq/tick64/QPC/unix_ms/TID/generation、native rect/client/placement/style/z邻窗/foreground/focus/active。
- callback仅固定4096项ring、固定64层配对栈、Win32只读状态和deferred PostMessage。没有同步IO/HTTP/GDScript/heap/pump；DefSubclassProc下游引擎原有行为不属于observer自行调用。格式化/WriteFile只在初始化、depth0的本进程message-only窗deferred drain或deinitialize安全点；记录drain_begin/end、overflow/dropped/dispatch异常。ring溢出/错线程/句柄或generation不符/嵌套不配对/CHANGED无CHANGING/NOSENDCHANGING均sticky失败，不能清掉失败再变PASS。
- NCDESTROY移除本subclass后继续DefSubclassProc，不销毁Godot主窗。结束核depth0、CHANGING配对、Remove结果/GetWindowSubclass无项、message-only窗Destroy且IsWindow=false、class Unregister；refdata为DLL静态存储，不会提前delete。installed期间持有本DLL一个模块引用；remove或helper释放失败时不卸载其callback地址。wrong-TID/depth非零deinit记录失败并保留引用而不是野指针卸载。shutdown日志与输出flush/close只在安全点，重复initialize不重新挂载。

## 官方ABI依赖可复现性

Godot4.6.1-stable已不直接存旧路径core/extension/gdextension_interface.h；对该旧路径请求404，未伪称下载成功。使用同tag官方文件gdextension_interface.json、make_interface_header.py、methods.py、platform_methods.py、misc/utility/color.py，在run/dependencies中通过未修改官方make_interface_header.run生成header。未克隆引擎/SDK，未执行Godot，未改公共SDK。

生成命令（从主执行树）：
python -c "import sys;sys.path.insert(0,r'coord/runs/WINDOW-TRANSITION-OBSERVER-1/dependencies');import make_interface_header;make_interface_header.run([r'coord/runs/WINDOW-TRANSITION-OBSERVER-1/dependencies/gdextension_interface.h'],[r'coord/runs/WINDOW-TRANSITION-OBSERVER-1/dependencies/gdextension_interface.json'],{})"

官方来源前缀https://raw.githubusercontent.com/godotengine/godot/4.6.1-stable/；JSON/生成器位于core/extension/，其他依赖在相同相对路径。全部源文件及生成header在dependencies中，SHA在dependency_hashes.json；生成header SHA256=D21AD8AB1362BE6C10E656E19A5DD8A5EC742154991F713D16BCA325F460C660。生成器Python缓存不提交，构建仅include该header。没有手写不完整ABI替代品。

## 命令/结果与原始证据

完整configure/build/CTest命令和退出码见commands.txt。VS18 x64 Release configure0；首次DLL+test build0、初次22项纯test0；加入溢出终态反例/生命周期与时间字段后最终23项纯test0，最后仅DLL门控/卸载核验修订再次build0。所有build日志保留，无编译warning/error。一个手工拼接的shell构建包装命令在执行前语法失败exit1、无输出/无新日志，随后以多行命令正确构建；它未执行测试/运行DLL，未掩盖测试失败。

CTest仅选择^window_transition_observer_policy$，未跑目录中已有其他窗口/manager测试。最终23个检查全部通过：nested成对、错配栈、固定ring溢出/保留顺序/空队列、溢出禁止clean终态、错TID/句柄/generation、destroy后消息、missingCHANGING/NOSENDCHANGING、未提交CHANGING、remove失败/仍在链、remove后回调、未remove/非零depth、depth上限、clean release及invalid arm。测试只包含.h纯数据与cstdio，不调用Win32窗口/loader/线程控制API。policy_test_01.log与policy_test_final.log分别保留22/23项原始输出；windows_created=0不是Godot覆盖声明。

final_hashes.json对应最终4源文件和最终DLL/testexe；build中二进制、lib/obj/cache不入库。输出观察DLL从未加载；未调用ABI entry，未执行初始化或任何窗口API。final_process_snapshot.json为末核，无Vit/Godot/probe/policy残留；recents维持807226F4AF3FC85C2B99BFD2FB827C1583E7C991826BDE5F0B4344A6DC13E353。旧cache保留无修改。

## 待证边界与下一关口

本轮只可证可构建和纯配对/失败策略反例，不能证真实Godot加载、UI线程/唯一主窗、actual nested WM链、deferred drain安全、normal release/异常卸载或first-transition geometry。对取消的WINDOWPOSCHANGING未收到CHANGED等实际系统行为会严格报告unpaired/coverage不足；不能先把真实出现的不支持形态豁免。

SCENE init晚于引擎最初show，故不承诺初始化前覆盖；终止进程前未收到shutdown尾则不能宣称卸载闭环。module引用故意保留是失败时防UAF策略，不是资源清理通过。所有runtime窗口/子进程、次数/止损及测试资源.gdextension装配仍需主管另批；本报告不消耗或继承403/404的运行预算，不授权生产机制或用户手测。观察目标是自有User32转换因果证据，不偷换成首DWM像素帧保证。
