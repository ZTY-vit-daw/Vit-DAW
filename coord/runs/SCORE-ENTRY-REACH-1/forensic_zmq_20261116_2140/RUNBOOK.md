# RUNBOOK：382 SCORE-ENTRY-REACH-1 取证腿（kernel-only ZMQ，D4 病理定性）

## 跑前口径（AGENTS §8，跑前成文）

- 运行次数：**正式 1 次**，单 run 无重跑；装置域修复后重跑须分记 attempt。
- 成功条件：driver exit 0 且 forensic_report.json 落盘，含 F1-F5 全部原始数值与定性 verdict（one of: `template_xml_unit_mismatch`〔模板资产拍数被 loader 当秒读，内核命令面换算正确〕/ `kernel_write_path_bug`〔insert_midi_clip 回显即拍数，写入侧换算失效〕/ `readback_inconsistent`〔insert 回显≠get 披露〕/ `inconclusive`〔证据形态不落上述任一类〕）。
- 失败分类：`env_interrupt`（内核起不来/端口冲突/ZMQ 超时）/`face_defect`（内核命令面 status≠error 之外的异常形态）/`probe_defect`（driver 解析缺陷）。环境中断只有留原始日志才可排除有效轮次。
- 止损线：30 分钟内未取得 F1-F3 或环境故障两次→释放栈位记录分类（真栈验收腿不受影响——其形态为 berth 脚本既有面）。同断点连败 2 次禁原样重跑。
- LLM 计费上界：**0**（全确定性 ZMQ 面，无 chat/无 LLM）。

## 取证问题（卡面目标 0/1 分叉点）

D4（377 上交）：80bpm 工程旋律 clip `get_midi_clip_data` 披露 `length_seconds=8.0`（拍数冒充秒嫌疑；真值 6.0s）。静态读码证据链（本轮新增）：

- 373 模板 XML（`coord/lesson-designs/.../template.v1.xml`，手写资产）`<MIDICLIP start="0.0" length="8.0">`——**拍的数值**（80bpm 工程 8 拍旋律 clip）。
- tracktion fork：`Clip::clipStart/length` 为 `CachedValue<TimePosition/TimeDuration>`（秒类型），`referTo(state, IDs::start/length)` 直读 XML 数值当**秒**，无 beat 换算（`tracktion_Clip.cpp:79-81`+`tracktion_Time.h`）。
- `insert_midi_clip` 命令路径有 `convertDurationValueToSeconds`（beats→seconds，`MidiService.cpp:129-161/1982`）——运行时创建的 clip 预期真秒。
- webui 桥 `clipLengthBeats=lengthSeconds×bpm/60`（kernel.ts:139）→ 8.0s@80bpm=10.6667 非网格 → `buildApplyPatch`→`beatsRoundedToTicks` 抛 ModelError（edit.ts:265）→ Panel:401 useMemo 渲染期无边界整树卸载（377 fatal.png）。

候选定性：A=模板 XML 单位错配（拍数被当秒读；内核命令面正确——修复面=webui 防御+模板资产越域上交）；B=内核写入路径换算失效（修复面=内核，卡面预设「1 行级」）；C=读写不一致。

## 断言面（确定性，零 LLM）

- F1 tempo：`analyze_midi_transformations` music_context_echo tempo_bpm=80（工程上下文真值）。
- F2 模板 clip 披露：`get_midi_clip_data`（旋律 clip）记录 start_seconds/length_seconds/offset_in_source_seconds（377 复现预期 length_seconds=8.0）。
- F3 写入路径：`add_track`→`insert_midi_clip {time_unit:"beats", start:0, length:8}`→**响应回显** length_seconds（6.0=换算正确 / 8.0=写入失效）。
- F3b 恒等检验：`insert_midi_clip {length:6 beats}`（80bpm 真值 4.5s；若回显 6.0=60bpm 恒等征）。
- F4 读回一致：两新 clip `get_midi_clip_data` 与各自回显对照（相等=读写一致）。
- F5 往返：`save_project`→`open_project`→三 clip 披露对照（模板 clip 若 8.0→8.0 延续=XML 写读秒语义一致，错值源于模板本身）。

## 被测物 provenance（§9）

- 内核=只读复用主仓构建 `D:/Vit_DAW/VitApp/build/VitApp_artefacts/Release/VitApp.exe` sha256 `A59473614152882B21433DCEE684F8B13AE0BD5B632A97CD5E2C8765BB18348E`（374/377/384 同一二进制；取证腿零内核改动）。
- boot 模板=373 规范资产 place-copy：源 `coord/lesson-designs/music-creation-unit2/templates/melody-drum-v1/template.v1.xml` sha256 `0CB22BA39E8B635B3B521F2B3CDEC03935F251BFBA811EF83CA540D5E32321B3`（源只读；run 副本供 VIT_PROJECT_XML）。
- 栈位：kernel-only（VitApp.exe 单进程 ZMQ 5555/5556+UDP 4445；无 agent/UI/Godot/浏览器）。
- S10 recents：before.json 落 run 目录→teardown 还原+哈希亲证（F5 save/open 写面触发）。

## 步骤

1. 占栈登记推 main 亲核 → ≤60s 复核净空 → 起 VitApp.exe（Hidden，VIT_PROJECT_XML=run 副本）→ ping；
2. `python forensic_d4_driver.py`（全部 REQ 直收，报告落 `forensic_report.json`）；
3. teardown：亲停 VitApp pid → TCP 5555/5556+UDP 4445=0 亲证 → recents 还原哈希亲证 → 释放行推 main。
