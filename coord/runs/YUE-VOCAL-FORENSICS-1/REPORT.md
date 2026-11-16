# YUE-VOCAL-FORENSICS-1 取证报告：YuE2 纯器乐断言矛盾（C3 人声 vs 实听歌词）+ v2 完整性

- 卡：池序 381 `coord/cards/todo/2026-11-16-YUE-VOCAL-FORENSICS-1.md`（发卡：主管 2026-11-16 晚）
- 执行：量产执行子代理（GLM-5.3-Flash），2026-11-16；零栈离线取证（未启动任何真实栈进程，未占 PC-RUNTIME-STACK）
- 领取基线：主仓 HEAD `d5c20765c8edf462416d93a938e4303c3e0c0e5f`（worktree `D:\Vit_DAW_wt_yueforensic`，分支 `port/yue-vocal-forensics-1`）
- 分析对象（主仓只读，未改动）：
  - `D:\Vit_DAW\coord\runs\GEN-YUE-WIRE-1\run_20261116_000009\downloads\yue-local-6d7289990beafcec.wav`（v1，sha256 `344c5a2f…bea81`，11,519,788 B）
  - `D:\Vit_DAW\coord\runs\GEN-YUE-WIRE-1\run_20261116_000009\downloads\yue-local-45b1a3cbc41bbef5.wav`（v2，sha256 `fedbfe3b…063a`，9,784,108 B）
  - 同目录 `*.replay.json`、`wire_report.json`、`engine_supervisor_stderr.log`、`w9_transcribe_result.txt`（372 run 原始工件）
- 本卡工件（worktree）：`coord/runs/YUE-VOCAL-FORENSICS-1/`（本报告 + `analyze_wav.py` + `wav_forensics.json` + `wav_forensics_summary.txt`）

---

## 问① C3 断言语义 + instrumental 参数生效链

### 1a. C3 测的是哪一侧？——请求侧（replay.abc 谱面），不是引擎实收/生效侧，更不是声学侧

证据锚点：

- 装置代码 `scripts/yue_wire_smoke.py` W3 腿（490-524 行）：`abc1 = replay1_doc.get("abc")` → `voice_lines_rest_only(abc1, "Vocal")` → `report["w3"]["vocal_rest_only"]`。检查对象是 **replay JSON 里的 `abc` 字段**，即 agent 准备下发的请求乐谱。run_20261116_000009 的 `wire_report.json` w3 节：`vocal_lines_checked: 1, vocal_rest_only: true`——对应 dispatched ABC 中唯一的 Vocal 行 `Z4|`（纯休止）。
- replay.json `abc` 字段（v1=v2 同谱）：`…V: Vocal\nZ4|\nV: Ins\nc8e8g8c'8|b8g8e8g8|a8f8d8f8|e8c8d8e8|`——C3 断言的「Vocal 零发声」仅描述这张请求谱面。
- 引擎实收侧证据（372 run 的 `engine_supervisor_stderr.log`）：引擎对 job body 的回显（13-20 行 v1 / 121-128 行 v2）只有 `style / abc / duration / lm_seed / seed / output_format` 六个字段。引擎照单全收了休止 Vocal 谱面——但回显只能证明「收到」，不能证明「生效为静音」。
- 声学侧反证：W9 转写回读（见问②）证明渲染音频里有成段人声主旋律。

### 1b. instrumental 参数的真实生效链——引擎无此参数；agent 用「style 标签 + 谱面休止」替代，链路如实但非引擎约束

生效链（代码锚点，本 worktree 与领取基线一致）：

1. `agent/internal/genapi/yue_prepare.go` `YuePrepareRequest`（75-93 行）：`req.Instrumental` 为真时——
   - style 文本追加英文词 `", instrumental"`（77-80 行）；
   - ABC 的 Vocal 声部整体替换为等长休止（81-92 行，`RestYueVocalPart`）。
2. `agent/internal/genapi/dialects.go` `yueRequestBuilder`（211-238 行）：发给引擎的 body 字段 = `cot/steps/output_format/style/lyrics/abc/duration` + extra_params。**没有 instrumental 布尔字段**。
3. disclosure 面：`YueDispatchedSummary` 的 `not_forwarded` 数组（yue_prepare.go，基线 162 行）如实列出 `"instrumental (as a switch)"`；`Limitations`（dialects.go 364 行）声明 `"yue_local has no instrumental switch; rest the Vocal part in the ABC score and use an instrumental style"`。
4. 引擎侧（外部，只读取证）：yue-server.exe（E:\AI\YuE2\engine，runtime.json commit `c8516db`）的 job 回显（上引 stderr 日志）中**无任何 instrumental 形态的参数**；`[Prompt] cot=full, songs=1, variations=1, 1 tracks`（stderr 32/140 行）——引擎把双声部 ABC 解析为 **1 条轨道**（`[VAE] Track 1/1`），静音的 Vocal 声部没有成为独立的静音轨约束。

**「下发但被忽略」面**：存在，且已由本次取证定性——被忽略的不是某个参数，而是**谱面休止本身不构成引擎的「禁止人声」约束**。YuE2 是 lyrics-to-song 架构：请求不带 `lyrics` 时（两次请求 `lyrics_chars: 0`，wire_report dispatched 节），语义 LM 自由生成整曲内容，是否唱、唱什么由模型采样决定。style 字符串里追加的英文词 `instrumental` 搭载在中文用户 prompt 的 verbatim 透传通道上（replay `style` 字段原文可查），无证据表明引擎 style 通道把它消费为指令。

**修复裁定：请求链无 bug**——所有参数丢弃/映射都有显式 disclosure，无静默面。发现并修复的是 **disclosure 措辞超卖**（见 §修复）：基线代码把谱面休止称作 `the validated instrumental (conditioning|score) path`（yue_prepare.go 21/88 行、yue_abc.go 71 行），"validated" 一词是 372 回执把 C3 写成「instrumental 真实下发亲证」的口径超卖上游。已按取证结论修正措辞并加防回退回归断言。

---

## 问② 实听对照取证：人声定位 + replay/style 交叉

### 2a. 模型级证据（最强）：SheetSage2 对 v1 的转写回读显示成段人声主旋律

`w9_transcribe_result.txt`（372 run 原始工件，转写对象为 v1 的 60s WAV，引擎 stderr 228 行 `Transcribe job 3454478ec0124dd8: 60.0 s of audio`）转写出的 ABC 中，`% verse` 段起的 Vocal 声部是**连续发声的主旋律**，而 Ins 声部反而全休止（`Z4|`）：

```
% verse
V: Vocal
"C"e2g2g2z2|"Em"b2g2e2g2|"F6"a2f2d2f2|"C"e2c2z2cd|
V: Ins
Z4|
（…此后 Vocal 连续 4 行发声旋律，Ins 全休止，至结尾 3 小节）
```

- 量级：verse 段 Vocal 每行 12-16 个十六分单位发声（每行约 8-16 个音），共 4 行 + 结尾行——**成段旋律级人声内容**。372 W9 只引用了 offender `"C"z6ed|`（间隙段行末 2 个音）并按「概率性 class」记录；本卡按卡面红线明确：该口径不能消化 verse 段的量级，二者不是同一现象。
- 时间粗锚：转写 35 小节对应 60s 音频（SheetSage 回写 `Q:1/4=60`、`M:2/4`，每小节名义 2s，总量 70s 超出实际时长——其节拍估计本身有偏差，只作粗锚）：verse 起点 ≈ 第 17 小节 ≈ 27-29s，持续到 ≈ 55s。即 **v1 的人声主旋律段约在中后段 27-55s**，intro（0-27s）以器乐为主。与用户实听「v1 有成段歌词/人声」吻合。

### 2b. numpy 粗判（`analyze_wav.py`，工件 `wav_forensics.json` / `wav_forensics_summary.txt`）

- v1：数据块时长 59.999s；有声段 `(0.08, 59.96)` 连续覆盖全曲；voiced（70-800Hz 音高内容）56.33s。f0 内容宽谱（70-800Hz、帧间变异 cv 0.43-0.96）——多旋律线交织的成曲形态。
- v2：数据块时长 **50.959s**（非 60s，见问③）；有声段 `(0.09, 50.91)` 连续；voiced 49.94s。
- 方法边界（如实声明）：numpy 自相关 f0 + 4 频带能量是**启发式粗判，不是人声/器乐分类器**——音乐全部 voiced 是正常形态，不能单独定位人声。人声定位的模型级证据以 2a 的 SheetSage2 转写为准；v2 未经转写（W9 只转写了 v1），v2 的人声定位只有用户实听「只有一句」这一个来源，本报告不虚构 v2 人声区间。

### 2c. 与 replay/style 的交叉

- 请求侧谱面 v1=v2 完全相同（同一 ABC，16 个 Ins 音符 + Vocal `Z4`，replay `abc` 字段）；两版差异只有 `style`（「欢快的钢琴独奏，突出学生旋律, instrumental」vs「深沉的弦乐合奏，温柔铺底, instrumental」）与 `lm_seed`（`8422682388455324884` vs `172023044544654264`）。
- 结论：**人声不是请求侧注入的**——谱面 Vocal 全休止、lyrics 缺失，两版亦无歌词来源字段（`lyrics: false`，wire_report dispatched）。人声只能来自引擎在无歌词约束下的自由生成；两版人声表现差异（v1 成段、v2 极少）与 `lm_seed`/style 差异导致的采样路径差异一致。style 中的英文词 `instrumental` 在该透传通道上无抑制效果（v1 实听与 W9 转写都是反例）。

---

## 问③ v2 完整性：引擎短输出，无填充、无管线截断

证据锚点：

1. **引擎侧提前终止**（engine_supervisor_stderr.log）：v1 `[AR] Semantic song 0: 1500 tokens (truncated)`（53 行）→ 60.0s；v2 `[AR] Semantic song 0: end token at step 1274`（158 行）→ `[NAR] Song 0: 1274 frames (51.0 s)`（162 行）→ `[VAE] Tiled decode done: T_audio=2446016 (50.96s @ 48kHz)`（224 行）→ `[Pipeline] Done: 1 tracks, 51.0 s of audio`（225 行）。引擎按语义 LM 的 end token 提前收束，生成了 51.0s 音乐。
2. **WAV 字节数精确对账**：v2 文件 9,784,108 B = 44 B RIFF 头 + 2,446,016 samples × 4 B（48kHz 双声道 16bit）→ **50.959s，与引擎 VAE 输出逐样本一致**。文件结尾即音频结尾——51-60s 区间不存在（不是「51s 内容 + 9s 静音填充」）；也没有截断痕迹（244,6016 样本完整解码链可回指）。
3. **agent 下载链零加工**：引擎日志与本地文件字节数/时长精确匹配，agent 无 pad/trim 行为面（artifact `source_url: "response:body"` 原样落盘）。
4. **「C8（WAV 60.0s）均绿」口径不成立**：装置 `scripts/yue_wire_smoke.py` 对 v1 只断言 40-150s 宽窗（476 行 `dur_v1 < 40 or dur_v1 > 150`）；对 v2 **只断言文件存在（547-549 行），时长从未被断言**。372 回执所写「C8 双版本 WAV 60.0s」超出了断言实际覆盖——**v2 的 50.96s 短输出在 372 run 中从未被检出，属断言覆盖缺口**（装置脚本在本卡文件域之外，登记待决策侧，见 §登记）。

定性：**「只有一句」= 引擎短输出 + 该短输出内人声占比低**，不是管线截断、不是静音填充。引擎在 v2 采样路径上（不同 lm_seed + 不同 style 文本）提前 end token，产出 51.0s 较短曲目；用户对内容密度的观感与「51s vs 60s」以及人声极少相符。无模型级转写可进一步定量（W9 未覆盖 v2），此边界如实声明。

---

## 三分定性结论

| 定性 | 判定 | 关键锚点 |
|---|---|---|
| **引擎能力边界** | **成立（主因）** | 引擎 API 无 instrumental 参数（stderr job 回显六字段，13-20/121-128 行）；无 lyrics 时自由生成人声主旋律（W9 转写 verse 段 + 用户实听）；谱面 Vocal 休止被解析为单轨合并（`1 tracks`），不构成静音约束 |
| **请求链 bug** | **不成立** | 请求链如实：`not_forwarded` 显式列出 instrumental 开关未转发（yue_prepare.go）；引擎回显确认实收 body 与声明一致；无静默丢弃面 |
| **断言设计/口径缺陷** | **成立** | C3 名义（「instrumental 真实下发」）超出覆盖面（请求侧谱面休止）；C8 口径「双版本 60.0s」超出实现（v1 宽窗 + v2 无时长断言）→ 50.96s 短输出漏检；上游措辞 `validated instrumental path` 已修（见下） |

产品面建议（供主管裁定，本卡不实施）：在 yue_local provider 的用户可见声明中明确「纯器乐不可保证」——当前引擎形态下 instrumental 只能请求侧条件化，声学层纯器乐不可达；若需纯器乐保证，待引擎侧提供开关或产品面做声明。

## 修复记录（agent 侧最小修复，文件域内）

- 性质：disclosure 措辞超卖修复（非行为变更——请求构造、字段、conditioning 逻辑零改动）。
- `agent/internal/genapi/yue_prepare.go`：包注释、style action、abc action 三处措辞改为「request-side conditioning only；engine-side muting not guaranteed — un-lyriced generation may still produce vocals」，并锚定取证卡号。
- `agent/internal/genapi/yue_abc.go`：`ConvertMidiToYueABC` 注释同义修正（移除 "validated"）。
- `agent/internal/genapi/yue_prepare_test.go`：`TestYuePrepareInstrumentalConditioning` 新增两条防回退断言（actions 必须含 `engine-side muting not guaranteed`；不得再出现 `validated instrumental`）。
- 回归：`go build ./...` 退出码 0；`go test ./internal/genapi/ -count=1` 退出码 0（`ok vit-daw-agent/internal/genapi`）。
- 真栈腿：本修复只改字符串/注释/测试断言，不触引擎请求体的字段与取值（`yueRequestBuilder` body 集合未变），零栈裁定不触发复现腿；若主管要求真栈复验，按 372 同款 `scripts/run_yue_wire_smoke.ps1` 占栈执行。

## 登记项（文件域之外，待决策侧裁定）

1. **装置脚本 C8 覆盖缺口**：`scripts/yue_wire_smoke.py` 对 v2 WAV 无时长断言（对 v1 仅 40-150s 宽窗）。建议给两版 WAV 加「请求时长比例窗」（如 0.6×duration-1.1×duration）断言，防引擎短输出再次漏检。脚本属 372 装置，不在本卡文件域，未改动。
2. **372 回执口径修正**：done 卡所写「C3 instrumental 真实下发亲证」「C8 双版本 WAV 60.0s」与断言实际覆盖不符，建议主管在 372 卡面追加勘误注记（本卡 REPORT 为证）。
3. **W9 口径**：372 以「概率性 class（legW9_vocal_detected）」记录的 offender `"C"z6ed|` 只是间隙段行末 2 音；完整转写 verse 段是成段人声主旋律，量级不可同日而语——372 的「稀疏配器残留」定性不得延伸覆盖本卡事实（卡面红线执行确认）。

## 红线遵守声明

- 引擎（E:\AI\YuE2）只读取证，未修改任何引擎文件。
- 分析对象（主仓 run 工件）只读；写入仅落 worktree `coord/runs/YUE-VOCAL-FORENSICS-1/`。
- 零栈：全程未启动内核/Godot/agent/引擎进程，未占 `coord/resources/PC-RUNTIME-STACK.md` 记录的栈。
- 未触碰主仓其他文件（回执节追加除外，属派发指令授权）。

## 执行命令与退出码（四件套之命令部分）

| 命令 | 退出码 |
|---|---|
| `git worktree add D:/Vit_DAW_wt_yueforensic -b port/yue-vocal-forensics-1` | 0 |
| `python -I analyze_wav.py <v1.wav> <v2.wav> <outdir>`（含一次阈值 bug 修复后复跑） | 0 |
| `sha256sum <双版 WAV>`（主仓 downloads） | 0 |
| `go build ./...`（worktree agent/） | 0 |
| `go test ./internal/genapi/ -count=1`（修复前后各一次，均绿） | 0 |
| commit + push（见回执） | 0 |
