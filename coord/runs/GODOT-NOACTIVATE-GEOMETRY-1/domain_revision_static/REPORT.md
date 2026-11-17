# 405 坐标域窄修：独立同system-DPI读回与逐角点诊断（无窗）

主管授权共享main9b36b52c卡末节读取后实施；owner/root/restore403。仅前端probe.cpp一个文件，前端提交87a5359。主scriptc43d66ad、observer、五步顺序、geometryTrace/readGeometryTrace、native geometry/z/focus/1px与JSONL判据均未改。前一runtime_01 runner1/targetmutation0原始工件完整保留，不归产品失败、不覆盖未知。没有Godot、DLL、窗口或DPIAPI运行。

## 官方契约与旧失败的可证范围

本目录references保存本轮官方页面：

- PhysicalToLogicalPointForPerMonitorDPI：lpPoint为physical/screen输入，按所给HWND的awareness变换，成功TRUE/失败FALSE；说明默认跨进程查询返回调用者坐标，而该API可取目标域。此页面没有明文说可把任意monitor角点都当合法窗口点，也没有GetLastError契约。
- LogicalToPhysicalPointForPerMonitorDPI：按同HWND transform，成功TRUE/失败FALSE；同样未规定GetLastError。
- **旧PhysicalToLogicalPoint**页面明确“coordinates ... have to be on the hWnd”，以及零宽/高窗口、某些旧平台hidden后的失败条件。但不能把旧API/平台限制未经证据直接套给PerMonitorDPI版本，更不能断言本次Windows11具体就是其中一条。
- GetDpiForSystem：调用上下文UNAWARE返回96，其余awareness返回actualsystemDPI；不应缓存，应每次使用时查询。
- DPI_AWARENESS_CONTEXT官方定义：SYSTEM_AWARE按systemDPI维持，monitor变化到不同DPI时系统缩放；PMv2按monitorDPI。限定同monitor且system/目标/guard DPI一致，可排除该已知跨DPI缩放差异；它不赋予混DPI通用转换保证。
- GetMonitorInfo指定HMONITOR填monitor/work bounds并返回成功/失败。其单独文档未承诺所有callercontexts永远同数值，因此新契约不预设相等，而在P与T实际独立读取后**exact比较**，任一差异拒绝。

旧run只记录短路组合false，没有逐调用结果。target_work被赋而其他零值未执行，无法确定哪个角点/API/前提失败。窗口当时iconic且屏幕work角点与窗口rect不同，提示原来把点转换当作任意monitor域校准过强，但只属可疑适用域，**具体firstcause保留unknown**。没有把失败追认成API不支持、没有凭无错误码解释成功。

## 新放行仅限显式证明的无缩放子域

唯一新准入模式same_monitor_system_dpi_exact_read，不是旧转换失败后的fallback：

1. P=PMv2 observer context独立GetMonitorInfo并核其rcWork/rcMonitor与guard启动冻结值exact相等；guard/child/current selected HMONITOR必须同一个，guard窗口context必须AreDpiAwarenessContextsEqual(PMv2)，目标必须AreDpiAwarenessContextsEqual(SYSTEM_AWARE)。旧physical preflight仍保留。
2. T=exact childwindow context作用域建立，当前thread与T必须AreDpiAwarenessContextsEqual。T独立GetMonitorInfo填rcWork/rcMonitor（不从转换输出复制），GetDpiForSystem(T)须非0且==GetDpiForWindow(child)==GetDpiForWindow(PMv2guard)。T/P独立monitor/work矩形必须**0px exact一致**；work须正面积且被同monitor容纳。错monitor/混DPI/错context/API读失败等拒绝，没有乘除猜scale。
3. p.workT/monitorT仅取独立T读值，workspace仍T work减workT-monitorT左上inset，五步所有写仍T，P只审计。原postshow外rect容差1px未扩大，准入domain相等反而0px。每个后续geometryEnvironment读回再核T/P frozen bounds、GetDpiForSystem与两windowDPI/context，不能只初始过后继续接受monitor迁移。
4. 此契约从实际sameDPI/同monitor/context/readback事实验证同一数值域；不因diagnostic转换成功就放行，也不因其失败就扩到不同scale域。其他monitor/PMv2child/unknownDPI/缺API仍拒绝。所用现代DPI API为静态import；OS无该API不能加载装置，不会降级到未知fallback。

## 八次逐角点转换诊断

Tscope有效时，work与monitor各两角正向PhysicalToLogical，以及从**独立T读取的work/monitor**各两角逆向LogicalToPhysical，共8次调用。每个调用独立运行，不以先前BOOL短路，输入/输出、方向/label/corner、BOOL、即时rawLastError、contextawareness/equality均单独保存。失败时输出值标output_valid0，不能拿“未改输入”当已转换结果。两permonitor API未文档化GetLastError，所以日志明确last_error_documented=false，rawError只作诊断、不是失败原因或放行条件。

coordinate_contract汇总分别保存P/Tmonitor读返回、system/child/guardDPI、P/T矩形、domainreason、四组diagnosticReturns和原始输出；admission_input=false使诊断与准入不混同。Tscope不可建立会拒绝，不能在P上下文伪做T转换。

## 纯验证与未变判据

configure0/build0；已知复制probe的C4244 nonce宽窄转换warning保留，无新增编译错误。--geometry-selftest当前61/61 exit0：原25geometry+16真实JSONL例全部保持，再加20pureSameDpiDomain例。覆盖合法168DPI exactdomain、system/child/guard/zeroDPI、错child/guard/缺monitor、目标非system/guard非PMv2/线程非T、P/Tscope或monitorAPI失败、仅1pxdomain差也拒绝、不同monitororigin、零work/越monitor、负坐标同域正例。纯数据测试没有调用任何窗口/DPI转换API，不能据61例声称真坐标域已过。

check_unchanged_criteria.py与unchanged_criteria_checks.json按24c6664前后精确函数段比较：geometryTrace、真实JSONLdecoder、godotProbe五步candidatepath、mutationhelpers及nativegeometry/1px函数全部unchangedtrue。observer与主script无diff；没有放宽首ordinary/早小窗/nested/postshow correction/guard后/无FG门。

完整命令退出码、build、61项原始输出和20个新case在本目录；同入口产生的JSONL反例使用本次独立PID目录，不写源码树或旧run。source/probebinarySHA在final_hashes.json；build/binary不入库。官方snapshot和引用见references，本轮只读下载，无浏览器/GUI/CUA。

## 后续边界

本轮只证明窄修可构建、纯domain谓词与未变旧trace/schema反例；8调用diagnostic和exactT/Pdomain尚未执行在真实HWND上。旧run未知因果不被新实现倒填。任何后续runtime须主管重新冻结预算/当前405预留/即时净空/recents与原字节证据；当前不启窗口、不补跑、不动生产，也不把406external成功算405新geometry通过。
