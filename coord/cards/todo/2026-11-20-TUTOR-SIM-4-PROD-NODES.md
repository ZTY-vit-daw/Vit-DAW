# TUTOR-SIM-4：生产 YAML 节点接入导师循环（brief/MIDI 材料装载）（v2 重发）

- 池序 438；P0；PC；发卡=lead（2026-11-20 二次发卡，v1 停卡后重设计）；派发确认=用户 2026-11-20「MIDI clip 也可以，直接装载进工程装上音源播放；音频可现在做也可不做」；验收负责人=lead。
- v1 停卡背景：音频资产不存在（勘察卷 `coord/runs/TUTOR-SIM-4/README.md`）+生产 done_when 非文本。v2 依用户裁定改道：**材料主形态=MIDI clip 装载**（DAW 原生），audio 形态留位不阻塞。

## 目标

把导师循环从算术实验升级为**生产乐局节点驱动**的一课（first-run-twinkle 单关卡、节点粒度）：

1. **腿A（服务端+YAML 内容）**：
   - 扩展实验 schema 至 `vit.tutor_sim.v1`：每步新增 `brief`（节点教学简报，多行文本，从 level.yaml 七节点语义起草）、`materials`（列表：`{kind: midi_clip, title, path}`；`kind: audio` 留 schema 位但本轮无内容）。v0 YAML 仍可解析（缺省空，UI 不渲染卡）。
   - 新装置 `coord/experiments/tutor-sim-2/`：**复制**（不改冻结源）`coord/levels/first-run-twinkle/assets/theme_twinkle_v0.mid` 进装置；节点步骤=YAML 化的教学步（建议 3-5 步覆盖听主题→跟做→确认，expect=作者写的模拟问答，**不映射生产 done_when**）。
   - `tutor_sim.go` start/chat 响应 workflow_data 携带 `brief` 与 `materials`（随当前步）。
2. **腿B（UI 控件）**：
   - TutorStepCard 增加 brief 展示（折叠/展开，默认收起）。
   - **MidiMaterialCard**：材料名+时长/音符数摘要+「装载到工程」按钮；装载走 agent 既有工具链（内核命令面 add_track/insert_midi_clip/add_midi_notes——`scripts/yue_wire_smoke.py` W1 与 277 ARR-SRCLOAD 装载链为锚点；解析 .mid 用仓库既有 MIDI 解析面，若无则最小解析），装载后轨道+clip 就位并挂可用音源（Surge 预设优先，参照 384 腿 D 模板音色）；已在工程内则按钮置灰防重复。
   - `kind: audio` 材料用既有 widgets/AudioPlayer 渲染（无真音频时仅 fixture 测试覆盖，同 gallery `WIDGETS_FIXTURE_AUDIO` 模式）。
   - 437 全部行为保持：TutorProgressCard、completed 禁用、普通工程零 UI 变化。

## 约束

- 无命令词；不显示 expect/答案；不改冻结 production design schema、levels 冻结文件与 theme MIDI 源文件；437 既有测试只加不减。
- 文件域：`agent/internal/chat/tutor_sim.go`(+tests)、`agent/internal/tutorsim/`（如需）、`coord/experiments/tutor-sim-2/`、`agent/webui/src/project/`、`agent/webui/src/widgets/`（只读复用）、`scripts/run_tutor_sim_smoke.ps1`（扩展断言）。
- 领取：worktree（建议 pc438b-tutormidi）+分支；领取记录同步 main 后开工；完成走 main 合入。
- 不启真实栈；真栈 smoke/手测由 lead 开。

## 验收（零栈）

- Go：v1 schema 解析、v0 兼容、brief/materials 进 workflow_data、expect 判定回归；`go build ./...` + `go test ./internal/chat` 全绿。
- vitest：brief 折叠、MidiMaterialCard 渲染/装载调用/防重复、audio fixture 渲染、437 用例不回归；`npm run test`+`npm run build` exit 0。
- smoke 脚本扩展：headless 断言 workflow_data 含 brief/materials。
- 装载链若需内核命令面超出既有已证命令集，停手上报锚点，不发明命令。

## 停止条件

证据推翻前提（theme MIDI 不可解析、装载链命令面缺口需新内核命令）即停卡上报。
