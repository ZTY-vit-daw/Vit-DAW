# EDU-ASK-ANCHOR-1：问询锚定——transport_set_loop 注册 + 参照物清单引导 + A/B 兜底编排（乐局 4 / 注册链④）

- 发卡：GLM 主管决策流 / 2026-11-05（依据=[rulings §五-8/15](../../rulings/2026-11-05-EDU-DEMO-SCOPE-RULINGS-1.md) + [256 报告](../../reports/2026-11-05-edu-cap-recon-1.md) 点②/④）
- 派发确认：已确认（用户 2026-11-05 批准入池）
- 验收负责人：GLM 主管决策流
- 池序 282；**注册链第 ④ 位（前置=278 已验收提交）**；P1 / ~1 天；L1 / PC；烟测腿可搭 281/286 便车
- **前置裁定 15（已下，2026-11-08 用户裁定）：问询协议坐标层+回流层落【通用】模型引导段（方案 a）**——用户理由：所有能力都需要坐标层（坐标层=全能力基础设施）；刹车句「问在判定权属用户处，不问模型自己能查到处」随文生效。引导文本落通用段所在文件（以实际代码结构为准），transport_set_loop 注册仍在本卡 catalog/policy 域。

## 目标

1. **transport_set_loop / transport_clear_loop 注册**（内核现成 CommandDispatcher.cpp:3224/3230，agent 零注册=256 缺口①）：catalog spec（参数 start_seconds/end_seconds）+ policy **RiskDirect**（256 验收已裁：传输态不改工程内容）+ 引导一行。
2. **参照物清单引导**（256 点②矩阵落地）：marker（position/range）/循环区/轨道与 clip 名单/transport 秒/music_context（tempo+拍号+tempo_setting_count 守门语义）/分析命令返回的 bar 坐标——写成"可锚定参照物清单"段+**问询守则**（问句指称必须落在可锚定参照物上；用户侧坐标须追问转换；刹车=问在判定权属用户处，rulings §五-15）。
3. **A/B 兜底编排引导**（256 点④兜底路径）：对话内 A/B 描述+seek+set_loop+play 循环段播放的组合编排次序（"先听 A 段再听 B 段"）；渲染主路径（区间渲染+audition mount）**不在本卡**（留乐局后续/产品线）。

## 约束

- 文件域=catalog.go/policy.go/capability_pack.go（引导段）+测试+（如搭车）烟测脚本参数；零内核；不触 M3 域。

## 验收标准

- 注册后 chat 驱动 loop 设置/清除可用（搭 281 或 286 烟测腿断言，或独立轻腿 exit 0）；
- 引导段=纯文本新增，既有 pack 行为零变化；回执附参照物清单全文+问询守则文本+commit hash。

## 停止条件

- 裁定 15 与默认落点冲突→按裁定重落（文本级改动，不阻塞）。

## 领取

- 2026-11-08 GLM L2-worker（ZCode/PC 子代理）领取。worktree `D:/Vit_DAW_wt_282impl`，分支 `port/edu-ask-anchor-1`（自 origin/main 新建）。
- 领取基线：HEAD `00722e1c68190e33b0c2e298bc3981c309c91413`（=origin/main），`git status --short` 空、`git diff --stat` 空——基线全净。
- 在飞卡域核对：310（工件+scripts，agent 源码只读）/296-R3（scripts）/309（C++ 域）与本卡 agent Go 域（catalog.go/policy.go/capability_pack 体系+测试）不相交；308 已合入 main（00722e1c），catalog.go 域已释放。

## 回执

- 2026-11-08 执行完毕。**实现 commit `873c0e9c`**（分支 `port/edu-ask-anchor-1` 已推 origin，不合入 main 归主管；领取后曾并 origin/main coord 侧提交，代码基线=00722e1c）。
- **diff 文件清单**（11 文件，+183/−2；capability_pack.go 零改动=九 pack 行为零变化）：
  - `agent/internal/tools/catalog.go` — transport_set_loop/transport_clear_loop spec（RiskDirect/不 mutates/不确认；参数 argHint start_seconds:end_seconds 秒域）
  - `agent/internal/policy/policy.go` — directCommands 兜底两行（主分类源=spec RiskDirect）
  - `agent/internal/chat/goalrunner_chat.go` — agentLoopTransportTools 加 transport.set_loop/clear_loop（chat 能力门控第四注册面，276 EDU-SYMBOL-CORE-1 同型；**注**：此文件在卡面文件域字面之外，但属"注册"的必要面——缺行则 transport 能力轮对模型遮蔽工具、chat 驱动不可达，领取节未预列由本回执显式申报，请主管裁定）
  - `agent/rules/ruleset/resources/sections/shared.discipline.ask_anchors.txt` — **通用引导段**（裁定 15 方案 a；落点=ruleset shared 段体系，precedent=shared.discipline.evidence_refs 跨族段，chat+neutral_family 双面渲染；非新建结构）
  - `agent/rules/ruleset/resources/manifest.json` + `agent/rules/ruleset/ruleset.go` — 段登记+SectionAskAnchorDiscipline 常量+postMigrationSectionIDs+RulesetVersion v2→v3
  - 测试五件：catalog_test/policy_test/ruleset_test/chat ruleset_parity_test/agentloop ruleset_parity_test
- **测试**（零栈单测，全部退出码 0）：`go build ./...`=0；`go test ./rules/... ./internal/tools/ ./internal/policy/`；`go test ./internal/chat/`（全包 176s）；`go test ./internal/agentloop/`；`go test ./... -count=1` 全量两轮均 0（无 run ID——零栈无工件目录义务）。
- **参照物清单全文**（引导段第一条）：
  > Anchorable references are the model-side coordinate system for describing, asking about, and acting on timeline material: timeline markers (position markers by start_seconds, range markers by start_seconds + end_seconds; read project.markers.list, write project.markers.upsert), the transport loop region (transport.set_loop with start_seconds/end_seconds sets the region, enables looping, and moves the playhead to start; transport.clear_loop disables it), track and clip identities (tracks[]/clips[] from project.state: stable id, name, start_seconds, length_seconds), transport time in seconds (transport.seek/play/stop), and the music_context block from project.state (tempo_bpm, time_signature, time_signature_numerator/denominator, tempo_setting_count).
- **问询守则文本**（引导段第二至四条）：
  > Bar (measure) coordinates are usable only as returned by analysis commands (for example midi.analyze_transformations findings bar_start/bar_end with music_context_echo), and only while music_context.tempo_setting_count is 1 with a disclosed time signature. When tempo_setting_count is greater than 1 or unknown, do not compute or offer bar coordinates yourself; fall back to seconds or markers.
  > Inquiry rule: every referent in a question you ask the user must land on an anchorable reference — a named marker, a track/clip name, the loop region, or explicit seconds (or command-returned bars). User-side coordinates such as mouse position, screen position, or "where I clicked" are not inquiry coordinates; ask the user to convert them to an anchorable reference before acting.
  > Brake rule: ask where the judgment authority belongs to the user (what their ear hears, taste, intent, which version they prefer); do not ask about facts you can look up yourself with read-only tools. Look it up first, then ask only what the user can decide.
- **A/B 兜底编排**（引导段第五条）：描述段用可锚定坐标 → transport.set_loop(A 段)+play（循环）→ set_loop(B 段)（playhead 跳 B）→ 用户耳判「listen to A first, then B」→ clear_loop 收尾；seek 补充定位；渲染/audition 主路径在场时优先，此为对话内兜底（渲染主路径不在本卡）。
- **既有行为零变化证据**：chat/agentloop 两侧 fallback parity 测试（embed 渲染减 postMigration 段 == legacy 模板）全绿=既有段字节零改写；capability_pack.go 零 diff；九 pack 与 S5 seam 面未触碰。
- **端测边界声明（AGENTS §5）**：零栈腿=单测级。真栈轻腿（chat 驱动 loop 设置/清除断言）**候栈**——不自行起栈，候主管排（卡面预排 310/296-R3/309 腿后、今晚手测前；296-R3 已登记占栈知悉）。已知覆盖边界两处申报：①普通族 ReAct 骨架（message_loop.go ordinary family skeleton，独立 seam 仪器化文本面）未含问询锚定段——该面工具可达（AllowedTools+目录行），引导文本仅 chat L1/中性族两 ruleset 面；如需覆盖另开小卡。②乐局纯 midi 关键词轮次（无 播放/听 等词）不披露 transport 工具（能力门控分面既有行为）——A/B 编排轮次实际含 播放/听 词或无能力匹配走全目录，工具可达。

## 验收

- （主管回填）

### 验收（实现腿，主管，2026-11-08 晚窗）

- **pass 轻验（终验候真栈轻腿）**——主管亲读核心交付件：①引导段全文=裁定 15 方案 a 三要素齐（参照物清单[marker/循环区/轨道 clip/transport 秒/music_context]+问询守则[指称落锚定物/用户侧坐标追问转换]+刹车句[问在判定权属用户处，不问模型自己能查到处]），bar 坐标守门（tempo_setting_count==1+披露拍号才可用，否则回落秒/marker）=§五-8+256 定案忠实落地，A/B 兜底编排含渲染主路径优先声明；②落点=ruleset shared 段体系（evidence_refs 跨族段先例复用，非新建结构）+RulesetVersion v2→v3+parity 测试（embed 减 postMigration==legacy 模板全绿，§11 持久化兼容口径履行）；③注册四面齐（catalog spec RiskDirect+policy+chat 门控+自动披露），描述诚实（"Transport state only"）。
- **域外申报裁定：goalrunner_chat.go +4 行批准**（agentLoopTransportTools 白名单补 set_loop/clear_loop=276 第四注册面同型，缺行则工具对模型遮蔽、卡面"注册"目标不可达——扩域追认，先例一致）。
- **两处覆盖边界记录接受**：普通族 ReAct 骨架面不含引导段（工具仍可达）；纯 midi 关键词轮次不披露 transport 工具（既有能力分面行为）。
- 真栈轻腿（chat 驱动 loop 设置/清除断言）候栈——排 310/296-R3/309 腿后、今晚手测前由主管排栈；候栈期间卡留 doing。

### 真栈轻腿回执（2026-11-09 晚窗，ZCode L2 执行子代理）

- **结论：卡面验收标准①「chat 驱动 loop 设置/清除可用」真栈兑现 exit 0（单 run 无重跑）**；标准②既有行为零变化以阶段 A 冒测不回退兑现（parity 证据在实现腿回执）。
- **领取（栈腿）**：worktree `D:/Vit_DAW_wt_282impl` 续用（实现分支 `port/edu-ask-anchor-1` @ `873c0e9c`，栈腿工件追加 commit `19ced01d`）；coord 协调分支 `coord/282-stackleg`（占用登记 `513c7aab` 推 main 成功+远端 owner 重读确认）。
- **栈协议两条登记**：占用 19:41（净空亲核 19:39:04 六 TCP+两 UDP+五类进程全零；≤60s 复核 19:40:10 全零）/释放 19:43（TEARDOWN_ALL_ZERO 亲核 19:43:26，killed VitAgent=8868/VitApp=31840）——见 `coord/resources/PC-RUNTIME-STACK.md` 顶部两行。
- **完整命令+退出码**：
  - 阶段 A：`powershell -NoProfile -ExecutionPolicy Bypass -File D:\Vit_DAW_wt_282impl\scripts\dev_agent_smoke.ps1 -RepoRoot D:/Vit_DAW_wt_282impl -StartKernel -RestartAgent -KernelExe D:\Vit_DAW_wt_228\VitApp\build_release\VitApp_artefacts\Release\VitApp.exe` → **exit 0**（agent 现场构建 873c0e9c；kernel=wt_228 F9E3DA0A66263F0B… 只读复用，零内核改动）。
  - 阶段 B：`python transport_loop_leg.py --agent-http http://127.0.0.1:7878 --kernel-zmq tcp://127.0.0.1:5555 --run-root .`（脚本在 run 工件目录）→ **exit 0**。
- **run 工件目录**：`coord/runs/EDU-ASK-ANCHOR-1/stackleg_20261109/`（RUNBOOK §8 冻结+驱动脚本+两阶段 console+set/clear chat 响应与事件流+summary；随 `port/edu-ask-anchor-1@19ced01d` 推送，分支在卷可核）。
- **断言细节（三层）**：①chat 轨迹层——set 轮 daw_action item.completed `tool=transport.set_loop / command_name=transport_set_loop`、clear 轮 `transport_clear_loop`（真实工具步非 turn body 回显，278 run6 同型判定面）；②工具回执层——VSP 回执（set: tx_1794224529334928900_100 loop_enabled=true loop=[10,15] position=10；clear: tx_1794224535909537700_104 loop_enabled=false）；③内核独立读面——chat 后经内核 ZMQ `stop`（幂等）读 buildTransportReply：set 后 loop_enabled=true loop=[10.0,15.0] 与回执逐字段一致、clear 后 loop_enabled=false。两轮 goal=completed/stop=done、requires_confirmation=false（RiskDirect 免确认=注册语义）、零治理确认 hop（r2 兜底未触发）。参数精确 10/15（软记录）。
- **既有行为不回退**：阶段 A 默认腿 exit 0（ChatSmoke 路由不劫持 ok+no-pending confirm ok+strip 工具面 ok）；strip chat 确认链未完成与 suggest pending 数两处 Fail-Or-Warn（非 Strict 警告不挡 exit 0）如实记录——域属模型行为面，与本卡改动（transport 注册+ruleset 新段）无交集。
- **LLM 计费（§8）**：telemetry 14 行 deepseek-flash 零 err（阶段 A 6+阶段 B 8；每 chat 轮含入口/loop/收口多次调用，行数超 RUNBOOK「≤8 调用」申报粒度如实注记）；in 120,644/out 7,485，计费 ≈¥0.02-0.05 ≪ ≈¥0.1 上界。
- **端测边界声明（AGENTS §5）**：本腿断言面=真实栈 agent HTTP（/agent/chat、/agent/events）+内核 ZMQ 命令面——传输态断言全部落在服务器/内核权威面；**渲染面（webui/Godot GUI）与用户旅程不在断言面**（282 改动无渲染面，E2E-WEBUI-1/JOURNEY-1 两层门槛不适用）；覆盖边界②按申报应对（话术含「循环播放」激活 transport 能力面，工具披露亲证——set 轮首步即 set_loop 调用）。
- 零源码改动（本腿仅新增 run 工件目录内驱动脚本+RUNBOOK，agent/VitApp/scripts 零触碰）；done=自验候主管终验，不合入 main 归主管（转交提示词：通过后主管验收→合入 main，commit message 含卡 ID 282）。
