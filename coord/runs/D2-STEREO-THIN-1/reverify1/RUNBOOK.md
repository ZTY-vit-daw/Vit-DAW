# D2-STEREO-THIN-1（265）exit 0 补跑腿 reverify1 §8 口径（跑前成文）

- 时间：2026-11-06 23:00 后非高峰窗（主管裁定选项①：修复后脚本化 exit 0 补跑一轮；22:40 gate 夜间排程=23:00 栈腿串行，265 补跑为首腿）；owner=GLM-5.3（PC，ZCode 执行会话 265rv）
- 依据：卡面验收节「exit 0 未达成→选项①补跑腿挂今夜 23:00 后非高峰自主批次（脚本 bug 四处已修入库，260 分支化已合入；补跑过=终验闭合，再败按 §8 取证）」+ RUNBOOK d2run1 §run2-run4 定性。
- 被测 worktree：D:/Vit_DAW_wt_265rv @ port/d2-stereo-reverify = origin/main 853bcfb1（干净树零 diff 亲证）；被测内容=**main 最新**，含 265 合入 7d6a5abe（D2 runtime 四文件+38df2eb2 门组加参+eeb92423/ca721b18 门组取证修复两笔）+268 门组合入 e98caae0（门B/D1 门D mix.report+守卫访问/门C durable walk/:658 分支检测 UTF8——同 runner 路径，本腿活栈顺带覆盖其读取面）+260 分支化（268 提交内证 leg6/leg8 零回退）。**零代码改动预期**：若发现必须改动才能跑=取证上交，禁止修。
- 被测 agent：worktree 现场构建 `agent\bin\VitAgent.exe` SHA256 F8790861CAB7F9874A5EE39B02A8817C420039EEBD543F77A942E62535F8488A（2026-11-06 23:03 构建，38,340,608 B）；kernel=D:/Vit_DAW_wt_228/VitApp/build_release/VitApp_artefacts/Release/VitApp.exe SHA256 F9E3DA0A66263F0B6E3FED5CDA0FB214C460FCE8053234240C470D515ED0D67A（23:02 亲核，225-265 同款只读复用）；引擎=本机默认无注入（~/.vit/config.json defaultModel=deepseek-flash 23:03 亲读）。
- 与 d2run1 最终执行形的差异仅两处：①`-RepoRoot` 换本 worktree；②被测基线从 port/d2-stereo-thin-1 实现树换 main 最新（=主管裁定口径，同参命令其余不动）。

## 命令

```powershell
powershell -NoProfile -ExecutionPolicy Bypass -File D:\Vit_DAW_wt_265rv\scripts\run_free_state_d1_smoke.ps1 `
  -RepoRoot D:\Vit_DAW_wt_265rv `
  -PublicManifest D:\Vit_DAW\temp\semantic-processor-agent-project-smoke-v1\fixtures\semantic_processor_project_smoke_v1_80085263a651cf20\fixture_manifest.json `
  -PromptFlavor reverb -ExpectDomain reverb -D2StereoCheck -SkipBuild `
  -KernelExe D:\Vit_DAW_wt_228\VitApp\build_release\VitApp_artefacts\Release\VitApp.exe
```

（不带 -LoudnessWatch/-D1LoudnessSettle 同 d2run1：D1 面已有 254/258 真栈先例，本腿只验 D2 面补跑 exit 0。）

## 运行次数 / 成功条件

- **单 run 无重跑**：一次起栈一次执行；同一确定性断点失败两次禁止原样重跑（AGENTS §8）。
- 成功 = **PS1 exit 0**，含：
  - runner 主链（D1-S1 场景，分支容忍口径同 256：1-mutation 主链/0-mutation 合法分支皆可——260 分支化后由 :658 分支检测读出实际分支）；
  - D2 门组四门全绿：check_turn（stop_reason=d2_stereo_checked+三态 verdict+action_face=disclosure_only_v1+零 mutation）/ verdict_on_disk（stereo_check_verdicts.json 带 d2_stereo_check trigger）/ action_face（零挂卡零 mutation）/ d2_node_lit（queue view D2 ∈ {in_progress, needs_confirmation}）；
  - probe 腿（P）如实记录：裸话术过/不过 semantic_entry 均如实入 d2_gates.json（过=不建词表基线再证；unresolved=263 型入口证据，记录交主管，不 fail 机制门）。
- verdict 三态取值 fixture-dependent 不硬断言（90 卡先例；阈值边界由零 LLM 单测钉死）。

## LLM 参与与预算

- runner 主链 + D2 probe 腿为 LLM 腿（deepseek-flash）；D2 显式检查 turn 词法命中先于 semantic_entry=零 LLM。
- 预算 ≈¥0.5-1.5（d2run1 本卡 D2 面单轮归属实测 ≈¥0.8-1.5；非高峰窗排干风险下降）；跑后以 telemetry 行数与 chars 亲数核对。

## 失败分类（预写）

1. **runner 排干族再现于非高峰窗**：=新取证（d2run1 已归因「晚高峰瞬态」，非高峰复现将推翻该归因前提），取证上交主管，不重跑；
2. **D2 门组断言失败**：取证（main 已含全部四处脚本修复，再失败优先怀疑产品面回归或新形态；区分脚本面残留 vs 产品面），不重跑；
3. **环境中断**（端口残留/进程冲突/启动 throw）：按中断记录拆栈，重建栈至多 1 次（256 先例③，仅限中断类）；
4. **发现必须改代码才能跑**：停止上交取证（零代码改动红线）。

## 止损线

- 单 run 结束即止：任何门失败→取证入 reverify1 工件→卡面回执上交主管；不自行重跑、不自行改断言凑过、不把失败改写成成功。

## run1 结果：环境中断（预写分类③），按预写重建栈重跑一次（2026-11-06 23:2x 记录）

- run stamp=20261106_230715（23:07:15 起栈：kernel pid=33348=wt_228 F9E3DA0A 复用/agent pid=13984=worktree 构建 F8790861/Godot pid=48076；栈起段全绿 tool_count=295）；**PS1_EXIT=1**。
- **失败形态=预写分类③环境中断（infra 瞬态），非排干族 throw、非断言失败**：
  - runner 主链确认链健康走完 5 响应（improvement_proposal_confirmation_required→plugin_selection_required→〔无 stop 中间步〕→semantic_dynamic_confirmation_required→limit_reached[completed_steps=12/executed=12]，d1_smoke_report.json responses 亲证）；
  - 23:16:16 agent 侧 `continuation.arm waiting_continue stop=limit_reached limit=timeout`（排干族签名在场但**非致死点**——d2run1 run2/4 的致死 throw 文本「did not drain within the smoke timeout」未出现）；
  - **致死点=python 驱动在 resp4（limit_reached）后的下一次 urlopen（wait_scheduler_drain 腿，目标 127.0.0.1:7878）获 WinError 10061 连接拒绝**（~23:16:17）——彼时三进程全活（VitApp 23:07:19/VitAgent 23:07:21/Godot 23:07:25 启动后零重启）且 7878 事后亲证由 VitAgent 13984 在听；单次 loopback 瞬态拒绝，无产品断言失败、零代码改动；
  - python 死后 agent 仍自主排干同一 goal（活日志 23:20:26 turn=36 持续推进）——owner 23:22 亲拆止血；
  - 8787 VSP 拒绝=248 在案既有形态（本栈无 VspHub 进程，非本腿引入变量）。
- **处置=按预写③「环境中断至多重建栈 1 次」**：owner 亲拆三进程（VitAgent 13984/VitApp 33348/Godot 48076），**TEARDOWN_ALL_ZERO 亲核 23:22:05**（六口 TCP 5555/5556/5557/7878/8787/18787=0+UDP 4444/4445=0+四类进程 0）；重建后同参重跑一次（run2=最终轮，再败即止损上交）。
- **run1 计费（拆栈冻结后亲数 telemetry）**：49 调用全 deepseek-flash in 845,711 / out 88,019 / cache_read 351,104 tok，0 err，估 ≈¥0.5-1（d2run1 chars 比价），入本腿总账；快照 telemetry_run1_interrupted{,final}.jsonl 在档。
- 工件：console_20261106_2307.txt+d1_smoke_report.json+agent_last.log（artifacts/free_state_d1_s1/20261106_230715/ 留盘，小件随本目录入库）。
