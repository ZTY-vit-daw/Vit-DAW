# EXPADMIT-EVIDENCE-RULE-1 验证腿 verifyrun1 — §8 跑前口径（跑前成文，2026-11-07）

- 执行：ZCode GLM-5.3 L2 PC 执行子代理（269 验证腿，主管派发）；worktree D:/Vit_DAW_wt_269impl @ port/expadmit-evidence-rule-1（origin/main 9e5af73c 已合入为 3ce30ce7，**无冲突**——main 增量与本卡域交集=goalrunner_chat.go 披露白名单[276 补丁]+server_test.go，与本卡 4 文件零重叠；合并后 go build ./... exit 0 + go test agentloop[4.6s]/chat[115.4s] 全绿）
- 被测改动：impl commit 914e5c48——**②**`validateD1FreshObservedTarget` 引用绑定放宽（任一 cited ref 经台账 G7 同源 resolution 族解析到同目标同 revision 行即过；G7 门本体 free_state_gate.go 零改动）+**①**准入拒绝 park 化（不再改写 blocked 终局；TerminalAdjudication 粘附挂确认面；孤儿收口保留兜底）
- 取证基线（对照形态）：coord/runs/MIXTICK-CONFIRM-CONTRACT-1/verifyrun1/evidence.md——262 原失败=提案 evidence_refs 引用同目标同 revision 更早观察 obs_154552(basic_energy)，门绑定 LatestObservation=obs_154607(time_dynamics) 必含其 ID → 拒绝且终局化（blocked→响应面 done→turn_close_guard 孤儿收口，确认卡未建）
- 预期（实现腿就绪声明）：observe 回合直停确认面——262 形态经②直接准入（improvement_proposal_confirmation_required+确认卡落库+loop.Experiment 非 nil），或残余拒绝经①出 park 确认卡（同 stop_reason 但 TerminalAdjudication 粘附在卷）；不终局 done

## 场景与命令

- 脚本：`scripts/run_mix_single_tick_e2e.ps1`（现行版；断言 observe stop_reason ∈ {needs_confirmation, improvement_proposal_confirmation_required}，后接 vocal 澄清守卫 + 双跳确认链——后段腿属 269 判定面之外，如实记录）
- 命令形态（参 262 verifyrun1，基线换本 worktree）：

```powershell
powershell -NoProfile -ExecutionPolicy Bypass -File D:\Vit_DAW_wt_269impl\scripts\run_mix_single_tick_e2e.ps1 `
  -RepoRoot D:\Vit_DAW_wt_269impl `
  -KernelExe D:\Vit_DAW_wt_228\VitApp\build_release\VitApp_artefacts\Release\VitApp.exe
```

- 被测 agent=worktree 现场构建（脚本内 dev_agent_smoke RestartAgent 构建 `agent\bin\VitAgent.exe`，改动在 agent 面），构建后记 SHA256
- kernel=wt_228 VitApp.exe SHA256 F9E3DA0A66263F0B6E3FED5CDA0FB214C460FCE8053234240C470D515ED0D67A（只读复用，225-276 同款；run 前重新亲核）
- 引擎=本机默认无注入（config defaultModel=deepseek-flash，run 前亲读）
- fixture=脚本默认（RepoRoot 下 test_100hz_10s.wav / test_target_3s.wav，两轨新建）

## §8 口径（跑前成文）

- **运行次数**：mix_single_tick 单 run，无重跑（含模型随机分支不重跑）；环境中断至多重建栈 1 次（重建后仍算同一验证腿，run ID 分开记）。
- **预算**：≈¥1-3（单 run：observe + vocal 守卫 + 双跳 3-4 个 chat 回合，flash 引擎；超 ¥3 上界即如实入账上报）。
- **成功条件（269 验证腿判定面）**：observe turn 响应 stop_reason ∈ {improvement_proposal_confirmation_required, needs_confirmation} 且确认卡（improvement_proposal_confirmation face）落库——两条到达路径均判 PASS：②直接准入（loop.Experiment 非 nil）或 ①park（TerminalAdjudication 粘附、Experiment nil）；到达路径如实记录。脚本整程 exit 0 为 §5 交付门槛。
- **失败分类（预写）**：
  - ① **PASS 形态**：observe 回合直停确认面（首选不进切片；经 settle 映射到达也算契约面达成，但如实记录路径）→ 269 验证腿 PASS；
  - ② **终局 done 再现**：observe 回合响应面 stop_reason=done（或等价终局）且确认卡未建 → 修复未覆盖 → 取证上交（console+runtime state+agent log 指印：LatestDecision 形态/AdmissionReceipt/turn_close_guard 行），不重跑；
  - ③ **环境/其他断言类按 §8 通用**：vocal/双跳等后段腿失败 → 269 判定面单独判（observe 面证据完整即判 PASS，后段失败如实记录待主管归类）；环境中断（端口冲突/进程死亡/网络）→ 有原始日志退出证据时记环境中断，至多重建栈 1 次。
- **止损线**：任一失败分类命中即停（除环境中断重建 1 次外不重跑）；止损只停止投入，不改写结论。
- **零改动边界**：发现必须改代码（agent/脚本/kernel）才能过 → 停止实现上交证据。

## 占栈协议（§2.2）

- 净空亲核→登记（逐文件 add 提交推 main）→≤60s 复核（fetch 重读+进程/端口即时复核）→启栈
- 释放：run 后 TEARDOWN_ALL_ZERO 亲核（六口 TCP 5555/5556/5557/7878/8787/18787 listen=0 + UDP 4444/4445=0 + VitApp/VitAgent/VspHub/Godot 四类进程=0）→ 释放登记推 main
