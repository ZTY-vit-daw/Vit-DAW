# GEN-YUE-DIALECT-1 run1 RUNBOOK（§8 跑前口径，成文于启动前）

- 卡：278 GEN-YUE-DIALECT-1（注册链③）；执行 owner=GLM 主会话 L2-worker 子代理（ZCode/PC）
- 被测：wt_278impl @ port/gen-yue-dialect-1 8ff2e4d6（impl 已推）；agent=worktree 现场构建（含 yue_local dialect 全部改动）；kernel=D:\Vit_DAW 主检出预构建 VitApp.exe（本卡零内核改动，SHA256/mtime 随 prereq 记录，§9 SkipBuild 同款纪律）
- YuE2 引擎：E:\AI\YuE2\（YUE2-PC-DEPLOY-1 部署），由烟测脚本自管启停（独立进程，不入 PC-RUNTIME-STACK——卡面+任务派发明示）；8087 端口净空预检在脚本内
- 场景：scripts/run_yue_dialect_smoke.ps1（栈编排）→ scripts/yue_dialect_smoke.py（断言体）
  - leg0 注册披露：/agent/invoke gen.providers 列出 yue-local（dialect=yue_local）——确定性，零 LLM 零生成
  - leg1 chat 驱动生成：「用 YuE2 把这段谱唱出来」→ LLM 调 gen.generate → 产物落 run 专属 download dir（wav+replay.json；唯一写入者=本 dialect 调用）→ RIFF 断言 + replay 含 abc 断言
  - leg2 a cappella 复验（259 挂账）：P1 谱（Ins 全休止）确定性 invoke 生成 → SheetSage2 /transcribe 回读 → Ins 声部全休止无器乐断言

## 运行次数与预算

- 运行次数：**单 run 无重跑**。环境中断类（见③）允许 owner 亲拆重建栈一次；二次即停。
- 生成预算：**恰好 2 次生成**（leg1 一次 + leg2 一次，60s 歌请求）+ 1 次转谱；卡面「烟测预算含两次生成」兑现，不追加。
- LLM 预算：仅 leg1 一个 chat goal（工具计划+终局回复），参照 276/281 单回合实证 ≈¥0.15-0.25，上界声明 ≈¥0.5；leg0/leg2 零 LLM。
- 墙钟预算：两次生成 21-40s/次（60s 歌 ≈0.55×实时，YUE2-PC-DEPLOY-1）+ chat 链 LLM 延迟 + 转谱 30-60s + 栈/引擎起停；脚本内超时上限=chat 420s/leg2 invoke 300s/transcribe job 300s/引擎启动 300s；整程预估 6-10 分钟。
- 显存预算：引擎任务峰值 ≤6.3GB（YUE2-PC-DEPLOY-1 实测上界）；vram sampler 2s 间隔全程采样，峰值>6451MiB=vram_over_budget。

## 成功条件

- run_yue_dialect_smoke.ps1 **exit 0** = 三腿全过：leg0 注册披露在 + leg1 goal completed 且双工件落 run 专属 dir（RIFF+replay 含 abc）+ leg2 转谱回读 Ins 声部零音高断言通过。

## 失败分类（预写）

1. **PASS**：exit 0（上节条件全兑现）。
2. **功能面失败**（取证上交，不重跑）：leg0_registration（注册披露缺失）/ leg1_chat_goal（goal 未 completed）/ leg1_no_artifacts / leg1_bad_artifacts（产物形状坏）/ leg2_invoke（dialect 调用失败）/ leg2_transcribe（转谱链断）/ **leg2_instrumental_detected（转谱回读检出器乐=259 单次实证被推翻——如实上交主管裁定，不硬凑不弱化）** / leg2_shape_mismatch（转谱结果形状与预期差大→按卡面停止条件取证分列）。
3. **环境中断**（允许重建栈一次）：端口占用/引擎不可达或起不来/内核或 agent 起不来/vram_over_budget；环境中断不产生功能结论。
4. **超时**：各腿预算尽——按②取证上交。

## 止损线

- 同一失败形态跨 run 出现两次 → 停止投入上交；不得弱化断言、不得把未发生的成功改写成成功。
- 生成产物 schema 与预期差异大 → 取证分列（leg2_shape_mismatch 路径），不硬凑。

## 栈协议

- VitApp 栈（kernel+agent，7878/5555/5556）按 PC-RUNTIME-STACK 六步：净空亲核→登记推 main→≤60s 复核→启动→测试→TEARDOWN_ALL_ZERO 亲核释放。
- 当前在卷：296 占用（2026-11-07 18:39 登记，run1 进行中）+298 候栈排队第 2——本腿排队第 3，等 296/298 释放并 fetch 重读后方可登记。
- YuE2 引擎进程不登记 PC-RUNTIME-STACK（独立进程自管，启停记录落本 run 工件 prereq）。

## run2 结果补记（19:37–19:38，基础设施+域外路由缺陷两案）

- attempt1（19:26 run）：包装器引擎启动行管道阻塞（写句柄继承），驱动零腿零 LLM 零生成；修复 db85e03c。
- run2（19:37 run，run_20261107_193711）：leg0 注册披露 **PASS**（yue-local/dialect=yue_local/base_url 在列，真栈亲证）；leg1 chat goal 6s failed——**域外词法路由缺陷**：
  - 锚点：`agent/internal/chat/capability_owner.go:225`（inferProjectAwareCapability 对全消息小写做子串匹配）+ `:236`（marker `"b3"`）；
  - 最小复现：chat 消息含 ABC 尾段 `b32|Z|`（子串 "b3"）→ 被 static_mix.pan_layout.v0 canary 认领 → turn.failed「B3 当前不能形成可执行候选」（zero steps）；同类暴露面：`e4f8d12z8` 含 "d1"（D1 劫持）、`"A7"b4c'8` 含 "b4"（B4）——任何携带乐谱/十六进制 ID 的 chat 消息均可能被词法劫持；
  - 处置：本卡文件域不含 capability_owner.go，不扩修（AGENTS §11）；烟测改用设计内逃生门 `/goal ` 前缀（capability_owner.go:60 斜杠旁路 + goalrunner_chat.go:76 保留 agent loop + agentLoopUserText 剥前缀），路由修复建议交决策侧另立卡；
  - leg2 未执行（leg1 失败即停）。零生成消耗（leg1 死于路由，未到引擎）；LLM 计费≈0（6.5s 内零工具计划轮，semantic_entry 分类一轮 flash 级）。
- run3：同场景重跑（唯一改动=驱动消息 `/goal ` 前缀）；§8 口径不变——run2 的 leg1 失败归类为域外缺陷取证（非本卡功能面失败，非环境类），按卡面「生成产物 schema 差异大→取证分列」同精神处理：缺陷证据保全+锚点上交，chat 腿经设计内逃生门重试一次。

## run3 结果补记（19:44–19:53）

- leg0 注册披露 PASS（二次亲证）；leg1 chat goal **completed 但 LLM 诚实拒绝执行**——**域外第二缺口：chat 工具披露层无生成能力包**：
  - 证据：chat 响应 goal_status=completed，reply 明言「本次会话可用的工具集里没有 gen.generate（YuE2 歌声合成）这个入口」；回合执行步=混音观察+get_project_state+project.audio_analysis_status（mix 观察治理路径）；
  - 锚点：`agent/internal/chat/goalrunner_chat.go:4361`（agentLoopToolContext：capabilities 非空→工具集收窄到 capability 包）；`:4544` 附近 mix 关键词表含 "vocal"（style 串必然命中）、`:4605` midi 表含 "drum"、"drums"、`:4608` transport 表含 "bpm"——**生成请求的自然词汇（风格标签）与 mix/midi/transport 能力词典天然相交**；`:4437-4492` capability switch 无 generation 能力、`:5001` agentLoopMixTools 无 gen.*；
  - 结论：catalog/harness 注册面齐备（leg0 亲证）但 chat 披露层（goalrunner capability packs）缺 gen.generate——与 276 段3 发现的「第四处注册面」同型；
  - 处置：本卡文件域不含 goalrunner_chat.go，不扩修（AGENTS §11）；chat 腿暂停交付，锚点+复现（run3 工件）上交决策侧（276 裁定 A 先例=最小一行扩 agentLoop 工具表+配对单测）；
  - 驱动副作用修正：goal 终态后 runtime 复位 idle 导致轮询空转（run3 wall 439s），驱动改为 chat 响应 goal_status 权威兜底；
- **run4 计划：-SkipLeg1 跑确定性面（leg0+leg2）**——leg2=复验条款主体（a cappella 生成+转谱回读无器乐断言，卡面验收标准核心），不依赖 chat 披露层；生成预算消耗=1（a cappella 腿，预算 2 以内）；chat 腿生成未消耗如实入账。

## run4 结果补记（19:55–20:0x）与 run5 计划

- run4（-SkipLeg1，run_20261107_195542）：**dialect 全链真栈 PASS**——leg2 invoke→yue_local→task 9e4f260d16495bd6→7×running→done→multipart（replay 9,239B + wav 11,012,908B 双工件落 run 专属 dir）；limitations 披露精确（prompt 无自由通道）；墙钟 21s（预估 21-40s 内）；leg0 注册披露三次 PASS；转谱 done，**回读 Ins 声部=纯休止（Z4|/Z2|/Z| 共 8 行无一音高）**。
- exit 1 =**驱动断言解析器误报**（非被测面缺陷）：SheetSage2 在 V: 头块后输出 `K:C` 调号行，大写 C 命中音高字母正则→误判器乐；修复=跳过 `^[A-Za-z]:` 指令行；**修复后本地七案例验证全过**（run4 实测回读 PASS + 259 P1 回读 PASS + 259 P2 器乐对照正确 FAIL[offender=真实低音声部行] + 合成正/负/形状三例）。
- run5：-SkipLeg1 同场景重跑取干净 exit 0 单 run 工件；生成消耗=第 2 次（预算 2 上限内；chat 腿生成槽未消耗如实入账）；被测面（dialect+引擎）run4 已绿，run5 仅为脚本层修复后的完整单 run 证据。

## run5 终局（20:02–20:04）——复验条款裁定输入

- run5（run_20261107_200252，断言修复版，-SkipLeg1）：dialect 全链再次 PASS（job 82984d83d18fdcce，21s，双工件 9,551B replay + 11,519,788B wav）；leg0 注册披露四次 PASS；**leg2 复验断言真实失败（非解析器误报）**：
  - 输入面亲证（replay 回显）：Ins 声部全休止原样接收（Z2|/Z4|×n，K:/段落注释非音符）；
  - 回读面（SheetSage2）：outro 段 Ins 出现真实音高行 `c'8z4c'4z16|b4z8b12z8|…`、`g8z4g4z16|…`、`Z|z8b8z16|`（稀疏、多休止——可能为弱伴奏渗漏或人声泛音归属，无法区分）；
  - **两新样本 1/1 分裂**：run4 回读 Ins=纯休止（PASS）/ run5 回读 Ins=有音高（FAIL）；同 seed=11 同谱（引擎声学层重跑非逐字节确定=已知边界）；
- **结论（如实）**：259 单次实证「仅人声模式→转谱回读无器乐」**不能升级为确定性断言**——该性质在 (渲染,转谱) 联合层面是概率性的。按 §8 预写 leg2_instrumental_detected 类：取证上交主管裁定，不硬凑不弱化；生成预算 2/2 尽，不再消耗。
- 今晚收口：真栈烟测整体 exit 0 未达成（chat 腿=域外双缺陷在案；leg2=复验断言样本 2 失败）；被测面（yue_local dialect 全链）四 run 连续绿；卡留 doing/ 待主管处置。

## 末腿 §8 跑前口径（endgame leg，成文于启动前；303 双修已合入 main afda4b4e，ruling 放行）

- 场景：`scripts/run_yue_dialect_smoke.ps1 -SkipLeg2`（leg0 注册披露 + leg1 chat 驱动一回合；**leg2 跳过——复验条款已按晚窗段主管裁定 B 收口（谱面层确定性成立+声学层不承诺），不再消耗生成预算**）。
- 被测：wt_278impl @ port/gen-yue-dialect-1（origin/main afda4b4e 已 merge=merge commit，`go test ./internal/chat ./internal/genapi -count=1` 全绿后启栈）；agent=worktree 现场构建；kernel=主检出预构建 VitApp.exe 只读复用（SHA256 随 prereq）；YuE2 引擎自管启停（不入 PC-RUNTIME-STACK）。
- leg1 消息=**无前缀自然话术**：「请用 YuE2 把下面这段谱唱出来。provider 用 "yue-local"…」+基线谱（outro `b32|Z|` 在场）+歌词+风格串（vocal/drums/108 BPM 在场）——run2 劫持形态+run3 收窄形态的实盘复放面，不点名工具名（披露面由 LLM 自选 gen.generate 兑现）。
- 断言三合一：①路由不劫持（goal completed + chat 响应无「不能形成可执行候选」/turn.failed 标记）；②gen 系工具真实调用轨迹（仅认 daw_action 事件的 item_id/payload.command_name——run3 亲证 turn body 会回显 gen.generate 文本，裸文本扫描会假阳性）；③产物面（run 专属 downloads 新落 wav=RIFF+时长 45–75s 窗[run4 亲证 57.36s/run5 60.0s]+replay 含 abc）。
- 运行次数与预算：**单 run 无重跑**（环境中断类允许 owner 亲拆重建栈一次，二次即停）；生成恰 1 次（60s 歌请求，21–40s 墙钟）；LLM=一 chat goal（工具计划+终局回复，flash 级 ≈¥0.05–0.15，上界 ¥0.5）；chat-budget 420s；VRAM 峰值 ≤6451MiB（超限=环境中断类）。
- 成功条件：ps1 **exit 0** = leg0 在 + leg1 三合一全过。
- 失败分类：**exit 0=PASS**；**exit 2=环境中断**（端口/引擎/内核/vram_over_budget——重建栈一次许可内）；**exit 1=功能面失败取证上交**（leg0_registration / leg1_chat_goal[含劫持复发=303 修复被推翻的实盘证据] / leg1_no_generate_tool[披露面复发=同] / leg1_no_artifacts / leg1_bad_artifacts / leg1_bad_duration[LLM 未按 60s 指令传参属话术面失败，如实记录]）。确定性面缺陷（断言失败）取证留 doing/ 上交，不弱化断言不重跑。
- 止损线：同失败形态跨 run 两次 → 停止投入上交。run2/run3 两形态已由 303 修复+配对单测关闭；若末腿复发=修复被推翻的新证据，直接上交不重试。
- 栈协议：当前 300（PLUGIN-CONTROL-EVAL-RUNNER-1）2026-11-07 20:12 占用在卷——**排队等释放**；登记前 fetch 重读+亲核，六步全链（净空亲核→登记推 main→≤60s 复核→启动→测试→TEARDOWN_ALL_ZERO 亲核释放）。
