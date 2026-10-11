# MIXLAYER-M1-3：C3 空间与深度·观察侧收口（只读能力 Session）

- 发卡：GLM 主管决策侧 / 2026-10-11 早窗（同 M1-1 依据；MIX-CAP-RECON-1 §3 C3 行+M1 表）
- 派发确认：已确认（用户 2026-10-11 /morning 选定 PC 混音线首批立卡）
- 验收负责人：GLM 主管决策流
- 池序 66；目标仓库=D:\Vit_DAW
- 优先级 / 预估 / 依赖：P1 / 0.5-1 天 / 无硬依赖（可与 M1-1 并行）
- 模型分级：L1 / GLM 或 flash 执行会话（PC）
- **与 M1-4 文件域高度重叠（canary seam/routing）——建议同 owner 串行或先后合入，不并行**

## 目标

`fine_mix.space_depth.v0` 以**只读能力 Session** 形态接入：canary seam 增 C3 分发（`capability_runtime_canary.go:109-126` 现六分支无 C3）；capabilityIDForStage 映射已有（`capability_routing.go:619`）。观察载体现有面：

- FXM 差分手动通道（fxm/types.go 结构齐备、`mixboard.go:3992` req.Args 反序列化消费点——**本卡不建自动差分生产者**）；
- 通用插件参数快照（plugin_param_guard 依赖面）；
- MOM static_level_relationship。

产出=空间/深度相关观察报告 Session（无写控制）。

## 边界（显式排除）

- 语义 reverb/delay 控制面不在本卡（内核 plugin_grabber 无 reverb/delay inspector，随 C3 控制侧另卡）；
- FXM 自动差分生产者不在本卡（时域线 M2 范畴）。

## 文件域

chat/ capability seam（capability_runtime_canary.go+capability_routing.go）+ 新 C3 只读 runtime 文件 + 单测。

## 并行与资源

见卡头：与 M1-4 串行；PC 真栈串行占用（PC-RUNTIME-STACK.md）。

## 验收标准

- 单测：Session 分发+只读性（无写命令注册）。
- 「C3 观察在自由态循环内可达」冒测断言（模式=fastpath 注册清单完整性契约，MIX-CAP §3 建议）。
- 真栈烟测 exit 0；`go test ./... -count=1` 0 FAIL + gofmt 净。

## 停止条件

canary seam 结构与锚点不符（六分支已变）→ 锚点上交。

- 领取：2026-10-11 20:28 PC / origin/main=09e5df456a3b5c06b6e7e7e3aceb42180df86584 / owner=GLM-5.3 执行流（ZCode PC 会话，2026-10-11 晚窗）/ 分支=port/mixlayer-m1-3 / worktree=D:/Vit_DAW_wt_mixlayer_m1_3 / 领取提交=4f185c27
- 回执：实现 commit **573fb046**@port/mixlayer-m1-3（已推远端）；栈占用 e7645a65 → 释放 40562294。
  - **锚点亲验**：领取时实测 canary seam 六分支仍在 `capability_runtime_canary.go:109-126`（109 panLayout/112 lowEndRelation/115 frequencyCleanup/118 dynamicControl/121 agentSemanticEQ/124 staticBalance fallthrough），`capabilityIDForStage` C3→`fine_mix.space_depth.v0` 映射在（capability_routing.go:619 区域实测 608-624），停止条件未触发。
  - **实现落点（4 文件+1 新文件）**：①`c3_space_depth_runtime.go`（新，341 行）= spaceDepthCapabilityID const（**v0，与 stage 映射同 ID 单一真相源**——C1/C2 的 v0 映射/v1 runtime 历史漂移不在 C3 复现，注册链契约测试锁定）+ handleSpaceDepthRuntime（VSP snapshot→sessionID→载体三面→报告）+ c3InvokeReadOnly（白名单外工具 fail-closed）+ c3SpacePluginsFromState（tracks[].rack.nodes[] 提取+词法分类，形状与 tim.RackSummariesFromProjectState 同源）+ c3ReadPluginParams（plugin_grabber_explain_controls→get_plugin_parameters 读端，上限 8+超出计入覆盖说明）+ 纯函数组装/渲染（c3AssembleSpaceDepthReport/c3RenderSpaceDepthReply）；②`capability_runtime_canary.go` +4 行=第七分发分支（agentSemanticEQ 后、staticBalance fallthrough 前）；③`capability_owner.go` +13 行=四点注册（显式白名单/activeCapabilitySessions/session base `cap_v1_c3_`/话术显式标记组 `fine_mix.space_depth`+`c3`——同 C2 反门语义，注释明言不做普通空间话术词法后继，"让空间更大/这个混响太大了"不劫持，单测覆盖）；④`capability_routing.go` **零触碰**（映射已有）；⑤`fs_largeproject_smoke.ps1` +100 行=`-C3Observation` 参数门控（默认关原行为零变化；phase 8.6：B64 中文消息 POST chat→三 needle 断言（报告 schema 行 c3.space_depth.observation.v0+只读声明行+A4 标记族零命中）；exit 门组合化=M1Q/C3OBS 各自叠加 A1+A4，`-M1QueueRoundtrip` 单开时语义逐字不变）。
  - **载体三面消费形态**：MOM static_level_relationship=mix.observe（`project_multitrack_relation_observation` 意图，B2 同款读面）→`mom_projection.static_level_relationship` 摘要（status/coverage/limitation，缺失如实 missing 不升级）；通用插件参数快照=工程态 rack 词法识别→逐插件 get_plugin_parameters 只读（读后自动进 plugin_param_guard 的 PluginSnapshotCache，供任何后续写前验证）→空间参数词级摘要（wet/size/decay 等 18 词标记，词级匹配防 threshold⊃hold 子串误命中）+其余计数；FXM 手动差分=`req.Context["fxm_measurement"]` 原样透传 mixboard finalizeFXMProjection（mixboard.go:3992 消费点），无手动输入如实 missing+limitation「不生产自动差分（M2-TIMEFX 线）」——**未建任何自动差分生产者**。
  - **只读性（卡面验收①）**：harness 调用白名单 `c3ReadOnlyTools` 恰 2 工具（mix.observe/plugin_grabber_explain_controls），全部调用经 c3InvokeReadOnly；无 Proposal 路径、无 StartSession/AttachFrozenPlan、无写命令注册。单测双断言：27 项写命令黑名单（rack_add_node/set_plugin_param/plugin_grabber_apply_* 族/mix_apply_* 族等）零交集+白名单成员 mutation 词根扫描+白名单基数锁 2+白名单外调用拒绝实测。
  - **单测 9 测**（`c3_space_depth_runtime_test.go`）：注册链完整性契约（**模式=fastpath.DefaultEntryNames 完整性契约，MIX-CAP §3 建议的落地形态**——五点同 ID：capabilityIDForStage(C3)/话术推断双标记形态/session base 前缀/显式 capability_id 解析 owner=EngineV1，任一漂移显式失败）/普通空间话术不劫持+C1/C2 标记不误入/只读白名单黑名单交叉/白名单外拒绝/canary seam 分发实测（nil kernel 下 observation_blocked+capability_id+read_only 证明落点）/mix.observe 参数意图+FXM 透传/rack 词法识别（含无 rack 轨、无 track_id 轨、非空间插件边界）/空间参数词级摘要/三面缺失归一/needle 契约（回复必含 schema+只读声明，锚定烟测 needle，文案变化须同步脚本）。
  - **「C3 观察在自由态循环内可达」冒测断言（卡面验收②）**：真栈 run 204949 phase 8.6——自由态 chat 消息「c3 观察空间与深度」（显式 c3 标记，无 capability_id 上下文提示，走话术推断路径）→ canary seam 分发 → C3 观察报告返回：reply 含 schema 行+只读声明（needle 断言过），workflow_data 实况 capability_id=fine_mix.space_depth.v0 / canary_stage=observation / read_only=true / mutation_performed=false，响应面 A4 六标记族零命中。
  - **真栈烟测（卡面验收③）**：`fs_largeproject_smoke.ps1 -C3Observation` run **fs_largeproject_smoke_20261011_204949 = PASS exit 0**（SMOKE_EXIT=0 显式捕获，无管道吞码）：C3OBS 三门全过+A1 pass+A4 pass；A2/A3 软记录（classification=capability_blocked，与 FS-LARGEPROJECT-SMOKE-1 基线同族归其返工线——**门槛语义沿用 M1-2 ruling 已裁定的同构模式**，exit 门=C3OBS+A1+A4）；栈自 teardown torn_down=true，21:16 复核端口/进程=0。`go test ./... -count=1` **GOTEST_EXIT=0**（显式退出码，92 包 ok 零 FAIL）；gofmt 净（新文件 LF 对齐净；CRLF 触碰文件 LF 复核净——M1-2 先例口径，真实对齐无问题）。
  - **端测边界声明（供验收裁定）**：①真栈 C3 面实证=**分发链（话术→canary seam→runtime）+只读纪律+报告渲染+needle+A4 零命中**；载体**数据面**该 run 如实缺席——61 轨 dry stems 工程无插件 rack（plugin_candidates=0，导入 stems 裸轨道合理）且该观察轮 MOM static_level 缺席（missing 如实保留，投影纪律：缺失不虚构不升级）——载体的 present 数据路径由单测覆盖（PresentSurfaces/SpacePluginsFromState/SpaceParamDigest），真栈正面数据证据待有插件装载/有 static_level 证据的工程（如需可另开补充轮）；②FXM 手动差分透传=单测覆盖语义，真栈无手动输入走 missing 分支（预期行为）；③C3 观察报告零 LLM 参与（纯确定性组装，无概率面）；④文件域如实声明：卡面「chat/ capability seam」实际含 `capability_owner.go`（seam 解析入口四点必改，无它 C3 分发不可达）；`scripts/fs_largeproject_smoke.ps1` 参数门控=验收标准②自身要求的冒测断言载体（M1-2 同模式先例）；`capability_routing.go` 零触碰。
  - **执行侧事故如实记录**：①首次烟测 bash 路径 `.\scripts\...` 反斜杠被吞 exit 127 未起栈（命令行事故，未构成有效轮；正斜杠绝对路径重跑）；②占用登记时间笔误 21:05（实际 20:49 run 启动起占用）——已在释放记录 40562294 修正说明；③前两轮全量 go test 输出经管道（exit 码属 grep），第三轮重跑显式 `GOTEST_EXIT=0` 补证（M1-2 同类事故的防线沿用）。
- 验收：（裁定文件 / 验收 commit）
