# evidence.md — WAVESHELL-OFFRENDER-FIX-1（309）栈腿（M1+插桩+判别+修法⑤落地+验证矩阵）

- 执行：ZCode GLM-5.3 L2-worker PC 子代理（309 栈腿）@ worktree `D:/Vit_DAW_wt_309recon` 分支 `port/waveshell-offrender-fix-1`；栈腿领取基线 0644ae89（全净），实现+回执见卡面。
- 栈窗口：2026-11-09 19:06 占用（main 5915bb9c）→ 19:29 释放（main 4711dfd3）；TEARDOWN_ALL_ZERO 亲核 19:28:47（终拆 killed VitApp=23644/VitAgent=24724；窗口内 K1a[26856/29704]→K1b[19624/28244]→K2→K3→K4 owner 亲停亲启，中段 PID 未逐个捕获如实记录）。
- 被测内核（最终态）：`VitApp/build_release/.../VitApp.exe` SHA256 `9241f3453f9916e76ad3800ca3dfa72069e4ccf9090bcc7de362624b10a10f4e`（①92b09da5=60ab1a49+VIT309 遥测+C 原型+形态3+RTLEVEL ②074a273b=tap 修复 ③9241f345=⑤ warn_only）；engine 子模块 `vit309-stackleg` = 60ab1a49+cd957f4+9b8cc8a（未推 engine 远端，处置交主管=301 同款）。
- 全程零 LLM：8 run telemetry jsonl 文件全 absent（亲证）。

## 0. run 清单（每 run 目录含 summary/console/vit_trace/ps1_console/fixture/kernel_log；wav 按 301 口径 gitignore 留盘本机）

| run | 内核/env | 结果 | 分类 |
|---|---|---|---|
| e1_realtime | K1a 92b09da5 / VIT309_RTLEVEL | verdict=silent 但 22 行全 -100.00 | **无效轮（装置缺陷）**：engine 快照 `setLevelCache` 零调用点=levelCache 死代码（`tracktion_LevelMeasurer.cpp` 亲证），测量面假阴性 |
| e1_realtime_v2 | K1b 074a273b（tap 改 LevelMeasurer::Client 直挂 8bc39aa6）/ 同 env | verdict=**silent**（max_db=-100.0，21 行，tap clients=1） | **有效判定：M1=RT-SILENT** |
| e1ctrl_realtime_surge | K1b 同内核同装置换 Surge XT | verdict=**sound**（max_db=-10.97） | 对照证：实时路径/MIDI 馈入/Client 测量面全健康；唯一变量=插件 |
| e2a_baseline_gr | K2 9241f345 / VIT309_DISABLE_BLACKLIST | SILENT（peak_abs=0）+形态 3 全链 | **S1 信令链运行时确证**+形态 3 验证 |
| e2b_c_consistency_gr | K3 / +VIT309_C_CONSISTENCY | SILENT+C 原型生效 | 判别完成（见 §2） |
| e3_fixb_warnonly_gr | K4 默认 env（首跑装置缺陷 exit1 不计） | — | `--expect-env` 空集传参丢失（PS 侧），修复 9da5 提交 |
| e3_fixb_warnonly_gr_v2 | K4 默认 env | SILENT+warn_only+形态 3 披露 | ⑤形态验证（直挂） |
| e5_rack_gr | K4 / rack 挂载（seg3a_r2/M0a2 同形） | SILENT+hint（rack 内检出）+形态 3 披露 | ⑤形态验证（rack=B 的 rack 感知验证） |
| e4_surge_xt | K4 / Surge XT | **NON-SILENT -12.6dBFS/rms -24.95**（301 M3=-13.63/-24.95 同款）+hint/fallback/degraded 全空 | **M3 不回退 PASS** |

## 1. M1 判定链（卡面目标 1）

- E1v2 GR 实时播放（RTZ→play→采样 5s→stop→RTZ）：全窗 db=-100（真静音）。
- E1ctrl Surge XT 同 fixture 同装置：-10.97 dB（出声）→ 排除「实时路径不馈声」「测量面死」两类假阴性。
- 结论：**RT-SILENT 成立**——GR（WaveShell1-VST3 17.1 容器内子插件）在本机四套 harness（chat-rack M0a2 / M2 直挂渲染 / 离线渲染 / 实时播放）一致全零静音 → 「non-realtime 特异」假说被推翻，静音归属=装载/license/壳层（S2/S3 族，闭源面）→ 裁定⑤应急预案激活。

## 2. S1 运行时确证与判别（卡面目标 2；五裁定②的 C 判别实验）

- E2a 遥测（e2a_baseline_gr/kernel_log_K2.log + vit_trace）逐字钉死 S1b 时序：
  1. `external_initialise sr=44100 bs=480 isNonRealtime=0`（装载期）
  2. `prepareToPlay 44100/480 → setupProcessing kRealtime`（T1）
  3. 渲染建图：`external_initialise sr=44100 bs=512 isNonRealtime=0`（**bs 480→512 变化触发 re-prepare，标志位仍 0**）
  4. `prepareToPlay 44100/512 isActive=1 guard_early_exit=0 → setupProcessing kRealtime`（**第二次 kRealtime——S1b 精化路径亲证**）
  5. `firstProcessAudio numSamples=512 perCallMode=kOffline setupMode=kRealtime isNonRealtime=1`（**渲染期不一致签名**）
- E2b：`cPrototype=1 reportedMode=kRealtime`（C 原型一致化生效亲证）渲染**仍静音** → 结合 M1（纯实时也静音）：**S1 信令不一致真实存在但不是 GR 静音的成因**；C 在本环境对 GR 无效，C 转正（裁定②条件「S1 证实且限速不可接受」）失去该插件面的依据——留主管裁定。
- 形态 3 全链（E2a/E2b/E3v2/E5 四次触发）：`VIT309_DEGRADED`→同 job realtime 重渲→`VIT309_RT_STILL_SILENT`→render_done `degraded_to_realtime_render=true`+`render_silence_observed=true` 双披露；Surge（E4）零触发。

## 3. 修法 B（裁定②）与⑤落地（卡面目标 3）

- 实现提交 3280352d（原形态：检出→realTimeRender=true+警告）→ **⑤改形 e1873478**：默认 `action=warn_only`（检出+WARN 日志+reply `shell_class_plugin_hint`/`realtime_fallback:false`/`blacklist_action` 三披露，渲染照走离线）；`VIT309_BLACKLIST_REALTIME_FALLBACK=1` opt-in 恢复原 B；`VIT309_DISABLE_BLACKLIST` 全旁路（判别实验用）。
- 谓词：`fileOrIdentifier containsIgnoreCase("waveshell")`，轨 pluginList+rack type 双遍历（E5 rack 检出亲证）；E4 Surge 零误伤亲证。
- **装载取证另立卡建议**（裁定⑤）：GR/WaveShell 在本机的装载/license 态取证（Waves Central/ilok 授权面、壳层初始化探针）超出本卡文件域，交主管发卡。

## 4. 验证矩阵与回归（卡面目标 5）

- M0a2 场景复跑：E5 rack 形态（seg3a_r2 同形）**仍静音**——在 M1=RT-SILENT 下「复跑=出声」在本机环境不可达（渲染静音非渲染路径缺陷；E4 Surge 同路径出声=渲染路径健康），属⑤预案内预期，如实记录；「出声」的达成前置=装载取证另立卡的修复。
- M3 Surge XT 不回退：**PASS**（E4）。
- 全包回归：`go build ./...` exit 0；`go test ./...` exit 0 零 FAIL（含 §6 五健康包 history/harness/capabilitycontext/mom/chat 全 ok）；webui vitest 39 文件/471 测试全绿；AB 烟测（run_ab_result_smoke.ps1）exit 0。
- C++ 构建：三次全量/增量构建 exit 0（92b09da5→074a273b→9241f345）。

## 5. §8 对账

- 申报：六腿单 run 无重跑、零 LLM、同型断点两连败止损。实际：8 个 run 目录=六腿+E1v1（装置缺陷无效轮，不占判定）+E3 首跑（装置缺陷 exit1，修复后 v2）；E1v2/E1ctrl 为 tap 修复后的新装置判定+对照（装置缺陷修复后的合法新 run，非同型断点重试）；零 LLM 兑现；无断言失败轮；止损线未触发。
- 失败分类实录：装置缺陷×2（levelCache 死代码/--expect-env 传参）+判别性结果（RT-SILENT）+无效轮记录在案；环境中断零。

## 6. 端测边界声明（AGENTS §5）

- 本腿端测=agent ZMQ 面真栈驱动（内核 ZMQ 直挂+rack 挂载+实时 transport）+内核日志留痕（VIT301_GUARD/VIT309_*）+wav 物理计量（py wave 逐样本）+render_done 披露字段捕获。
- 不覆盖：webui 渲染面（无该域改动，webui 单测全绿）；Godot 用户旅程（无该域改动）；M0a2 chat 全链复跑（chat 路径渲染入口与驱动路径同一 start_render 面，未另跑付费轮——主管如需 chat 端到端复跑可另派零栈腿或并入装载取证卡）。

## 7. 提交与工件索引

- port 分支提交：896a18c8（栈腿领取）→3280352d（VitApp B+形态3+RTLEVEL）→61e29e1e（驱动器+ps1）→3cee3f42（RUNBOOK）→8bc39aa6（tap 修复）→e1873478（⑤落地）→9da5（expect-env 修复，见 git log）→本回执提交。
- engine 子模块（vit309-stackleg，本地分支未推远端）：60ab1a49（301）+cd957f4（VIT309 mode 遥测）+9b8cc8a（C 原型）——diff 全文见 `git -C D:/Vit_DAW_wt_309recon/tracktion_engine show cd957f4/9b8cc8a`；主仓子模块 gitlink 保持 b4397498 不动（301 同款处置，landing 交主管）。
- 工件：本目录 `stackleg_20261109_1906/`（8 run 子目录）；RUNBOOK-STACKLEG.md（跑前口径）。
