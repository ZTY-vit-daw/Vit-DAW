# ARR-SRCLOAD-1 段 2 真栈烟测腿 smokerun1 — §8 跑前口径（跑前成文，2026-11-07）

- 执行：ZCode GLM-5.3 L2 执行子代理（277impl 段 2）；worktree `D:/Vit_DAW_wt_277impl` @ `port/arr-srcload-1`（origin/main `37fd7c97` 已合入为 `df36c850`，零冲突如实记录；合并后 `go build ./...`=0 + `go test ./internal/chat -count=1`=ok(119.5s) + tools/policy=ok 方上栈）
- 被测改动：段 1 实现（arrangement pack+注册链②，impl `965046f0`）+ 本段两件——①已授权域外微补丁：`agent/internal/workflows/plugingrabber/commands.go` hasInstrumentPluginSubject 补 钢琴/piano 单独词（一行级，配对单测 `commands_test.go` TestInstrumentSubjectIncludesPiano 正负例钉）；②冒测扩展：`scripts/arr_srcload_smoke.py` 新建（断言体）+ `scripts/dev_agent_smoke.ps1` 加参 `-ArrSrcLoadSmoke`（参数声明/隔离条件列/telemetry env/腿块四处，透传缝预声明，零 Go 生产码改动——`--kernel-sub` 透传 5556 供 render_done 订阅）
- 被测 agent=worktree 现场构建（ps1 -RestartAgent 构建 `agent\bin\VitAgent.exe`，277 段 1+微补丁在 agent 面），构建后记 SHA256；kernel=wt_228 VitApp.exe SHA256 `F9E3DA0A66263F0B6E3FED5CDA0FB214C460FCE8053234240C470D515ED0D67A`（266/276 同款只读复用，零内核改动面）；引擎=本机默认无注入（RUNBOOK 申报同 276 形态）；预期 LLM 参与=1 个 chat 回合（含回合内工具往返），另有 /agent/confirm 一次（确认面无 LLM）

## 场景与命令

- 断言体：`scripts/arr_srcload_smoke.py`（退出码 0/2/3/4 预声明）——Phase A 环境预检+fixture（内核 ZMQ：plugin_list_available 断言 is_instrument 计数>0【==0=卡面停止条件环境前置上交】→new_project→save_as_project 落 run 工件目录→add_track 单轨）→ Phase B 一个真实 chat 回合（/agent/chat agent_mode=chat，编曲话术「给这条 MIDI 轨加一个钢琴音源」；装载 RiskConfirm 门控→驱动器以 /agent/confirm decision=approve 批准一次；路由证据=capability 轮 vs plugin_grabber 工作流如实记录不硬断言——两通道均为合法 chat 驱动装载）→ Phase B2 executed rack_add_node 断言（status=ok + **zone_id=="Z2"** + plugin_instance_ready 记录）→ Phase C 内核 ZMQ 确定性链（insert_midi_clip 16 beats+add_midi_notes 16 音 C 大调琶音+回读核对→get_project_state 轨上插件含已装载件→start_render 16-bit WAV 落 run 工件目录→SUB 5556 render/render_done 等完成【文件面兜底】→**WAV 非静音断言** peak_abs>500/32768）
- 命令形态（kernel=wt_228 同款只读复用先例）：

```powershell
powershell -NoProfile -ExecutionPolicy Bypass -File D:\Vit_DAW_wt_277impl\scripts\dev_agent_smoke.ps1 `
  -RepoRoot D:\Vit_DAW_wt_277impl `
  -StartKernel -KernelExe D:\Vit_DAW_wt_228\VitApp\build_release\VitApp_artefacts\Release\VitApp.exe `
  -RestartAgent -NoChatSmoke -NoStripSilenceSmoke -ArrSrcLoadSmoke `
  -RunArtifactsDir D:\Vit_DAW_wt_277impl\coord\runs\ARR-SRCLOAD-1\smokerun1
```

（-NoChatSmoke/-NoStripSilenceSmoke 关闭两条默认 LLM 冒测腿省预算；本腿自带 chat 断言）

## §8 口径（跑前成文）

- **运行次数**：单 run 无重跑（含模型随机分支不重跑）；环境中断（端口冲突/进程死亡/loopback 拒绝）有原始退出证据时至多重建栈 1 次（重建后仍算同一烟测腿，run ID 分开记）。
- **预算**：≈¥1-3 上界（单 chat 回合+回合内工具往返+确认执行，flash 引擎按 262/265/266/276 同引擎实测约 ¥0.02-0.3/回合，上界留足余量；超上界如实入账上报）。
- **成功条件**：ps1 脚本整程 **exit 0**（=arr_srcload_smoke.py exit 0=chat 驱动装载 executed zone Z2+clip 写入+离线渲染非静音全过）；§5 交付门槛即此。
- **失败分类（预写，装载链各环分列）**：
  - ① **PASS**：py exit 0——chat 驱动 rack_add_node status=ok+zone Z2、轨上插件可见、clip 16 音回读一致、渲染产物 peak_abs>500；
  - ② **环境类（py exit 2）**：栈/传输/fixture/渲染产物缺失失败（端口占用、进程死亡、内核命令失败、fixture 回读不符）+**环境前置**（预检 is_instrument==0——卡面停止条件，环境前置上交不视为代码障碍）→ 有原始证据时记环境中断，至多重建栈 1 次；无证据按断言失败取证上交；
  - ③ **LLM 行为未达（py exit 3）**：模型未发起装载（无 plan_id 无 executed rack_add_node 行；澄清停泊/无关动作）→ 如实记录模型行为面，不重跑，交主管归类（装载通道本身不作判）；
  - ④ **装载链缺陷（py exit 4）**：装载已执行但链与设计矛盾——确认后 status≠ok / **zone_id≠Z2** / 轨上插件不可见 / **instrument+clip 在场仍渲染静音或 no_renderable_audio_content 拒绝** / render_failed → 停止取证上交（console+chat/confirm response+summary 工件），不重跑（派发令明示：装载链缺陷发现=停止取证上交）。
- **止损线**：任一失败分类命中即停（除环境中断重建 1 次外不重跑）；止损只停止投入，不改写结论。
- **零改动边界**：若发现必须改 Go 生产码才能跑通（装载链缺陷）→ 停止实现上交证据；脚本面小修（非 Go）若必要如实记录后再跑（记为同 run 的脚本卫生修正，py/ps1 语法面已预验证）。

## 占栈协议（§2.2）

- 净空亲核→登记（逐文件 add 提交推 main）→≤60s 复核（fetch 重读+进程/端口即时复核）→启栈
- **281 候栈让位记录**：281（EDU-LEVEL-RUNTIME-B 段 1）候栈在先（`c11ac302`），主管派发令明示 277 先行占栈；本腿释放登记后 281 可登记。
- 释放：run 后 TEARDOWN_ALL_ZERO 亲核（六口 TCP 5555/5556/5557/7878/8787/18787 listen=0 + UDP 4444/4445=0 + VitApp/VitAgent/VspHub/Godot 四类进程=0 + smoke python=0）→ 释放登记推 main
