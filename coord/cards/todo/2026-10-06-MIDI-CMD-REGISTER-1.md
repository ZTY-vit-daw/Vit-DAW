# MIDI-CMD-REGISTER-1：内核 CommandDispatcher 注册缺口补线——import_midi_to_track + apply_midi_note_patch（MIDI-RECON-1 K2 腿）

- 池序 5（MIDI-RECON-1 验收产出，节目功能线地基第一腿）；目标仓库=D:\Vit_DAW（PC 执行侧）；来源=[rulings/2026-10-06-MIDI-RECON-1-pass.md](../../rulings/2026-10-06-MIDI-RECON-1-pass.md) 缺口 K2 + [勘察报告](file:///C:/Users/timoz/.zcode/workspace/default/queue/reports/2026-10-06-MIDI-RECON-1.md) 问一
- 优先级 / 预估 / 依赖：P1 / 0.5 天 / 无（实现体已存在，仅差注册接线）
- 模型分级：L0 写卡即走 / flash·deepseek·codex 均可接（两行注册+重建+真栈验证；内核 C++ 编译重，注意构建时间）

## 已核实事实（MIDI-RECON-1 双核实证，下游不必重复）

1. `MidiService.h:20-28` 已声明 `handleApplyMidiNotePatch` / `handleImportMidiToTrack`；实现体在 `MidiService.cpp`（apply_midi_note_patch=1349-1683 八种 op；import_midi_to_track=1784-1946 含 SMF 解析/PPQ 校验/merge_tracks）。
2. **注册表缺口**：`CommandDispatcher.cpp:2609-2655` midi 命令族注册段无这两条目（全文 grep 均 0 次）——真栈直发即 `Unknown command`（工件 p03/p10）。
3. 通道形态：写命令经 VSP `legacy.command` 逃生舱可达内核（p06 insert_midi_clip 同通道走通）；不带 base_revision 直接放行，无 CAS 面。
4. agent 侧现行为：纯 insert patch 被翻译层译成已注册的 `add_midi_notes`（`harness.go:9509-9538`）——**修复后翻译层保留为兼容路径，不删不改**。

## 目标

1. CommandDispatcher 注册段补两条 emplace：`import_midi_to_track`→`handleImportMidiToTrack`、`apply_midi_note_patch`→`handleApplyMidiNotePatch`（照抄邻条 add_midi_notes 形态，const juce::DynamicObject& + juce::String）。
2. 内核重编译并部署到烟测栈使用的运行时位置（`Export/staging/runtime/VitApp.exe`——被测二进制须含本改动，§9 记录新二进制时间戳/sha256）。
3. 真栈烟测扩展：`scripts/dev_agent_smoke.ps1` 加 `-Scenario midi_register`（或并入既有场景参数模式）——断言面：①`midi.import_file` 确认执行 ok 且回读 clip 落轨（fixture .mid 复制到 run 目录用，参照 MIDI-RECON-1 的 midi_arp.mid 用法）②`midi.write_clip_notes` 混合 op patch（至少 transpose_note 或 delete_note 一类非 insert op）确认执行 ok 且回读音符合预期 ③回读一致。**exit 0 方算交付**。
4. 门：agent 侧零改动 → `cd agent && go build ./... && go test ./... -count=1` 0 FAIL（回归面）；内核侧以烟测 exit 0 为准。
5. 回执：注册 diff 锚点、新内核 sha256、烟测 run ID 与工件路径、泊位声明、HEAD。

## 文件域

- `VitApp/Source/Service/CommandDispatcher.cpp`（注册两条目，≤4 行）
- `scripts/dev_agent_smoke.ps1`（新场景）
- 禁改：MidiService.cpp/.h（实现体已存在；若实现体有缺陷→停手上交，不越域修）、agent 侧任何文件、VspKernelReference.cpp（canonical 面不动=A3 另议）

## 约束

- 真栈 §9 所有权与泊位纪律（起前端口核验、finally 拆净、不复用他栈）；与其它烟测错峰。
- 内核构建入口：仓库根 `build_release.ps1` 或既有 CMake 增量路径（先探明烟测栈实际加载哪个 exe 再部署——MIDI-RECON-1 被测为 Export/staging/runtime/VitApp.exe）。
- 提交：分支 `port/midi-cmd-register-1`，coord 卡面直推 main，实现等验收 cherry-pick。

## 停止条件

- 注册后真栈仍 Unknown command（分发面还有第二层映射）→ 实证上交，不盲目扩域。
- handleImportMidiToTrack/handleApplyMidiNotePatch 实现体本身缺陷（参数契约不匹配/崩溃）→ 取证上交，不修 MidiService。

## 领取：（时间 / origin/main hash / 分支名）
## 回执：（commit hash / 烟测 run ID / 泊位声明）
## 验收：（裁定文件 / 验收 commit）
