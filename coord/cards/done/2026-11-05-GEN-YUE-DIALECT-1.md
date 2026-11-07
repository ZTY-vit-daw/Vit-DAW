# GEN-YUE-DIALECT-1：YuE2 生成接入——genapi yue-local dialect 直连（生成首张 / 注册链③）

- 发卡：GLM 主管决策流 / 2026-11-05（依据=[259 报告](../../reports/2026-11-05-gen-recon-1.md) + [generation-mcp-pivot](../../decisions/2026-11-05-generation-mcp-pivot.md) §四-5）
- 派发确认：已确认（用户 2026-11-05 批准入池）
- 验收负责人：GLM 主管决策流
- 池序 278；**注册链第 ③ 位（前置=277 已验收提交）**；目标=genapi 包 dialect 扩展+注册；P1 / 1.5-2 天；L1-L2 / PC；真栈烟测腿一次（agent 侧），YuE2 引擎腿独立进程

## 目标

1. **yue-local dialect**：genapi 增加 YuE2 本地 REST（127.0.0.1:8087）dialect——输入（ABC 方言谱+歌词/风格参数）→ 生成任务 → 产物捕获（artifact 五件套：catalog spec / policy / harness case / 实现包 / artifact 捕获，对照 browser_fetch 先例 catalog.go:141/:462 模式）。
2. **注册**：catalog+policy 行级（若 genapi 命令族已注册则仅扩 dialect 参数面，注册面最小化）。
3. **复验条款执行（259 验收挂账）**：重跑"仅人声模式"探测（a cappella+Ins 全休止谱→生成→转谱回读无器乐断言）——单次实证转正式断言。
4. **真栈烟测腿**：chat 驱动"用 YuE2 把这段谱唱出来"→ dialect 调用→产物落 artifact→回读确认；YuE2 引擎独立进程启停（不占 VitApp 栈窗口时段）。

## 约束

- 文件域=agent 侧 genapi 包+catalog/policy 行级+测试+烟测脚本；零内核；YuE2 侧零改动（纯客户端）；
- 生成产物落 artifact 体系（不入源码树）；60s 歌墙钟 21-40s，烟测预算含两次生成；
- 显存预检（≤6.3GB 峰值实测值，超限→环境中断上交）。

## 验收标准

- 真栈烟测 exit 0（含复验断言：仅人声回读无器乐）；既有 genapi 测试零回退；
- 回执附：dialect 调用形态、两次生成产物路径、复验断言输出、commit hash。

## 停止条件

- YuE2 引擎不可达/显存不足→环境中断取证（不重跑原样超两次）；MCP 形态不在本卡（归 285）。

## 领取

- 2026-11-07 / origin/main=56c53bd1（领取 fetch 后亲核）；owner=GLM 主会话 L2-worker 子代理（ZCode/PC）
- 分支 port/gen-yue-dialect-1 / worktree D:/Vit_DAW_wt_278impl；协调分支 coord/278-claim（worktree D:/Vit_DAW_wt_278coord）
- 领取基线：worktree 自 56c53bd1 新建，HEAD=56c53bd1，status 全净，diff --stat 空
- 前置核对：277 pass（c6c7f5a3）/281 pass（90126560）已合入；卡在 todo/、派发已确认；doing/ 三卡（ARRLOOP-AUTHBRIDGE/B3-ABLATION/CLOSURE-ROUTE-ADAPT）文件域与本卡不相交
- YuE2 引擎勘察：E:\AI\YuE2\ 部署在场（yue-server.exe+启停脚本，259 工件 YUE2-PC-DEPLOY-1）；当前未运行（127.0.0.1:8087 连接拒绝）——烟测腿自管启停，不入 PC-RUNTIME-STACK（非三件套栈）

## 回执

- 2026-11-07 晚窗段交付（卡留 doing/ 待主管处置：复验条款被推翻+chat 腿域外双缺陷，见下）；分支 port/gen-yue-dialect-1（实现 8ff2e4d6→终笔 f4c69d43 全程已推）；工件 coord/runs/GEN-YUE-DIALECT-1/（precheck+run1 RUNBOOK+attempt1/run2/run3/run4/run5 全工件）

### 目标 1 yue-local dialect——完成，真栈四 run 连续绿
- genapi：GenerationRequest.ABC 谱面通道（P2 式纯器乐谱=合法输入）；dialects.go 注册 yue_local（POST /synth→hex job id→/job?id= 轮询→/job?id=&result=1 multipart=JSON replay+audio/wav）；yueRequestBuilder 钉验证基线 cot=full/steps=32/wav16（extra_params 可覆写）；prompt/model/tempo/key/count/title/instrumental 走 Limitations 披露不静默丢；poll done-branch 新 resultPath 语义=fetchMultipartResult（音频走既有 sniff/cap/save，replay 落 .replay.json sidecar Kind=replay=逐字节复现基底）；essentiallyEmpty 含 ABC
- harness：gen.generate 增 abc/abc_notation/score_abc 参数面；ingest 仅收 Kind=audio（sidecar 不上 timeline）
- 真栈实证（run4 job 9e4f260d16495bd6 + run5 job 82984d83d18fdcce，均 21s 墙钟、replay+wav 双工件、limitations 披露、status 迹 running×7→done→multipart 两 marker）

### 目标 2 注册——完成（最小面）
- catalog：genapi_generate argHint 扩参数面（abc 等）+desc 提及 yue_local（命令族 276 时代已注册，spec 零改）；policy 零行改动（风险级由 catalog spec 承载，Classify 经 spec 派生——与 recon 3.1 的 policy 行级预期不同，实查后按卡面「注册面最小化」从 spec）；ExampleConfig 增 yue-local 条目
- leg0 注册披露真栈四连 PASS（gen.providers 列 yue-local/dialect=yue_local/base_url）

### 目标 3 复验条款（259 挂账）——**单次实证被推翻，取证上交**
- 两新样本（同 P1 谱同 seed=11，各 1 次生成）：run4 回读 Ins=纯休止 8 行 PASS；**run5 回读 outro 段 Ins 出现真实音高行（c'8z4c'4z16|b4z8b12z8|… 等 3 行）FAIL**；输入面 replay 两 run 均亲证 Ins 全休止原样接收（谱面控制层成立）
- 结论：「仅人声模式→SheetSage2 转谱回读无器乐」在 (渲染,转谱) 联合层面**是概率性质不是不变量**（引擎同 seed 声学重跑非逐字节确定=已知边界）；259 自己也声明过「转谱回读=弱证据」——本卡把弱证据升级为正式断言的企图被样本 2 否决
- 生成预算 2/2 尽，按 §8 预写 leg2_instrumental_detected 类停止不重跑；处置选项交主管：A 改断言为统计口径（N 样本无器乐比例，需另批生成预算）/ B 维持「谱面层成立+声学层不承诺」的诚实表述收卡 / C 其他

### 目标 4 真栈烟测腿——**部分达成，chat 腿被域外双缺陷阻断（锚点上交）**
- 达成：YuE2 引擎独立进程自管启停全协议兑现（不入 PC-RUNTIME-STACK；起/health/props/停/端口释放全亲证；直连 Start-Process 修复了包装器管道阻塞）；产物落 run 专属 downloads artifact 体系（wav 由 .gitignore 排除不入仓、replay 文本入库）+回读确认（RIFF/replay 含 abc）
- **域外缺陷 ①（词法路由劫持）**：chat 消息内嵌 ABC 谱即触发——`agent/internal/chat/capability_owner.go:225` inferProjectAwareCapability 对全消息小写子串匹配，谱内 `b32|Z|` 含 "b3"→pan_layout canary→turn.failed 零步（run2 工件）；同类暴露面 b4/d1/c2/c3 谱面十六进制自然相交；最小复现=含 `b32` 的任意 chat 消息
- **域外缺陷 ②（goalrunner 能力包无生成工具）**：`agent/internal/chat/goalrunner_chat.go:4361`（capabilities 非空→工具集收窄）+ :4544/:4605/:4608（mix "vocal"/midi "drum"/transport "bpm" 词典——生成请求风格词汇天然命中）+:4437-4492（switch 无 generation 包）+:5001（agentLoopMixTools 无 gen.*）→ LLM 诚实申报「工具集里没有 gen.generate」并拒绝假装生成（run3 工件，276「第四处注册面」同型）；276 裁定 A 先例=最小一行扩表+配对单测
- 处置选项交主管：A 两缺陷各立小卡（②先行——chat 腿是生成线产品入口）/ B 合一张 chat 生成路由卡 / C 本卡扩域（需明示授权）
- 附带：驱动/包装器自身三处缺陷已在本卡域内修复（引擎启动管道阻塞 db85e03c、goal 终态 idle 复位空转、断言 K: 指令行误报+七案例本地验证）

### 测试与退出码
- go build ./... exit 0；go test ./... 全绿（含 genapi 新 3 例：yue 全链 multipart/请求形状钉死/fail status 分类/纯谱请求；既有零回退——唯一改动=ExampleConfig 三→四条目断言随实现更新）；tools/policy/harness 包全绿
- 烟测 run5 py exit 1（=leg2_instrumental_detected 预写类，被测面全绿）；leg0 四连 PASS；chat 腿未再耗（域外在案）

### 计费/资源/边界
- 生成恰 2 次（60s 请求 a cappella×2，各 21s 墙钟，本地 GPU 零计费）；转谱 2 次；LLM 仅 run3 一 chat goal（flash 级 ≈¥0.05-0.15，≪RUNBOOK ¥0.5 上界）；VRAM 采样峰值 3,903/4,582 MiB ≪6.3GB 线
- 栈协议：占用/释放五轮全登记（eedc5c07/dbe99d7d+b5276f22/93376bc5/f1538d3a/30f9cb15/5edb5d23），TEARDOWN_ALL_ZERO 亲核 20:09:51；排队纪律兑现（让位 296→298 后首位登记）
- 端测边界声明（AGENTS §5）：单测级=genapi 包 21 例（含新 3 例）；真栈级=dialect 全链（invoke 腿×2+注册披露×4+转谱回读×2）；**未覆盖=chat 驱动腿（域外双缺陷阻断，run2/run3 取证在卷）与渲染面/用户旅程（本卡不涉 webui，Godot 未起——栈为 kernel+agent 最小形态，chat 经 HTTP 面，先例 reply_gen_toolgate）**


## 验收

- **晚窗段验收+复验条款裁定（2026-11-07 晚窗，主管）：段 pass+复验条款选 B 收口**——①dialect/注册/invoke 全链实证采信（生成×2 真实 21s 墙钟+VRAM 4.6GB 线内+注册披露四连 PASS+go 全绿）；②**复验条款（259 挂账）前提推翻采信**：同谱同 seed 1/1 分裂=「无器乐回读」为 (渲染,转谱) 联合概率性质——裁定 **B=谱面层确定性成立+声学层不承诺**（诚实边界收口；A 统计口径断言留论文需要时另批预算）——记良性互动一笔（证据推翻 259 挂账前提）；③域外双缺陷（canary 误劫持/capabilities 词典缺生成）**转卡 303 GEN-CHAT-ROUTING-1**，chat 驱动腿候 303 合入后作本卡末腿。卡留 doing。L1 台账 48h 窗口。

## 末腿回执（run6，2026-11-07 21:38–21:43，接手代理：前代理 21:17 限额中断后续做）

- **结果：真栈烟测末腿 PS1 exit 0 = PASS，卡面验收标准「真栈烟测 exit 0」全部达成**；run ID=coord/runs/GEN-YUE-DIALECT-1/run_20261107_213958（run6_console.txt+全工件随 port/gen-yue-dialect-1 7aa0f10c 已推）
- 被测：wt_278impl @ port/gen-yue-dialect-1，HEAD=f66c0cee（origin/main c16daea0 merge 进分支，含 303 双修 afda4b4e+295 修；merge 后 `go test ./internal/chat ./internal/genapi -count=1` 全绿=exit 0 亲证）；agent=worktree 现场构建 21:40:02（39,192,576B）；kernel=主检出预构建 VitApp.exe 只读复用（SHA256 C1E32DFE…随 prereq）；YuE2 引擎自管启停（8087，不入 PC-RUNTIME-STACK）
- **leg0 注册披露五连 PASS**：yue-local/dialect=yue_local/base_url=127.0.0.1:8087 在列
- **leg1 chat 驱动一回合「用 YuE2 把下面这段谱唱出来」三合一全过（52s 墙钟，goal_841baf81794c2513 completed/stop_reason=done）**：
  - 路由不劫持（run2 形实盘复放）：无前缀自然话术+ABC 谱 outro `b32|Z|` 原样在场（chat 响应 original_intent 亲证），route_hijack_markers=[]——零「不能形成可执行候选」零 turn.failed=**303 缺陷①修复实盘验证通过**
  - gen 工具披露含生成（run3 形实盘复放）：风格串 vocal/drums/108 BPM 在场，mix 观察步与生成步共存——daw_action 轨迹=observe_b1_gain_staging→b1_gain_staging_project_state→**genapi_providers→genapi_generate**（真实工具步，仅认 item_id/command_name，裸文本扫描假阳性已排除）=**303 缺陷②修复实盘验证通过**
  - 产物面：run 专属 downloads 新落双工件——wav 11,012,908B=RIFF 时长 **57.36s（45-75s 窗内）**+replay 含 abc 逐字（`b32|Z|` 原样+style 全串+lyrics+duration=60.0）；生成恰 1 次（预算 1，60s 歌引擎侧 21s 级）
  - 对话摘录（reply）：「YuE2（yue-local）已按你的原始 ABC 谱生成完成，音频已落盘。任务号：46797cf8bea5cde6…ABC 谱逐字原样传入，未改写、未截断…该引擎没有自由 prompt 通道…未传 track_id…只把文件写入磁盘，没有上轨」——诚实边界（限制披露+未上轨声明）与 run3 同型良好
- 计费/资源：LLM=1 chat goal flash 级 ≈¥0.05-0.15（≪RUNBOOK ¥0.5 上界）；VRAM 原始采样峰 5171MiB ≪6451 线（ps1 峰值解析逗号 bug 沿 run5 注记，13 样本在工件）；墙钟：栈+引擎+build+chat 全程 21:39:58–21:41:35≈97s
- 栈协议：净空亲核 21:38:31 全零→占用登记 6d1b43c8 推 main→21:39:40 ≤60s 复核全零→启动→run→脚本自拆（agent 25416/kernel 29456/engine 21684）→**TEARDOWN_ALL_ZERO 亲核 21:43:33**（七口 TCP+UDP 4444/4445+五类进程全零）→释放登记（见 PC-RUNTIME-STACK run6 收口条）
- 端测边界更新（AGENTS §5）：chat 驱动腿已覆盖（run6）；渲染面/用户旅程仍不涉（本卡无 webui 改动，Godot 未起，chat 经 HTTP 面——先例 reply_gen_toolgate 维持）
- 卡处置：末腿 PASS=卡面四目标全部落定（目标 1/2 完成+目标 3 裁定 B 收口+目标 4 末腿 PASS），mv done/ 待主管终验归档
