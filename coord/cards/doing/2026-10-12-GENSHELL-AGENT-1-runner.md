# GENSHELL-AGENT-1：agent 执行腿（实施③）

- 发卡：Mac 决策侧 / 2026-10-12（依据=件五 卡3）
- 派发确认：已确认（用户 2026-10-12「可以发卡」）
- 验收负责人：Mac 决策侧先行裁定+PC 主管复核留档
- 池序 97；**规格全文=[件五 卡3](../../runs/GEN-PLUGIN-SHELL-DESIGN-1/05-impl-card-specs.md)（权威）**
- 归属：Mac 生成线；P1 / 1-2 天 / **GEN-CTX-INJECT-1 合入（✓已合入）+GENSHELL-HOST-1（✓ 2026-10-16 pass 合入 db84003e——双前置满足，可领；锚点=HOST-1 ruling 对卡 3 交接：providers 由 runner 自填、job upsert 走 aigc_register_job 同面（H4 实证）、gen_shell.pending 应答形态以 H3 断言为准、壳内部 genshell_state_sync 按未知 messageID 丢弃）**
- 模型分级：L2 / GLM 执行会话

## 修订（2026-10-17 晚，决策侧）：管理面增 gen_shell.submit + provider 槽说明（用户裁定"agent 对话流自动行使调用"+网页端主路线）

- **新增工具腿**：`gen_shell.submit`（RiskDirect 级）——agent 对话流提交生成请求→内核 HOST 服务队列（提交面=HOST-1 debug_submit 钩子产品化，内核侧零新增）→runner 走既有领取链。对话中"生成一段…"即触发完整 GenShell 链（队列→runner→provider→ingest 落轨→壳参数镜像）。
- **provider 槽说明（不改变本卡验收）**：本卡烟测继续用回环模拟器 provider（零依赖）；生产 provider 按网页端主路线=边车（131 设计件定稿后实现卡以 genapi provider 形态交付），fal/官方 API 通道配置层保留。genapi 内部抽象不拆（decisions/2026-10-17-gen-web-first.md）。
- 验收标准追加第 5 条：对话流提交探针——经 gen_shell.submit 提交→gen_shell.pending 应答 queued→runner 领取（模拟器）→终态 done 链路断言。

## 要点摘录（以规格为准）

agent/internal/genshell 新包——后台 runner（2s 轮询 gen_shell.pending→claim→genctx 渲染（profile=gen_plugin）→genapi.Generate→ingest 落轨→终态 upsert）；dry-run/取消/孤儿清扫/失败分类透传；gen_shell 管理工具（RiskDirect）。验收=真栈烟测（回环 provider 模拟器先例）：注入块在场证据断言（MUSIC CONTEXT/CONSTRAINTS 在模拟器收到的请求体中）+产物落轨+多候选 take+取消/失败/free_creation（零注入块+不落轨）负探针+孤儿清扫+密钥零暴露断言+全量 0 FAIL+避让声明。停止条件=INJECT 形态与假设不符→回设计侧修订对接清单（件三 §5——注：INJECT 已合入，领取时以 agent/internal/genctx 实际签名为准回填）。

- 领取：2026-10-17 21:38 +0800 / origin/main=b532ed3f44090e7b4f3f9300846710326544e155（领取前 fetch 亲核=本地实现基线一致） / owner=GLM-5.3 L2 Mac 执行侧 ZCode 会话（本卡会话，用户口令指定领取） / 分支 port/genshell-agent-1 / worktree /Users/timozty/Documents/Vit-DAW-genshell3 / 领取提交=coord/genshell3-claim（本提交）。领取时核对：卡在 todo、派发已确认；双前置满足（GEN-CTX-INJECT-1=0ba2d75d、HOST-1=db84003e 均为 origin/main 祖先亲核）；锚点亲核=genctx RenderGenerationPrompt(snap,userFreeText,RenderProfile{ConsumerID}) 签名+profile 枚举+scene 三枚举+GuardWriteCommand（bridge_ingest_generated_asset 在守卫表）与卡面假设一致、gen_shell.pending 应答形态以 H3 断言为准（{status,jobs:[{job_id,state,cancel_requested,request}],providers:[]}，providers v1 恒空由 runner 自填）、HOST-1 交接四锚点（providers runner 自填/job upsert 走 aigc_register_job 同面/pending 应答 H3/壳内部 genshell_state_sync 丢弃）全命中——停止条件不触发。doing 池核对：LOUDNESS-B 修订腿只动 scripts/run_free_state_d1_smoke.ps1（chat/harness 域已按裁定 5e9d2f65 先落）；SENDROUTE 域含 harness（CCB view 目录区）与本卡 harness.go invokeLocal 分发区（:2602 邻域）不同区同文件（CommandDispatcher.cpp 不同区先例）；GEN-SUNO 零生产代码。内核零改动：烟测复用 HOST-1 内核二进制（genshell2 worktree 构建 Oct 16 22:11，HEAD=dce49fac）并记录哈希（AGENTS §9）。
- 回执：（commit / run ID / 端测边界声明）
- 验收：（裁定文件 / 验收 commit）
