# GODOT-NOACTIVATE-GEOMETRY-1：普通大窗首次恢复的geometry/z准备

- 池序405；P1；PC；发卡/验收=Codex主管/root；派发确认=已确认（用户授权主管自主拆卡取证，403/404新证据必要延续）；未领取/未运行。
- 前置：403/404已验收取证，不含产品修复；E/E2完整工程失败候选仍保留不复跑。最小Godot已证API恢复/引擎模式同步，但native425×311与逻辑720×480不等，只通过正几何门，尚不能直接作为普通大窗产品方案。
- 单目标：在隔离最小Godot验证“首次非最小化可见即目标工作区大窗且在自有守卫后、无FG/无TOPMOST”的准备顺序，再决定394产品修订范围。不能先小窗show再resize冒充首帧正确，不用SW_RESTORE/激活式最大化，不用Godot no_focus带TOPMOST。
- 只读先核：SetWindowPos在iconic时对normal placement的影响，SetWindowPlacement normal rect的工作区/屏幕坐标与DPI语义，正确HWND是否PMv2/客户区约束；按官方源/精确源码锚点设计单一候选。不能仅凭文档PASS，不直接改manager。
- 预期执行域：前端原`VitShell/tests/win32_noactivate_restore_probe.cpp`、必要主仓现`scripts/vitshell_smoke.ps1`；隔离project/observer只run内生成；不动生产GD/UI/manager，独立worktree可复用已结束403/404树但需新owner领取记录+基线status/diff。当前模型建议Sol/high；无Astra常规调用。
- 先交静态方案与冻结次数/判据/失败分类/止损，主管审后再登记资源并运行；本卡未默认授新窗口启动。原则最多两前台条件各1进程、任何异常停、无补跑；精确HWND、原生WinEvent与engine mode、首次noniconic/show时geometry/z证据、≥2s保持、原始ZIP与hash/recents/全零继续。
- 成功只解锁另行生产修订，仍不替真实产品bootstrap/工程ready/395/完整矩阵。发现DPI/placement域外需求先报告，禁止通过放宽geometry或首显门槛凑通过。

## 领取与首段授权（2026-11-17 17:31 +0800）

- 用户本轮询问进展及是否可以继续，主管核对无新产品裁决事项后继续既有授权范围。owner=`/root/restore403`（GPT-6.1 Sol/high，PC），403/404已收口停写，现显式转405；验收=主管/root。
- 沿用独立执行树：主仓`C:/Users/timoz/.codex/worktrees/win32-restore-403/Vit_DAW`、前端`C:/Users/timoz/.codex/worktrees/win32-restore-403-fe/vit-daw-frontend`，branch均`codex/win32-restore-forensics-403`。主HEAD3b8a71d4、前端HEADcdf1924；tracked status/diff为空，主仅4个已知隔离shader cache目录untracked，保留不动不入库。共享main领取基线73a7f409；远端领取成功并重读owner后开工。
- 首段只读方案：核官方Win32/Godot源码、DPI上下文与placement坐标、当前探针与失败候选的geometry准备；不运行Godot/probe窗口、不改任何代码或旧工件。允许报告工件仅`coord/runs/GODOT-NOACTIVATE-GEOMETRY-1/PLAN.md`及必要官方引用快照。
- 回执短：单一候选API顺序、可证与待证、具体文件域、冻结≤2场景判据/分类/止损、可能需要用户裁决的产品语义（有则列，无则明确无）。先主管审方案，再决定代码与新runtime，不默认为已授启窗。
- 当前真实栈亲核全零且空闲，本段不预留栈，不授接管其他进程；不复跑E/E2，不操控他人窗口，不CUA/输入/截图，不改manager/生产GD/UI。

## 静态复核补段（17:47 +0800）

- 方案ab2206d4/0ced980e/8f1b7c8a已提交；DPI坐标契约明确，但异步WinEvent不足以证首次转换geometry，当前单候选不能给严格PASS。主管与Astra最疑难只读参谋复核官方契约；参谋无实现/运行/验收。证据边界见`coord/decisions/2026-11-17-first-visible-evidence-boundary.md`。
- 本段增加只读可行性勘察：既有目标Godot extension能否由应用自身在其窗口线程配合转换记录/geometry约束（合法自进程，不跨进程注入或subclass、不全局hook）。明确extension实际装配、主线程/生命周期、同nonce/PID/HWND协议、所有中间几何变化如何覆盖及停止/卸载边界。
- 仅写本卡`PLAN.md`补充或`TARGET-COOPERATION.md`报告；不改生产/探针/脚本代码、不起任何窗口。若需超3实现文件域、引入维护性差的原生机制或无法保留legacy，明确方案风险并上交；当前无用户产品语义待裁定，不因取证难而放宽首显门。

## 继续到手测的主管裁定（18:03 +0800）

- 用户本轮明确“继续，直到可以交由我手测”。主管接受405只读成果为前置，不继续盲跑；先拆406测试专用目标自进程观察器。观察对象为User32自有主窗转换的geometry/z因果证据，不宣称像素/首个DWM合成帧原子保证，也不改变“不抢前台、普通大窗在后”的产品要求。
- 406采用隔离GDExtension测试DLL，由Godot自身官方加载、在所属线程观察自己的HWND；不改产品extension/manager/GD，不跨进程注入/subclass、不全局hook。先实现和纯无窗检查，再单独冻结隔离运行。成功后405实施唯一placement/DPI候选，后续394修订与完整产品矩阵仍逐次验收。

## 唯一几何候选实现放行（18:45 +0800）

- 406 external真实加载/原生转换/自主卸载已主管定向核查；本轮整体exit2与synthetic缺失保留，不称整卡PASS。足以支持405使用已证观察渠道，405新增候选本身仍须独立验。406此段停写；同owner串行转回405。
- 前端改在新406树`codex/window-transition-406`，baseline1a4ff1c，tracked clean；主baseline1056214c，script2d7eb8f2。允许4测试源域：win32_noactivate_restore_probe.cpp、window_transition_observer.h/.cpp仅必要DPI线程上下文字段（不改核心策略）、主scripts/vitshell_smoke.ps1；必要pure geometry predicate自测在probe原文件显式无窗入口，不另框架。
- 实施PLAN唯一顺序：PMv2创建自有guard/只读物理审计→exact child T上下文几何API与往返→protect+hide→SetWindowPlacement(SW_HIDE,工作区normalrect)一次→隐藏态z置于可见自有guard后→单次SW_SHOWNOACTIVATE，无show后geometry/style/z修正。不能清WS_MINIMIZE试图恢复。external自有guard非激活可见且不变外部FG。
- 父/目标DPI记录包括GetThreadDpiAwarenessContext与GetAwarenessFromDpiAwarenessContext，不能以opaque handle数字猜枚举；几何写/读固定T，P只读审计，API转换精确记录。GetDpiForWindow/monitor冻结与roundtrip失败即拒绝。所有父mutation记录QPC begin/end与参数/return，child native QPC可关联；初始首次ordinary可见state必须目标rect/z，不能只看2s终态。
- 准备一个无窗native-trace评估器自测（可同probe）覆盖可见小窗后修正仍FAIL、嵌套错误提交、错坐标/monitor/身份、记录缺口、postshow修正、no-send等反例；最终用原生CHANGING after/CHANGED entry-after、全部可见非iconic快照和API区间因果核对，不只parent sample。范围仅User32状态，不升级为DWM像素帧保证。
- 仍仅代码/build/纯无窗/真实PS5静态装配检查（显式卡405/geometry/observer参数与metadata，ForensicsCard白名单及时更新），不启动窗口。完整提交+短回执后主管复核，再冻结≤2runtime、任一异常停、无补跑。不得把既有406external0归新geometry结果。

## 首个runtime预算（19:18 +0800）

- 前端ea643bf+24c6664、主scriptc43d66ad/工件4784b563；主管已读全部候选/DPI/native-trace解析与schema补测diff、真实PS5装配报告，并亲跑41/41纯geometry+真实JSONL解码反例exit0。测试装置允许进入限次运行，不替新geometry实证。
- 新RUNBOOK-runtime先冻结，新GODOT-NOACTIVATE-GEOMETRY-1/runtime_01，显式`-GodotRestoreForensics -ForensicsCard 405 -TestTransitionObserver -TestNoActivateGeometry`，前端shell-window-transition-406 @24c6664。synthetic/external各最多1新Godot child、总≤20min，条件not_exercised可如实结束该条件不强造；任一异常停止余下、无环境/装置补跑、无运行中修改代码/标准。
- 通过必须唯一五步目标T上下文顺序、physical/target/workspace往返/冻结monitor、safe hidden stagedrect/z先成立，target-native所有ordinary可见与proposal/嵌套提交已目标workarea且guard后/无FG或focus/非TOPMOST，single show内首ordinary转换因果记录、后无纠正、mode0/client一致≥2s、zero nativefail/drop/IO与自然卸载exit0全闭环。synthetic缺失仍保持overall coverage不足，不冒称2/2。
- 只隔离装置，不开完整DAW工程/内核/agent，无LLM。source/ABI/binary/HEAD/status/命令exit/原始父+targettrace/ZIP/hash/recents before/finally/全零齐；失败归类正确且首cause留存。主管启前全零已亲核，执行启前fetch owner+≤60s再次核，长build后重查。

## 坐标前提失败后的窄修订（19:25 +0800）

- runtime_01 runner1/targetmutation0，synthetic未覆盖，external SYSTEM_AWARE/同monitor/DPI168前置已过，联合坐标转换拒绝；工件544431ee原始保存。主管核报告与0候选调用事实，不归产品或API失败。单个角点返回未记，不能凭零矩形归因。
- 允许原probe一个文件窄修：转换逐角点单独记录输入/输出/BOOL/即时GetLastError适用边界，不短路吞后续事实；只读审官方PhysicalToLogical/LogicalToPhysical适用域（最小化窗角点在窗外的限制须核实，不猜）。为本机已证同monitor/systemDPI domain优先采用T/P各自GetMonitorInfo独立读同rcWork/rcMonitor并exact一致的无缩放同域契约，须有官方依据、同时核GetDpiForSystem(T)==GetDpiForWindow(child)==GetDpiForWindow(PMv2guard)，T=SYSTEM_AWARE、guard=PMv2，任一不一致拒绝；不能扩大到混DPI/其他monitor/未知domain。
- 这不是盲目跳过转换：明确限定原physical/T坐标数值一致的已验证子域，保留P/T独立读回与postshow物理审计；workspace差值仍用T monitor/work。旧转换拒绝原证据不改，报告分别说明原前提为何不适用与新契约证明范围。不改五步候选/native-trace几何/z/focus判据。
- 只无窗/静态官方核对与pure domain反例，代码提交后主管审再新预算；当前不授权窗口。代码若无法用官方契约支持该同域直接读法，停止只交诊断，不能自行补fallback或改容差。

## 同域窄修复核与R2预算（19:31 +0800）

- 前端87a5359/主工件2dbb76a6，主管读单文件全diff、官方契约/61pure例及未变判据逐段比对；支持严格同systemDPI/同monitor/T与P独立exact读回子域。旧角点失败原因保持unknown，8逐角点诊断不作为fallback或放行输入。
- 新RUNBOOK-R2/runtime_02，已审前端87a5359、主scriptc43d66ad不变，参数同R1显式405/observer/geometry。最多2child/15min、每条件1次，any anomaly停剩余、无补跑，不允许再同版本原样retry；若到五步候选失败需保全target消息/proposal/geometry首cause再裁定，不循环试API。
- 所有geometry/原生trace/首ordinary/z/FG/自然卸载/recents门原样，首domain准入exact0px，最终rect仍1px。synthetic前提缺失不追跑，overallcoverage如实。本轮不是完整产品验收。
