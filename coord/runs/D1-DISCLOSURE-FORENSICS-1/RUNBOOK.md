# D1-DISCLOSURE-FORENSICS-1 验收腿 RUNBOOK（§8 跑前口径，成文于占栈前）

- 2026-11-06 早窗 / owner=ZCode GLM-5.3 L1 PC 执行会话（258 卡会话）
- 目的：修复后单 run 重跑 **254/256 门组**（卡面验收腿）——LW 四门不回退+D1 settle 四门+queue view D1≠not_started，过则 **D1 真栈点亮收口**，254/256 运行面挂账清。

## 被测面

- 被测改动=agent 三文件（agentloop/free_state_gate.go G7 准绳漂移治愈 + chat/audio_closure_controller.go 落账前 revision 治愈 + contextruntime/model_projection.go ledger 降级序最后）+三回归测试；`-RepoRoot D:\Vit_DAW_wt_258impl`（worktree 检出即含修复；**agent 由脚本现场构建**——本卡修复在 agent 面，-SkipBuild 不适用，构建产物须含本卡 diff）
- kernel=`D:\Vit_DAW_wt_228\VitAppuild_release\VitApp_artefacts\Release\VitApp.exe`（SHA256 F9E3DA0A66263F0B6E3FED5CDA0FB214C460FCE8053234240C470D515ED0D67A 只读复用，225/249/251/253/254/256 同款，C++ 面零改动）
- 引擎=本机默认无注入（config `~/.vit/config.json` defaultModel=deepseek-flash，跑前亲读，同 254 armrun2/256 verifyrun）
- 命令=与 256 verifyrun 同参，仅 -RepoRoot 换 wt_258impl 且**不带 -SkipBuild**（agent 含本卡修复）：
  `run_free_state_d1_smoke.ps1 -RepoRoot D:\Vit_DAW_wt_258impl -PublicManifest <主检出 fixture> -PromptFlavor reverb -ExpectDomain reverb -LoudnessWatch -D1LoudnessSettle -KernelExe <wt_228 显式>`
- fixture=spv1_p01（public_source_project 逐字节同路径，同 133/254/256 全系）

## 成功条件（254/256 门组）

1. PS1_EXIT=0（runner 分支容忍两分支皆合法：1-mutation 主链 / 0-mutation 诚实边界分支）；
2. LW 四门不回退（①observe 披露 loudness_watch ②verdict store schema+round trigger ③dedupe ④gate4 config-off silent）；
3. D1 settle 四门（A stop_reason=d1_loudness_settled+观察面+零 mutation/B d1_settle verdict 落盘/C action face 按 verdict 分支/D queue view D1 node ∈ {in_progress, needs_confirmation}）；
4. 全程零崩溃、栈可拆净（TEARDOWN_ALL_ZERO）。

## 运行次数 / 预算 / 止损（§8）

- **单 run 无重跑**；deepseek-flash 预算 ≈¥1-3（armrun2 实测 10 调用 ≈¥0.2-0.3、verifyrun1 ≈12-20 调用 ≈¥0.3 上界容轮数分支浮动）。
- 失败分类预写：
  ①模型随机分支再现 verifyrun1/2 原始形态（披露/准入修复后预期两形态上游已消：轨道标识 ledger 存活+G7 准绳治愈；若再现=修复面未覆盖的新分支形态）→取证上交不重跑；
  ②断言失败（新 0 分支断言/主链断言/LW/settle 门任一）→取证上交不重跑（卡面停止条件）；
  ③环境中断（端口冲突/进程残留/API 额度/LLM 服务中断）→按环境中断记录，至多重建栈 1 次续跑（同 run 预算内），再失败即上交。
  ④PCA 认证腿并发再现（pcactl 漂移）→**本次属修复覆盖面**：G7 治愈+closure 落账治愈下应不再致准入不可满足；若仍有下游新形态→取证上交。
- 止损线：本 run 失败即收口上交，不烧第二轮；不因偶然成功倒推验收标准。

## 占用与工件（§2.2 全流程）

- 占用登记：coord/resources/PC-RUNTIME-STACK.md（净空亲核→登记推 main→重读确认→≤60s 即时复核→启栈→finally 拆栈核净→释放登记）；与 263 卡（C4-ABLATION-1）ablation run 互斥——占栈前 fetch 重读其占用状态，对方已登记则等待。
- 工件：coord/runs/D1-DISCLOSURE-FORENSICS-1/verifyrun1/（随 port/d1-disclosure-forensics-1 入库）；大件留盘 worktree artifacts/。

## verifyrun 终态（2026-11-06 11:39 收口）

| run | stamp | 结果 | 形态 | 门组 |
|---|---|---|---|---|
| 首次启动 | 11:27 | **启动腿形式失败（零 LLM）** | 外层 ps1 无 -SkipBuild 强制 cmake 内核构建，fresh worktree 无 VitApp/build 目录→throw（栈未起） | 未起栈 |
| verifyrun1 | 20261106_113023 | **runner exit 0（D1-S1 PASS）+PS1_EXIT=1（LW 相位 throw）** | resp0=**improvement_proposal_confirmation_required（admission.boundary=admitted，G1-G8 全 pass，proposal_present=true）**→driver 确认→resp1=semantic_treatment_capability_boundary（渐进披露审计 partial，armrun2 同型诚实边界）——**runner 判 pass/branch=zero_mutation_boundary**（256 合法化分支），target_ref=track 1032（轨道标识可见修复亲证）+selected_domains=[reverb]（提案面达成）+forward/journal mutation=0 | runner 门 ✓；LW 四门 ✗（watch hook 懒注册无触发器：0-mutation 分支无 project_change/settle——**非 LW 面回退，本卡零改 LW**）；D1-SETTLE 四门+queue view D 门=**未执行到**（ps1 相位序 LW→D1-SETTLE，LW throw 拦断） |

- **两 verifyrun 形态上游修复真栈亲证**：①轨道标识披露——终局轮模型成功填写提案目标 track 1032（对照 verifyrun1 的「未披露任何轨道标识」）；②证据新鲜度准入——**admitted 全门过**（对照 verifyrun2 的 G7 单项 fail/冻结 rev2）；本 run 无 pcactl 并发漂移（G7 在 rev2 一致基准下过——漂移治愈面由单测+守卫测试覆盖，真栈漂移复现属随机外部事件不可编排）。
- **被测 agent**=wt_258impl 现场构建 bin/VitAgent.exe SHA256 B012294033B576AAF1847A31A0D413412652604C3133A3F824DCADEEBB38EE92（38,302,208B 11:30:05，含 impl 1b9321e9）；kernel F9E3DA0A 同款只读复用（binary face 行亲证两者）。
- **LW/D1-SETTLE 门组的分支结构性**：254/256 门组字面全过仅在 1-mutation 分支可达（watch hook 需 project_change/settle 触发懒注册；verdict store 需 settle 落盘；133 run2 参照=1-mutation 分支）。256 的分支容忍只覆盖 runner 断言面（D1-S1 场景），未覆盖 ps1 的 LW/D1-SETTLE 相位门。**处置三选项交主管**：①接受 zero_mutation 分支的 D1 点亮判定+LW/settle 相位门分支化（脚本面卡）；②再派单 run 赌 1-mutation 分支（模型随机性，133 序列 8 run 仅 1 次 mutation）；③本卡收口「披露/准入修复面 pass+D1 场景 runner 面 点亮」，LW/settle 全门组点亮另卡。
- **计费**：d1_s1 会话 6 调用全 deepseek-flash in 84,597/out 8,413 ≈¥0.1-0.2（预算 ¥1-3 内，§8 单 run 无重跑兑现——首启形式失败零 LLM 不构成有效运行）。
- **TEARDOWN_ALL_ZERO 亲核 11:39:23**（VitAgent 47952/VitApp 9036/Godot 44212 亲停；五口 TCP 0+UDP 4444/4445=0+四类进程 0）。
- **结论**：修复面目标 1/2 达成且真栈生效（两形态上游消解+D1-S1 runner 面 PASS）；验收腿字面门组（LW 四门+D1 settle 四门+queue view）在本分支结构性未全达——**运行面验收交主管裁定**。
