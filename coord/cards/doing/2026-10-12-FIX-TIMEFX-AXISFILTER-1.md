# FIX-TIMEFX-AXISFILTER-1：reverb/delay 轮内治理过滤空集缺陷修复（no_axis_covered）

- 发卡：GLM 主管决策侧 / 2026-10-12 深夜（依据=QUOTA-RECOVERY-LEGS-1 腿①按停止条件上交；[NIGHT-BATCH3 裁定①](../../rulings/2026-10-12-NIGHT-BATCH3-rulings.md)）
- 派发确认：已确认（主管依已验收卡的上交换证据发卡）
- 验收负责人：GLM 主管决策流
- 池序 106；目标仓库=D:\Vit_DAW
- 优先级 / 预估 / 依赖：P1 / 0.5-1 天 / 无（TIMEFX-1 已合入 main）
- 模型分级：L2 / GLM 执行会话（PC，取证+修复+真栈腿）

## 缺陷画像（上交证据浓缩）

D1 reverb 全链（run 20261012_211011，新模型 gpt-6.1-sol）：G1-G7 准入全绿+selected_domains=[reverb]+自由态触发（loop 至 fs6_target_confirmed）+冻结轴单主轴 decay_time（=9ae3aa91 修复目标形态）——但**轮内治理过滤输出空集**→stop=no_axis_covered_candidates→0 干预→"exactly one forward mutation" 门 fail。独立面三绿：pcactl query-v2 eligible=promoted+指纹匹配（leg1_d1_211011/pcactl_query_v2_proR_decay_time.json）+~/.vit 白名单 reverb 节+v2 认证 promoted 覆盖 decay_time+semantics（machine_state_snapshot.json）→**断点在运行时过滤路径，非机器数据**。旧模型 110817 曾命中同门（当时修复后无再跑机会）——本轮为该修复首轮真栈验证=不充分。

## 目标

1. **取证定位**：沿「intent 家族→PCA v2 coverage 轴表→attestation 读取器→候选过滤」链逐层打点，钉死空集产生层（锚点+最小复现）；与 9ae3aa91（TIMEFX-1 收口轮）的修复面 diff 对照。
2. **修复**：按取证定界（候选域=processorattestation/processorintent/chat seam 注册面，领取时与 M3 步骤 3+ 卡核对）。
3. **收口**：修复后 `run_free_state_d1_smoke.ps1 -PromptFlavor reverb -ExpectDomain reverb` 真栈 **exit 0**——即 M2-TIMEFX-1（111841）/M2-TIMEFX-2（195448）双挂账+本缺陷三账同收；delay 族同话术一轮可选加分。

## 文件域

以取证定界为准（候选=TIMEFX-1 注册面域）；真栈 PC-RUNTIME-STACK 排队登记。

## 验收标准

取证锚点+修复 diff+D1 reverb 真栈 exit 0（工件含 console+summary）+`go test ./... -count=1` 0 FAIL+gofmt 净；**工件随实现分支入库**（*.log 除外）——回执声明与 git 状态核对。

## 停止条件

取证指向机器数据面（与上交证据矛盾）→ 带锚点上交复核。

- 领取：2026-10-13 09:43 / origin/main=e7089c78（领取时本地 main=6413d23c 落后，以远端为准） / owner=GLM-5.3 执行会话（PC，ZCode 本会话，L2） / 分支=port/fix-timefx-axisfilter-1 / worktree=D:/Vit_DAW_wt_fix106 / 领取提交=本提交（仅含 coord/ 卡状态变更）
- 回执：**实现**=cf5bf33f@port/fix-timefx-axisfilter-1（工件=37641a2a 同分支）；**取证层定位**=TIMEFX v2 族（reverb/delay）在 chat 运行时共 **7 处注册面漏接**（TIMEFX-1/9ae3aa91 建好底层——registry 两族定义、CoverageProofs、v2 认证、plugingrabber builder、attach refs、apply 工具函数——但 chat 侧消费面 switch 全部未同步）：①`plugin_recommendation_axis_coverage.go axisCoverageWhitelistIdentifiers` switch 缺两族 case=**原卡缺陷 no_axis_covered_candidates 空集根因**（白名单 identifiers 恒空→候选全 skip；与上交证据三绿一致——断点确在运行时过滤）②`semantic_treatment_strategy.go semanticTreatmentBuildSurfaces` 未接 BuildReverb/DelaySummaryWithBoundary→post-load "family reverb was not proven"③`semantic_dynamic_workflow.go readSemanticDynamicSurface` 双 switch 缺两族→unsupported_dynamic_family④同文件 `semanticDynamicBindings` 缺两族 sections⑤同文件 `semanticDynamicRoleAliases` 缺两族轴词⑥同文件 `applySemanticDynamicController` 缺两族分派→typed_controller_failed（干预 receipt 缺失根因）⑦`plugin_reverb_apply.go reverbUnitCompatible` % 分支漏 decay/size=9ae3aa91 decay_pct 量纲同步漏项→unit_role_mismatch。修复①-⑦各附回归测试（TestAxisCoverageGovernedCandidatesKeepTimeDomainWhitelistForm / TestSemanticTreatmentBuildSurfacesExposesTimeDomainFamilies / TestReverbUnitCompatibleAcceptsPercentDecayAndSize，均 stash 还原验证 RED→GREEN）；`go test ./... -count=1` EXIT=0×2（首处修复后+全七处修复后）、0 FAIL；gofmt 净（LF 规范化口径——worktree autocrlf 环境差异已排除）。**真栈**=11 run：095626 fixture 复制腿 / 100927→103532 三轮逐层暴露②③⑥⑦并修复 / **105238 干预落账轮**（G1-G8 全绿→治理过滤→Pro-R 装载→post-load 复核→ticket(decay_time 轴 percent 目标)→typed 执行→settlement receipt→**interventions=1+parameter_applied=True+readback_verified**）/ 110320-112913 NOT_EXERCISED×3+180315 服务过载 / **181843 干预二次复现**（applied=True）但 post-action CCB 观察被连续 60s LLM 超时阻断。**exit 0 未达成**：settle 尾（post-action 观察→materiality→audition）全部 6 个环境阻断轮均由上游 LLM 服务不稳/过载所致（60s timeout err=true 日志多帧+服务端原文 "Our servers are currently overloaded"，证据=coord/runs/.../console_agent_last_181843.txt 与各 console），按 §8 止损——代码无确定性断点（干预链两轮复现），继续重跑在服务恢复前无意义。**[等待用户：LLM 服务（gpt-6.1-sol）稳定后复跑 `run_free_state_d1_smoke.ps1 -RepoRoot <worktree> -PromptFlavor reverb -ExpectDomain reverb -SkipBuild -KernelExe 主检出` 一轮收 exit 0，需重新登记栈占用；delay 族同话术可选加分轮同待]**。**端测边界声明**：真栈实测覆盖=内核装载/参数读回/typed 写/settlement/干预账本全链（HTTP/ZMQ 面）；未覆盖=webui 渲染面（本卡零涉及）与 settle 尾之后的人听确认面。占用登记 bb3f9c42、释放 4459ccfe（09:50–18:31，拆栈核净）。**域外新缺陷上交（不扩域）**：`c2_dynamic_control_runtime.go c2VerificationViews`（C2 能力会话面，不在 D1 冒测链）switch 同样缺 reverb/delay——C2 若接 TIMEFX 族需同款补 case，锚点=该函数 default 返回 nil。基线既有非本卡引入：`semantic_treatment_strategy.go` 两处 unreachable code（go vet，stash 对照确认行号平移）。
- 验收：（裁定文件 / 验收 commit）
