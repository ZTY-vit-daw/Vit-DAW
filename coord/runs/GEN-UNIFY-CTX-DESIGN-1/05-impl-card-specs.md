# 设计件五：最小实现拆卡规格（4 张，可直接发卡）

- 归属：GEN-UNIFY-CTX-DESIGN-1 / 2026-10-11
- 拆卡原则：披露键先行（其余卡的数据源）→ 字段持久化（同文件域串行）→ 注入与机检（agent 侧，可部分并行）→ 消费接入（依赖首张生成卡落地）。每卡独立可验收、可独立回滚（AGENTS §12 一卡一 commit）。

## 卡 1：GEN-CTX-DISCLOSE-1（披露键卡）

- 优先级 / 预估 / 依赖：P1 / 0.5 天 / 无
- 目标：`get_project_state` 响应新增 `music_context` 键（tempo_bpm/tempo_setting_count/time_signature/time_signature_numerator/time_signature_denominator/time_sig_setting_count 六键；key_signature/style_note 占位为 null/""，卡 2 挂接值）。设计规格=件二 §1。
- 文件域：`VitApp/Source/Service/CommandDispatcher.cpp`（handleGetProjectState 披露块 :3389-3420 区域+一个 static 辅助函数）；冒测脚本 `scripts/`（Mac=`*_smoke_mac.sh` 体系扩展或 PC=`dev_agent_smoke.ps1` 参数模式，按执行端定）。
- 验收标准：
  1. 真栈只读冒测：get_project_state 返回 music_context 六键，与 `set_tempo`（既有命令）联动——set_tempo 128 → 再查 → tempo_bpm=128.0；键值与 `getTempo(0)/getTimeSig(0)` 引擎真值一致（脚本断言，exit 0）。
  2. 既有键零变更：同响应中 project_uuid/tracks/markers 等键与改前逐键一致（快照 diff 断言）。
  3. 全量编译零警告增量+`go build ./...`/`go test ./...`（agent 侧零改动应零 diff）。
- 停止条件：披露块挂点与 :3389-3447 现状重大不符（上游重排）→ 锚点上交。

## 卡 2：GEN-CTX-KEYFIELD-1（字段持久化卡）

- 优先级 / 预估 / 依赖：P1 / 1 天 / **GEN-CTX-DISCLOSE-1 合入后领取**（同文件域 CommandDispatcher.cpp 串行）
- 目标：调性+风格两工程级字段全链——`set_project_key`/`set_project_style_note` 命令（含 clear/校验 fail-closed/auto-persist）、`vit_music_key_signature`/`vit_music_style_note` edit state 属性、music_context 键挂接真值。设计规格=件二 §2。
- 文件域：`VitApp/Source/Service/CommandDispatcher.cpp`（两 handler 注册+实现）；`agent/internal/tools/catalog.go`（两命令 argspec/风险级注册，RiskConfirm）；冒测脚本 `scripts/`。
- 验收标准：
  1. §11 往返三验（真栈）：旧工程（无属性）load→save→load 缺省保真（key_signature 仍 null）；set_project_key {root:"A", scale:"minor"} → runAutoPersist 路径 save→load 值逐字节保真；save_as_project 新路径（.vit 加密容器）往返保真。
  2. fail-closed 负探针：scale:"ionian"（合法）通过、scale:"dorian23"（非法）拒绝、root:150 拒绝、style_note 501 字符拒绝——四负例 status=error 回执（脚本断言）。
  3. 披露联动：set→get_project_state → music_context.key_signature/style_note 回显规范值；clear→null/""。
  4. agent 全量 `go test ./... -count=1` 0 FAIL+gofmt 净（catalog 注册面单测）。
- 停止条件：edit state 属性随 .vit 容器往返失败（VitHeadlessService 保存链剥离未知属性）→ 上交换存储位（改 projectAgentRuntimeState 侧方案的决策点）。

## 卡 3：GEN-CTX-INJECT-1（注入与机检卡）

- 优先级 / 预估 / 依赖：P1 / 1 天 / GEN-CTX-DISCLOSE-1（快照组装读真键；开发期可 mock 键先行，**验收须真键**——故领取可在卡 1 合入后、卡 2 并行期间启动）
- 目标：`agent/internal/genctx/` 新包四件——快照组装（get_project_state→MusicContextSnapshot，含 markers→sections 换算）、RenderGenerationPrompt+RenderProfile（四块模板/未设置项可见/用户块保真/坐标系 MUST）、ValidateNotes/ValidateMidiFile（音级集合 11 scale 表/网格对齐/音域密度长度/SMF meta 预检）、回读核验；scene 三枚举贯穿（in_project 默认强制、free_creation/experiment_control 豁免+写前门哨兵）。设计规格=件三全件+件四。
- 文件域：`agent/internal/genctx/`（新包）；**零触碰** capabilitycontext/contextruntime/promptruntime（避让声明写入回执）。
- 验收标准：
  1. 单测全绿：件三 §6 清单（音级表 11×采样根音、容差边界、违例分类完整、渲染未设置项、profile 裁剪、用户块保真攻击样例、豁免 scene 哨兵拒写命令）。
  2. 合成集成：mock get_project_state 响应→快照→渲染→验证器全链 golden 用例（含一例 key=C major 含黑键→constraint_violation 拒写、一例 4/4 网格 1/32 偏移超容差拒写、一例合法通过）。
  3. 真键冒测：卡 1 已合入的真栈上组装真快照（tempo/拍号与 set_tempo 联动一致）。
  4. 全量 `go test ./... -count=1` 0 FAIL+gofmt 净。
- 端测边界声明（AGENTS §5）：本卡纯库面（无 webui/无用户旅程改动），真栈冒测=agent HTTP 面读断言；生成端到端（LLM 实际产 note→门→写入）归卡 4 与 AIGC-DRUM-GEN 烟测腿。
- 停止条件：无（纯新增包，无上游耦合；卡 1 键形态变更除外→锚点上交）。

## 卡 4：GEN-CTX-CONSUME-1（生成接入卡）

- 优先级 / 预估 / 依赖：P2 / 0.5-1 天 / **AIGC-DRUM-GEN-1 落地+GEN-CTX-INJECT-1 合入**
- 目标：鼓点生成编排（AIGC-DRUM-GEN-1 的"固定 prompt 模板"）改造为消费共用模板引擎——编排器 prompt 组装改调 RenderGenerationPrompt（profile=gen_plugin）、note 产物过 ValidateNotes 写前门+回读核验、违例入回执 schema；`dev_agent_smoke -Scenario drum_gen` 断言升级（含一例注入块出现在发送 prompt 的证据断言+一例违例拒写负探针）。
- 文件域：AIGC-DRUM-GEN-1 落地后的编排器文件域（届时卡面回填；预期 agent/internal/ 编排相关包+scripts/ 烟测）。
- 验收标准：真栈烟测 exit 0（数据级：注入块在场证据+合法写入回读一致+违例负探针拒写）；全量 go test 0 FAIL。
- 停止条件：AIGC-DRUM-GEN-1 编排形态与本卡假设重大不符→回设计侧修订 profile。

## 序列与并行图

```
GEN-CTX-DISCLOSE-1 ──→ GEN-CTX-KEYFIELD-1（同文件域，严格串行）
        │
        └──────→ GEN-CTX-INJECT-1（键可用即领；与卡 2 并行窗口）
                      │
AIGC-DRUM-GEN-1 ──────┴─→ GEN-CTX-CONSUME-1
```

- 总量：3-3.5 天（关键路径 DISCLOSE→KEYFIELD→CONSUME ≈2-2.5 天+并行 INJECT 1 天）。
- 外部生成 API 适配器接入（gen_api profile 消费）与 VitReel 接入（vitreel profile）**不另开卡**——分别归各自线首批卡的自然验收项（适配器卡届时引用件三 §2 profile 契约）；本拆卡组只保底"生成插件"消费方先行（用户裁定生成线首批=AIGC-DRUM-GEN）。
- 编曲消费=编曲线前置就绪后随正典前置卡接入（AIGC-DRUM-GEN-1 卡面注记"后续由汇合段重构吸收"同口径）。

## 互斥文件清单（并行领取核对用）

| 卡 | 独占文件 |
|---|---|
| 1 | CommandDispatcher.cpp（披露块）；scripts/ 冒测 |
| 2 | CommandDispatcher.cpp（handler 区）；tools/catalog.go；scripts/ 冒测 |
| 3 | agent/internal/genctx/**（全新） |
| 4 | AIGC-DRUM-GEN 编排域+scripts/ 烟测 |

卡 1/卡 2 与任何其他在飞卡若叠 CommandDispatcher.cpp（如 MIXLAYER 系内核腿），领取前按 PROTOCOL §3 冲突规则核对。
