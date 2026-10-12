# M3-RECON-1：tracktion automation 层级/快照能力/VSP 暴露形态实锚勘察报告

- 卡：coord/cards/（池序 93）；执行：闲时代跑·主管会话（GLM-5.3）@ D:\Vit_DAW / 2026-10-12 早窗
- 基线：origin/main=f9a0d7cd（领取时，领取提交后 origin/main 推进至 de522840）；**tracktion_engine submodule=b4397498**（engine 仓 origin/main，锚点稳定基）
- 方法：grep+读源码，零代码改动、零真栈启动、零探针；全部锚点领取基线当日亲核，标 文件:行号
- 端测边界声明：纯只读勘察零代码零真栈，不适用端侧烟测门槛
- 结论一句话：**tracktion 引擎 automation 能力完整在场（数据模型/API/宏/曲线道/录制回放全套源码可编译），MIX-CAP-RECON「引擎原生未暴露」经复核精确成立=VitApp 服务层与命令面无曲线载体，而参数解析链已就位（getCurve 一跳可达）；暴露建议走新命令族（automation.*），agent 侧观察/披露/编舞三处接入点全部实锚，其中披露契约今天显式排除 automation（四处 Excluded）**

---

## 一、问1：tracktion automation 层级/快照实锚

### 1.1 模块与文件

| 文件（tracktion_engine/modules/tracktion_engine/model/automation/） | 内容 |
|---|---|
| tracktion_AutomationCurve.h/.cpp（178/1136 行） | 曲线数据模型+全 CRUD API |
| tracktion_AutomatableParameter.h/.cpp（451 行） | 参数面：曲线持有/曲线源/迭代器/监听 |
| tracktion_AutomationCurveList.h/.cpp | 曲线道（track 级曲线列表→CurveModifier 指派） |
| tracktion_MacroParameter.h/.cpp | 宏参数（多参数集合自动化） |
| tracktion_AutomationRecordManager / tracktion_AutomationMode / tracktion_ParameterChangeHandler | 录制/模式/回放值分发 |
| tracktion_AutomatableEditItem.h/.cpp / tracktion_AutomatableParameterTree.h | 可自动化条目基类/参数树 |

### 1.2 数据模型（层级实锚）

- **AutomationCurve**（tracktion_AutomationCurve.h:14）：ValueTree 持久化（`parentState, state` :117；构造 :23-25）。点三要素 **AutomationPoint{time(EditPosition), value, curve(−1..1 bezier)}**（:42-63，point.toValueTree :53）。
- **树内形态**：curve 状态树类型 **IDs::AUTOMATIONCURVE**（cpp:15/:25）；点属性 **IDs::t / IDs::v / IDs::c**（cpp:86-93）；**IDs::paramID** 绑定目标参数（cpp:52-57）；bypass 键（:17）。挂接位置=**参数所属可自动化条目的 parentState**：AutomationCurveSource 构造（AutomatableParameter.cpp:189-196：`curve(edit, TimeBase::time, ap.parentState, state)`+`setParameterID(ap.paramID)`）；取回=`getCurve()`（h:40→cpp:1325-1328 经 curveSource:260）。tracktion Edit=ValueTree 序列化（工程 XML），故**快照能力=ValueTree 深拷贝/UndoManager 事务**（curve 全 API 收 UndoManager*），另 `removeAllAutomationCurvesRecursively`（h:112）与点级 toValueTree（:53）可作快照取用面。
- **TimeBase：time|beats**（h:17-21）——beats=拍号网格（与 10-12 已合入的 music_context 节奏/拍号披露键 GEN-CTX-DISCLOSE-1 语义对位）。
- **参数面**：AutomatableParameter（h:40 getCurve/:89 hasAutomationPoints；curveSource 成员 :260；Listener automationChanged/parameterChanged 双回调 :213-222；AutomationIterator 播放游标缓存 :420-446）。
- **宏层**：MacroParameter : public AutomatableParameter（h:18，多参数集合）+ Assignment 树（h:28，IDs::MACRO cpp:1067-1074）、按 ID 全 Edit 查找（h:59）。
- **曲线道层**：AutomationCurveList（h:330；addCurve(destParam) :350）→AutomationCurveModifier 指派给参数（CurveModifierType h:52；assign 树创建 AutomatableParameter.cpp:1070）——track 级曲线经 modifier 复用给多参数。

### 1.3 API 面（写入/读取/区域/整形全套，h:86-109 与自由函数 :154-176）

clear(:86)/addPoint(:89)/removePoint(:90)/removePoints(:91)/removePointsAndCloseGap(:92)/removeRedundantPoints(:93)/getPointsInRegion(:95)/movePoint(:98)/setPointValue(:101)/setCurveValue(:102)/rescaleAllPositions(:105)/rescaleValues(:106)/addToValues(:107)/simplify(:109)；自由函数 simplify(:155)/mergeCurve(:159)/getValueAt(AutomatableParameter&,EditPosition)(:170-173)/getFullRange(:176)。**结论：读写双侧 API 完备，无需引擎侧补码。**

## 二、问2：VitApp 侧现状复核+最短暴露路径

### 2.1 MIX-CAP-RECON 结论复核（§2「自动化曲线面不存在」）

**成立，且精确化**：VitApp/Source 全仓曲线级调用（getCurve()/hasAutomationPoints/addPoint/getValueAt）**零命中**；三处 AutomationCurve/AutomatableParameter 引用全为**参数级**——resolvePluginParameterByID（CommandDispatcher.cpp:441-449、PluginRackControlService.cpp:387-395：getAutomatableParameterByID+master volume/pan 回退）、set 写入（CommandDispatcher.cpp:552 targetParam->setParameter+clamp，handler 注册 :2842→PluginRackControlService::handleSetPluginParam）、display probe（VitPluginGrabber.cpp:22/:35/:57/:101/:121）。即：**引擎能力全在（§1.3），缺的只是 VitApp 服务层到命令面的载体——不是引擎限制**。另注意 agent 观测恒 0 的另一半在 harness 摄入白名单（§四.1）。

### 2.2 内核既有 automation 触点（三个，暴露时可直接利用）

1. **hasAutomationState**（TransportAudioService.cpp:57-63：XML 字符串级检测 "automationcurve"/"automationpoints"）→ :70/:198 用于轨透明判定（带 automation 的 volume/pan 不得视作透明）——**内核已有 automation 在场性读判先例（字符串级）**。
2. **grabber 已有字段位**：VitPluginGrabber.cpp:164 `supports_automation=true`（每参数行无条件置位）——agent 可见的 grab 面已携带该标志，暴露后改实判（getCurve().getNumPoints()>0）一行。
3. **参数解析链就位**：§2.1 resolve 链返回 te::AutomatableParameter::Ptr→getCurve() 一跳可达（AutomatableParameter.cpp:1325）。

### 2.3 最短暴露路径（供问3命令族挂接）

新命令 handler 注册于 **CommandDispatcher（既有注册段，参照 :2842 起 handler 模式）**，实现体落 **PluginRackControlService（参数控制域现役归属，:387 resolve 先例）**或新建 AutomationControlService；读面可直接产出 grabber 同风格 JSON points（DisplayProbe 先例 VitPluginGrabber.cpp:57）。处理器参数与轨内建参数（volume/pan）同链可达（resolve 回退逻辑已在）。ProcessorService 不评：曲线按参数挂接而非按处理器（§1.2），参数面是正确载体。

## 三、问3：VSP 暴露形态建议清单（**不越权定案，供主管裁定**）

| # | 建议项 | 内容 | 依据锚点 |
|---|---|---|---|
| 3.1 | 通道选型：**新命令族**（暂名 automation.*） | 复用 set_plugin_param 载曲线（加 points 数组）会让单点写/曲线覆盖语义混线、readback 口径冲突——不推荐；control_add_macro（CommandDispatcher.cpp:2850，VitParamLinkGraph 静态参数链接）语义不同不承载 | :2842-2860 handler 族先例 |
| 3.2 | 建议命令面（最小集） | `automation.get_curve`（读，track_id+plugin_id+param_id→points 数组+bypass+TimeBase）；`automation.set_points`（写，整段或区域替换）；`automation.clear_region`（区域清除，removePointsAndCloseGap 可选参数裁定）；可选 `automation.add_point/move_point`（单点面，待定） | API 全集 §1.3 |
| 3.3 | 只读入册 | automation.get_curve 应入 **legacyReadOnlyCommands**（VspKernelReference.cpp:146，get_plugin_parameters :161 先例） | :146-175 |
| 3.4 | renderAllowlist 裁定点 | 渲染中是否允许曲线读（冻结判定/观察）→建议入 CommandDispatcher.cpp:2244 renderAllowlist（get_plugin_parameters :2254 已在）；写命令明确不入 | :2243-2254 |
| 3.5 | CAS/幂等 | 写命令建议带 base_revision，粒度（edit 级/插件 state 级/曲线级）**待裁定**；vsphub 幂等封套按 set_plugin_param 现役模式；set_points 批量原子性参照 grabber apply_* atomic 面（catalog.go:507-508） | catalog.go:1027（RiskUndoable+confirm+readback 三旗） |
| 3.6 | 风险级建议 | get=RiskDirect；set_points/clear_region=RiskUndoable+确认（同 set_plugin_param :1027 形态） | catalog.go:1026-1027 |
| 3.7 | 坐标系裁定点 | 点坐标 time|beats（引擎双 TimeBase §1.2）——beats 与拍号网格/DISCLOSE 节奏键对位、time 与秒级区间操作对位；建议随裁定定主形态+另一形态换算 | h:17-21 |
| 3.8 | 显式不扩 | 宏参数/曲线道/CurveModifier 暴露不在最小集（MacroParameter/AutomationCurveList §1.2）——后续批次再议 | h:330/:18 |

## 四、问4：agent 侧消费面三处+接入点锚点

### 4.1 观察面（曲线回流两案）

- **摄入口现状**：harness.go:3964 IngestKernelTelemetry→:3977 `audio_feature_data_ready`+isL3AcousticSummaryFeature（:4111，三型白名单 :4113 `band_energy_summary/stereo_relation_summary/loudness_summary`）——automation 事件不在任何分支，进 agent 即丢（ARR-CAP-RECON S3 主缺口同款断点，基线漂移后亲核）。
- **两案**：①**DAD 证据层扩型**——新 feature_type（如 automation_curve_summary）入 :4113 白名单+落盘，文件域=harness.go 单文件（与 M1-4 已合入无在飞占主）；②**新 peer 投影**——仿 TOM disclosurePlan 模式新建包，需同步 docs/OBSERVATION_PROJECTION_MANIFEST.md §2（拓扑纪律：peer 同级不链）。裁定项。
- **FXM 通道排除**：fxm/projection.go:20-41 纯 A/B 测量对逻辑（baseline/processed same-source/same-window 校验）——曲线回流**不应**挂 FXM（FXM=同源同窗 bypass/processed 渲染对比，MIX-CAP-RECON §2 已记其缺生产者，语义不同）。

### 4.2 披露/意图面（今天的契约=显式排除）

- **bounded disclosure 四处 Excluded 显式列 "automation"**：capabilitycontext/context_manifest.go:215/:257/:301/:336——解锁时四处措辞须同步改（披露契约变更=显式行为）。
- **意图词表零命中**：goalrunner_chat.go 无 automation/自动化/包络/曲线触发词（全 agent 侧 grep 仅波形包络词族 message_loop.go:9626-9627 与黑板词表 project_blackboard_report.go:94，均非参数自动化）。
- **需求侧实证**：c1_frequency_cleanup_planner.go:31 清理规划 schema 已有 **defer_automation** 分类——模型已能产出「该用自动化」判定而无执行面，M3 落地后此分类获得动作出口。
- **域表冲突窗口**：意图/域表面 d1s1_domains.go 同级文件=M2-TIMEFX-1 文件域（在飞 coord/close-m2-timefx1）——M3 实施卡的意图面扩展应在 M2-TIMEFX-1 合入后排（或单点协调）。

### 4.3 goal 编舞（IMPL-1-STOP 保全件清点）

runs/FREESTATE-REGION-IMPL-1-STOP/：free_state_range_goal.go+free_state_range_goal_test.go+impl_tracked.diff+STOP-NOTE.md。**可复用面**：range_goal loop 状态字段（fail-open 旧 JSON 兼容+往返测试在位）、武装判定（treatment 类话术+ranges 在场最简形态）、测试骨架（10 组新用例全绿存档）；**按卡面语义届时重写**=两轮制改单轮终态（写包络=单变更），卡面与 DESIGN.md 保留为输入。冲突窗口：goalrunner 区域=ARR-CAP-RECON §三三高风险窗口之一，M3 编舞卡立卡时按其文件域冲突清单核对。

## 五、停止条件核查

tracktion automation **源在仓内**（submodule tracktion_engine@b4397498，model/automation 全套 .h/.cpp 在场可编译链接）——未触发停止条件。

## 六、遗留裁定清单（汇总，供主管立 M3 步骤2+卡）

1. 命令族命名与最小集（§3.2）；2. CAS 粒度（§3.5）；3. 坐标系主形态 time|beats（§3.7）；4. 曲线回流=证据层扩型 vs 新 peer（§4.1）；5. context_manifest 四处 Excluded 解锁时机与措辞（§4.2）；6. M3 实施卡与 M2-TIMEFX-1 的 d1s1/goalrunner 冲突窗口排程（§4.2/.4.3）。
