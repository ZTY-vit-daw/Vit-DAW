# Ruling：GEN-API-ADAPTER-1 pass（2026-10-12，决策侧）

- **裁定：pass**。合入 main = cherry-pick 1e23fa8f（实现+脚本+工件；SMOKE_TESTS.md 与 DISCLOSE 条目双保留合并）；卡片状态链执行侧已直推 main。
- **决策侧独立复验（Mac 决策会话亲核，非转述）**：
  - 文件域亲核：agent/internal/genapi 新包（7 文件）+harness 接线（gen_generate.go+测试+harness.go）+catalog 两条 spec+scripts+coord 工件——**零内核改动兑现**（产物走既有 aigc_register_job+bridge_ingest_generated_asset，GEN-MAC 实证面）。
  - 代码抽验：错误分类十一类实存（Auth/Config/Moderation/Network/NotFound/Parse/Provider/Quota/RateLimit/Timeout/UnsupportedFormat——grep 亲核）；音频魔数嗅探五族（RIFF+WAVE/ID3/OggS/fLaC/MThd）fail-closed；跨域凭据剥离与 providers 工具不回显凭据（单测+真栈门双断言申报）。
  - **独立复跑**：`go build` exit 0+genapi/harness 定向 ok（3.0s/12.7s）+**全量 93 包 0 FAIL exit 0**（本机亲跑）。
  - run4 工件亲读：17/17 门 PASS（模拟器承载 suno_relay 契约链路+ingest 落轨+多候选 take）；sim_shape_verdict 在案。
- **诚实边界裁定（本卡核心价值）**：真栈轮的 suno 面=本地模拟器（按 new-api 公开契约逐字段仿真）——证明的是适配器链路正确性，**非真实 Suno 端到端**；外源无凭据=环境阻塞如实申报（§8），不冒充。有凭据后填 config 即跑同链路零代码改动。**风险四条入档采信**（非官方接口断代/账号风险/条款合规/中转站第三方），官方 API 型为天然降级路径（sync_*/async_poll 方言承载）。
- **用户须知（决策侧上呈）**：真实外源验证需用户提供 API 配置（`~/.vit/genapi_providers.json` 或 VIT_GENAPI_* 环境变量）——建议作为后续真源验证小卡或用户手测项，不影响本卡 pass（卡面验收"不可行→如实记录"分支已按环境阻塞正确行使）。
- 统一化收口项（gen_api profile 注入块）按卡面留待 GEN-CTX-INJECT-1 合入后，接口已留（GenerationRequest 归一载体）。
- 后续解锁：生成集成面首张落地；GEN-CTX-CONSUME-1 依赖（生成集成面首张）满足条件由主管复核。
