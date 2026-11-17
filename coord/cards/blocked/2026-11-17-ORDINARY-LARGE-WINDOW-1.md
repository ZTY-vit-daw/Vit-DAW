# ORDINARY-LARGE-WINDOW-1：保留普通态与真实前后层级

- 池序407；P1；PC；发卡/验收=root；派发确认=用户“继续直到手测”的主管细拆。owner=/root/restore403，Sol/high，PC；405已停写转407，前端同独立shell-window-transition-406 branchcodex/window-transition-406 @87a5359，主win32-restore-403 branchcodex/win32-restore-forensics-403 @377ca68b；trackedclean，旧隔离cache保留。共享main领取基线eab989fe；push领取重读后才实施。
- 单目标=首次ordinary窗已是workarea四边16DIP内缩的大窗/engine mode0，guard在child上方、非紧邻也可，无FG/focus/TOPMOST、无后纠正。依据decisions/2026-11-17-ordinary-large-window-contract.md，原405失败不改。
- 文件域4：前端probe.cpp、observer.h/.cpp；主原vitshell_smoke.ps1增加显式407门/参数动态。保留同域DPI与五步candidate顺序，只改冻结targetrect与z判据/原始观测；不改生产，不引新API候选，不扩多monitor支持。
- childobserver用仅测试env reference HWND+parentPID（child创建前记录，nonce绑定）取得自有guard，只读核有效/同声明parentPID，snapshot记录有界前驱HWND链、reference是否在上方/chain截断循环。来源为本parent新建guard，不操纵他人窗；回调仍固定容量，无heap/IO/pump。可保持最多64或128个前驱，有界且未找到reference的截断/错误不得当PASS。
- firstordinary与每nested native state都核targetrect/engine mode0/guard above，不要求neighbor；链是采集时读数非全OS原子快照，若原native proposal/读链相互矛盾则fail/coverage。parent原子show区间/targetnative所有changes因果核对与2s保FG、自主卸载自然0全门保留。只有目标rect生成和z语义换契约，不删mode0/早小窗/后修正/无activation/zero error门。
- pure tests更新原geometry反例：16DIP数值/负origin/tinyworkarea拒绝、helper/interveningwindow仍guardabove、childabove拒绝、missing/loop/truncatedchain拒绝、前后reference错PID/nonce拒绝等，真实JSONLschema往返与实际formatter静态核对；真实PS5卡407/门控兼容先验。仅实现build/无窗，不授新窗口；提交后主管审再独立≤2runtime，任何异常停/无补跑。生产394改动与完整栈另授权。

## 首轮runtime授权（19:53 +0800）

- 前端5a92006、主scripte61f636e/工件eb7725ed；主管审全4文件diff/原schema与82pure/34policy及PS5门禁原结果。保留core五步/T-P严格域与原首ordinary/2s门，new契约来源已记录，不回改405失败。
- 先RUNBOOK-runtime、新ORDINARY-LARGE-WINDOW-1/runtime_01，显式407/Godot/Observer/Geometry/Ordinary开关全有，最多2child/15min，每条件1次，任一异常停，no补跑/no运行中改代码。第一次ordinary target={28,28,2532,1412}仅在当前DPI168/该workarea预期，实际冻结按16DIP计算，不硬编码本机rect来凑。
- 原生reference/parentPID/nonce、up/down数组与loop/truncated完整性核对；guardabove允许中间窗，guardbelow失败；unknown链coverage不足不强推PASS。engine0/client一致≥2s、zero FG/focus/nativefail/drop/IO、自然unload0与recents/全零全门不变。只隔离取证，不产品验收。

## 关系证明的有界范围修订（20:00 +0800）

- R1 runner1/probe20，guard已在目标完整up链首位而无关down链64截断，show0；工件62558375保全，不追认R1PASS。主管接受观测器拒绝符合旧冻结门，但旧门“两个方向必须到整个链末端”超出产品guardabove关系所需。
- 唯一policy窄修：referenceOrder用有界有向prefix证明；从目标沿prev逐节点有效/PID/nonce绑定、无重复直到首次到reference即above证据；沿next同理到reference为below。未到reference而先loop/invalid/truncated必须unknown。已证prefix之后或无关方向tail截断是诊断边界，不能推成关系未证；也不能隐瞒真实prefix缺失或ref同时出现矛盾。
- 现有raw up/down数组保留，新增必要纯predicate/prefix完整性反例；不扩容量、不改geometry/fiveops/native首ordinary/FG/mode0/自然卸载标准。可只改observer.h关系predicate与probe纯selftest，observer.cpp不需要改则不动；如须分方向错误锚点才能无歧义决定prefix，只在同授权observer字段域补精确字段，先证明其必要性。
- 原“truncated无条件reject”反例改为ref缺失或截断位于ref前仍reject，补ref已在连续up前缀且down截断可证above、连续down前缀而up截断可证below、crosschain重复/双ref矛盾拒绝。真实JSONL仍原raw解析，不能把缺record或非法handle当prefix成立。
- 只代码/无窗自验/提交后主管审，再新R2独立预算；当前不启窗口、不补跑。

## 有向prefix复核与R2（20:08 +0800）

- 前端ae86c7d/主工件d9add341，主管审完整direction stop/verified采集与predicate/JSONL字段、96pure和未变geometry门报告；此修订有新证据范围，旧R1 coverage结果不改。保容量64，缺reference/有效prefix/身份仍fail-closed。
- 放行新RUNBOOK-R2/runtime_02，前端ae86c7d、主scripte61f636e保持。参数同407全部门，≤2child/15min，first anomaly停/无补跑/无运行中修代码。target16DIP/mode0/原生首ordinary全nested/QPC/有效refprefix/无FGfocus/2s/自然卸载/recents全门不变。

## R2真实层级反例（20:14 +0800）

- 工件1aec3670，synthetic到全部五步，native首ordinary尺寸16DIP/engine0/noFG正确，但有效downprefix第5项guard，首ordinarychild确在guard上方。runner1/probe40、external停，2s未完成；84native/41对/自然卸载健康。主管核原报告，保持FAIL，不改API追跑。
- 407 blocked；另408用本进程UI线程CHANGING-afterdownstream临时只约束z的因果必要接缝，先纯反例/静态审再新runtime。观测器记录before/afterguard，绝不将postshow repair抵销首显反例。
