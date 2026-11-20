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

## 领取（2026-11-20）

- 领取时间：2026-11-20 深夜（PC，free 车道子代理 scorexfer440）
- 领取时 main HEAD：`7e77c7b71c9a5bd3bf3ebb6dffc4d4aa3d4d710d`
- owner：免费车道执行子代理（GLM-5.3 flash 产线 scorexfer440）／PC（Windows Git Bash）
- 分支：`scorexfer440`；worktree：`D:/Vit_DAW_wt_pc440`（基线=领取时 main HEAD，干净检出）
- 领取时 main 工作树 diff（与本卡无关、不重叠）：CURRENT-STATE.md / VitApp/Workspace/Settings/Settings.xml / VitApp/Workspace/default_project.xml / tracktion_engine（4 files changed, 4360+/4487-），均为领取前既有改动，不整理不覆盖
- 本轮范围：腿A+腿B 零栈（金样对账单测+两段式 dry-run 假引擎单测）；腿C 真栈+人耳留 lead。文件域 `agent/internal/genapi` + `agent/internal/harness`（gen_generate.go 接线）+ 本卡 coord 文件
- 金样前置验证：仓库 py 脚本重跑 original.abc→迁移谱与金样 LF 逐字节一致、receipt 197→0/78→275/unaltered 78/affected=[]（证据 `coord/runs/YUE-INSTRUMENTAL-SCORE-TRANSFER-1/py_repro_check/`）——Go 对账以逐字节一致为目标
- 执行进度日志：`coord/runs/YUE-INSTRUMENTAL-SCORE-TRANSFER-1/PROGRESS.md`


## 回执（执行完成待验收，2026-11-20）

- 执行：scorexfer440（worktree D:/Vit_DAW_wt_pc440，分支 scorexfer440）；腿C 真栈+人耳未动，留 lead。
- 提交：cab1cadb（腿A：abc_tools/compile_score/instrumentalize 三件精确 Go 移植 4 文件）+ 9354bc99（腿B：两段式接线+假引擎单测+披露更新 9 文件）；merge 05be3d6f 合入 main（无冲突，合入后 main 上复跑两门仍绿）。
- 验收命令+退出码（worktree 与合入后 main 双跑一致）：`cd agent && go build ./...` exit 0；`go test ./internal/genapi ./internal/harness -count=1` exit 0（两包 ok）。
- 金样对账（TestYueInstrumentalizeGoldPairBytes/PerNote）：迁移输出与金样 instrumental.abc **逐字节一致（LF）**；vocal 197→0、Ins 78→275、unaltered 78、affected=[]、bpm 87/bars 84、nominal_seconds 精确；逐音 pitch/onset/duration=源 Vocal+Ins 有序并集；和弦/拍号/速度保留——**达成**，未放宽任何断言。
- 两段式要点：pass1=条件化请求原样生成（人声=迁移源，非失败）→ replay sidecar 取生成谱（缺失=classified response_parse_error，pass1 工件留盘）→ Go 谱迁移 → pass2=同引擎固定迁移谱重渲染（214808 口径：abc 逐字节[builder 改 verbatim]、style 保持、cot 按迁移谱 Vocal 和弦重推导、歌词官方纪律重推导/用户词保留）；yueRequestBuilder/迁移谱全链路零逐音改写。收据入档：genGenerate out["two_stage"] + genJobRecord.TwoStage（双 job id+守恒表+固定 seed+final_version_source=pass2）；每段各自 provider 超时预算。
- 披露文案变更（S6/381 新口径）：必含 two-stage score transfer / double generation cost / structural / pass1 may still sing / user listening gate；禁含 validated instrumental / engine-side muting / guaranteed by the engine / single-stage engine-level suppression；Limitations 显式否定句「no single-stage engine-level suppression is claimed」；381 回归断言已按此重写（yue_prepare_test.go）。
- 测试卫生：假引擎/金样全部内嵌或 t.TempDir()，未触碰 sealed/holdout、源码树运行态与真实栈；TestGenJobsLifecycleYueLocal 按两段式更新 fixture 与断言（覆盖面扩大，原断言未放宽）。
- 遗留/风险：①腿C 未做——端到端真栈 1–2 次生成+用户人耳判零人声前，本卡不构成「已接通」宣称；②pass1 生成谱若偏离 native dialect（引擎侧概率输出）→ 两段式 classified 失败（invalid generated abc），如发生属 372 §8 概率类，如实记录不重跑；③两段=双倍生成成本已入披露；④go vet shm_windows.go 既有 unsafe.Pointer 提示（非本卡域，未动）。
- 证据：`coord/runs/YUE-INSTRUMENTAL-SCORE-TRANSFER-1/`（PROGRESS.md 全日志+py_repro_check 金样复跑+两个生成脚本）。
