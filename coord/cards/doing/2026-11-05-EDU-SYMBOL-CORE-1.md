# EDU-SYMBOL-CORE-1：符号分析引擎——变换族检测核 + analyze_midi_transformations 命令（乐局 1a / 注册链①）

- 发卡：GLM 主管决策流 / 2026-11-05（依据=[rulings §五-9/§六](../../rulings/2026-11-05-EDU-DEMO-SCOPE-RULINGS-1.md) + [256 报告](../../reports/2026-11-05-edu-cap-recon-1.md) 点①草案 + [执行排序](../../reports/2026-11-05-nov-three-line-execution-plan.md)）
- 派发确认：已确认（用户 2026-11-05 批准入池，明日按序执行）
- 验收负责人：GLM 主管决策流
- 池序 276；**注册链第 ① 位**（动 catalog.go/policy.go/capability_pack.go；本卡提交前 277 不得领取）；目标=`agent/internal/edu/`（新建包）+三处注册；P1 / 2.5-4 天 / 依赖=无；L1-L2 / PC 执行会话；真栈烟测腿一次

## 目标

1. **变换族检测核**（`edu/transform.go` 或同包）：音程序列提取（pitch→半音差）+ 四变换统一检测（repeat/sequence/inversion/retrograde=音程序列在变换下的相等性）+ 节奏比对；确定性纯函数，同输入同输出。
2. **命令面**：`analyze_midi_transformations`（工具名 `midi.analyze_transformations`，RiskDirect 只读）——输入/返回 schema 按 256 报告点①草案（clip_id / range_beats / transformations / min_match_notes；返回 bar 锚定 findings + music_context_echo + llm_context 字段族借 EPM 惯例）；**bar 守门**：`music_context.tempo_setting_count==1` 才出 bar 字段，否则 null+limitations。
3. **判定器**：学生片段 vs 原型比较（错音定位=逐音程序列 diff），同核复用。
4. **三处注册**：catalog spec 行 + policy directCommands + capability_pack（midi pack Tools 补一行；详细引导归 282，本卡只注册不加引导段）。
5. **黄金用例单测**：小星星主题（A 段后半↔B 段调内模进+B 段两遍原样重复）——检出两关系、区分 repeat/sequence、shift 值正确；负例（无关系→空 findings）；边界例（同节奏异轮廓不误报）。fixture 新建测试资产，不碰样例工程（§10）。

## 约束

- 文件域=`agent/internal/edu/`（新建）+ catalog.go/policy.go/capability_pack.go 各一处行级注册 + 测试；**零内核改动**；不触 semantic_entry/automation（M3 域）。
- JSON 返回字段名与 256 报告草案一致（消费方=乐局卡/编曲 SYMLINK）。

## 验收标准

- `go build ./...` + `go test ./internal/edu ./internal/tools ./internal/policy -count=1` 全绿；既有测试零回退；
- **真栈烟测腿**（§5）：chat 驱动调命令→返回断言（小星星 fixture 导入后分析，findings 与单测一致），扩现有冒测脚本，**脚本 exit 0**；
- 回执附：注册三处 diff 摘要、单测清单、烟测 run ID 与工件路径、commit hash。

## 停止条件

- 检测核对黄金用例反复不过→保存失败输出上交，不硬编码曲目特例（§10）；栈/环境中断按 §9 取证。

## 领取

- 2026-11-06 / **段 1/N 领取**（变换族检测核+判定器+黄金用例单测，全新包 `agent/internal/edu/`；段 1 不做三处注册/命令面/真栈烟测）
- origin/main hash：`84f86f9e01b9cef764e5ae8fce6060fe0a8345b3`
- owner：GLM 主会话 L2-worker 子代理（GLM-5.3 / PC）
- worktree：`D:/Vit_DAW_wt_276impl`；分支：`port/edu-symbol-core-1`（自 origin/main 新建）
- 领取基线：HEAD=`84f86f9e01b9cef764e5ae8fce6060fe0a8345b3`；`git status --short`=空；`git diff --stat`=空（worktree 全净，无领取前已有 diff）

## 回执

- **段 1/N 回执（2026-11-06）**：变换族检测核+判定器+黄金用例单测完成，全绿。
- 实现 commit：`20d4115d`（分支 `port/edu-symbol-core-1`，worktree `D:/Vit_DAW_wt_276impl`）；领取提交 `d4acae60`（已推共享 main）。
- **diff 文件清单**（全部为新建，零既有文件改动）：
  - `agent/internal/edu/edu.go`（139 行）——包契约+Note/Key 类型+音程序列提取/节奏比对/大调音级映射
  - `agent/internal/edu/transform.go`（356 行）——检测核 DetectTransformations：四变换统一检测（repeat / sequence[chromatic|diatonic] / inversion / retrograde=音程序列在变换下的相等性）+节奏比对（identity 逐位相等、retrograde 镜像）+极大匹配过滤+退化优先级链（identity>inversion>retrograde）+确定性排序
  - `agent/internal/edu/judge.go`（102 行）——判定器 JudgeAgainstPrototype：学生片段 vs 原型逐音程序列 diff 错音定位（移位假设换算期望值、恢复点抑制），同核复用
  - `agent/internal/edu/transform_test.go`（408 行）/ `agent/internal/edu/judge_test.go`（159 行）——19 用例
- **黄金用例**（fixture=测试内联 42 音小星星，C 大调 4/4，未触碰样例工程）：乐句2↔乐句3 调内模进检出（DiatonicSteps=+1、ShiftSemitones=2）且与乐句3↔乐句4 原样重复（Kind=repeat、shift=0）严格区分；A 段整体再现（14 音 repeat）附带检出；负例（无关系→空 findings）、边界例（同节奏异轮廓纯调内序列→空）、chromatic/inversion/retrograde 正例、同音退化只报 repeat、MinMatchNotes 阈值、乱序输入/重复调用确定性、无 Key 时 diatonic 不报、变换白名单、非法选项 fail-closed 均覆盖。
- **测试命令与退出码**（`agent/` 下）：`go build ./...`=0；`go test ./internal/edu -count=1`=0（19/19 PASS）；全量 `go test ./...`=0（零回退）；`go vet ./internal/edu`+`gofmt` 干净。
- 设计决策存档：sequence 双语义（chromatic=半音恒定移调 / diatonic=大调音级恒定模进，后者需 Key——小星星 A 段后半↔B 段为调内模进，半音差 [2,1,2,2] 非恒定，纯 chromatic 检不出，故引入 diatonic 档）；ShiftSemitones=首音半音差（段 2 命令面 label 用）；段对约定=不相交且匹配段在后；O(N²·L) 复杂度，长 clip 由段 2 range_beats 分窗。
- 段 1 未做（按派发指令留守段 2/3）：catalog.go/policy.go/capability_pack.go 三处注册、analyze_midi_transformations 命令面、真栈烟测；零内核改动、零栈占用（无 PC-RUNTIME-STACK 登记需求）。
- **段 2/N 回执（2026-11-06）**：命令面+三处注册完成，全绿。
- **main 合入记录**：领取后 `git fetch origin`，`origin/main` 领取基线 `84f86f9e` 之后新增 18 提交（264/265/258 多卡：chat 包 d2_stereo_check_runtime 等、scripts/dev_agent_smoke.ps1、coord 工件），`git merge origin/main` 进本分支=**零冲突**（本分支文件域 edu 新包+卡面与 main 改动不相交；卡面段 1 回执经 cherry-pick 内容一致自动合并），merge commit `60d5a165`；合并后 `go build ./...`=0 + `go test ./internal/edu`=0 先行验证再开工。
- 实现 commit：`15276ca4`（分支 `port/edu-symbol-core-1`，worktree `D:/Vit_DAW_wt_276impl`）。
- **diff 文件清单**（8 文件 +1419 行：2 新建 + 6 行级改动，零内核）：
  - `agent/internal/edu/command.go`（新，690 行）——命令面全逻辑：fail-closed 参数解析（clip_id 必填 / range_beats=[start,end) 半开窗 1e-6 容差 / transformations 白名单 fail-closed / min_match_notes>=2 且 0=默认 3 / key{tonic:0-11, mode:"major"|省略}）；数据取三条既有内核只读命令（get_project_state→music_context、get_midi_clip_data→clip 全局起点秒×bpm/60、get_midi_clip_notes→notes）；接线段 1 检测核；**bar 守门**（tempo_setting_count==1 且 tempo_bpm/拍号可用且 clip 起点可得才出 bar_start/bar_end；多段 tempo、多段拍号 time_sig_setting_count>1、music_context 缺失、tempo 未披露、clip 起点不可得五种降级态均 null+limitations 披露具体原因）；返回 schema=256 草案点①+段 1 追认：findings 含 sequence_quality(chromatic|diatonic)/中文 label（"模进（调内，上移 2 半音）"）/segments[prototype|match]{start_beats,length_beats,bar_start,bar_end,note_ids 证据回指}/interval_pattern/shift_semitones/diatonic_steps/inversion_axis_semitones + confidence 双字段（high/0.99，检测核只产精确匹配）+requires_confirmation 恒 false；llm_context 借 EPM LLMContext 字段族（summary_md/compact_facts/do_not_include_raw_package:true/evidence_refs=clip:<id>/notes:…/limitation_notes/suggested_next_step）；music_context_echo 回显换算依据（指针 null=未披露）；analysis_id=输入规范化 sha256 稳定哈希（确定性，供 trajectory/课时卡回指）；辅助命令（state/clip_data）失败只降级 bar 不致命，notes 命令失败 fail-closed 报错
  - `agent/internal/edu/command_test.go`（新，654 行）——9 顶层用例（33 含子测试）：黄金接线（findings 与 DetectTransformations 直调逐条对齐+乐句2↔3 模进 bar 3-4→5-6+乐句3↔4 repeat bar 5-6→7-8 严格区分）、bar 守门五态（可用/多段 tempo/缺 music_context/多段拍号/clip 起点失败）、clip 起点非零换算（7.2s@100bpm→12 beats→bar 偏移）、schema 字段逐键核对（顶层/echo/finding/segment/llm_context 与草案+追认一致）、参数 fail-closed 15 例、内核回复 fail-closed 5 例（notes 失败致命/state 失败降级）、range 分窗不变式（[0,32)=28 音、越窗 finding 不报、A 段再现被窗截断、窗口内黄金保留、空窗口空 findings+披露）、选项透传（白名单/min=14 只剩 A 段再现/无 key 不报 diatonic+披露）、确定性（重复调用逐字节一致）
  - `agent/internal/tools/catalog.go`（+3 行）——defaultSpecs spec 行（`analyze_midi_transformations`/`midi.analyze_transformations`/midi 类/RiskDirect/required clip_id）+argHint 输入披露行（256 草案 usage 惯例）
  - `agent/internal/policy/policy.go`（+1 行）——directCommands 加 `analyze_midi_transformations: true`（只读 direct 级）
  - `agent/internal/tools/capability_pack.go`（+1 行）——midi pack Tools 加 `midi.analyze_transformations`（只注册不加引导段，引导归 282）
  - `agent/internal/harness/harness.go`（+14 行）——invokeLocal 一处挂线（256 草案指定 agent-local 先例 mix.propose_tick 同款）：kernel send 闭包转接 edu.AnalyzeTransformationsCommand。**域内说明**：派发指令文件域字面为"edu 新文件+三处行级注册"，harness.go 挂线是草案点①明示的"handler 挂法对照既有 agent-local 命令"的必要落点（无此 case 命令落入内核路径必失败，段 3 chat 驱动烟测无法通过）——一处 case+一处 import，最小挂线，未动 harness 其余任何逻辑
  - `agent/internal/tools/catalog_test.go`（+35 行）/`agent/internal/policy/policy_test.go`（+21 行）——注册断言（spec 元数据/argHint 关键词/midi pack Tools 含新工具；policy 两条路径均 direct 且 directCommands 兜底在卷）
- **测试命令与退出码**（`agent/` 下）：`go build ./...`=0；`go test ./internal/edu ./internal/tools ./internal/policy -count=1`=0（edu 52 PASS 含子测试=段 1 19+段 2 33、tools 33、policy 7）；全量 `go test ./... -count=1`=0（零回退）；`go vet ./internal/edu ./internal/tools ./internal/policy`=0（harness 包 vet 仅有 main 已存在的 shm_windows.go unsafe.Pointer 提示，非本卡改动）；gofmt 干净（mod EOL——本机 core.autocrlf 下全仓 Go 文件均以 LF 为 gofmt 基准核过）。
- 段 2 设计决策存档：bar 守门在派发指定 tempo_setting_count==1 之上加同族两条件（拍号可用、time_sig_setting_count<=1、clip 起点可得）——守门目的是"换算依据齐全才出 bar"，多段拍号/缺起点下出 bar 即造假；range_beats 取半开窗 [start,end)（beat=end 的音符属下一窗口，乐句切分语义自然）；key 只显式传参不自动读工程 key_signature（显式优先，模型可从 music_context_echo.key_signature 获知后传参——自动回退留乐局卡裁决）；evidence_refs 双段形态 `clip:<id>/notes:<proto首-末>+<match首-末>`（段 1 检测核对全量短匹配不剪枝，min_match_notes 是调用方控噪手段）。
- 段 2 未做（按派发留守段 3）：真栈烟测腿（chat 驱动调命令→断言，扩现有冒测脚本）——就绪态：命令面已全链路可调（catalog→policy direct→harness invokeLocal→edu→内核三条只读命令），待主管调度栈窗口；零栈占用、零 LLM 调用（本段无 PC-RUNTIME-STACK 登记需求）。

## 验收

- **段 1 验收（2026-11-06 晚窗，主管）：pass**——5 新文件 +1164 行零既有改动零内核零栈；主管交叉复跑 wt_276impl `go test ./internal/edu -count=1` ok+vet 干净；实现抽查合格（fail-closed 错误族+零值选项语义+JSON 字段对齐 256 草案+仓库注释风格一致）。**sequence 双语义设计决策（chromatic/diatonic）追认**：小星星 A↔B 调内模进半音差非恒定，纯 chromatic 检不出——引入 diatonic 档理由充分，`sequence_quality` 字段入段 2 schema 定稿；两处 fixture/断言自查修正（边界例真倒影+chromatic 断言过强）诚实且人工复核在案。**段 2/3 未发**（按用户边界：265 检查过后放行自主派发），就绪态=命令面 schema 可直接定稿。
- **段 2 验收（2026-11-06 深夜，主管）：pass**——8 文件 +1419 行（命令面 690+测试 654+三处注册行级+harness 挂线 14 行）；主管交叉复跑 edu/tools/policy 三包全绿+全量零回退（执行侧）；main 合入 60d5a165 零冲突。**harness.go 挂线追认**（域内说明预声明）：256 草案点①明示 agent-local 挂法必要落点（mix.propose_tick 先例同款，无此 case 命令落内核路径失败）——最小 case+import；教训=主管派发段 2 时未预写该挂点（263 立规「执行必需挂点卡面预写」的执行侧反向兜底成功）。注册链①就位：277（ARR-SRCLOAD）解锁待领。
- （整卡验收：段 3 完成后主管回填）
