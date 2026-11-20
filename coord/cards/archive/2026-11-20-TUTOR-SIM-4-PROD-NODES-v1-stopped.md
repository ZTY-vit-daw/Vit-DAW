# TUTOR-SIM-4：生产 YAML 节点接入导师循环（brief/材料/音频）

- 池序 438；P0；PC；发卡=lead（2026-11-20 早窗）；派发确认=用户「flash 车道」；验收负责人=lead。
- 前置：435/436/437 已合 main（437 真栈手测由 lead 并行进行，不阻塞本卡零栈腿）。
- 背景：tutor_sim 当前是算术三步实验（`coord/experiments/tutor-sim-1/tutor_sim.yaml`，schema `vit.tutor_sim.v0`）；生产乐局节点的 teaching brief/材料/音频尚未接入（436 裁定边界）。

## 目标

把导师循环从算术实验升级为**生产乐局节点驱动**的一课（单关卡、节点粒度）：

1. **腿A（服务端+YAML 内容）**：
   - 扩展实验 schema 至 `vit.tutor_sim.v1`：每步新增 `brief`（节点教学简报，多行文本）、`materials`（材料列表：`{kind: audio, title, path}`，path 为工程/装置内相对路径）。v0 YAML 仍可解析（缺省 brief/materials 为空，UI 不渲染卡）。
   - 从生产乐局关卡产出一份真实节点 tutor YAML（新实验装置 `coord/experiments/tutor-sim-2/`）：优先复用 EDU-LEVEL-CONTENT-1 定稿关卡与既有材料音频资产（VIT_EDU_LEVELS_DIR 装置），不自造新音频、不改生产 levels 冻结文件。
   - `tutor_sim.go` 的 start/chat 响应在 workflow_data 中携带 `brief` 与 `materials`（随当前步）；expect 判定仍服务端精确匹配。
2. **腿B（UI 控件）**：
   - TutorStepCard 增加 brief 展示（折叠/展开，默认收起或摘要，不撑爆对话流）。
   - materials 中的音频用既有 `agent/webui/src/widgets/AudioPlayer`（WIDGET-KIT-1）以对话流卡片渲染，点击可播；无 materials 不渲染卡。
   - TutorProgressCard、completed 禁用、普通工程零 UI 变化等 437 行为全部保持。

## 约束

- 无命令词；不显示 expect/答案；不改冻结 production design schema 与 levels 冻结文件；不改 437 既有断言语义（只加不减）。
- 文件域：`agent/internal/chat/tutor_sim.go`(+tests)、`coord/experiments/tutor-sim-2/`、`agent/webui/src/project/`（TutorProgress/TutorStep 卡与挂载）、`agent/webui/src/widgets/`（如需只读复用）、`scripts/run_tutor_sim_smoke.ps1`（扩展断言）。
- 领取：新 worktree（建议名 pc438-tutorprod）+ 分支；领取记录（HEAD/status/diff-stat）同步 main 后开工；完成同样走 main 合入。
- 不启真实栈；真栈 smoke/手测窗口由 lead 开。

## 验收（零栈）

- Go：v1 schema 解析、v0 兼容、brief/materials 进 workflow_data、expect 判定回归全绿（`go test ./internal/chat` 相关 + 全量 build）。
- vitest：brief 渲染/折叠、AudioPlayer 卡渲染、无 materials 不渲染、437 既有用例不回归；`npm run test` + `npm run build` exit 0。
- smoke 脚本扩展：断言 start/chat workflow_data 含 brief/materials（headless 装置自测，不需要真实栈）。
- 若生产关卡节点结构无法映射到步进模型（如检查非文本 expect 可判定），停手上报，不硬塞。

## 停止条件

证据推翻前提（生产 YAML 无可复用 brief/音频结构、或需要动冻结 schema）即停卡上报，附锚点。

## 勘察停卡（2026-11-20，tutorprod438）

入场勘察阶段证据推翻卡前提，按卡面停止条件停卡上报。**未建 worktree、未领取实现、产品文件域零改动**（tutor_sim.go / tutorsim / webui project / smoke 脚本 / coord/levels / 冻结 schema 全部未触碰）；证据入档 `coord/runs/TUTOR-SIM-4/`。完整证据：[coord/runs/TUTOR-SIM-4/README.md](../../runs/TUTOR-SIM-4/README.md)。

### 证据 1：无「既有材料音频资产」可复用（音频腿前提被推翻）

腿A 要求 materials=`{kind:audio,title,path}` 且「优先复用…既有材料音频资产（VIT_EDU_LEVELS_DIR 装置），不自造新音频」。实测全部落点：

- VIT_EDU_LEVELS_DIR 两落点均仅 MIDI：`coord/levels/first-run-twinkle/assets/` = `theme_twinkle_v0.mid` + `theme_twinkle_corrupted_v0.mid`（各 419B）+ MANIFEST（level.yaml:34-50）；`~/.vit/levels/first-run-twinkle/` 与仓库副本 level.yaml 字节一致（diff 验证）。
- `coord/levels/first-run-twinkle/assets/MANIFEST.yaml:6-7` 版权口径明示「不使用现有录音」；`:34-37` deferred 仅谱面 PDF，无音频登记。
- 生产乐局卡资产 kind 词表只有 `midi_reference`（`agent/internal/edu/lesson_card.go:692`），无 audio 类资产位；.mid 不能作为 kind:audio 材料供 `<audio>`/AudioPlayer 播放。
- 仓库 git 追踪音频仅 7 个校准/测试 wav；其余 wav/mp3/ogg 全为运行工件（coord/runs、artifacts、.vit_agent）、JUCE 子模块 demo 或用户个人排除文件（Paper Crown.mp3 在 .git/info/exclude）；`coord/lesson-designs/music-creation-unit2/assets/midi/` 亦仅 21 个 MIDI、零音频。
- 「不自造新音频」约束下无可复用音频 → 生产派生 tutor YAML 的 materials 只能空载，卡题「brief/材料/音频」的音频/材料内容腿不可真实交付。

### 证据 2：生产节点检查全部非文本，无法映射 trim 精确匹配 expect

first-run-twinkle 七节点 done_when 全为非文本检查：n0 clip_exists+human_replied（:139-143）、n1/n3 note_sequence_match（:152、:182-206）、n2 user_confirmed+tempo_set（:166-170）、n4 event user_chose（:214）、n5 user_feedback_given+user_confirmed（:221-225）、n6 export_completed（:233）。文本 expect 须自造教学内容与答案（生产卡内不存在该问答面），卡验收第 4 条「（如检查非文本 expect 可判定）停手上报，不硬塞」触发。

### 失败分类

`premise_falsified`（依赖缺失：卡前提「既有材料音频资产可复用」不成立）+ `mapping_blocked`（生产检查非文本，硬映射=自造教学内容）。非执行失败、非环境中断。

### 仍可执行面（供主管拆卡参考，本腿未动）

schema v1（brief/materials 字段 + v0 兼容缺省空）、tutor_sim.go workflow_data 携带、UI brief 折叠/AudioPlayer 卡/无材料不渲染（fixture src 测试，同 gallery 模式）、smoke 断言扩展——均不被上述证据阻塞；仅「生产派生 YAML 携带真实音频材料」内容腿不可执行。

### 处置选项（由主管裁定，执行侧不自行扩域）

a) 拆卡：schema/UI/smoke 腿先行，派生 YAML 以 brief+空 materials 交付；b) 另开音频资产制作卡（内容权威归 lead）后再派本卡；c) 指认本报告遗漏的音频资产位置。
