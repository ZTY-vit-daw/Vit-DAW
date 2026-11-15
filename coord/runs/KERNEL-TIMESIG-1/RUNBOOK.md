# KERNEL-TIMESIG-1 真栈烟测 RUNBOOK（§8 跑前口径）

- 卡：doing/2026-11-16-KERNEL-TIMESIG-1.md（池序 375，P0）
- owner：GLM-5.3 L2 执行子代理（ZCode PC 会话，KERNEL-TIMESIG-1 专项）
- 实现分支：`port/kernel-timesig-1` @ `318f2f17`（已推）；领取基线 409435e2；领取提交 636d31b3
- 本 RUNBOOK 写于正式轮之前（2026-11-16 04:1x +0800）——口径成文先于运行，不因结果改判。

## 1. 被测面与 berth 形态

- berth：kernel + agent（`-StartKernel -RestartAgent`，不启 UI）；断言体走内核 ZMQ 命令面（VSP hello + legacy envelope，automation_curve_smoke.py 同形态）。
- 被测内核：worktree 构建 `C:\Users\timoz\wt_timesig\VitApp\kernel-build\VitApp_artefacts\Release\VitApp.exe`（源=port/kernel-timesig-1 @ 318f2f17，含本卡全部内核改动；sha256 前缀 7f20cb54abe63cae；构建于 00:31，VitApp 目标 Release exit 0）。内核运行时拼装=主仓部署 staging 的 Settings/+default_project.xml 拷贝+新 Logs 目录（Cache/历史 Logs 不带），exe 换为本卡构建；经 `-KernelExe` 显式指向（脚本自身不建内核，回执记录该哈希与时间——AGENTS §9）。
- 被测 agent：worktree `agent\bin\VitAgent.exe`（脚本无 -SkipBuild 时 go build @318f2f17——含 catalog/归一化接线；本腿不对其 HTTP 面做运行时断言，覆盖边界见 §7）。
- 工程隔离：`-TimesigSmoke` 在脚本 GUI-leg 隔离列表内 → VIT_PROJECT_XML=run 目录 git-clean default_project.xml 副本（内核 new_project 的 saveProjectToDefaultXml 落 run 目录，部署 default_project.xml 零触碰）。
- 写面（S10 兜底，351 结构隔离未在基线）：`%APPDATA%\Vit-DAW\global_project_config.json` 跑前快照（recents_before.json；本轮基线 sha256 前缀 d9b53ba7c1c9a33，=373 还原后态）→ teardown 恢复+哈希亲证一致；恢复失败=装置非零退出按环境中断分类。

## 2. 断言腿（scripts/timesig_smoke.py，T1-T5 全绿 + 脚本 exit 0 才算 PASS）

- T1 `new_project {"time_signature":"2/4"}`（W01 wire form，§13 本课工程拍号）→ 回执 echo 2/4 + music_context 2/4；
- T2 `set_time_signature {3,4}` → 回执 echo 3/4 + music_context 3/4；
- T3 fail-closed 四反例（字符串分子/零分子/分母 128/缺分母）逐例 error 回执，且每例后 music_context 仍 3/4（零写入）；
- T4 `save_project`(.vit→run 目录) → `open_project` 同文件 → music_context 仍 3/4（真实内核进程存→开回读一致）；
- T5 无字段 `new_project` → music_context 4/4（旧调用零变化）。

## 3. 运行预算与成功条件

- 正式轮 **1 次单 run 无重跑**；成功 = `dev_agent_smoke.ps1` 进程 exit 0 且 T1-T5 全绿且工件齐（timesig_smoke_summary.json + console + 内核/agent 日志 + recents 快照）。
- LLM 计费上界 = **0**（`-NoChatSmoke` 显式关默认 chat 腿；本腿全确定性 ZMQ 面，零 chat 调用）。

## 4. 失败分类（分记，不得互吞）

1. **环境类**：端口被占/残留进程/运行时拼装缺件/python 环境缺 zmq/recents 恢复失败——有原始日志或退出证据才可排除出有效轮次；
2. **face_defect（产品缺陷）**：T1-T5 任一断言失败=内核命令面/持久化缺陷→停止投入上交（本卡验收面）;
3. **probe_defect（装置缺陷）**：探针自身缺陷（如断言写错键名/超时参数错）→修复装置（装置域文件可改），与产品缺陷分开记。

## 5. 止损线

- 同一代码版本同一确定性断点连续失败 2 次 → 禁止原样重跑，先补锚点/缩假设/拆取证卡；
- 止损只停止投入，不把未发生的成功改写为成功；环境中断的下游断言失败不作功能结论。

## 6. 栈纪律

- 占用登记按 PC-RUNTIME-STACK 协议：净空亲核（04:16 Vit 系进程=0+TCP 7878/7879/8787/5555/5556=0+UDP 4445=0 亲证）→登记推 main 确认→≤60s 复核→启栈；finally 拆栈 TEARDOWN_ALL_ZERO 亲核→释放记录+run ID。374 谱面工作台真栈腿排本卡之后，尽快释放。

## 7. 覆盖边界声明（AGENTS §5）

- 本腿覆盖：内核命令面（创建时应用/设值/fail-closed/存→开往返/缺省零变化）+music_context 读面，真实三件套中的内核+agent 两件。
- 本腿不覆盖：agent HTTP 工具面运行时调用（catalog/归一化由 Go 单测覆盖：TestTimeSignatureToolIsCataloged+TestNormalizeCommandArgsSetTimeSignature；374 M3 消费时自然走到）；webui 渲染面与用户旅程（本卡 webui 零触碰，渲染面烟测基线即红=域外在飞卡事项，锚点见卡面回执）。

## 8. 执行记录（无效轮分记）

- **无效轮 A（04:17 登记→04:19 driver 终止→04:20 teardown，分类=probe_defect 装置域）**：driver 在 TimesigSmoke 腿 python Traceback 终止——探针误用 canonical VSP command 形发 `new_project`，内核 canonical 命令图不含工程生命周期命令（`VspKernelReference.cpp canonicalCommandMap` 无 new_project/open_project/save_project/set_time_signature；set_tempo 同不在图内——此类命令走 `legacy.command` 信封）。零产品断言执行（T1 分发即被拒，无产品命令到达内核命令处理器）；零 LLM。处置=装置域修复（本卡文件域内）：`timesig_smoke.py send_vsp_command` 改 legacy.command 信封（command.response/command.error 双型，断言统一作用于 payload.legacy_reply）。owner 亲停 VitAgent(19760)/VitApp(25704)，TEARDOWN_ALL_ZERO 亲证 04:20（Vit 系进程=0+TCP 7878/7879/8787/5555/5556=0+UDP 4445=0）；recents 哈希未动亲证（d9b53ba7c1c9a332f8b…=基线，无效轮零 open/save）。正式轮随后以同一二进制/同一断言体重跑（§3 预算=正式 1 轮，无效轮 A 不占正式轮——与 373 RUNBOOK §6 同口径）。
- **无效轮 B（04:21 起→04:22 driver 终止，分类=probe_defect 装置域）**：**产品断言 T1/T2/T3 首例全绿**（T1 new_project 2/4 回执 echo+music_context 2/4；T2 set_time_signature 3/4+music_context 3/4；T3 string_numerator 拒绝后拍号仍 3/4），T3 第二例 zero_numerator 处探针断言缺陷终止——require_error 模式误锚命令名（"set_time_signature"），而范围类拒绝的内核逐字理由是共享校验器的 "time signature values must be integers in 1..64"（applyToEdit 路径，与 set_project_key 先例同结构：字段型拒绝带命令名/范围型不带）。**产品面零缺陷**（拒绝本身正确生效、fail-closed 零写入已证）。处置=装置域修复：T3 四反例逐例锚逐字理由（字段型="requires an integer numerator/denominator"；范围型="must be integers in 1..64"）。owner 亲停 VitAgent(27012)/VitApp(24008)，TEARDOWN_ALL_ZERO 亲证 04:22（进程=0+TCP 五口=0+UDP 4445=0）；recents 哈希未动亲证（d9b53ba7…=基线；本轮 T4 open_project 未达）。同断点连败计数：两次终止均为装置域断言缺陷、非同一产品断点（且产品断言逐轮推进），不触发 §5 止损；正式轮 C 随后重跑。

## 9. 正式轮 C 结果（run_20261116_0422）

- **driver exit 0**（`dev_agent_smoke.ps1 -StartKernel -RestartAgent -TimesigSmoke -NoChatSmoke -NoStripSilenceSmoke -KernelExe run_20261116_0422\runtime\VitApp.exe`；完整 console=run 目录 driver_console.txt）。
- **T1-T5 全绿**：T1 new_project time_signature "2/4"（W01 wire form）回执 echo 2/4+music_context numerator=2/denominator=4；T2 set_time_signature {3,4} 回执 3/4+music_context 3/4；T3 四反例（string_numerator/zero_numerator/denominator_above_64/missing_denominator）逐例 error 回执理由逐字锚定+每例后 music_context 仍 3/4（零写入）；T4 save_project→run 目录 timesig_project.vit→open_project 同文件→music_context 3/4（真实内核进程存→开回读一致）；T5 无字段 new_project→music_context 4/4（旧调用零变化）。
- 工件齐：timesig_smoke_summary.json（result=PASS，含各腿逐项）+timesig_smoke_console.txt+driver_console.txt+timesig_project.vit+gui_project_isolation.json+recents_before.json+runtime/（被测内核运行时）。
- 被测二进制亲核：run 目录 runtime/VitApp.exe sha256 `7f20cb54abe63cae…c0a56baefca6f30`（=worktree kernel-build @318f2f17 构建，00:31 产物）；agent=worktree go build @318f2f17（driver 内建，bin/VitAgent.exe）。
- 计费 ¥0 亲证：-NoChatSmoke（默认 chat 腿关闭）+全确定性 ZMQ 面+零 /agent/chat 调用。
- teardown：owner 亲停 VitAgent(1840)/VitApp(23544)；TEARDOWN_ALL_ZERO 亲证 04:23:20（Vit 系进程=0+TCP 7878/7879/8787/5555/5556=0+UDP 4445=0）。
- S10 recents：T4 open_project 曾写 recents（跑中哈希 f73451ba…≠基线，预期写面）→teardown 后自 recents_before.json 还原→全哈希亲证一致 `d9b53ba7c1c9a332f8bf384f67645b788a601c742ba3dd4b409c3fcc54fa9f05`（=基线=373 还原后态）。
- 工件所在分支：`port/kernel-timesig-1`（§6.1 纪律——run 工件入独立 worktree/port 分支，主检出不留未跟踪副本）。
