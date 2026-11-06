# MIXTICK-CONFIRM-CONTRACT-1 验证腿 verifyrun1 — §8 跑前口径（跑前成文，2026-11-07）

- 执行：ZCode GLM-5.3 L1-L2 PC 执行会话（262impl 验证腿）；worktree D:/Vit_DAW_wt_262impl @ port/mixtick-confirm-contract-1（origin/main e2651b74 已合入为 0b686c80，无冲突；合并后 go build ./... exit 0 + go test agentloop/chat 全绿）
- 被测改动：裁定 A patch（impl commit 72718892）——`ccb_model_prompt.go:292` 闭合句族（"The evidence you hold is sufficient for a bounded proposal … use needs_experiment with one bounded improvement_proposal.v1 …"）在「台账对该 view set 已返回 usable 证据」条件（去重门同判据 messageLoopFreeStateAlreadyObservedIssue）下提前进 turn_directives，不再等 180s 排干切片后才注入
- 取证基线（对照形态）：coord/runs/MIXTICK-CONFIRM-CONTRACT-1/forensics.md——原失败=模型 27 次重复请求已返回 view set 被去重门弹回×27 + 弹回反馈不进重建请求（指纹 fd94d36a×26 恒定）→180s 排干切片→T32 闭合句注入后一击产提案→settle 读 goal.status 得 done → 断言 FAIL（got 'done'）
- 预期（实现腿就绪声明）：observe 回合内直停 improvement_proposal_confirmation_required（或 settle 映射 needs_confirmation 等价提案面）且确认卡落库，不进切片路径

## 场景与命令

- 脚本：`scripts/run_mix_single_tick_e2e.ps1`（现行版，1059 行；断言 :926 observe stop_reason ∈ {needs_confirmation, improvement_proposal_confirmation_required}，后接 vocal 澄清守卫 + 双跳确认链）
- 命令形态（参 257 G6 工件 mix_tick_20261105_224459 console 形态，kernel=wt_228 同款只读复用）：

```powershell
powershell -NoProfile -ExecutionPolicy Bypass -File D:\Vit_DAW_wt_262impl\scripts\run_mix_single_tick_e2e.ps1 `
  -RepoRoot D:\Vit_DAW_wt_262impl `
  -KernelExe D:\Vit_DAW_wt_228\VitApp\build_release\VitApp_artefacts\Release\VitApp.exe
```

- 被测 agent=worktree 现场构建（脚本内 dev_agent_smoke RestartAgent 构建 `agent\bin\VitAgent.exe`，改动在 agent 面），构建后记 SHA256
- kernel=wt_228 VitApp.exe SHA256 F9E3DA0A66263F0B6E3FED5CDA0FB214C460FCE8053234240C470D515ED0D67A（2026-11-07 亲核，225-265 同款只读复用）
- 引擎=本机默认无注入（config defaultModel=deepseek-flash 亲读）
- fixture=脚本默认（RepoRoot 下 test_100hz_10s.wav / test_target_3s.wav，两轨新建）

## §8 口径（跑前成文）

- **运行次数**：mix_single_tick 单 run，无重跑（含模型随机分支不重跑）；环境中断至多重建栈 1 次（重建后仍算同一验证腿，run ID 分开记）。
- **预算**：≈¥1-3（单 run：observe + vocal 守卫 + 双跳 3-4 个 chat 回合，flash 引擎；超 ¥3 上界即如实入账上报）。
- **成功条件（262 验证腿判定面）**：observe turn 响应 stop_reason=improvement_proposal_confirmation_required（或 settle 映射 needs_confirmation 等价提案面）且确认卡（improvement_proposal_confirmation face）落库——即修复生效。脚本整程 exit 0 为 §5 交付门槛（后续腿 vocal/双跳属 262 判定面之外，另记）。
- **失败分类（预写）**：
  - ① **PASS 形态**：observe 回合直停提案面（首选直停不进切片；经 settle 映射 needs_confirmation 到达提案面也算契约面达成，但如实记录走了切片路径）→ 262 验证腿 PASS；
  - ② **弹回族再现**：模型重复请求已返回 view set 被去重门弹回（连续弹回≥2 轮/进入 180s 排干切片路径）→ 修复面未覆盖的新形态 → 取证上交（console+loop debug+prompt render 指纹），不重跑；
  - ③ **断言/环境类按 §8 通用**：断言失败（observe 面既非提案也非弹回族，如 done/其他 stop_reason）→ 取证上交不重跑；vocal/双跳等后段腿失败 → 262 判定面单独判（observe 面证据完整即判），后段失败如实记录待主管归类；环境中断（端口冲突/进程死亡/网络）→ 有原始日志退出证据时记环境中断，至多重建栈 1 次。
- **止损线**：任一失败分类命中即停（除环境中断重建 1 次外不重跑）；止损只停止投入，不改写结论。
- **零改动边界**：发现必须改代码（agent/脚本/kernel）才能过 → 停止实现上交证据。

## 占栈协议（§2.2）

- 净空亲核→登记（逐文件 add 提交推 main）→≤60s 复核（fetch 重读+进程/端口即时复核）→启栈
- 释放：run 后 TEARDOWN_ALL_ZERO 亲核（六口 TCP 5555/5556/5557/7878/8787/18787 listen=0 + UDP 4444/4445=0 + VitApp/VitAgent/VspHub/Godot 四类进程=0）→ 释放登记推 main
