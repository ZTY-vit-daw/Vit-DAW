# M3-KERNEL-1：内核 automation 命令面（M3 步骤 2——automation.get_curve/set_points/clear_region）

- 发卡：GLM 主管决策侧 / 2026-10-12 晚窗（依据=用户 /morning 里程碑「M3 自动化拆卡」+M3-RECON-1 勘察回执（已验收 pass）§三/§六）
- 派发确认：已确认（用户 2026-10-12 早窗里程碑+晚窗「可以发卡」）
- 验收负责人：GLM 主管决策流
- 池序 94；目标仓库=D:\Vit_DAW
- 优先级 / 预估 / 依赖：P2 / 1-1.5 天 / **M2-TIMEFX-2 合入后领取**（主线序保守串行，池深充足）；设计输入=[runs/M3-RECON-1/RECON.md](../../runs/M3-RECON-1/RECON.md)（锚点权威）
- 模型分级：L2 / GLM 执行会话（内核 C++ 命令面+agent 白名单接线）

## 主管六裁定（依 RECON §六，已定案入卡，可推翻——推翻须上交证据）

1. **命令族命名与最小集**=`automation.get_curve`（读）/`automation.set_points`（写，整段或区域替换）/`automation.clear_region`（区域清除）；单点 add/move 不进最小集（区域替换可表达）；宏/曲线道（MacroParameter/AutomationCurveList）不暴露（后续批次再议）。
2. **CAS 粒度**=复用既有 plugin state 级 base_revision（与 set_plugin_param 现役 readback+base_revision 同粒度）——不建曲线级第二 revision 身份（M1-4「不留第二身份」先例）。
3. **坐标系**=time（秒）为写入主形态（agent 区间操作秒级口径现役）；get_curve 回执附 time_base 披露（引擎 TimeBase 原样）；beats 写入面不做。
4. **曲线回流**=DAD 证据层扩型先行（harness 摄入 feature_type 白名单扩 automation 曲线摘要型）——新 peer 投影不做（拓扑纪律+最小步，后续按需再议）。
5. **披露面解锁时机**=本卡**不改** context_manifest 四处 Excluded（能力未达不披露）；automation 从 Excluded 摘除随 M3 步骤 4（意图层/goal 编舞卡）解锁。
6. **文件域窗口**=内核命令注册段与 M2-TIMEFX-2 探针注册相邻——领取时按 PROTOCOL §2.1 核对（本卡排在 TIMEFX-2 合入后，天然规避）。

## 目标

1. **内核三命令**：handler 注册于 CommandDispatcher（:2842 起 handler 模式），实现体落 PluginRackControlService（参数控制域现役归属；resolvePluginParameterByID :387→getCurve() 一跳=M3-RECON §2.3 最短路径）。get_curve 回执=points[{t,v,c}]+bypass+time_base+param_id；set_points=区域替换语义（越界 fail-closed：t≥0、|c|≤1、value 域校验复用参数 valueRange）；clear_region=removePointsAndCloseGap。全部经 UndoManager 事务。
2. **VSP 面**：get 入 legacyReadOnlyCommands（VspKernelReference.cpp:146）+renderAllowlist（CommandDispatcher.cpp:2244，渲染中允许读=冻结判定观察）；写带 base_revision（裁定 2）；VSP 信封幂等按 set_plugin_param 现役模式。
3. **agent 面**：catalog 三工具 spec（automation.get_curve=RiskDirect；set_points/clear_region=RiskUndoable+确认，形态同 set_plugin_param catalog.go:1027）+别名+argspec；harness 摄入白名单扩曲线摘要型（裁定 4，DAD 证据层扩型——feature_type+落盘行）。
4. **真栈冒测**：扩展 ps1 体系新腿——get 空曲线→写三点→回读一致（逐点）→区域清除→回读空；CAS 反例（错 base_revision 拒绝）；exit 0。

## 文件域

VitApp/Source/Service/CommandDispatcher.cpp+PluginRackControlService.cpp（+VspKernelReference.cpp 白名单行）；agent/internal/tools/catalog.go+agent/internal/harness/harness.go（摄入白名单）+对应单测；scripts 冒测脚本。领取时与在飞卡核对。

## 验收标准

- 内核构建 exit 0（build_release 或 CMake 目标）+三命令真栈往返（冒测腿 exit 0 含 CAS 反例）；
- agent 单测：catalog 三工具+摄入白名单+越界 fail-closed；`go test ./... -count=1` 0 FAIL+gofmt 净；
- 回执附 run ID+曲线读写样本工件；端测边界声明。

## 停止条件

getCurve/resolve/白名单锚点与 M3-RECON 报告不符（内核已重构）→ 锚点上交。

- 领取：2026-10-12 20:33 / origin/main=095f79edb285c32ee337d7d4f48a673cd5743b51 / owner=GLM-5.3 执行会话（L2 内核 C++）@ PC（D:\Vit_DAW-wt-m3k1）/ 分支 port/m3-kernel-1-automation / worktree D:/Vit_DAW-wt-m3k1 / 领取提交见回执（本提交）
- 回执：（实现 commit 12da018c @ port/m3-kernel-1-automation 已推远端 / run ID=automation_curve_smoke_20261012_221700（PASS exit 0）+20261012_220826（exit 1 设计发现轮留证），工件 coord/runs/M3-KERNEL-1/（summary.json+console.txt+head.txt 随 port 分支入库；ps1_console.log 被 .gitignore *.log 规则忽略留盘可回指）/ 端测边界声明：①真栈冒测腿打内核 ZMQ 5555 命令面（裸命令+VSP 信封，与 Godot 宿主同面）——内核三命令+VSP CAS/幂等/白名单全链真栈验证；②agent 面改动（catalog/harness 摄入/pre-journal 校验）以 go test 95 包 0 FAIL+新单测 19 例覆盖，冒测栈中 agent 用 worktree 构建启动正常（HTTP state ok）但未走 /agent/invoke 驱动新工具（authority 面不在本卡腿内）；③webui 渲染面与用户旅程零改动不适用；④设计发现上交：clear_region 原裁定机制 removePointsAndCloseGap 经真栈证有边界值补偿锚点（AutomationCurve.cpp:630-635，区域覆盖全曲线后仍余补偿点），已按卡面"区域清除"语义改用 removePoints（RECON §1.3 在册 API）实现并复测通过——机制层修正，六裁定语义未动；⑤context_manifest 四处 Excluded 未动（裁定 5 遵守）；验收数据：内核构建 exit 0（全量+增量）、冒测 13 检全过含 CAS 反例（错 base_revision 拒绝且曲线不动）与逐点回读一致、go test -count=1 0 FAIL、gofmt 净（LF 级；工作树 CRLF 为检出约定，blob 级干净）；真栈占用登记 141635df/释放 29fe99ab（QUOTA-RECOVERY-LEGS-1 排队等待约 100 分钟后占用，两轮腿均手动拆栈复核 proc=0 ports=0）
- 验收：（裁定文件 / 验收 commit）
