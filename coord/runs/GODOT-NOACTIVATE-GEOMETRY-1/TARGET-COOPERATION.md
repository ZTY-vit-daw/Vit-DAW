# 405：目标自进程转换配合的只读可行性勘察

结论：**既有extension具备取得自有Godot主HWND、在应用调用路径执行原生代码的接缝，但目前没有覆盖该主窗转换内部的同步观察/约束机制。仅把恢复API移到主线程仍不足。自进程、所属UI线程上的短寿命主窗subclass可以成为候选因果记录机制；这是新的生产机制，不能当既有WebView WndProc复用，也尚不能宣称严格首合成帧问题已解决。** 本报告不授权代码、窗口、Godot或新的API实验。

依据：本轮fetch并读取共享main76fc0fb6的405卡末节及`coord/decisions/2026-11-17-first-visible-evidence-boundary.md`。主执行树HEAD8f1b7c8a，前端执行树cdf1924；前端remote main2d92d55仅观察器合入，生产extension/启动路径与所查基线相同。未merge/cherry-pick。403/404旧工件及四个shader缓存保全。

## 1. 既有装配及生命周期事实

| 实际入口/锚点 | 事实与局限 |
|---|---|
| `extension/vit_extension.gdextension:2`；`extension/src/register_types.cpp:10–35` | DLL入口`vit_extension_library_init`，最低SCENE初始化，注册`VitWebViewHost`/`VitWaveformReader`；terminator当前为空。不是早期DisplayServer hook。 |
| `extension/src/vit_webview_host.h:14` | `VitWebViewHost`是RefCounted，不是常驻Node；没有自动`_process`或UI线程调度器。GDScript正常直接调用在哪个线程执行，必须实际验证。 |
| `extension/src/vit_webview_host.cpp:1171–1178,1239–1257` | 已能从DisplayServer.WINDOW_HANDLE取得Godot主HWND，存入`native_->parent`；当前只判非空，没有exact PID/TID/生命周期generation绑定。 |
| `extension/src/vit_webview_host.cpp:278,384,427` | 既有`webview_child_wnd_proc`和notify WndProc对应extension自己CreateWindow创建的WebView/通知窗口，**不是Godot Engine主窗WndProc**。不能据此说已覆盖Godot WM。 |
| `extension/src/vit_webview_host.cpp:821` | `set_parent_window_shown`直接在主HWND调用SW_SHOW/SW_HIDE，无转换因果日志/geometry限制，不能作为严格首显证据。 |
| `extension/src/vit_webview_host.cpp:710–751`；`app/shell/agent_chat_window.gd:166` | RefCounted析构调用close，close释放WebView/自建子窗/COM等；autoload退出时close。新增主窗subclass不能沿用“DestroyWindow(child)”来销毁Godot主窗，必须有独立撤除顺序。 |
| `app/shell/agent_chat_window.gd:99–108` | 只有`VIT_AGENT_WINDOW_MODE=autoload`才实例化host；缺省/legacy惰性。VitShell direct明确不设置该flag，所以不能假设主窗机制会因这个autoload自动安装。 |
| `app/startup/start_page.gd:117–123,148` | direct入口在_ready先判`VIT_SHELL_DIRECT`，随后走异步bootstrap；这是可选的应用级调用接缝，但已晚于引擎最初show。场景切换释放start_page，不能让guard引用随之丢失。 |
| `extension/SConstruct:21` | Glob src/*.cpp；目前Win32 WebView实现被`_WIN32 && VIT_WITH_WEBVIEW2`包围。新机制若塞进这两个文件，必须明确SDK缺失/非Windows返回unsupported且零副作用，不能悄悄把legacy支持绑定到WebView是否创建。 |

官方Godot4.6.1源码`main/main.cpp`的重要顺序：3323行`display_server->show_window(MAIN_WINDOW_ID)`，3656–3657行才SCENE module/extension初始化；scene/GDScript_ready更晚。现SCENE extension不能从HWND创建前接管初始显示，也不能回溯早先可见普通窗。生产`project.godot:46`当前`window/size/mode=2`，不是已证明的minimized-first契约。旧E2的feature/minimized候选未合入，不得把其前提算既有生产事实。

因此，三文件接缝最多从“已证明安全最小化、尚未首次普通可见”的阶段开始保护；若之前出现普通可见主窗，必须startup_path_fail，不能安装后修正冲销。完整产品启动安全前提仍需另卡/另域，不能在这里偷改project/manager。

## 2. 同线程合法性与最小接缝候选

只在新机制经主管授权后考虑：

1. GDScript direct分支在任何await/工程bootstrap之前调用extension的独立guard API，传launch nonce/PID/expected exact主HWND、guard/reference、冻结work-area/DPI参数。extension自己再次从DisplayServer取MAIN_WINDOW_ID句柄，并用GetWindowThreadProcessId验证PID=GetCurrentProcessId且TID=GetCurrentThreadId，class/owner/hierarchy/IsWindow/generation成立。错线程直接拒绝，**不跨线程SetWindowSubclass、不全局hook、不注入、不SendMessage伪装自己线程**。
2. 已有类增加与WebView创建无关的guard方法；此调用不得触发create/create_window/COM/WebView/message pump。应用自身使用SetWindowSubclass/DefSubclassProc/RemoveWindowSubclass在自己的主窗上安装短寿命链。属于**合法自进程self-subclass，但仍是新原生生产机制**，需显式设计裁决、comctl32 linkage和生命周期测试，不能直接实施。单纯调用现有set_parent_window_shown或GDScript resize/focus信号无法填补WM_WINDOWPOSCHANGING内部盲区。
3. 主线程持有此次事务令牌，统一发起原计划唯一candidate序列；父进程只交配置/校验nonce并观察，保护期内不再与child同时改geometry/z。否则跨进程竞争可打破“所有mutation由owner串行发起”的因果边界。

≤3生产文件域的条件性方案：

- `extension/src/vit_webview_host.h`：独立guard API和不依赖WebView控制器的状态声明。
- `extension/src/vit_webview_host.cpp`：guard状态机、同线程self-subclass、原生固定缓冲记录/生命周期、方法绑定；保留原WebView方法语义。
- `app/startup/start_page.gd`：仅strict direct feature分支的安装/事务发起与root生命周期引用，不动legacy分支。

此三文件范围是**原生接缝候选，不是完整产品修复承诺**。真正独立、清晰的专用RefCounted类需要新.h/.cpp+register_types.cpp，再加启动调用就是4生产文件；若主管不接受在WebView类中容纳独立guard，应另拆该4文件卡而非为了计数压缩维护边界。现SConstruct的条件link也可能需要单独扩域；MSVC源级pragma可免改构建文件，但不能因此假称其他toolchain自动支持。不能在计数限制下省略必要后端/卸载保护。

## 3. 如何覆盖中间变化，而非只采前后快照

可维护self-subclass的目标是同步记录并约束**User32窗口状态转换**。建议先observer-only单独验证，不直接把约束代码当产品正确性：

- 状态`inactive -> armed_hidden -> presenting -> stable -> released/failed/destroyed`，每个状态关联nonce、PID、HWND generation、owner TID、冻结monitor/DPI/work-area/z reference。首次guard安装前状态只可由既有父WinEvent/实际安全iconic前提佐证，不能伪造已覆盖创建之前。
- 原生callback入口记录单调seq、depth、TID、message、wParam/lParam快照；WM_WINDOWPOSCHANGING保存完整WINDOWPOS的入口值，调用DefSubclassProc后记录downstream改写值/所有嵌套消息，再在回到User32应用变更之前检查最后值。若要强制位置/尺寸/insertAfter，必须在downstream返回之后按冻结目标改写并记录。Godot/DefWindowProc可能在其内部发起嵌套MoveWindow或WM_GETMINMAXINFO，因此每个递归depth都必须记录/受同规则检查，不能只记录最外层。
- WM_WINDOWPOSCHANGED入口/返回记录实际rect/client/style/z与engine可延后读数，WM_SHOWWINDOW、WM_SIZE、WM_STYLECHANGING/CHANGED、WM_DPICHANGED、WM_ACTIVATE/APP/NCACTIVATE、WM_SET/KILLFOCUS、WM_DESTROY/NCDESTROY分别原样记录。不要吞掉Godot应收到的消息，不在callback调用GodotClassDB/GDScript、HTTP、分配大对象、同步IO或嵌套pump；固定容量ring buffer超限即coverage不足，主线程安全点批量落盘。
- 官方WM_WINDOWPOSCHANGING契约允许改WINDOWPOS位置/大小/z；**修改SWP_NOACTIVATE/SWP_NOOWNERZORDER在此消息中会被忽略**，所以不能靠callback事后补这两flag防激活。所有发起API仍须原生带NOACTIVATE，WS_EX_NOACTIVATE从安全前提起保持；发现activation/focus记录即失败，不宣称靠拦消息已撤销抢焦点。
- `SWP_NOSENDCHANGING`可绕过CHANGING；未配对CHANGED、意外visible-normal、未知mutation、WndProc链替换、线程/句柄/DPI变化都必须fail-closed/coverage不足，不能因final rect正确通过。Godot官方4.6.1当前可见SetWindowPos路径未搜到NOSENDCHANGING，但这只是源审范围，不是动态完整证明。其他插件/窗口库、驱动、UI线程异步系统事件均需记录边界。
- 记录由child主线程自己的native thread DPI上下文产生，沿PLAN固定T/P坐标表；比较outerrect/workarea与engineclient时保留单位标签，不能再以size720与425混作“引擎改回小窗”。

已有Godot4.6.1 `DisplayServerWindows::WndProc`在WM_WINDOWPOSCHANGED中更新缓存并可能嵌套MoveWindow（5820–5954），返回0可能抑制WM_MOVE/WM_SIZE；这说明仅监听Godot resize信号并不完整。文件末user_proc是平台内部原生链，不是本GDExtension公开GDScript扩展点；不应直接依赖内部结构布局或使用引擎私有指针。

### 仍不能直接给出的保证

self-subclass使目标自身的CHANGING/CHANGED链可见并可限制一次正常User32提交，强于父进程OUTOFCONTEXT snapshot；但Godot链可能在downstream尚未返回时嵌套提交，系统/驱动显示合成不受该callback序列完全代表。仅拥有完整WM日志仍不能自动证明“首个DWM实际合成帧”；WM_SHOWWINDOW也不是compositor present回执。未配对、消息绕过、嵌套中已出现错误普通可见状态时，即使外层最后强制正确也必须失败，不能抵销。

若主管要求字面意义的首合成帧无竞态，仍需另行论证可控显示屏障/自有创建路径或引擎级支持；DWM cloak/uncloak/DwmFlush边界沿既有decision，不重复包装成原子保证。现SCENE extension无法保护最初show，是另一独立限制。最现实可验边界是“从已证明安全iconic阶段安装的目标线程原生转换事务，所有已配对User32状态提交geometry/z正确且保FG”，其与严格首合成帧的差额必须保持未证；**本报告没有权力将用户first-frame要求降级到此边界**。

## 4. 生命周期、释放和legacy

- 对象不能由start_page局部RefCounted独占；direct分支把事务对象放在SceneTree root metadata或同等root级持有处，随场景切换仍存活，nonce只能安装一次，重复/并发事务拒绝。不能沿AgentChatWindow autoload路径强行设flag以激活legacy WebView。
- 在owner UI线程安装并记录subclass ID、refdata、generation、原始exstyle；后续回调必须发现owner正在closing时只按安全链转发。避免RefCounted析构时仍存在callback指针、DLL卸载时悬挂指针。
- 正常稳定/用户可操作前：主线程先停止新mutation，关闭事务记录边界，核对entry/exit depth归零、seq无洞/ring无溢出、原生与engine状态一致，按顺序RemoveWindowSubclass并GetWindowSubclass确认未在链中，再释放refdata；期间不泵消息。只还原本事务新增的NOACTIVATE位，且同HWND/generation/PID仍成立，其他style不覆盖；保护解除后下一用户操作不再被geometry锁死。
- 若WM_NCDESTROY先到：标记destroyed，保留终结事件/记录，撤除自身subclass并继续DefSubclassProc，回调返回且depth归零后才回收状态；不DestroyWindow Godot主窗来“释放guard”。remove失败、线程错位、日志未flush必须记录teardown失败，不能标完成。应用退出/崩溃无法证明完整flush则coverage不足；父进程exact-child生命周期/原始尾记录做独立核对。
- `_exit_tree`/对象析构是后备而非唯一撤除点。现extension terminator为空，因此任何新self-subclass必须在DLL终止前证明全部guard已撤；不能只寄希望于进程结束。若root持有/termination回调闭环在上述3文件无法可靠实现，应扩域另卡，当前停止而不是省略。
- legacy、非Windows、未请求strict direct、缺nonce/feature/主HWND/TID不匹配时完全不安装/不改style/不发show/不创建守卫。现WebView展示/关闭/托盘语义不改；如guardAPI故障，direct路径报失败且保持隐藏，不回退激活式show/旧UI。

## 5. 建议主管裁定顺序（不是运行授权）

1. 先决定是否值得引入短寿命self-subclass新机制。若是，另拆纯数据状态机/记录卫生/重入与撤除验证卡，严禁复用生产完整工程来试错。先精简native observer-only，不把WM强制策略和产品发布混成一张。
2. 单独证明应用UI线程/生命周期、message配对、受控mutation拒绝及结束释放，需可证的反例（nested change、destroy/reuse、wrong TID、skip CHANGING、log overflow、remove失败、scene切换）；其后才讨论一次isolated Godot合作转换是否可关闭证据缺口。任何不支持的路径coverage不足，不修改标准续跑。
3. 若坚持绝对首合成帧且不授权引擎/更早创建路径，静态结论应记录“现extension层尚不能证明”，保持405暂停；用户产品语义无需再问，限制来自机制与证据能力。

当前无代码/构建/窗口授权，本报告未实施或运行以上机制。生产文件数、线程真实性及首show前保护均有明示待决边界。建议**仅评审是否另建目标自进程观察器卡，不放行现placement候选runtime，不直接修manager**。

## 参考与审阅命令

只读`git fetch/show`、`rg`、Get-Content及官方HTTPS源码/文档读取；无编译、Godot、probe或窗口API运行。引用URL：

- https://raw.githubusercontent.com/godotengine/godot/4.6.1-stable/main/main.cpp （3323与3657的先后顺序）
- https://raw.githubusercontent.com/godotengine/godot/4.6.1-stable/platform/windows/display_server_windows.cpp （WndProc/WM_WINDOWPOSCHANGED、缓存同步及嵌套MoveWindow）
- https://learn.microsoft.com/en-us/windows/win32/api/commctrl/nf-commctrl-setwindowsubclass （禁止跨线程helper subclass）
- https://learn.microsoft.com/en-us/windows/win32/api/commctrl/nf-commctrl-defsubclassproc （链转发）
- https://learn.microsoft.com/en-us/windows/win32/api/commctrl/nf-commctrl-removewindowsubclass （精确ID/PROC撤除）
- https://learn.microsoft.com/en-us/windows/win32/winmsg/wm-windowposchanging （WINDOWPOS修改及NOACTIVATE/NOOWNERZORDER修改被忽略）
- https://learn.microsoft.com/en-us/windows/win32/winmsg/wm-ncdestroy （销毁生命周期尾）
- https://learn.microsoft.com/en-us/windows/win32/api/winuser/nf-winuser-getwindowthreadprocessid （取得所属线程与PID）
