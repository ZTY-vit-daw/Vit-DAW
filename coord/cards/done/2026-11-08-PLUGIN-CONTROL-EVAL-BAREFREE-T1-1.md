# PLUGIN-CONTROL-EVAL-BAREFREE-T1-1：T1 补完腿（第三层 harness 守卫 seam+T1×3 定向复跑）

- 发卡：GLM 主管决策流 / 2026-11-08 深夜（依据=[318 终验裁定①](../done/2026-11-08-PLUGIN-CONTROL-EVAL-BAREFREE-1.md)+用户裁定「确保模型真的能够裸写参数控制」——T1 是最后一个未测及案例）
- 派发确认：已确认（主管发卡·定向补完）；验收负责人：GLM 主管决策流
- 池序 319；目标仓库=主仓 agent（harness 守卫缝）+工件；P2 / 0.25 天 / 依赖=318 已合入；L2 / PC；真栈一次

## 目标

1. **第三层守卫解剖+seam**：`broadMixObserveFirstWriteGuard`（`agent/internal/harness/harness.go:2590`+Invoke ~1040，「不认 mix.observe 完成」的写入门）——同 318 模式入 `VIT_PLUGIN_CONTROL_EVAL_SEAM_BARE_WRITE` 缝（默认 unset 字节不变+配对单测：缝开=T1 型写入放行/缝关=现状拦截）；**harness.go 域预声明**（与 273 未合入分支不同函数区，合入冲突归主管）；
2. **T1×3 定向复跑**（非全 18）：A-pure 面+全守卫许可（318 缝+本腿缝）——T1 案例三轮同判据；装置门=守卫许可亲证扩展（T1 型写入 probe 落地）；
3. 判定：T1 pass 率=最后一个案例的真裸写答案；guard_blocked_write 分类维持；
4. 预算 ≈¥1（3 轮×~13 调用级）；§8 跑前成文；栈排队（L1 真栈腿协调）。

## 停止条件
守卫与 mix.observe 语义深度耦合无法缝隔离（需要合法完成态证明）→取证上交转裁定（此时 T1 答案=「守卫要求完成态是设计语义」）。

## 领取/回执/验收
- 领取：2026-11-08 深夜 +0800 / origin/main=51a419094a5f7458d64879e481db0c8de4d85646（fetch 亲证，≥发卡要求 51a41909，含 318 缝与全套工件）/ owner=ZCode GLM-5.3 L2-worker PC 执行子代理（会话 319 t1 补完；派发已确认）/ 分支=port/plugin-control-eval-barefree-t1-1 / worktree=D:/Vit_DAW_wt_319t1 / 领取提交=coord/319-claim→main。领取基线：fresh worktree 自 origin/main 51a41909，`git status --short`=空、`git diff --stat`=空（全净）。在飞卡域核对：doing 八卡（EDU-ASK-ANCHOR-1=282impl 已完未合入、ARRLOOP-AUTHBRIDGE-1、CLOSURE-ROUTE-ADAPT-1、WAVESHELL-OFFRENDER-FIX-1=C++ 域、LEGAME 三卡、PLUGIN-CONTROL-EVAL-S1B-1=工件目录+编排+tools 缝域）与本卡（agent/internal/harness/harness.go 守卫函数区±配对单测 + coord/runs/PLUGIN-CONTROL-EVAL-BAREFREE-T1-1/ 工件目录 + 本卡面）不相交；**harness.go 域预声明**（卡面原文）：本腿守卫缝与 273 未合入分支（Authorize* 家族，harness.go:2223-2248 一带）同文件不同函数区——本腿只动 broadMixObserveFirstWriteGuard 守卫函数区，diff 自检不触 Authorize*，合入冲突归主管。318 缝在位亲证：agentloop `pluginControlEvalSeamBareWriteOpen`（message_loop.go:4360 一带）+chat 同名助手（goalrunner_chat.go:153 一带）均在 HEAD。
- 回执：commit 链 @ port/plugin-control-eval-barefree-t1-1（已推 origin）：8dac20c3（harness 第三层缝）+d628fd29（装置四件+RUNBOOK §8 冻结）+d94d4d9d（真栈 run 工件）。续跑会话完成真栈腿（前会话账号中断于实现+装置毕、真栈未跑；本会话自断点续：复验缝单测 ok→推分支→栈协议全链）。
  - **缝 diff**（8dac20c3，test-only 默认 unset 字节不变）：`broadMixObserveFirstWriteGuard` 在 broadMixWriteCommand 判定后、qualified 豁免前——`VIT_PLUGIN_CONTROL_EVAL_SEAM_BARE_WRITE`（与 318 两半同键）开+插件参数写（set_plugin_param 族三变体）→放行；其余 broad-mix 写仍拦；retired 检查在缝前不豁免。配对单测 7 项双向钉（缝开=T1 型写入过/缝关=现状拦/缝外 broad-mix 写仍拦×4/精确值+TrimSpace/retired 不豁免/Invoke 级 pre-journal 双向）全绿；续跑复验 `go test ./internal/harness -run TestBarefree -count=1`=exit 0；域预声明兑现：diff 只增 37 行守卫函数区，不触 Authorize*（273 域）。
  - **真栈（终局 run=arm_barefree_t1_20261108_231036_r2，exit 0 全绿）**：占用 e7fe425c（净空亲核 23:05:43 全零+≤60s 复核 23:07:01 全零+320 排队注记亲证）→起栈（agent=本 worktree 现场构建 SHA256 060C46DE…9D56BB95，重启前后一致断言过；kernel=wt_228 F9E3DA0A… 只读复用）→释放 b7ed5c07；**批内环境重建 1 次（预冻结上限内）**：首跑 arm_barefree_t1_20261108_230751 s0 T3 Waves shell 冷载 JUCE 超时=307 签名#1/310 首试同型 fixture_failure（T1/T2/T4/T5/T6 五案 verified 唯 T3 装载超时；零 LLM 亲证=telemetry 缺席；取证目录全量入库）。
  - **GW-LIVE-T1 双味活体 probe（零 LLM）双 PASS**：open=Pro-R param6 Mix 写 0.35 携 T1 任务原文（「请把 Pro-R 混响的 Mix 设为 35%。」自然混音语境=318 被拦同形）落地+独立回读 1.0→0.35（0.3499…）+零 broad-mix 文本=**第三层缝生效亲证**；off（unset+agent 重启=A-pure 态）=同写被拒+`broad_mix_text_present=true`+typed_text absent=**harness 归层亲证**+值不变 1.0=拦截恢复；318 T3 受害者味（Plates/Damper 0.2→0.7+off 拒）同卷双 PASS。
  - **装置门**：seam_verify_t1 十二面（310 六面+W0 零检索+P0 零 317 段+GW+GWH 建议+GW-LIVE+GW-LIVE-T1）probe 门与语料收口双 PASS；S0 重冻六案 verified+dryround 零 LLM 零遥测行；probe 载体轮 T1 r0（付费不计成绩）pass。
  - **成绩（T1×3 定向轮，318 同判据同冻结件同模型）**：**T1=3/3 pass**——三轮全裸写（write_paths=bare/bare/bare；裸写尝试 2/2/2；落地 3/3；guard_rejection=0；harness_write_guard=0）+单 mutation 精准写+readback 过判据（display 35±2.0）+stop 全 goal_completed+self_probed；**同面单变量对照：BAREFREE-T1 3/3 vs BAREFREE-318 0/3（三轮 guard_blocked_write:harness_write_guard 在卷）——第三层缝打开即转化，卡面判定核心=T1 真裸写能力证实=barefree 线最后案例答案「能」**；七方对照：BAREFREE-T1 3/3=A307 3/3=P pass > BAREFREE-318/B-R1/B312/A-pure 0/3（全 value_encoding_error）。
  - **预算与模型面**：20 任务轮+6 probe=26 LLM 调用/400,733 in/5,013 out/cache 203,776 全在段 caps（36/1.44M/288k）内；telemetry 26 行全 deepseek-flash（产品默认 config）零错误；卡面 ≈¥1 上界内（318 的 1/6 量级）；cost=missing（价目未核不折算）。
  - **三态处置**：pass 分支命中（缝开即转化）——无守卫耦合取证、无 seam_leakage 旗标；guard_blocked_write 分类器本轮零命中（与 318 语料干跑 3 轮全命中形成装置对照）。
  - **端测边界声明**：本卡为实验腿（agent HTTP/ZMQ 真栈 3 任务轮+probe 轮+零 LLM 活体 GW probe），不涉渲染面与用户旅程——无 E2E-WEBUI/JOURNEY 覆盖主张；agent 侧源码改动=harness 第三层守卫许可缝单点（37 行），默认 unset 字节不变，生产启动路径永不设置。
  - **栈登记**：coord/resources/PC-RUNTIME-STACK.md 头两条（占用 23:06/释放 23:15）；TEARDOWN_ALL_ZERO 编排 finally+执行侧复核 23:14:16 双亲证；320（PROQ3-FREE）可承接已注记。
- 验收：（主管回填）
