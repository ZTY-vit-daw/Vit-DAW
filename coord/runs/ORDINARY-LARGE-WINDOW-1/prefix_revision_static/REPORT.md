# 407 有界有向prefix关系窄修（无窗）

Owner/root/restore403；主管/root。读取共享main655393bb的407末节后实施。R1 runner1/probe20/show0及原raw/ZIP保全，不能用新语义追认PASS。前端提交ae86c7d，仅observer.h/.cpp与probe.cpp；主scripte61f636e无改。64容量不变，不改target16DIP/同DPI域/五步/engine0/首ordinary/QPC/nativeFG/2s/自然卸载。

## 为什么需要补方向字段

旧ZTrace只有global chain_loop/truncated/invalid_link以及up/down数组，无法知道失败发生哪个方向，或是在reference前还是已验证prefix之后。单看globaltruncated不能无歧义放行。因此依卡允许的必要字段域增加up_verified/down_verified、up_stop/down_stop；不是扩大容量、绕开unknown或抹除旧flag。

stop定义：0到链末；1下一节点将loop；2固定64capacity已满但还有下一节点；3下一节点IsWindow失败。采集器只把已检查IsWindow且未重复、未绕回target的连续节点写数组，每写一项verified=count。stop位置严格在所存verifiedprefix之后；原globalflags继续按两方向stop做OR保留原边界。子callback仍固定数组/只读/无IOheap，parent相同采集逻辑只读。格式化仅safe-point，JSONL raw新增4字段。

## 关系predicate

先核exactreferenceHWND/declaredparentPID/actualPID/valid/nonce；counts≤64、verified==count、stop码合法、global与各directionstop一致。所有存储节点非0、方向内无重复、crosschain重复/双ref矛盾拒绝。reference位于合法连续up数组即可above；位于down即可below，**可见native/parent门仍将below判FAIL**。已验证reference之后或无关方向的loop/invalid/truncated尾边界不再否定该reference关系，但仍完整记录。未到reference而先stop，或缺identity/verified/原始record、非法节点/计数、reference只出现在不可验证数据中，仍unknown/coverage不足。

这证明的是采集时有向prefix关系，不声称全桌面链完整或原子；没有把parentimmediateprevious当关系替身。祖先proposal锚点逻辑与几何native门未改，仍需已知anchor相对guard可证；未知不能pass。

## 无窗检查

configure/build exit0；knownprobeC4244ASCII nonce宽窄转换warning保留，没有新增error。--geometry-selftest96/96 exit0：原82反例保持覆盖，其中旧“globaltruncated无条件拒绝”案例具体化为reference前已截断/未到reference；新增14例含：up到ref+opposite截断pass、同方向ref之后loop、反向ref之后invalid、down到ref+up截断仍belowFAIL、ref超verified、非法节点、双ref/crosschain重复、非法stop码，以及真实JSONL完整schema往返/已证prefix反向截断接受、缺verified/verified不足、双ref、ref之前invalid等。所有真实JSONL仍调用实际readGeometryTrace，不修改运行判据为自验凑通过。

实际observerformatter与生成fixture字段静态比对68paths，missing[]/extra[]，exit0。observerpolicyCTest34check0。check_unchanged_gates.py逐段精确比较5a92006：inset+sameDPI、freezeGeometry、geometryTrace原ordinary/nativegeometry门、完整godotProbe五步/engineFG/teardown函数均unchangedtrue；主scriptdiffexit0。新解析器只追加prefix4字段，缺字段转换成越界值被predicate拒绝，不能缺record假pass。

commands.txt/configure/build/96原始输出+全部JSONLfixtures/34policy/schemaaudit/unchangedchecks/final_hashes.json入本目录。build/binary/DLL/cache排除提交。生成fixtures仅本次PID目录，旧工件不动。没有Godot/observerDLL加载、任何测试窗口/DPI/windowAPI运行，纯test只用内存+本run文件。

## 尚未证明

新directionprefix字段的真实采集、窗口自然转换后的z关系、首ordinary目标16DIP/engine0≥2s与自然卸载仍需主管审后独立R2。R1在旧门下coverage不足维持，不改结果。当前未授启窗/重跑；只交代码及无窗证据。来源自有guard/parentPID/nonce仍必要，不操作foreignwindow，不回退扩大64/邻接近似。后续产品/手测门未变化。
