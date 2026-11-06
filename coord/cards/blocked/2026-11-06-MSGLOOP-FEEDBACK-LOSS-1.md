# MSGLOOP-FEEDBACK-LOSS-1：弹回反馈在请求重建中丢失（模型看不到拒绝史）

- 发卡：GLM 主管决策流 / 2026-11-06 晚窗（依据=[262 取证](../runs/MIXTICK-CONFIRM-CONTRACT-1/forensics.md) 域外发现①——L2 层：去重门弹回反馈未进入重建请求，T6-T31 指纹 fd94d36a×26 恒定亲证）
- 派发确认：已确认（主管发卡，L1 台账）；验收负责人：GLM 主管决策流
- 池序 267；目标仓库=主仓 agent（message_loop 重建面）；P2 / 0.5 天 / 依赖=262 取证工件在库（262 裁定 A patch 先行可避让）；L1-L2 / PC；验证可零栈（单测+构造）优先

## 背景

262 取证钉死：模型重请求被去重门弹回后，弹回反馈（拒绝原因）在请求重建时被丢弃（`agent/internal/chat/message_loop.go:10947` 附近 append 逻辑）——模型 26 轮指纹恒定=完全看不到自己被拒的历史。262-A 方案（292 闭合句提前注入）是绕过式修复（直接告知证据足够）；本卡修根治面：**让弹回反馈进入重建请求**，模型可从拒绝史中调整行为。

## 目标

1. 修复 append 丢弃点：弹回反馈随重建请求进入上下文（形式=directives 增量或 history 摘要，按现有 message_loop 惯例最小面）；
2. 单测：构造「请求→弹回→重建请求含反馈」三步断言（t.TempDir，零栈）；
3. 回归：chat 全包零回退；既有去重门语义不变（弹回本身仍正确——修的是反馈可见性不是门行为）。

## 约束

文件域=message_loop 重建面+测试；禁动去重门判据本体；禁触 M3 措辞链；治理红线不变。与 262-A patch 文件域（ccb_model_prompt.go）不相交，可并行。

## 停止条件

修复导致重复反馈膨胀（上下文预算超限）→改为摘要式并记录设计决策；断言失败取证不重跑。

## 领取/回执/验收

- 领取：2026-11-06 晚窗 / origin/main=46497e3bc0b32110e71c42b706cad8eb63206c20 / owner=GLM-5.3 PC 主会话 L2-worker 子代理 / 分支 port/msgloop-feedback-loss-1 / worktree D:/Vit_DAW_wt_267impl / 领取提交=96aa9240（已推 main，远端 owner 已核对）
- 回执：**卡面核心前提被工件证据推翻，转 blocked 上交**（AGENTS §11 / PROTOCOL §2）。零生产代码改动；交付=三步单测锁死反馈可见性回归契约 @ port/msgloop-feedback-loss-1（实现提交 59c12e68，已推）。
  - **推翻点**：262 取证域外发现①表述「弹回反馈在请求重建时被丢弃（10947 append 后丢失）」不成立——弹回反馈**已**随每轮重建请求进入上下文（中性族 History 面）。
  - **证据链**（全部原始工件可复核）：
    1. 262 run 工件 `D:/Vit_DAW_wt_257impl/VitApp/Workspace/Logs/agent_last.log`：T1-T5 `messages=2`；T6-T32 `messages=3` 且新增 `1:user:167` 恒定在场——反馈消息自首次弹回（T5 输出）后的下一轮起每轮进入请求；
    2. 167 = `len("<final_gate>"+issue+"</final_gate>")` 逐字符精确吻合（issue=free_state_reasoning.go project-level 判据句 142 字符）；
    3. 消费机制 `messageLoopNeutralFamilyFeedbackHistory`（message_loop.go:3907，2026-08-25 起 commit 69e499d3 在库，远早于失败 run）保留最近一条 final_gate 进 History；既有测试 `TestMessageLoopNeutralFamilyAssemblyExposesFinalGateFeedback`（message_loop_feedback_test.go，同日入库）已锁该可见性；
    4. 本卡三步单测 `TestMessageLoopBounceFeedbackEntersRebuiltRequest`（真实弹回路径驱动：assembly 无反馈→interpretModelReply 走 dedup gate 弹回 append→重建 assembly 含反馈）在**零生产代码改动**下 PASS——不存在可修的丢弃点。
  - **指纹恒定 fd94d36a×26 的重定性**：非反馈丢失，而是「请求逐字节不变」（每轮弹回 issue 文本相同+ledger 无新观察+directives 恒定）→ 模型确定性重采样同一回答。**真实缺口**=模型看不到累积弹回史（第 N 次弹回与第 1 次在请求上无差别；History 同时裁掉 assistant 轮，模型看不到自己此前的重复输出）——属新修复面（弹回计数/累积史），超出本卡「修反馈可见性」目标域，**交决策侧裁定是否另开卡**。
  - 测试（全部 exit 0）：`go test ./internal/agentloop -run 'TestMessageLoopBounceFeedbackEntersRebuiltRequest' -count=1`；回归 `go build ./...`、`go test ./internal/chat -count=1`（121s ok）、`go test ./internal/agentloop -count=1`（4.7s ok）零回退。去重门判据与弹回行为零改动（弹回断言在测试内）。
  - 工件：worktree D:/Vit_DAW_wt_267impl @ port/msgloop-feedback-loss-1（claim 96aa9240→test 59c12e68）；本状态提交 @ coord-close-267。零真栈、零 LLM 计费。
- 验收：（主管回填：裁定关卡或按新事实开「累积弹回史可见性」修订卡）
