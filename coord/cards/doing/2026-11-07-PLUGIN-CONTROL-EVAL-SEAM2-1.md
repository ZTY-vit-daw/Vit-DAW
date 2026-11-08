# PLUGIN-CONTROL-EVAL-SEAM2-1：A-pure 第二组缝（S5 工具目录裁剪+S2b 渲染面剥离）

- 发卡：GLM 主管决策流 / 2026-11-07 深夜（依据=[307 两项装置发现](../done/2026-11-07-PLUGIN-CONTROL-EVAL-S1A-1.md)——①A 臂菜单仍含类型化适配器（plugin_grabber_inspect_*/apply_*_controls 族，10/18 轮被模型自选）②contextruntime 结果渲染面（context.go:515-533）把内核 binding_targets 语义行（alias/normalized_role/template_role）回灌模型上下文，S2 摄取剥离不覆盖）
- 派发确认：已确认（主管发卡·装置类，305 同模式）；验收负责人：GLM 主管决策流
- 池序 308；目标仓库=主仓 agent；P1 / 0.5 天 / 依赖=305 已合入；L2 / PC；零栈

## 目标（两缝，并入既有单 env `VIT_PLUGIN_CONTROL_EVAL_SEAM`，默认 unset 字节不变）

1. **S5 工具目录裁剪**：缝开时 plugin_grabber 类型化适配器族（inspect_*/apply_*_controls/explain_controls 等语义适配工具）从模型工具目录裁剪——**裸参数工具必须保留**（plugin.get_parameters/set_plugin_param/get_plugin_parameters/通用读写回读）；307 工件 write_path 清单为裁剪面蓝本；
2. **S2b 渲染面剥离**：contextruntime context.go:515-533 的内核结果渲染路径下，binding_targets 行的 alias/normalized_role/template_role 语义键剥离（原生参数行保留）——与既有 S2 摄取面互补成全覆盖；
3. 配对单测：S5 缝开=目录无类型化适配器+裸工具在场；S2b 缝开=渲染结果无语义键+受染两轮形态（307 工件 T3_r1 payload 为负锚）复钉；默认 unset 字节不变三态；
4. 回归：四包全绿+全包兜底；与 305 既有 16 测不冲突。

## 约束
- 文件域=tools/catalog.go 工具目录面+contextruntime/context.go 渲染面±测试；生产默认不变红线；不触 300/307 scripts。
- 禁止 git reset --hard / git clean。

## 停止条件
裁剪面与在案工具治理裁定冲突→取证上交。

## 领取/回执/验收
- 领取：2026-11-08 +0800 / origin/main=8c68c186f86f32c9e97c0af135a19c1bc31393 / owner=ZCode GLM-5.3 L2-worker PC 执行子代理（会话 308 seam2 runner）/ 分支=port/plugin-control-eval-seam2-1 / worktree=D:/Vit_DAW_wt_308seam / 领取基线：HEAD=8c68c186（fresh worktree 自 origin/main），git status --short=空、git diff --stat=空（全净）。在飞卡域核对：296-R2=scripts 冒测断言+真栈、309=C++ 引擎域只读取证、273/271 分支未合入不进本树——与本卡（agent Go tools/contextruntime±测试）不相交。
- 回执（执行侧 2026-11-08 +0800 收口，候主管终验）：
  - **实现 commit**：d04ff09d @ 分支 `port/plugin-control-eval-seam2-1`；diff=2 实现+2 测试，4 文件 +461/-2：`agent/internal/tools/catalog.go`（S5）、`agent/internal/contextruntime/context.go`（S2b）±各自 `plugin_control_eval_seam2_test.go`。不触 scripts/、不触 VitApp/、不触 307 工作树（只读取证）。
  - **两缝行为**（并入单 env `VIT_PLUGIN_CONTROL_EVAL_SEAM`，仅精确 "1" 开缝，与 305 共用总开关）：
    | 缝 | 锚点 | 开缝行为 | unset |
    |---|---|---|---|
    | S5 工具目录裁剪 | catalog.go `Tools()`/`ModelSummary()`/`ModelSummaryForTools()` 三出口 + `pluginControlEvalSeamTrimsCatalogSpec`（CommandName 前缀 `plugin_grabber`） | plugin_grabber 类型化适配器全族（20 spec：inspect_*/apply_*_controls/explain_controls/apply_eq_edits/set_eq_point 及其 dot/underbar/plugin_explain_controls 别名）从模型目录三面裁剪→AllowedTools 名单（toolNamesForAgentLoop 上游）与全部目录摘要文本不再披露；agentloop `allowedTool` 准入拒 hallucinate 调用；**裸参数工具保留**（plugin.get_parameters/plugin_get_parameters/plugin.set_parameter/plugin_set_parameter 菜单在场+get_plugin_parameters/set_plugin_param command 可达）；执行面 LookupCommand/LookupTool/Commands/DirectCommandNames 保留全部 spec（policy 与 free-state 守卫面不弱化） | 三出口逐字节现行为（plugin_grabber 工具与摘要行全在场） |
    | S2b 渲染面剥离 | context.go `summarizeToolResult`（原 :515-533 域）+ `stripBindingTargetsSemanticOverlay` 族 | 对 `result.Result` 做渲染净化拷贝（递归全树找 binding_targets，行内剥 alias/normalized_role/template_role，param_id/param_name/plugin_id/plugin_name/plugin_type/target_kind 原生键保留；嵌套 vsp 包裹同覆盖；输入 map 不变异）再喂 important_fields+result_preview（=message_loop.go:6413 `compactMessageLoopExecution`→`SummarizeToolResult` 泄露源，307 T3_r1/T6_r1 受染形态） | 原 map 直通，渲染逐字节现行为 |
  - **测试命令与退出码**（worktree D:/Vit_DAW_wt_308seam/agent）：`go build ./...`=**0**；新缝配对单测 `go test ./internal/tools -run TestSeamS5 -count=1`=**0**（2 测）+`go test ./internal/contextruntime -run TestSeamS2b -count=1`=**0**（3 测，含 T3_r1 受染形态负锚复钉：fixture 按 307 payload 实录键面构造，缝开断言渲染无 `"alias"`/`"normalized_role"`/`"template_role"` 任何出现+原生键在场+输入不可变）；三态=unset 现行为（字节对照 json 相等断言）/env=1 开缝/仅精确 "1"（""/"0"/"true"/"off" 与基线逐字节相等）；卡面四包回归 `go test ./internal/chat ./internal/agentloop ./internal/tools ./internal/harness -count=1`=**0**（chat 190.8s/agentloop 5.8s/tools 0.4s/harness 46.3s）+本卡改动包 contextruntime=**0**（0.8s）；305 既有 TestSeamS1-S4 全 16 测全 PASS 零冲突；全包兜底 `go test ./... -count=1`=**0（97 包 ok 零 FAIL）**。
  - **裁剪面与 307 蓝本对应**：write_path=typed_adapter 的 10/18 轮自选工具（inspect_reverb/apply_reverb_controls/apply_delay_controls/explain_controls 各命名形式）逐名断言缝开不在菜单（seamS5BlueprintTools）+缝关全在场（拼写 sanity）。
  - **治理等价性/技术精化记录（待主管追认）**：①S5 挂点=模型目录三出口裁剪而非 catalog 整体移除——执行面 spec 保留使 policy/free-state 守卫面零弱化，hallucinate 调用由 agentloop allowedTool 白名单拒（LookupCommand 映射后无 allowed 名可匹配）；②S5 裁剪判据=CommandName 前缀 `plugin_grabber` 全族 20 spec（含 apply_eq_edits/set_eq_point，按卡面「等语义适配工具」从宽覆盖——若主管要窄化到 307 实测自选名单可收）；③**S2b 残留声明**：`control_shell.template_role:"other"`（explain_controls 回复顶层 control_shell 内，非 binding_targets 行）按卡面字段清单（binding_targets 行三键）不剥——307 corpus template_role 208 计数混含此形态，A 臂复验若再报 template_role 其来源将是 control_shell（binding_targets 行已净），如需覆盖请主管裁定扩域；④嵌套 binding_targets（vsp 包裹等）一并净化（净化拷贝递归全树，超出负锚实测形态的防御性覆盖）；⑤EQ 守卫交互：缝开时 goalrunner EQ 拓扑写守卫（goalrunner_chat.go eqGenericWriteBlockedResponse）拒绝裸写 EQ 拓扑参数的指引文本仍指向 plugin_grabber.apply_eq_edits（目录已无该工具）——仅 test-only 态的装置事实（A 臂 T4 本为 fail 案，P/A 两臂同守卫可比），非治理冲突。
  - **端测边界声明（AGENTS §5，零栈）**：本卡全部验证为 Go 单测/包级回归+编译（§7 等级：单元契约级）；未跑真栈烟测、未启栈、零 LLM 调用零计费、无 run ID。缝开态装置有效性（A-pure 臂 menu 无 typed adapter+corpus 无 binding_targets 语义键）由 310 A-pure 装置门在真栈腿承接。渲染面/用户旅程不涉及（contextruntime 渲染为模型上下文面，非 webui 渲染面）。
- 验收：（主管回填）
