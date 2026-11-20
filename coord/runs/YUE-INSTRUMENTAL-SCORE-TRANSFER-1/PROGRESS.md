# PROGRESS — scorexfer440 (YUE-INSTRUMENTAL-SCORE-TRANSFER-1)

任务：腿A instrumentalize 移植 Go + 金样对账单测；腿B Instrumental 两段式接线 + dry-run 假引擎单测 + S6/381 披露更新。腿C 真栈留 lead。禁真实栈、禁 git reset/clean。

## 步骤日志

- [x] 步骤0 入场阅读完成（2026-11-20）。已读：任务卡全文、scripts-score-transfer-20261120 四个 py + original.abc→instrumental.abc 金样 + conversion.json、成功轮 214808 的 plan.json / original.request.json（pass1 无 abc 自由生成）/ instrumental.request.json（pass2 口径：abc=迁移谱、cot=full、style=Instrumental 前缀+负条件、seed=23、lyrics 为空）、original.replay.json（关键发现：pass1 无 abc 时 replay JSON 的 abc 字段=引擎生成的谱，1904B=金样 original.abc）、yue_abc.go / yue_prepare.go / dialects.go / engine.go / genapi.go / engine_test.go / yue_prepare_test.go / harness gen_generate.go。
- [x] 步骤0.5 金样可复现性验证：用仓库内 py 脚本（runpy -I 隔离引导，SHA 与 conversion.json 一致：instrumentalize=a7691116…）重跑 original.abc 迁移 → 输出与金样 instrumental.abc LF 内容逐字节一致（diff GOLD_MATCH_LF）；receipt 197→0、78→275、unaltered 78、affected=[]。产物：py_repro_check/{score.abc,transfer.json}。结论：Go 移植以「与金样逐字节一致（LF）」为对账目标。
- [x] 步骤1 worktree+领取：D:/Vit_DAW_wt_pc440 分支 scorexfer440，基线 7e77c7b7；卡面「领取」节写入并提交 main（commit cd6032c7，只含本卡 coord 文件）。
- [x] 步骤2 腿A Go 移植完成（worktree）：
  - agent/internal/genapi/yue_native_score.go：yq 精确分数 + abc_tools.py parse 严格移植（fail-closed：调号/拍号/和弦/时值/八度/延音线全部按原版拒绝面）。
  - agent/internal/genapi/yue_native_compile.go：compile_score.py compile_events 移植（事件→ABC 文本，内部独立重解析 roundtrip）。
  - agent/internal/genapi/yue_transfer.go：instrumentalize.py convert_score 移植（vocal 优先重叠策略、和弦保留、五项迁移后不变量）+ YueTransferReceipt（conversion.json 口径）。
  - agent/internal/genapi/yue_transfer_test.go（gen_gold_test.py 生成，金样对逐字节嵌入）：4 测试全 PASS——金样对账字节一致、vocal 197→0、Ins 78→275、unaltered 78、affected=[]、bpm87/bars84、逐音 pitch/onset/duration、和弦/拍号/速度保留、fail-closed 边界。
  - 修复记录：①voice.Time 零值 yq{0,0} panic→初始化 yqInt(0)；②compile 片段窗口 a 用错 min→max（Python max(start,onset)），修复后金样通过。
  - 长命令：cd D:/Vit_DAW_wt_pc440/agent && go test ./internal/genapi/ -run 'TestYueInstrumentalize|TestYueCompileEvents' -count=1 → exit 0。
  - 下一步：腿A 提交分支；腿B 两段式（yue_twostage.go + 假引擎 dry-run 单测 + S6/381 披露更新 + harness 接线）。

- [x] 步骤3 腿B 两段式完成（worktree）：
  - agent/internal/genapi/yue_twostage.go：YueGenerateInstrumentalTwoStage——pass1=条件化请求原样生成（人声=迁移源，非失败）→ 从 replay sidecar 提取生成谱（replayABCFromResult，缺失=classified parse error 且 pass1 工件留盘）→ YueInstrumentalizeTransfer → pass2=同 cfg 固定迁移谱重渲染（214808 口径：abc 逐字节、style 保持、cot 按迁移谱 Vocal 和弦重推导、歌词按官方纪律重推导或用户词保留）→ ReceiptMap（双 job id、守恒表、固定 seed 记录、final_version_source=pass2）。
  - agent/internal/genapi/yue_twostage_test.go：假引擎（/synth+/job+multipart replay+audio）dry-run 三测试——两 job 断言、pass2 abc 逐字节=迁移谱、cot=full、seed/lm_seed、收据入档；无 replay abc 与不可解析生成谱两条 classified 失败路径。
  - dialects.go：yueRequestBuilder abc 改 verbatim（保留空值守卫）——引擎请求回显与准备/迁移谱逐字节一致（214808 锚点）；Limitations instrumental 文案改为两段式口径。
  - yue_prepare.go：instrumental 分支披露按 S6 新口径（style 动作、vocal-rest 动作、新增 route 动作「two-stage score transfer … double generation cost … pass1 may still sing … structural … user listening remains the acceptance gate」）；YuePrepareResult.LyricsUserProvided 新字段供 pass2 歌词纪律。
  - yue_prepare_test.go：381 反超卖断言按新口径更新——必含 two-stage score transfer/double generation cost/structural/pass1 may still sing/user listening gate；禁含 validated instrumental/engine-side muting/guaranteed by the engine/single-stage engine-level suppression；Limitations 必含「no single-stage engine-level suppression is claimed」显式否定句。
  - harness/gen_generate.go：yue_local+Instrumental 路由到两段式（每段各自 provider 超时预算，不包单一 deadline）；out["two_stage"]=收据；gen_jobs.go：genJobRecord.TwoStage 字段+genJobRun 归档+genJobsStatus 展示。
  - harness/gen_jobs_test.go：TestGenJobsLifecycleYueLocal 按两段式更新（假引擎双提交+replay 回显模型；pass1 断言=原条件化请求面保持、pass2 断言=迁移谱逐字节+cot/style；status+归档 JSON 断言 two_stage 收据与守恒表）——原测试断言未放宽，覆盖面扩大。
  - 长命令与退出码：cd D:/Vit_DAW_wt_pc440/agent && go build ./... → exit 0；go test ./internal/genapi ./internal/harness -count=1 → exit 0（两包 ok）。go vet 仅 shm_windows.go 既有 unsafe.Pointer 提示（非本卡文件域）。
- [x] 步骤4 分支提交：cab1cadb（腿A 4 文件）+ 9354bc99（腿B 9 文件），分支 scorexfer440。

## 失败/环境分类区

- 步骤2 两次代码缺陷（voice.Time 零值 panic、compile 片段窗口 a 用错 min/max）本地单测捕获并修复——实现缺陷，非环境。
- 步骤3 假引擎测试三轮断言修正（JSON 数字 float64、生成器 TrimSpace 尾换行、fixture 缺 prompt）——测试面修正，非环境。
- 步骤3 验收门首次运行 TestGenJobsLifecycleYueLocal 失败：假引擎 replay 回显 "X:1"（1 行非法谱）触发两段式正确 fail-closed → 属测试 fixture 未跟上新路线，按两段式更新 fixture 与断言（未放宽任何原断言）。非环境中断，不计有效轮次损失。

## 终态

DONE（2026-11-20）。腿A+腿B 零栈完成；金样对账 197→0 / 78→275 逐字节达成；腿C（真栈 1–2 次生成+用户人耳）留 lead。遗留/风险见卡面回执节。
