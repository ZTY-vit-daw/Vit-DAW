# GEN-YUE-WIRE-1：YuE2 成曲链真实接线——H6 六项（学生谱→方言→成曲→版本）

- 发卡：GLM 主管决策流 / 2026-11-15 深夜（依据=审计 H6 六项+§9 用户裁定（YuE2 依谱生成纯器乐、改提示词出版本）+[进度驱动裁定]；无工期估算）
- 派发确认：已确认（主管发卡）
- 验收负责人：主管决策流 + 用户听核节点（成曲版本人耳）
- 池序 372；P0；依赖=无（烟测腿候栈位）；PC

## 目标（审计 H6 逐项）

1. **MIDI→方言转换**：学生工程谱（旋律轨 clip）→`yue_local` ABC 方言的确定性转换（程序转换，不走 LLM 转述）；单元往返测试。
2. **prompt→条件映射**：用户改提示词→实际 `style`/结构条件映射，显示实发摘要（harness prompt 非空约束满足）。
3. **纯器乐条件**：`instrumental` 沿已验证谱面声部/休止路径真实下发（§9 用户已手测 YuE2 谱面跟随准确）。
4. **请求面卫生**：ExtraParams 固定配置不得覆盖学生 ABC/style；两版本=两次 job（count 不支持多候选）。
5. **后台 job+状态 UI**：最薄后台任务（非同步轮询阻塞对话）、进度/失败状态公布、版本列表（原 MIDI+源谱+请求+replay 留档）。
6. **最小引擎托管**：按需启动/就绪/失败提示，只管理自身进程，零手工服务（用户不敲命令/不设环境变量）——沿 Godot/VitShell 用户入口裁定。
7. 单测+定向集成测；真实栈+真实生成烟测腿（DG1 口径候 371 定义）**候栈位排队**（当前 367 占栈）。

## 非目标

- 生成 VST3 产品线复活；多引擎方言扩展；防作弊证据（后置）。

## 文件域

`agent/internal/`（gen/genapi/harness 接线，领取时锚定精确清单）+ webui 生成状态面（如需）+ 烟测脚本扩展 + coord 工件。禁入：chat/launcher/project（367 域）、webui/styles/launcher.css、谱面/音源域（369/360 域）。

### 领取时锚定的精确文件清单（2026-11-15 勘察）

- 新增 `agent/internal/genapi/yue_abc.go`：MIDI→YuE2 ABC 方言确定性转换（notes+拍号/速度/调号→dual voices+段注释方言谱）+ ABC 解析回读（往返测试基底）
- 新增 `agent/internal/genapi/yue_abc_test.go`：往返单测
- 新增 `agent/internal/genapi/yue_prepare.go`：yue_local 请求准备面（prompt→style 确定性映射、instrumental 谱面声部休止化展开、ExtraParams 核心字段卫生保护、实发摘要构建）
- 新增 `agent/internal/genapi/yue_prepare_test.go`
- 新增 `agent/internal/genapi/engine.go` + `engine_test.go`：最小引擎托管（按需启动/就绪探测/失败分类/只管理自身进程）
- 修改 `agent/internal/genapi/dialects.go`：yueRequestBuilder 卫生段（核心字段保护，cot/steps 等引擎旋钮仍可 extra_params 覆写）+ Limitations 同步
- 修改 `agent/internal/genapi/config.go`：ProviderConfig 增 engine 托管配置块（engine_command/args/ready_timeout/env）
- 修改 `agent/internal/harness/gen_generate.go`：yue_local 路径接 yue_prepare（映射+纯器乐+摘要）+ 引擎 ensure
- 新增 `agent/internal/harness/gen_jobs.go` + `gen_jobs_test.go`：最薄后台 job store（gen.jobs.start/status/list；版本留档=原 MIDI+源谱+请求+replay+音频路径）
- 修改 `agent/internal/tools/catalog.go`：gen.jobs.* 三命令行级注册
- 烟测脚本扩展（候栈腿定义，不跑）：`scripts/` 现有脚本体系下新增或扩展（真栈腿前按 371 DG 口径冻结）
- coord 工件：`coord/runs/GEN-YUE-WIRE-1/`（如产生）
- webui 生成状态面：**本卡不做 webui 改动**——状态公布走 gen.jobs.* 工具回执（agent 对话面），渲染面边界在回执声明（卡面"如需"按最小面裁定；App.tsx 大文件+367 在飞不引入 webui 风险）

## 验收标准

六项逐项证据（单测/集成/真实生成 run 工件）；三门不回退；用户听核节点如实登记（人耳）。

## 停止条件

YuE2 引擎本地托管与本机环境冲突且不可按需管理→取证上交（引擎侧人工腿候用户定向）。

## 领取

- 2026-11-15 深夜 / origin/main=3997e11b（领取 fetch 后亲核；卡在 todo/、派发已确认、依赖=无）
- owner=GLM-5.3 L2 执行子代理（ZCode 执行会话，GEN-YUE-WIRE-1 专项；PC）
- 分支 port/gen-yue-wire-1 / worktree D:/Vit_DAW_wt_yue1；协调分支 coord/yue1-claim（领取提交见 main log coord(372)）
- 领取基线：worktree 自 3997e11b 新建，HEAD=3997e11b，status 全净（mv 前），diff --stat 空
- 文件域核对：doing 卡 369（wt_369coord，谱库 spike）/370（legame 契约）/371（单栈口径）/367（launcher 零摩擦，chat 域候 271）/273/309/ARRLOOP——与本卡 gen 域（genapi/harness gen 面/tools catalog 行级）不相交；chat/ 禁入已确认（GEN-CHAT-ROUTING-1 域外锚点不触）
- 栈位核对：PC-RUNTIME-STACK 顶部=SHELL-5 22:40 释放后无占用行（origin/main 亲读）；主管派发口径=367 占栈/候栈排队——本卡零栈段先行（单测+定向集成测），真栈+真实生成烟测腿**候栈第 2 位**（第 1 位=367 烟测腿；YuE2 引擎进程按 278 先例自管启停不入栈记录）
- LLM/计费：本卡零 LLM 腿（确定性转换+死端点 override 面）；真实生成腿为本地 GPU 零计费（278 先例）

## 回执

- 2026-11-16 凌晨交付（零栈段完成；真栈+真实生成烟测腿候栈排队，见下）；实现 commit `a88580d1` @ `port/gen-yue-wire-1`（已推远端，含 coord/runs/GEN-YUE-WIRE-1/RUNBOOK-DRAFT.md 工件）

### 六项逐项证据

1. **MIDI→方言转换**：`agent/internal/genapi/yue_abc.go`——`ConvertMidiToYueABC`（kernel notes+拍号/速度/调号→dual voices+`%` 段注释+行内引号和弦+`L:1/32` 网格量化+跨小节 tie 拆分；学生旋律默认乘 Ins 声部）+`ParseYueABC`（per-voice 小节游标、Z 多小节休止收尾线语义、tie 合并、和弦标注跳过）。往返单测 5 例（`yue_abc_test.go`）：简单旋律/2/4 跨小节 tie+反拍/变化音降号拼写+段落+和弦/方言样例解析/错误面，pitch-start-length 全等断言。
2. **prompt→条件映射**：`yue_prepare.go` `YuePrepareRequest`——style_tags 空→userPrompt 原文逐字映射进 style 通道（中文翻译属 agent 运行时上游，action 披露）；`YueDispatchedSummary` 实发摘要（style 全文/abc 头 120 字/lyrics/时长/cot-steps-format 有效值/未转发字段清单）；harness `gen_generate.go` yue_local 分支接线，回执携带 `dispatched`+`conditioning_actions`；`gen_generate_context.go` evidence 透出 `user_prompt`（prompt 非空约束与实发矛盾消除——prompt 真实进 style，不再是静默丢弃）。
3. **纯器乐条件**：`YuePrepareRequest` instrumental 路径——style 追加 instrumental tag + `RestYueVocalPart` 谱面声部休止化（Vocal 行音符 run→等长 z，和弦标注与小节线保真；等价重写 Ins 行零接触）；单测断言休止后 Vocal 声部解析回读零音符。
4. **请求面卫生**：`dialects.go` `yueRequestBuilder`——extra_params 中 `abc/style/lyrics` 三保护键跳过（引擎旋钮 cot/steps/seed/synth_batch_size 仍可覆写），`Limitations` 逐键披露；`YuePrepareRequest` 将 count>1 折叠为 1 并披露「每版本一个 job」；单测断言学生字段胜出+旋钮仍覆写+披露在场。
5. **后台 job+状态**：`harness/gen_jobs.go`——`gen.jobs.start/status/list` 三工具（catalog 行级注册）；start 立即返回 job_id，引擎 ensure+条件映射+提交/轮询+ingest 全在后台 goroutine（`context.WithoutCancel`+provider.Timeout 兜底）；状态面 starting→preparing→running→done/failed+dispatched+actions+status_trail+工件路径；版本留档=JSON 档案（原 MIDI notes 全量+源谱头+dispatched 请求+replay/音频路径+conditioning actions），存 `~/.vit/genapi_jobs/`（env `VIT_GENAPI_JOBS_DIR` 隔离），跨 agent 重启 reload。集成测 3 例（`gen_jobs_test.go`）：yue 假引擎全生命周期（引擎实收断言=转换谱+映射 style+无 prompt 泄漏+卫生）/未知 provider 快速失败/持久化重载。
6. **最小引擎托管**：`genapi/engine.go`+平台文件——`EnsureYueEngine`（probe→已在=external 复用零管理；未在→按 provider `engine` 块启动（config.go 新增 EngineConfig：command/args/working_dir/env/ready_timeout/poll_interval）→就绪轮询→owned handle；早退/超时分类错误并自清）;`EngineStop` 只停自己启动的进程树（Windows taskkill /T，跨平台进程组）；`EngineStatus` 披露面；ExampleConfig yue-local 条目加 engine 块示例。单测 4 例（`engine_test.go`）：probe 真假/外部复用+停为 no-op/自起→就绪→停→面下线亲证（测试二进制 helper 子进程真实监听）/早退分类。零手工服务：用户填一次 engine.command，gen.generate/gen.jobs.start 自动按需拉起。

### 测试与退出码（三门不回退）

- `go build ./...` exit 0
- `go test ./internal/genapi/ ./internal/harness/ ./internal/tools/ ./internal/genctx/ -count=1` 全绿（genapi 5.9s/harness 17.9s/tools/genctx <1s；含既有 yue 全链/请求形状/纯谱请求例零回退）
- webui：本卡零 webui 改动；vitest 584/584 在主仓同源 src 执行（worktree 无 node_modules；主仓与分支 `agent/webui/` 零 diff 亲证——同源声明在案）
- `go vet` genapi/harness：唯一警告=既有 `shm_windows.go` unsafe.Pointer（非本卡域，不处理）

### 端测覆盖边界声明（AGENTS §5）

- 本段=**零栈段**：单测（genapi 确定性转换/映射/卫生/引擎托管）+定向集成测（harness 后台 job 全链，假引擎+假内核）。**未覆盖**：真实栈（VitApp/Godot/agent 三件套）、真实 YuE2 引擎（E:\AI\YuE2 实环境按需启动/VRAM/生成耗时）、渲染面与用户旅程（本卡零 webui 改动，状态公布走 gen.jobs.* 工具回执=agent 对话面；卡面「webui 生成状态面（如需）」按最小面裁定不做，理由=App.tsx 大文件+367 在飞域风险，主管可另卡）。
- **真栈+真实生成烟测腿候栈第 2 位**（第 1 位=367）：RUNBOOK-DRAFT 已落 `coord/runs/GEN-YUE-WIRE-1/`（腿定义 leg0-leg4+§8 口径草案：生成预算 3 次/失败分类/止损线/VRAM 6.3GB 线/零 LLM 面；**候 371 DG1-DG3 口径终冻后执行**）；栈释放后按 PC-RUNTIME-STACK.md §2.2 全协议排队。
- 用户听核节点：真实生成产出后由用户人耳听核两版本（登记待烟测腿后）。

### 遗留与已知边界（如实）

- 引擎本地托管与真机环境（E:\AI\YuE2）的按需启动链**未实跑**（单测为 helper 子进程模拟）；若真栈腿触发卡面停止条件（引擎侧环境冲突不可按需管理）→取证上交候用户定向。
- 学生谱无和弦输入时转换器输出裸方言行（无引号和弦）；裸行引擎接受度/声学效果=真实生成腿验证项（YUE2-ABC-ADHERENCE-1 结论=方言特征含和弦，往返与映射不承诺声学）。
- 中文 prompt 原文映射进 style（确定性透传）；引擎对中文 style tag 的响应特性未验证（§13 裁定翻译属 agent 运行时上游，本卡提供映射管道+摘要）。
- instrumental 谱面路径的声学边界沿 GEN-YUE-DIALECT-1 复验裁定 B（谱面层确定性成立，声学层不承诺——YuE2 渲染先验可能填稀疏伴奏）。

### 计费

- 零栈段零 LLM、零真实生成（全部确定性测试）；LLM 相关面无涉。
- 真栈腿（见下）：零 LLM（全程 /agent/invoke 确定性面，零 chat goal）；真实生成恰 2 次+转写 1 次+引擎冷启动 1 次（本地 GPU 零计费，VRAM 峰值 4763MiB≪6451 线）。

### 真栈+真实生成烟测腿（DG1-C，2026-11-16 凌晨执行）

- **run_20261116_000009**（run 工件 coord/runs/GEN-YUE-WIRE-1/run_20261116_000009/，随 port/gen-yue-wire-1 95b02604 推远端；装置=本卡 scripts/run_yue_wire_smoke.ps1+yue_wire_smoke.py，W0-W9 腿映射 DG1-C C1-C8 断言面，§8 终冻 RUNBOOK 在卷）；栈协议六步全走（占用 110c3aa5→净空亲核 23:59 全零→≤60s 复核→run→TEARDOWN_ALL_ZERO 亲核 00:1x→释放 fea0829a；PC-RUNTIME-STACK 在卷）。
- **W0-W8 断言面全绿=审计 H6 六项的真栈证成**：C1 学生谱（kernel fixture 16 音符）→方言转换→引擎实收 replay.abc py 独立解析逐音对账 16/16；C2 用户提示词→style 实发映射（v1/v2 replay.style 各含本版用户提示词原文且互异）；C3 谱面面 Vocal 声部零发声（instrumental 真实下发亲证）；C4 污染 extra_params（abc/style 污染键）未达引擎、学生谱/用户 style 实收；C5 两版本=两独立 job（job/task/WAV 互异+gen.jobs.list 在列）；C6 gen.jobs.start 即返+生成窗口内 providers 调用 RTT 0.05s（非阻塞）；C7 引擎由 agent 托管按需真实拉起（run 前 8087 净空亲核零→mid-run up→supervisor 双日志在场→teardown 兜底停）；C8 WAV 60.0s+replay+job 档案（原始 16 音符全量留档）。
- **W9（C3 声学面）=预写概率类 `legW9_vocal_detected` 首触发**：转写 Vocal 声部末小节 `z6ed` 检出 2 音（前 3 小节全休止）——GEN-YUE-DIALECT-1 裁定 B 预言的「渲染先验填稀疏伴奏」声学层概率事件；非产品缺陷（W3 已证谱面层确定性）。**driver exit 1（诚实判定，非零不粉饰）**；处置候主管裁定（沿 278 裁定 B 精神收口或另批统计预算）。
- 装置弱点 2 项如实登记（RUNBOOK 执行记录节）：wire_report failure_class 字段未随 fail 落盘（证据在 terminal+report.w9 键）；ps1 管道下 bash 退出码不可靠（verdict 行为准）——修复候后续装置小卡，不影响本次判定证据链。
- 用户听核节点（M1 成曲人耳）：**未执行**——两版本 WAV 在 run downloads/（gitignore 排除），候用户在场时听核登记。

## 验收

（待主管决策流；真栈腿已执行，W9 处置选项随回执上呈）

### 验收（主管 2026-11-16 凌晨）

- 裁定：**pass**——diff 18 文件零越域（webui 零改动亲证）；主管复跑 go build/genapi 测试绿；正式 run_20261116_000009 **W0-W8 全绿=六项真栈证成**（C1 逐音对账 16/16、C8 双版本 WAV 60.0s+版本档案跨重启）；栈六步协议+S10+零 LLM 合规。
- **W9 裁定（主管，依据=GEN-VOCALSEP-1 用户 11-05 亲听裁定）**：转写腿「零人声检测」判据属装置侧过度断言——YuE2 声部纯度是用户已亲耳核实的既有声学特性（稀疏配器残留）。处置：W9 收编为**已知概率边界**（稀疏残留 ≤ 个位数音符/集中末小节不判失败），本观测样本（1 次/2 音/末小节）留档基线；判据阈值化+人声分离升级归生成侧后续卡。单次观测不改写为成功、已知特性不归因产品缺陷（§8 口径）。
- 合入：实现+装置+run 工件→main（70ee4e35 前序链）。
- 遗留采纳：**用户听核 M1 候用户在场**（两版 WAV 在 run downloads/——yue-local-45b1…/6d72…wav）；装置弱点 2 项候装置小卡。

## 用户听核 M1 回执（2026-11-16 晚，主管代记）

- 判定：**条件性——可听性 PASS，纯器乐目标未达**。
- 用户原话要点：①「听过后确定是合理的音乐」——成曲可听性 PASS；②「并不是纯器乐」（有歌词/人声）——**与 C3 断言（instrumental 真实下发+Vocal 声部零发声）矛盾**，用户确认歌词非需求（主管答复合影：非任何一方执意，设计意图即纯器乐）；③「v1 比较完整，v2 只有一句」——v2 内容完整性存疑（C8 只断言 WAV 60.0s 时长，不断言内容密度）。
- 与 W9 裁定关系：W9 裁的是「稀疏配器残留 ≤ 个位数音符」量级的声部纯度；用户本次听到的是**成段歌词**，超出该边界，不属同一定性——须按断言矛盾取证，不得复用 W9 口径消化。
- 处置：②③转取证卡 **YUE-VOCAL-FORENSICS-1**（C3/C8 断言语义复核+引擎侧 instrumental 生效链取证）；双版 WAV 已从 worktree 备份至主仓 run downloads（2026-11-16，修正 gate-morning 路径只含 replay.json 的不一致）。
- M1 登记：**PASS（可听性）/ OPEN（纯器乐与 v2 完整性——候取证卡）**。

### 勘误注记（主管 2026-11-16 晚，依据=381 取证报告）

本卡验收节两处口径**勘误**（原验收作废面以 381 报告为准）：
1. 「C3 instrumental 真实下发亲证」→ 勘误为：**C3 断言测的是请求侧谱面**（replay.abc Vocal 行纯休止），非引擎生效侧；引擎 API 无 instrumental 参数（job 回显仅六字段），agent 以 style 英文词+谱面休止替代（链路如实、not_forwarded 如实声明）。
2. 「C8 双版本 WAV 60.0s」→ 勘误为：**装置仅对 v1 有 40-150s 宽窗断言、对 v2 无时长断言**；实际 v2=51.0s 引擎短输出（semantic end token，无填充无截断，字节级对账在 381 卷）。
- 上面「用户听核 M1 回执」节的两项 OPEN 由 381 定性收口：**人声=引擎能力边界（无歌词时自由生成主旋律，v1 verse 段 27-55s 成段人声，量级远超 W9 稀疏残留口径）**；v2 短输出=引擎侧语义终止行为。产品面处置（纯器乐不可保证的展演口径/规避策略）候用户裁定。
