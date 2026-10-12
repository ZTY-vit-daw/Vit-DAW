# Ruling：GEN-PLUGIN-SHELL-DESIGN-1 pass（2026-10-12，Mac 决策侧先行裁定）

- **裁定：pass**。设计基线工件 cherry-pick a51c3509 随本裁定链合 main（五件+风险表+README）；done 移动 e18fd9db 执行侧已直推。
- **决策侧审阅（亲读，非转述）**：
  - 卡面验收八项逐条对照齐（通道契约/异步作业模型/模板+GUI/PCA 接入面/拆卡 6 张/与统一化件三对齐/风险表）。
  - **与 genctx 实现的咬合点亲自验证**：件三 §1"模板=USER 块编排器、自动块恒由 genctx 渲染器独家产出"与已合入的 RenderGenerationPrompt 契约（profile 只裁不能加料、无自定义模板文本入口）完全互锁——两设计零冲突，单一渲染器原则在产品消费方形态下保住。
  - 关键裁定采信：三段式架构（壳零协议/零密钥/零自动块）；通道分层 T0-T3 各司其职；作业模型复用 VitAIGCJobRuntime 零结构改动（仅 job_state 词表扩展，§11 兼容）；轮询 at-least-once 不依赖 PUB；scene 落实（free_creation 不落轨/experiment_control 不进 GUI）；PCA 最小扩展（generation_shell family+coverage v1 空集+装载与产物分工）；实现基=VST3 SDK 直实现+VSTGUI（许可路线双许可闭源可行入风险表）。
  - 拆卡 6 张（GENSHELL-CPP→HOST→AGENT（依赖 INJECT 已合入✓）→GUI/PCA/AUDITION 并行）达可直接发卡程度。
- 零生产代码/零源码 diff/零真栈（引用既有 run 证据）与卡面一致。
- 后续：实施六卡入池归主管发卡流（Mac 生成线排程序列 CPP→HOST→AGENT 优先）；SDK 许可与签名分发两项用户须知已在风险表（对外分发前需 Developer ID 决策）。
