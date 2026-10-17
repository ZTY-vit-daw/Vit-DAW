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

- 领取：（时间 / origin/main hash / owner / 分支 / worktree / 领取提交）
- 回执：（commit / run ID / 端测边界声明）
- 验收：（裁定文件 / 验收 commit）
