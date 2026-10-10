# AIGC-DRUM-GEN-1：鼓点 pattern 生成链（窄面版）【中枢迁移卡】

- 发卡：GLM 主管决策侧（迁移）/ 2026-10-10 夜
- 派发确认：已确认（迁入依据=用户 2026-10-10 三线放行+Mac=生成线裁定；来源=中枢 queue/todo/2026-10-05-AIGC-DRUM-GEN-1-drum-pattern-generation.md（2026-10-05 冻结）+GEN-CAP-RECON-1 §2.1 重核结论随卡修订）
- 验收负责人：GLM 主管决策流
- 池序 59；目标仓库=D:\Vit_DAW
- 优先级 / 预估 / 依赖：P2 / 1 天 / **GEN-MAC-BASELINE-1 合入后领取**（Mac 生成线首批第二张；PC 执行亦可）
- 模型分级：L2 / GLM 执行会话（生成编排涉 prompt 约束与确认链）

## 目标（中枢卡原文+重核修订）

自然语言一句话（固定词表内）→ LLM 生成鼓点音符序列 → `midi.write_clip_notes`/`add_midi_notes_bulk` → 既有 RiskConfirm 确认卡 → 写入鼓轨 → 回读一致。**管道四环中写入/确认/回读三环现成（K2 注册+真栈实证比冻结时更牢），本卡只新做"生成编排"一环，全 agent 侧、内核零改动。**

## 重核修订（相对中枢原卡，GEN-CAP-RECON §2.1）

1. **验收面裁剪为数据级**：写入+`get_midi_clip_notes` 回读与生成序列一致即收卡；**"写入后出声"环显式挂 A1**（挂 instrument 无 agent 通道——A1 形态是用户裁定项，A1-MAC-FORM-1 供事实），出声腿不入本卡验收。
2. 生成编排形态：固定 prompt 模板+固定种子（同输入同输出可复现）；参数封闭词表（style/complexity/density），词表外走自由对话拒绝窄面接管；音符映射按 MIDI-RECON-1 结论+越界 fail-closed（音域/密度/长度上限→明确报错不写入）；生成参数/种子/候选入回执（FREE_STATE_IMPROVENMENT_EXECUTION_RECEIPT_V1 schema 承载）。
3. 演示级窄面性质注记保留：不依赖/不改动汇合段正典编曲前置，后续由汇合段重构吸收，不冒充正典。
4. 单候选收卡（多候选+铺底声部归 AIGC-DRUM-GEN-2，仍冻存中枢待 -1 收口迁移）。

## 验收标准

- 单测：生成约束（词表/种子复现/越界 fail-closed）+合成测试（固定输入→固定音符序列）。
- 真栈烟测：`dev_agent_smoke -Scenario drum_gen`（SMOKE-SCEN-RANGE-1 参数模式先例）——一句自然语言→确认卡→写入→回读一致，exit 0（数据级）。
- 全量 `go test ./... -count=1` 0 FAIL+gofmt 净。

## 停止条件

MIDI 写入/回读命令面与锚点不符（K2 后再变）→ 锚点上交。

- 领取：（时间 / origin/main hash / owner / 分支 / worktree / 领取提交）
- 回执：（commit / run ID / 回读一致证据 / 端测边界声明）
- 验收：（裁定文件 / 验收 commit）
