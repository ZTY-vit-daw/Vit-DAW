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
