# YUE-INSTRUMENTAL-SCORE-TRANSFER-1：谱迁移法纯器乐（两段式生成）

- 池序 440；P1；PC；发卡=lead（2026-11-20 深夜）；派发确认=用户 2026-11-20 23:0x（free 车道生效；「生成谱后把 vocal 全部替换至 ins 就能实现纯器乐生成」+ Codex 实验记录在卷）；验收负责人=lead（腿C 人耳=用户）。
- 背景：439 四路证伪后，用户+Codex 独立实验（8 轮，20:19–21:56，`coord/runs/YUE-INSTRUMENTAL-PATH-1/standalone_*`）发现**第五条路成立**：生成谱级 Vocal→Ins 迁移 + 固定谱重渲染（=官方 transfer 工作流，RECON §2d「不需要」判断只对输入谱成立；AR 生成时会自由写人声，迁移必须在生成后）。成功轮=standalone_instrumental_repeat_20261120_214808（conversion.json：vocal 197→0、Ins 78→275 精确守恒、pitch/onset/时值/拍号/和弦保留；用户人耳=纯器乐）。**引擎=现有 yue2.cpp**（yue_local dialect 固定 abc 二段请求），无需 audio.cpp。

## 目标（三腿）

**腿A（Go 移植，零栈）**：
- 把 `coord/runs/YUE-INSTRUMENTAL-PATH-1/scripts-score-transfer-20261120/`（abc_tools/common/compile_score/instrumentalize.py，SHA=a7691116… 在卷）的 ABC 谱迁移逻辑移植为 Go（放 `agent/internal/genapi` 既有 yue_abc 域或同层新文件，与 ConvertMidiToYueABC 同域）。
- **金样对账单测**：以 scripts-score-transfer-20261120/{original.abc→instrumental.abc} 为金样，断言 vocal_notes 197→0、ins 78→275、逐音 pitch/onset/duration、和弦标注与拍号速度保留（对账数据源 conversion.json）。
**腿B（genapi 两段式接线，零栈）**：
- Instrumental 请求路径改两段式：pass1=现行为生成（取 replay abc）→ Go 谱迁移 → pass2=同引擎固定 abc 重渲染（第二个 job，请求体=214808 instrumental.request.json 口径：abc=迁移谱、cot、style 保持 Instrumental 前缀+负条件、固定 seed 记录）。
- job 档案记录两段与转换收据（vocal/ins 音符数守恒表）；最终 WAV/版本归 pass2。
- S6 披露更新：「instrumental=谱迁移法两段式（双倍生成成本，人声抑制由谱面结构保证）」；381 反超卖断言按新口径更新（禁止宣称「单段引擎级抑制」）。
**腿C（真栈+用户听核，lead，不在本执行轮）**：端到端 1 次生成（预算 1–2 次）；用户人耳判零人声→PASS 后 390 fallback 口径升级勘误为「已接通（谱迁移法）」。

## 约束与验收

- 文件域：`agent/internal/genapi`（含 yue_abc/harness 测试）；不改 webui/内核；两段超时与失败分类沿用 372 §8（pass2 短输出=概率类 legW8 已知边界，如实记录不重跑）。
- 验收：`go build ./...`+`go test ./internal/genapi ./internal/harness -count=1` 全绿（含金样对账与两段式 dry-run 假引擎单测）；领取/完成按 §12 走 worktree+main 合入。
- 停止条件：金样对账无法逐音达成（谱解析歧义）→ 停卡上报锚点，不放宽断言。

## 证据索引

- 成功轮：`coord/runs/YUE-INSTRUMENTAL-PATH-1/standalone_instrumental_repeat_20261120_214808/`（original/instrumental 双 job 全卷）
- 失败对照（token 编辑不可行）：`standalone_instrument_token_edit_20261120_215640/LISTENING.md`
- 439 重开依据：本卡背景节+用户裁定；439 裁定将补重开附注。
