# 405 唯一geometry候选：实现与无窗自验

Owner /root/restore403；主管/root验收。读取共享main2ed2c65c的405末节、406交接后串行回405；406旧runtime与整体exit2/synthetic缺失保留，不归本候选。前端新406树base1a4ff1c，主base1056214c/script2d7eb8f2；只改授权4文件：probe.cpp、observer.h/.cpp必要DPI字段、现vitshell_smoke.ps1。未改生产、CMake、policy_test或旧证据；未启动Godot、任何测试窗口、DLL或GUI。

代码提交：前端ea643bf，主scriptc43d66ad，独立分支push，不合main。纯数据自验不执行DPI/windowAPI路径。

## 唯一操作顺序与坐标域

显式--godot-geometry，script须同时GodotRestoreForensics/TestTransitionObserver/TestNoActivateGeometry/ForensicsCard405；旧默认403/404和406入口保留。guard在PMv2线程上下文创建，可见normal/nonTOPMOST。synthetic只请求自身guard前台；external的自身guard用WS_EX_NOACTIVATE+SHOWNOACTIVATE非激活显示后放底，要求原externalFG不变，绝不操作其HWND。guard/monitor/DPI被冻结。

绑定exact engine nonce/PID/TID/HWND/generation与observer safe_armed后，读childwindow context、GetAwarenessFromDpiAwarenessContext、GetDpiForWindow；要求实际SYSTEM_AWARE且目标/PMv2guard DPI相同、同monitor。P上下文读取物理rcWork/rcMonitor，T=目标window context下执行PhysicalToLogicalPointForPerMonitorDPI两角转换和LogicalToPhysical反向核对，≤1px往返；另核T上下文GetMonitorInfo一致。workspace rect为workT减(workT.topLeft-monitorT.topLeft)，保留monitor虚拟桌面origin，不直接混screen/workspace。

所有child几何写读固定T上下文，作用域结束还原原线程context；P仅guard/物理审计，不在PMv2传target-logical二次缩放。监控冻结workarea/monitor/windowcontext/双方DPI；变化拒绝且不迁移/重算。物理审计每次读值及T/native参数都有日志，opaquehandle和awareness枚举分开，不猜43025含义。observer.cpp只补thread/window awareness、context-equality与thread handle字段，原callback/ring/drain/卸载策略不变。

childmutation固定五步：(1)仅加EX_NOACTIVATE；(2)NOACTIVATE/NOZORDER隐藏；(3)SetWindowPlacement一次、flags0/showCmdSW_HIDE/rcNormalPosition=workspace目标；(4)hidden SetWindowPos仅z插到可见guard后，NOMOVE/NOSIZE/NOACTIVATE/NOOWNERZORDER/FRAMECHANGED；(5)单次ShowWindow(SW_SHOWNOACTIVATE)。禁止清WS_MINIMIZE/MAXIMIZE，不要求hidden预置已noniconic；也不调用SW_RESTORE/最大化/其它candidate。placement readback必须exact，显示后没有geometry/style/z纠正。Post-show只读状态/DPI作用域、2s观察、原406gracefulunload。

每mutation含API名、精确参数、意图QPC、API区间begin/end、真实return/error适用性与native状态。begin在意图日志IO之后，show end在返回后立即QPC，目标native QPC同域可关联。正常SetWindowPos/ShowWindow内部时序仍为待证；代码不把文档当运行成功。

## Native trace评估门（不是parent snapshot PASS）

运行时readGeometryTrace从目标原生JSONL取完整已drain前缀，按seq/QPC/identity/awareness/generation、entry-after LIFO depth、CHANGING/CHANGED计数核对；停止自然卸载后再读取完整final trace。前缀未完成的正在写入尾只在prefix检查丢弃，最终文件partial/缺drain则coverage不足。所有字段bool必须明确0/1，缺新DPI字段不能被无符号-1误当true。

mutation须五步一次且T一致，任何show后targetmutation就是postshow_correction失败。全部native可见非iconic状态要求目标outerrect/非TOPMOST/非zoomed/guard为直接上邻/无本窗FG和focus；较早ordinary、首次/任一nested小窗、短暂隐藏或重新iconic、后纠正均stickyFAIL。CHANGING after记录downstream提议的位置/尺寸/insertAfter；未写出的NOMOVE/NOSIZE恢复geometry必须由normal placement目标佐证。NOSENDCHANGING、缺CHANGING/不配对/seq或clockgap、错误坐标/身份为coverage。第一个目标native普通可见状态须落在sole show的QPC区间内，不能用调用后延迟snapshot冒充transition。任何错误native状态不被最终2s正确抵销。

实际P/T外rect目标各≤1px（与冻结往返舍入口径相同），hidden placement readback0px。engine logicalclient须在500ms内与T读client一致，mode0后≥2s保持、noFG/focus/guard后与native一致。record中的system-aware DPI变化/无效scope/api/readback失败拒绝，不修改坐标容差追PASS。协议仍要求native safe_arm、zero fail/drop/IO、自然stop/unload/removed_verified/shutdown/engine_ack/child0。

这些门覆盖User32目标自身消息链；不声称DWM首个像素合成帧原子屏障，不补405以外产品bootstrap/大窗ready/395证据。未知可见状态间隙、原生proposal无法推定时coverage不足，不降到first-sampled。

## 无窗检查及原始失败

configure0；第一次build1，原build_01.log保存两个实现编译问题：Windows near宏撞局部lambda名，DWORD*与unsigned*类型不符。分别改within/使用DWORD字段后build_02及最终产物构建exit0；移除新增shadowwarning，旧复制probe的C4244 ASCII nonce宽窄转换warning仍保留。无窗口运行失败被覆盖或重跑。

--geometry-selftest最终25/25 exit0：正确nested；nesteddepth/pair错误；可见小窗随后correct仍fail；earlyordinary；nested小窗提交；重新minimized/hidden后恢复；坐标/monitor域无效；PMv2二次转换写域；nativeawareness/DPI变化；错PID；seqgap/clock倒退；postshow修正；NOSENDCHANGING/CHANGED缺CHANGING；downstream小proposal/z-top；非guard后/TOPMOST/focus；latefail与firsttransition在API外。数据是纯构造trace，不造窗口冒Godot。

--identity-selftest23/23 exit0；CTest仅observerpolicy34check exit0（旧策略未改，重编译验证新Snapshot布局）。所有原始log保留；最早geometry23项与后续25项分别记录，不把旧binary归最终hash。最终仅加P只读audit日志后又build0，未改纯policy，未以此引入窗口测试。

真实powershell.exe5.1.26100.9818/ACP936 AST ParseFile0；sourceASCII无BOM。10个资源helpercase全过（403/404/406兼容、405预留、错card/owner/released/history/缺header/empty）；8个geometry/observer/card显式optioncase全过。实际remote顶部仍旧406非占用/释放记录，因此405当前资源应拒绝且确实拒绝：不是实际405启动许可。AST只抽helper/guards/生成片段，未执行top-level。复用原静态装配harness生成四个配置无BOM，ABI入口/hash/官方loader、自身env及stop/unload文本一致；不是GDScriptruntime parse通过。

## 工件与待证

commands.txt、各build与25/23/34输出、PS5harness/正负例/实际resource、无BOM生成资源和final_hashes.json在本目录。最终source+binary/DLLhash齐；build/binary/fixtureDLL/cache排除提交。旧406observer固定ring行为未扩域；源码diff可定向审3前端+1script。

本轮没有任何测试窗口、Godot/observerDLL加载、真实project/fixture/内核/agent/CUA；仅无窗selftest/CTest进程，最终进程空与recentsSHA807226F4…13E353在final_process_snapshot.json。当前未获runtime预算；本报告只支持build+纯门与静态装配。待主管审单一candidate，再另登记≤2场景，任一异常停，无补跑；不得把406external成功或本25构造trace当405几何实证。生产修订/完整矩阵/实际入口刷新与用户手测仍在后续关口。
