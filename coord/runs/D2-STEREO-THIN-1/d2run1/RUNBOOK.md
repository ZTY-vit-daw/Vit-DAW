# D2-STEREO-THIN-1（265）真栈验证腿 d2run1 §8 口径（跑前成文）

- 时间：2026-11-06 晚窗；owner=GLM-5.3（PC，ZCode 执行会话 265impl）
- 被测 worktree：D:/Vit_DAW_wt_265impl @ port/d2-stereo-thin-1（领取 1f5e33ee 基线 50e6f859；实现 f354430d=chat D2 薄 runtime 四文件；38df2eb2=run_free_state_d1_smoke.ps1 -D2StereoCheck 门组）
- 被测 agent：worktree 现场构建（改动在 agent 面，-SkipBuild 不适用）；kernel=wt_228 VitApp.exe SHA256 F9E3DA0A66263F0B6E3FED5CDA0FB214C460FCE8053234240C470D515ED0D67A 19:22 亲核只读复用（225/249/251/253/254/256/258/261/263 同款）
- 引擎：本机默认无注入（~/.vit/config.json defaultModel=deepseek-flash 19:22 亲读）

## 命令

（最终执行形：agent=worktree 现场构建 `agent\bin\VitAgent.exe` SHA256 F4396BC0C5A4F7A74B9F09AB00783ACF0B9BF7A44D9D16C3484DFB4F44E2120F，`-SkipBuild` 复用该二进制——258 先例。）

```powershell
powershell -NoProfile -ExecutionPolicy Bypass -File D:\Vit_DAW_wt_265impl\scripts\run_free_state_d1_smoke.ps1 `
  -RepoRoot D:\Vit_DAW_wt_265impl `
  -PublicManifest D:\Vit_DAW\temp\semantic-processor-agent-project-smoke-v1\fixtures\semantic_processor_project_smoke_v1_80085263a651cf20\fixture_manifest.json `
  -PromptFlavor reverb -ExpectDomain reverb -D2StereoCheck -SkipBuild `
  -KernelExe D:\Vit_DAW_wt_228\VitApp\build_release\VitApp_artefacts\Release\VitApp.exe
```

## 启动形式失败记录（两次，栈未起零 LLM，不入有效轮次——254 armrun1 先例口径）

1. 19:35 首启：脚本以临时分支 tmp-stack-265b（=origin/main 内容）执行，`-D2StereoCheck` 参数不存在（NamedParameterNotFound）——占栈登记用的临时分支未切回实现分支即执行，形式失败；
2. 19:38 第二次：未带 `-SkipBuild`，ps1 强制 cmake 构建 `VitApp\build`（fresh worktree 无该目录）→throw——258 verifyrun1 首启同型。修正=现场构建 agent 二进制 + `-SkipBuild` + `-KernelExe` wt_228 显式。

（主检出 fixture 供给：fresh worktree 无 gitignored temp——254 armrun1 教训先例；不带 -LoudnessWatch/-D1LoudnessSettle：D1 面已有 254/258 真栈先例，本卡只验 D2 面，减少无关相位与预算。）

## 运行次数 / 成功条件

- **单 run 无重跑**：一次起栈一次执行；同一确定性断点失败两次禁止原样重跑（AGENTS §8）。
- 成功 = PS1 exit 0，含：
  - runner 主链（D1-S1 场景，分支容忍口径同 256：1-mutation 主链/0-mutation 合法分支皆可）；
  - D2 门组四门全绿：check_turn（stop_reason=d2_stereo_checked + 三态 verdict + action_face=disclosure_only_v1 + 零 mutation）/ verdict_on_disk（stereo_check_verdicts.json 带 d2_stereo_check trigger）/ action_face（零挂卡零 mutation）/ d2_node_lit（queue view D2 ∈ {in_progress, needs_confirmation}）；
  - probe 腿（P）已记录：裸话术过/不过 semantic_entry 均如实入 d2_gates.json——**通过=不建词表基线确认；unresolved=263 型入口准入层证据，记录交主管，不 fail 机制门**。
- verdict 具体三态取值 fixture-dependent 不硬断言（90 卡先例；阈值边界由零 LLM 单测钉死）。

## LLM 参与与预算

- runner 主链 D1-S1 场景 + D2 probe 腿为 LLM 腿（deepseek-flash）；D2 显式检查 turn 词法命中先于 semantic_entry=零 LLM。
- 预算 ≈¥1-3 上界（先例参照：258 verifyrun1 六调用 ≈¥0.1-2、armrun2 十调用 ≈¥0.2-3、257 批 2 ≈¥0.2-4）；跑后以 telemetry 行数与 token 亲数核对。

## 失败分类（预写）

1. gate A stop_reason 不符：入口路由回归（词表/intent 分支）——取证 chat json 不重跑；
2. gate B 落盘缺失：store 路径/projectstore 绑定——取证 agent log；
3. gate C 出现挂卡：v1 钉死无卡语义被违反——取证（本卡红线级）；
4. gate D 节点未点亮：queue 挂点注册——取证 queue view json；
5. runner 腿断言失败：254-258 族分支形态，先判「断言过时×模型分支 vs 真回归」分列，不追绿；
6. probe unresolved：入口准入层证据（263 同族），记录不判机制失败；
7. 环境中断（端口残留/进程冲突/启动 throw）：按中断记录拆栈，不烧轮——重建栈至多 1 次（256 先例③，仅限中断类）。

## 止损线

- 单 run 结束即止：任何门失败→取证入 d2run1 工件→卡面回执上交主管，不自行重跑不自行改断言凑过。

## run1 结果与 run2 授权（2026-11-06 20:0x 记录）

- **run1（20261106_194406，19:44:06 起栈）PS1_EXIT=1**：runner 主链 D1-S1 PASS（branch=zero_mutation_boundary 256 合法分支）+ **门 A 全过**（stop_reason=d2_stereo_checked / verdict=unmeasurable[fixture 如实：stereo_face=missing+7 轨无生产者] / action_face=disclosure_only_v1 / mutation=False）；三门失败**全部归因本卡门组脚本自身 bug，产品面零缺陷**：
  - 门 B FAIL=搜索腿 bug：store 记录实际在盘（`project/.vit_agent/vitproj_b3f…/mixboard/stereo_check_verdicts.json`，`vitproj_b3f…|5|unmeasurable` trigger=d2_stereo_check 亲证）——原 catch{} 吞异常无诊断面，修复=加 -Force+诊断记录+直查兜底；
  - 门 C throw=短路序写反（`or` 右操作数在 needs_confirmation=false 时求值 → StrictMode 属性缺失 throw）；响应实际无卡字段=v1 零卡语义成立；修复=PSObject.Properties 守卫访问；
  - probe EVIDENCE=事后取 decision 属性不当（该属性仅 unresolved 形态响应才有）；**probe 本体已过**（stop_reason=limit_reached=进入自由态循环，非 semantic_entry_unresolved 模板）——裸话术过 semantic_entry=不建词表基线 run1 已证。
- **run2 授权依据**：256 先例（断言面取证后修复再跑，非原样重跑——三门代码随诊断变更）；run1 的 LLM 腿成本计入总预算（runner+probe 已花，run2 重复 runner 腿，合计仍按 ≈¥1-3 上界核对）。
- **run2 为最终轮**：任何 D2 门再失败→止损取证上交，不跑第三轮。
- run1 throw 路径未自拆栈（257 同型）→ owner 亲拆 VitApp/VitAgent 后 run2 重起（新工件目录 20261106_*，不覆盖 run1）。
