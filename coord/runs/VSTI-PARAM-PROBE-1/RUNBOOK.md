# VSTI-PARAM-PROBE-1 RUNBOOK（§8 跑前口径，零栈准备先行 + 一次真栈）

- 卡：298 VSTI-PARAM-PROBE-1（coord/cards/doing/2026-11-07-VSTI-PARAM-PROBE-1.md）
- owner：GLM-5.3 L2-worker 执行子代理（ZCode PC）；分支 `port/vsti-param-probe-1`；worktree `D:/Vit_DAW_wt_298probe`
- 领取基线：origin/main=`3f222128`（worktree 检出全净）；领取提交 `958d4e85`

## 被测件与栈计划

- agent = wt_298probe 现场构建（dev_agent_smoke.ps1 -RestartAgent 自建 VitAgent.exe；SHA256 随 run 记录）；**Go 源码零改动**（探测腿纯脚本域）
- kernel = wt_228 `D:\Vit_DAW_wt_228\VitApp\build_release\VitApp_artefacts\Release\VitApp.exe`，SHA256 `F9E3DA0A66263F0B6E3FED5CDA0FB214C460FCE8053234240C470D515ED0D67A`（跑前亲核）只读复用（225–281 同款，C++ 面零改动）；经 `-KernelExe` 显式指定
- 不启 UI（无 -StartUI）；项目隔离 = 共享 GUI-leg 块（VIT_PROJECT_XML → run 目录 git-clean 副本；rack_add_node 走内核 saveProject，217 事故教训族）
- 探针对象：Grand Rhapsody Piano Stereo（首选，Waves shell，identifier 解析）+ Surge XT（对照）；装载链 = 内核 ZMQ rack_add_node（M2-TIMEFX leg-A 先例），参数面 = get_plugin_parameters，写读恢复 = set_plugin_param normalized_value（内核面）+ plugin.set_parameter（agent HTTP 面，full_project_access 权威 + finally 还原）

## 命令（单 run）

```powershell
powershell -NoProfile -ExecutionPolicy Bypass -File D:\Vit_DAW_wt_298probe\scripts\dev_agent_smoke.ps1 `
  -RepoRoot D:\Vit_DAW_wt_298probe -StartKernel -RestartAgent -NoChatSmoke -VstiParamProbeSmoke `
  -KernelExe D:\Vit_DAW_wt_228\VitApp\build_release\VitApp_artefacts\Release\VitApp.exe `
  -RunArtifactsDir coord\runs\VSTI-PARAM-PROBE-1\probe1_<启动时间戳>\
```

（py 驱动：`scripts/vsti_param_probe_smoke.py`，exit 0/2/4；PS1 完成态不自拆 → owner 亲拆+TEARDOWN_ALL_ZERO 亲核。）

## §8 跑前口径（AGENTS §8 成文）

1. **运行次数**：单 run，无重跑。任何 exit≠0 → 取证上交，不原样重跑；确定性断点连续两次失败禁重跑的红线在本卡天然满足（本卡只允许一次真栈 run）。
2. **预算**：探测腿**零 LLM**（无 chat 调用；断言面=agent HTTP/ZMQ 接口面）。例外：dev_agent_smoke.ps1 默认 body 的 strip-silence 子腿 1 次 /agent/chat（281 先例：-NoChatSmoke 不关该腿），deepseek-flash 计费 ≪¥0.01；telemetry 落本 run 目录如实计数。总预算上界 ≈¥0.01。
3. **成功条件（exit 0 全门）**：
   - A 预检：plugin_list_available ok 且两件 probe 对象 identifier 在库且 is_instrument；pcactl fingerprint（同源 processorattestation.FingerprintPath，只读子命令，不触 attestation 库）两件 sha256 落工件；
   - B 装载（per plugin）：add_track ok → rack_add_node(identifier) ok + plugin_id + instance_ready≠false + load_state enabled=true → get_plugin_parameters ≤90s 出非空参数面；
   - C 清单：per-plugin 全量参数行（范围/装载时值/value_text/枚举标签 display_probe）+ 差异观察扫描（preset/program/midi/channel 等词面命中计数）落工件；
   - D 内核往返：每件 2–3 个连续可写参数 apply(normalized)→回执 readback→独立面 readback→restore→readback；全参数快照恢复核对（认证线同款容差 1e-4 normalized）；
   - E agent HTTP 控制面：/agent/authority full_project_access（GET 基线→POST→GET 复核）→ plugin.get_parameters（守卫缓存预置）→ plugin.set_parameter 写→内核独立 readback→restore→快照核对；单位区间参数加码 normalized 契约断言（catalog 声明 value=0.0–1.0）；
   - F 清理：delete_track ok（重试一次）；权威模式还原到基线。
4. **失败分类**：
   - exit 2 = 环境中断（栈/传输/两件对象不在库/fingerprint 工具链失败）——按环境记录，可另跑前先交主管；
   - exit 4 = 确定性面缺陷/卡面停止条件：**get_plugin_parameters 对 VSTi 空/报错 = 卡面停止条件**（「尚不可控制」如实证据，取证上交）；往返/恢复/快照核对失败；agent 控制面写失败或违反声明的 normalized 契约；清理失败。取证转卡，不扩修（源码零改动红线）。
   - PS1 层 throw（栈启动失败等）= 环境中断类，与 py exit 2 分列记录。
5. **止损线**：单 run 后无论结果即停（本卡预算一次真栈）。止损只停止投入，不把未发生的成功改写为成功。
6. **栈纪律**：登记前 fetch 重读 PC-RUNTIME-STACK.md（候栈第 2 位：296 已领取未登记；若其已登记→等释放再核再登）→ 占用登记推 main → ≤60s 即时复核（六口 TCP 5555/5556/7878/8787+UDP 4444/4445 listen=0 + 五类进程=0）→ 启栈 → run → 亲拆（VitAgent/VitApp 残留进程亲杀）→ TEARDOWN_ALL_ZERO 亲核 → 释放登记随工件批推 main。
7. **工件**：coord/runs/VSTI-PARAM-PROBE-1/probe1_<stamp>/（RUNBOOK 本件、vsti_param_probe_summary.json、<plugin>_parameters.json×2、vsti_param_probe_console.txt、logs/ telemetry、ps1 console 由 -RunArtifactsDir 收口）；**不写 attestation 库**（准入后置裁定）。

## 零栈准备核对（已完成）

- py_compile exit 0；ps1 Parser 0 错（BOM 保持）；离线自测 22/22（纯函数：目标挑选/快照比对/枚举扫描/装载态解析/退出码契约）；
- dev_agent_smoke.ps1 四透传缝（参数声明/共享 GUI-leg 隔离条件/env 块/调用块），既有腿字节不变；
- 红线核对：源码零改动（diff=scripts 两文件）；attestation 库零写入（pcactl 只读 fingerprint 子命令）；kernel 零改动（wt_228 只读复用）；临时轨/插件实例清理自含（finally delete_track×2 重试）。
