# EDU-SYMBOL-CORE-1 段 3 真栈烟测腿 smokerun1 — §8 跑前口径（跑前成文，2026-11-07）

- 执行：ZCode GLM-5.3 L2 执行子代理（276impl 段 3）；worktree `D:/Vit_DAW_wt_276impl` @ `port/edu-symbol-core-1`（origin/main b8a3d060 已合入为 ee9be3c1，零冲突；合并后 `go build ./...` exit 0 + 全量 `go test ./... -count=1` exit 0 零失败）
- 被测改动：段 1+2 实现（检测核+判定器+命令面+三处注册，impl commits 20d4115d / 15276ca4）+ 本段冒测扩展（`scripts/edu_transform_smoke.py` 新建 + `scripts/dev_agent_smoke.ps1` 加参 `-EduTransformSmoke`：参数声明/隔离条件列/telemetry env/腿块四处，透传缝预声明，零 Go 代码改动）
- 被测 agent=worktree 现场构建（ps1 -RestartAgent 构建 `agent\bin\VitAgent.exe`，edu 改动在 agent 面），构建后记 SHA256；kernel=wt_228 VitApp.exe SHA256 F9E3DA0A66263F0B6E3FED5CDA0FB214C460FCE8053234240C470D515ED0D67A（225-265 同款只读复用，零内核改动面）
- 引擎=本机默认无注入（config defaultModel=deepseek-flash 亲读）；预期 LLM 参与=1 个 chat 回合（内含工具往返）

## 场景与命令

- 断言体：`scripts/edu_transform_smoke.py`（退出码 0/2/3/4 预声明）——Phase A fixture（内核 ZMQ：new_project→save_as_project 落 run 工件目录→add_track→insert_midi_clip 52 beats→add_midi_notes 42 音小星星→回读核对音符数/音高/起点+music_context bar 守门前置）→ Phase B 一个真实 chat 回合（/agent/chat agent_mode=chat，指令点名工具+clip_id+key 参数，不泄漏黄金 findings）→ Phase C 对 executed_kernel_reply 断言（analyzed_note_count=42；乐句2↔3 调内模进 proto 8→match 16 shift2/steps1/label「模进（调内，上移 2 半音）」/bar 3-4→5-6；乐句3↔4 原样重复 proto 16→match 24 shift0/label「原样重复」/无 sequence_quality/bar 5-6→7-8——与段 1 黄金用例期望一致）
- 命令形态（kernel=wt_228 同款只读复用先例）：

```powershell
powershell -NoProfile -ExecutionPolicy Bypass -File D:\Vit_DAW_wt_276impl\scripts\dev_agent_smoke.ps1 `
  -RepoRoot D:\Vit_DAW_wt_276impl `
  -StartKernel -KernelExe D:\Vit_DAW_wt_228\VitApp\build_release\VitApp_artefacts\Release\VitApp.exe `
  -RestartAgent -NoChatSmoke -NoStripSilenceSmoke -EduTransformSmoke `
  -RunArtifactsDir D:\Vit_DAW_wt_276impl\coord\runs\EDU-SYMBOL-CORE-1\smokerun1
```

（-NoChatSmoke/-NoStripSilenceSmoke 关闭两条默认 LLM 冒测腿省预算；本腿自带 chat 断言）

## §8 口径（跑前成文）

- **运行次数**：单 run 无重跑（含模型随机分支不重跑）；环境中断（端口冲突/进程死亡/loopback 拒绝）有原始退出证据时至多重建栈 1 次（重建后仍算同一烟测腿，run ID 分开记）。
- **预算**：≈¥0.5-2 上界（单 chat 回合+回合内工具往返，flash 引擎按 262/265 同引擎实测约 ¥0.02-0.2/回合，上界留足余量；超上界如实入账上报）。
- **成功条件**：ps1 脚本整程 **exit 0**（=edu_transform_smoke.py exit 0=chat 驱动调用发生+executed_kernel_reply 黄金断言全过）；§5 交付门槛即此。
- **失败分类（预写）**：
  - ① **PASS**：py exit 0——executed 行 tool=midi.analyze_transformations 且黄金断言全过；
  - ② **环境类（py exit 2）**：栈/传输/fixture 失败（端口占用、进程死亡、内核命令失败、fixture 回读不符）→ 有原始证据时记环境中断，至多重建栈 1 次；无证据按断言失败取证上交；
  - ③ **LLM 行为未达（py exit 3）**：模型未调工具/回合停在澄清/按指令仍漏 key（回复 limitations 含「未提供 key 参数」可判别）→ 如实记录模型行为面，不重跑，交主管归类（命令面本身不作判）；
  - ④ **命令面缺陷（py exit 4）**：工具已执行 status=ok 且 key 已传但 findings 与黄金不符（或 bar 锚定错）→ 段 2 验收面出问题=重大，立即停止取证上交（console+chat_response+summary 工件），不重跑。
- **止损线**：任一失败分类命中即停（除环境中断重建 1 次外不重跑）；止损只停止投入，不改写结论。
- **零改动边界**：若发现必须改 Go 代码才能跑通（命令面缺陷）→ 停止实现上交证据（派发令明示）；脚本面小修（非 Go）若必要如实记录后再跑（记为同 run 的脚本卫生修正，py/ps1 语法面已预验证）。

## 占栈协议（§2.2）

- 净空亲核→登记（逐文件 add 提交推 main）→≤60s 复核（fetch 重读+进程/端口即时复核）→启栈
- 释放：run 后 TEARDOWN_ALL_ZERO 亲核（六口 TCP 5555/5556/5557/7878/8787/18787 listen=0 + UDP 4444/4445=0 + VitApp/VitAgent/VspHub/Godot 四类进程=0）→ 释放登记推 main
