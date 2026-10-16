# GENAPI-REALVERIFY-1：genapi 真源端到端验证（零代码+首跑）

- 发卡：Mac 决策侧 / 2026-10-16 晚（依据=GEN-API-ADAPTER-1 pass ruling 留口"真源验证小卡"+用户 2026-10-16 gate 解锁：key 已配好）
- 派发确认：已确认（用户 2026-10-16 晚「继续发卡」批准本线报告§三.③立卡建议）
- 验收负责人：Mac 决策侧先行裁定+PC 主管复核留档
- 池序 125；**前置=GEN-API-ADAPTER-1（✓ 2026-10-12 pass 合入，适配器链路已被模拟器 17/17 门证明）**
- 归属：Mac 生成线（生成集成面）；P2 / 0.5 天
- 模型分级：L1 / GLM 或 flash 执行会话

## 目标

用真实外源 provider 首跑 `gen.generate` 端到端，证明 GEN-API-ADAPTER-1 的诚实边界补齐（模拟器→真源）：

1. **配置**：`~/.vit/genapi_providers.json` 按 `agent/internal/genapi/config.go` ExampleConfig schema 填入用户已配好的真源凭据（id/dialect/base_url/api_key/model；dialect 三型 suno_relay/async_poll/sync_binary 按真实 provider 选）。**key 材料只进用户级配置文件，不入仓不入卡面不入工件**（genapi SecretFree 先例；卡面只记 provider id 与 dialect）。
2. **真源首跑**：真栈起 agent → `gen.generate`（经 harness 工具面或 gen_api_adapter_smoke 同模式）→ 真源 HTTP 200 → 产物落盘（WAV 魔数断言）→ `bridge_ingest_generated_asset` 落轨断言（clip_id/asset_ref）→ job 终态 done。
3. **失败分类如实**（AGENTS §8）：真源失败按 genapi 错误分类（Auth/RateLimit/Quota/Network/Provider...）记录并镜像 job 终态——失败是合法结果，**不冒充成功不烧 quota 重试**；同一确定性失败两次后停，记录分类上交。
4. 工件入 `coord/runs/GENAPI-REALVERIFY-1/`（报告含请求参数/不含 key/真源响应摘要/产物哈希/失败分类如适用）。

## 文件域

- `~/.vit/genapi_providers.json`（用户级配置，不入仓）；
- `coord/runs/GENAPI-REALVERIFY-1/**`（工件）；
- `scripts/gen_api_adapter_smoke_mac.sh`（仅在需加 `-RealSource`/`--provider` 参数承载真源腿时最小扩展；能以现有参数+环境变量承载则零脚本改动）。
- **零生产代码**——需改 agent/internal/genapi/** 即越域停止上交。

## 验收标准

- 真源链路 exit 0（200+产物魔数+ingest 断言+终态 done）；或真源失败时按 §8 分类如实记录（卡面"不可行→如实记录"分支合法收卡，区分 no_key 环境阻塞 vs provider 真实错误）。
- 日志/工件/卡面零 key 材料（SecretFree 断言沿用）。
- 全量 `go test ./... -count=1` 0 FAIL（若零代码零脚本改动可声明豁免理由）。

## 停止条件

- 用户提供凭据与三 dialect 均不匹配（schema 装不下）→ 上交换 provider 形态；
- 真源连续同类失败两次 → 分类记录后停，上交裁定。

## 端测边界

agent HTTP/harness 工具面数据断言；无 webui/前端改动；真源音频质量不评估（可听性留给后续 AUDITION 卡）。

- 领取：2026-10-16 晚 CST / origin/main 3cd3f325 / GLM-5.3 L2 Mac 执行侧 ZCode 会话（本卡会话）/ 实现分支 port/genapi-realverify-1（零生产代码，脚本最小扩展+工件）/ worktree Vit-DAW-genapi-rv1 / 协调 worktree Vit-DAW-genapi-rv1-coord 分支 coord/genapi-rv1-claim / 领取提交 5ba87c42（本行哈希回填提交随后）
- 回执（2026-10-16 22:00 CST，blocked 上交）：**真源生成调用未执行——前提"key 已配好"被推翻**。用户 2026-10-16 21:49 在执行会话确认"目前的确还没有明确的音乐生成源，我们还没定"（生成源选型回决策流讨论）；核实 `~/.vit/genapi_providers.json` 不存在、无 `VIT_GENAPI_*` 环境变量（`~/.vit/config.json`=agent 的 deepseek LLM 配置非音乐源）。**§8 失败分类=no_key 环境阻塞**（非 provider 真实错误；零外呼零 quota；"同类失败两次即停"不适用——无生成调用轮次）。已交付（选型定后即可续跑）：①脚本最小扩展 `--real-source` 真源模式（单次调用零重试+诚实分类 provider:<class>:<http>+key 不打印/就地脱敏/终扫零出现门）@ port/genapi-realverify-1（36b756e1+33c551cc 已推）；②回归轮 **run1 exit 0 21/21 门全绿**（重构零破坏；内核 099327af 本卡 worktree 从基线 5ba87c42 新建构建，非旧二进制；agent 93fd00b2）；③**run2 no_key 实证 exit 1 + env:no_key**（未启栈）——卡面"区分 no_key vs provider 错误"机制实证；④用户级模板 `~/.vit/genapi_providers.json.template`（三 dialect 占位+选型指引，不入仓）。工件 `coord/runs/GENAPI-REALVERIFY-1/`（README=报告+两轮证据，零 key 材料——本卡从未接触真实凭据）。全量 go test 豁免理由：`agent/` 对基线零 diff（实际跑 ./internal/genapi/ ok）。端测边界：agent HTTP 工具面断言；无 webui/前端改动；音频质量不评估（留 AUDITION 卡）。占用登记 48a54f72→释放记录随后同批；续跑命令见 run README。**[等待用户:生成源选型与使用方式（决策流讨论后填 ~/.vit/genapi_providers.json 重派或按 README 续跑）]**
- 验收：（裁定文件 / 验收 commit）
