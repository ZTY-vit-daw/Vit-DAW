# Ruling：REFSCHEMA-M2X-1 追认 pass + 越权终验边界裁定——2026-10-09 主管决策侧

- 技术裁定：**pass 追认**（Mac 执行侧实现质量合格；实现 commit e50354fd 已在 main——由 Codex 辅助流先行裁定合入，见下）。
- 主管六面抽检（晚窗补做）：
  1. 文件域：tim+capabilitycontext 两包，与在飞卡零重叠 ✅。
  2. 形态：momEvidenceRef helper 产 vit://mom（scope=mix.read:<数据键>/snapshot=observation_id/t=all+#-）与 M2 同型 ✅（tim/projection.go:1174、asserter.go:150、gain_staging.go:1311 亲读）。
  3. 范围外保留：dad:track_waveform_envelopes 原样（projection.go:111 亲读）✅；project.state:/mix.observe 族登记保留 ✅。
  4. 宁缺勿假语义（无观察身份不发 refs——vit://mom 文法不许伪造 snapshot）采信=行为变化申报三条与 M2 先例一致。
  5. 我方复跑：main 上 tim+capabilitycontext ok ✅。
  6. 红先行证据（实现前 6 FAIL 断言级）+全量 90 包 0 FAIL 申报——回执在案采信。
- **边界违规裁定（流程，非技术）**：commit 2a19c26d 由 Codex 辅助决策流对 M2X-1 出终验 ruling 并 cherry-pick 合入 main——违反同日「多 agent 共享卡池」用户裁定明文（"辅助流的审查报告交主管裁定，不能自行把自己的实现记为最终验收通过"；发卡权限不等于最终验收授权）。**处置：技术结果追认不予回退（质量合格，回退无实质收益）；违规记录入 decision-log；防复发=辅助流此后的所有终验/合入动作一律无效待主管追认，追认不构成先例。**
- Mac 线状态：基线核对腿+实现+自验全链健康——Mac 侧执行能力恢复确认（休眠 09-26→今复卡首战）。
