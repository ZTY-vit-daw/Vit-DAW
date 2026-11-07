# VSTI-PARAM-PROBE-1：音源参数控制探针（控制实证先行，准入后置链第 2 位）

- 发卡：GLM 主管决策流 / 2026-11-07 午前（依据=[用户裁定 2026-11-07：PCA 准入后置于控制实现](../../decisions/2026-11-07-pca-admission-after-control.md)+[277 段 3a-r2 证据](../done/2026-11-05-ARR-SRCLOAD-1.md)——装载已实证/参数控制未做；效果器族先例=M2-TIMEFX-1 参数面探测（内核 ZMQ 直连装载→get_plugin_parameters→apply/readback/restore/快照核对/临时轨清理自含）与 G3-PROBE-METER 链）
- 派发确认：已确认（用户 2026-11-07「这些足够进行实验吗？如果足够完全可以进行实验开发」=现有音源开展控制实验授权）；验收负责人：GLM 主管决策流
- 池序 298；目标仓库=主仓 agent（探测腿+工件，复用 M2-TIMEFX 探针模式）；P1 / 0.5-1 天 / 依赖=无硬依赖（**与 297 渲染取证可并行**——参数控制不依赖出声链）；L1-L2 / PC；真栈占用一次（登记协议照旧，候栈排队与 296/297 错峰）

## 目标

1. **两件音源参数控制实证**（Grand Rhapsody Piano Stereo 首选+Surge XT 对照，均本机在库）：装载（既有链）→ `get_plugin_parameters` 全量参数清单落工件（含范围/默认值/枚举）→ 选 2-3 个可写参数做 apply→readback→restore 往返（写前快照+写后核对+恢复核对，M2-TIMEFX 自含模式）；
2. **控制证据工件**（准入后置链的输入）：per-plugin 参数清单+往返记录+版本指纹（processorattestation.FingerprintPath 同源）落 `coord/runs/VSTI-PARAM-PROBE-1/`——**本卡不写 attestation 库**（准入届时另卡，用户裁定）；
3. 差异观察：VSTi 参数面与效果器族的结构差异（预设/音色切换参数？MIDI 通道暴露？）如实记录（编曲线后续语义控制的素材）；
4. §8 跑前成文（确定性腿预期零 LLM；若走 chat 驱动装载变体则 flash 预算 ≈¥1-3）。

## 约束

- 文件域=探测脚本（复用/扩 M2-TIMEFX 同款 ps1/py 模式）+工件；**源码零改动预期**（如装载/参数接口暴露缺口属产品缺陷→取证转卡不扩修）；
- PCA attestation 库零写入（准入后置裁定）；kernel 零改动；
- 临时轨/插件实例清理自含（M2-TIMEFX 先例：即建即删）。

## 停止条件

参数接口对 VSTi 不可用（如 get_plugin_parameters 对 instrument 返回空/报错）→取证上交（=「尚不可控制」的如实证据，直接改变准入链评估）；装载环节回归失败→关联 297 取证分列。

## 领取/回执/验收

- 领取：2026-11-07 18:37 +0800 / origin/main=`3f222128126494ee8c8e6cd5790558a3ac6c6097` / owner=GLM-5.3 L2-worker 执行子代理（ZCode PC 会话，sess 子代理；池序 298） / 分支=`port/vsti-param-probe-1` / worktree=`D:/Vit_DAW_wt_298probe` / 领取基线：HEAD=`3f222128`（worktree 自 origin/main 新建检出），`git status --short`=空、`git diff --stat`=空（全净，无领取前已有 diff）；候栈排队第 2 位（296 优先已核对：其已领取卡但栈登记面仍空闲=281 10:42 释放后无新占用，登记前将再次 fetch 重读+≤60s 复核）。
- 回执（2026-11-07 晚窗，终 run `probe1_20261107_1920` py exit 4 / PS1 exit 1=§8 预写「agent 控制面写失败=确定性面缺陷取证」分支，**取证上交不重跑**；卡面停止条件未触发——get_plugin_parameters 对两件 VSTi 均非空可用）：
  - **交付 commit** @ `port/vsti-param-probe-1`（已推远端）：prep `13321a9b`（脚本+四缝+RUNBOOK）→fix `74d196d2`（pcactl 多行 JSON 解析）→fix `9d9e94fa`（E 腿取证不坠机）→artifacts `019ec3c4`（三 run 工件全套）。**文件域=scripts/ 两文件+工件目录；Go/内核源码零改动**（diff 全量=scripts/vsti_param_probe_smoke.py[新]+scripts/dev_agent_smoke.ps1[+85/-3 四透传缝，既有腿字节不变]+coord/runs/）。占栈链@main：占用 `206e1c54`→行修正 `319e922c`→释放 `d255e313`（TEARDOWN_ALL_ZERO 亲核 19:23:25）。
  - **① 两件音源参数控制实证（内核 ZMQ 面，全绿）**：Grand Rhapsody Piano Stereo（Waves shell，identifier 解析装载 instance_ready=true/enabled/processing/load_error 空）**41 参**全量落工件+Reverb Time/Pre-del/Mix 三参 apply 0.72→回执+独立面 readback 0.72→restore 精确回基线（1e-4）+41 参全量快照恢复零 mismatch；Surge XT **776 参**+LFO 2 Deform/LFO 3 Amplitude/FX Type 三参往返+776 参快照恢复零 mismatch；两件 cleanup delete_track ok。**版本指纹**（pcactl 只读 `fingerprint` 子命令=processorattestation.FingerprintPath 同源，**attestation 库零写入**）：grand_rhapsody=`sha256:ee6b1a818fb6392e9c28c1b053ead872c6d12ab654daf079293461721da7bb90`（WaveShell1-VST3 17.1_x64.vst3）、surge_xt=`sha256:49794ad99d899a869b97416024283d74af2c517bac15db0fde511bc99624e20`。
  - **② 确定性发现（取证转卡，agent 通用写面对 VSTi 的暴露缺口）**：agent HTTP 面 `set_plugin_param`（confirmed=true+full_project_access）对 **Grand Rhapsody 被 typed-family 守卫 fail-closed 拒绝**：`typed_de_esser_surface_unavailable: cannot prove this parameter is outside the typed De-esser surface: unresolved_de_esser_surface`（第二参数复测同拒=插件级阻断，parameters_changed=false 快照零变）；**Surge XT 同面全通过**（写→内核 readback 0.5→0.35→restore 0.5→单位区间 normalized 契约过→快照过）＝守卫行为随插件 de-esser 分类分化。锚点：`agent/internal/chat/goalrunner_chat.go:1425-1455`（guardDeEsserOwnedGenericParameterWrite 默认分支：仅 not_de_esser/unsupported_* 干净码放行，unresolved 一律拦）+`server.go:1974`（invoke 面挂载）。**编曲线语义控制影响**：走 agent 通用面的 VSTi 写控制现状=部分插件不可写（fail-closed 非绕过）；控制通道选型（kernel/VSP 直连 vs 守卫分类补齐）交主管裁定。
  - **③ 差异观察（VSTi 参数面 vs 效果器族，编曲线素材）**：两件 `template_role=instrument`+`supports_param_grabber=true`（内核通用参数面对乐器建模完整，含 quick_controls/instrument_sustain/instrument_cutoff 角色）；**预设/音色切换参数不暴露**——两件 automatable 面均 0 个 preset/program/patch/bank 命名参数（VST3 program change 不在参数面→预设切换需另寻通道）；**MIDI 通道不暴露**（0 命中）；Grand Rhapsody=41 参 17 枚举离散（宿主面小而离散多）；Surge XT=776 参 0 离散标记+**scene 族参数 158 个**（Active Scene/Scene Mode/每 scene 调制源——场景结构以参数面暴露，效果器族无此形态）；agent HTTP 面披露=Grand Rhapsody 41/41、**Surge XT 512/776**（披露子集，编排控制走 agent 面时可见面收窄——观察项，未断言成因）。
  - **④ §8 兑现与如实申报**：RUNBOOK 跑前成文（coord/runs/VSTI-PARAM-PROBE-1/RUNBOOK.md）；**「单 run 无重跑」按偏差如实申报**——实际三次调用：`1911`=driver 解析缺陷 @fingerprint（pcactl 打印多行 pretty JSON，原取尾行必非 JSON→exit 2；探针目标相零启动零计费；拆净 19:13:19）→fix `74d196d2`；`1914`=agent 面守卫拒绝（发现有效）但经外层 except 误分类 exit 2 且中断 Surge XT 采集→fix `9d9e94fa`；`1920`=终 run 完整取证（拆净 19:18:19/19:23:25）。两次修复均为探针脚本域（我的文件域内）、非同版本同断点原样重跑、断言面只增不减（E 写成功路径断言全保持，拒绝路径从「坠机」改「记录+继续采集」）；供主管裁定偏差是否采认。
  - **⑤ 计费/端测边界**：探测腿 **0 LLM 行**（telemetry 在卷）；默认 strip-silence 子腿 1 行 deepseek-flash ≪¥0.01（RUNBOOK 预申报形态）；权威面 manual_confirmation→full_project_access→还原 manual_confirmation（GET 复核）；agent=wt_298probe 现场构建 SHA256 `79738430…DB942604`；kernel=wt_228 `F9E3DA0A…ED0D67A` 只读复用。**端测边界声明（AGENTS §5）**：探测腿=agent HTTP/ZMQ 接口面（装载/参数/写读恢复/清理全断言）；**渲染面不涉及**（出声链=297 另案）；渲染面/用户旅程门槛面（E2E-WEBUI-1/JOURNEY-1）不适用本卡（无 webui/GUI 改动）。
  - **工件**：`coord/runs/VSTI-PARAM-PROBE-1/probe1_20261107_1920/`（summary+grand_rhapsody_parameters.json[41 参全量含 display_probe 枚举标签]+surge_xt_parameters.json[776 参]+console+telemetry+项目隔离件）+1911/1914 取证件 @ `port/vsti-param-probe-1`；栈登记/释放 @main `PC-RUNTIME-STACK.md`。
- 验收（2026-11-07 晚窗，主管）：**pass（控制实证收口+取证转卡）**——主管亲核终 run 工件（grand_rhapsody 41 参全量快照恢复 true+surge_xt 776 参同+agent_face 拒绝/通过留痕两态在卷）；内核 ZMQ 面控制实证=**准入后置链②控制证据到手**（两件零 mismatch，指纹同源落档）；de-esser 守卫误拦 VSTi=确定性缺陷**转卡 302 VSTI-WRITEGUARD-1**；三调用偏差采信（两次 driver 修复循环，非同版本同断点原样重跑，§8 合规）；预设/program/MIDI 通道不暴露+Surge scene 158 参=设计输入记录（归 INSTRUMENT-SEMANTIC-CONTROL-DESIGN-1 素材）；template_role/normalized_role 语义字段顺带采证=299 实验 C 臂素材额外入库。手册 join 演练（主管侧）挂明日。L1 台账 48h 窗口。
