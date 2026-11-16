# YUE-INSTRUMENTAL-PATH-1 勘察报告：官方 instrumental 渲染路径 vs 本地 yue_local 封装（勘察腿·零栈）

- 卡：池序 390 `coord/cards/todo/2026-11-16-YUE-INSTRUMENTAL-PATH-1.md`（勘察腿·零栈，未启动任何引擎/内核/agent 进程，未占 PC-RUNTIME-STACK）
- 执行：量产执行子代理（GLM-5.3-Flash），2026-11-16
- worktree：`D:\Vit_DAW_wt_instr`，分支 `port/yue-instrumental-path-1`
- 领取基线：主仓 HEAD `ebfee4270ad0f41c098bbd309ad1530b552d478b`（worktree 干净；主仓工作树领取前已有大量与本卡无关的脏文件，未触碰）
- 本报告只读外部面：本地引擎部署目录（`E:\AI\YuE2`，只读未写）、官方 YuE repo（GitHub raw）、本仓 run 工件与源码。**未改任何生产代码。**

---

## 一、本地 YuE2 引擎封装定位（渲染入口与配置现状）

### 1a. 引擎本体（外部，只读盘点）

- **引擎进程**：`E:\AI\YuE2\engine\yue-server.exe`，**yue2.cpp 社区第三方实现**（MIT，来自 YuE2-Studio v3.6.0 portable），commit `c8516db`（2026-10-09 构建），sha256 `6db1bd3ee9f8849f698f5636bf9e89a62d6d3ee1d145e20e819221ed0ed78a3a`（`coord/runs/YUE2-PC-DEPLOY-1/run1_20261019_201652/engine_hash.txt`；`E:\AI\YuE2\engine\runtime.json`）。**非官方 PyTorch 栈**（372 部署回执 results.md 明确声明）。
- **启动参数**（`E:\AI\YuE2\start_engine.ps1`，与 372/381 run 的 engine 块一致）：`--model / --vae / --transcriber / --tokenizer / --host 127.0.0.1 / --port 8087 / --max-seq 24576`。
- **模型根盘点**（`E:\AI\YuE2\models`，5 个 GGUF）：

| 文件 | 大小 | 角色 |
|---|---|---|
| `YuE2-3B-Q8_0.gguf` | 3.81 GB | 歌曲模型（LM+NAR 同文件，627 tensors，Qwen3 28L H=2048 V=184704） |
| `YuE2-Vae-F32.gguf` | 530,497,344 B | VAE（FP32；与官方 m-a-p/YuE2-Vae safetensors 530,512,720 B 同量级，GGUF 容器化） |
| `SheetSage2-Q8_0.gguf` | 105 MB | 转写回读（W9 用） |
| `yue2-mothersuperior-realaudio-tokenizer-v4-Q8_0.gguf` | 46 MB | codec tokenizer |
| `MERT-v2-FullSong-Q8_0.gguf` | 902 MB | 音频理解（cover 转谱用） |

### 1b. agent 侧封装链（代码锚点，本 worktree 与领取基线一致）

调用链（真栈生成时）：

1. `agent/internal/harness/gen_generate.go:295` `genYuePrepareLocal`：
   a. `genapi.EnsureYueEngine(ctx, provider)`（`agent/internal/genapi/engine.go:75`）——按 provider `engine` 块按需拉起引擎进程（command/args/workdir/env/stdout_log/stderr_log/ready_timeout，`config.go:156` EngineConfig）；探活 `cfg.BaseURL`；只管自启进程。
   b. `genReadStudentScore`（gen_generate.go:329）——kernel 只读命令取学生 MIDI 真值 + `music_context`（拍号/速度/调号）。
   c. `genapi.YuePrepareRequest`（`agent/internal/genapi/yue_prepare.go:57`）——确定性条件映射（见 1c）。
2. `agent/internal/genapi/dialects.go:211` `yueRequestBuilder`——组装 POST `{base}/synth` body；随后 poll `GET /job?id=` → `GET /job?id=&result=1`（multipart：application/json replay + audio，dialects.go:106-118 `yue_local`）。
3. 引擎回执面：job id（hex）+ replay JSON（确定性重放基底）+ WAV。

### 1c. 当前渲染配置（固定基线）

- `yueRequestBuilder` 固定：`cot=full`、`steps=32`、`output_format=wav16`（extra_params 可覆盖 cot/steps/output_format/seed 等；`abc/style/lyrics` 为保护键，provider 固定 extra_params 不可覆盖，yue_prepare.go:54）。
- `Instrumental=true` 时的现状条件（yue_prepare.go:79-95，即卡面所说「降级」）：
  - style 尾部追加英文词 `", instrumental"`；
  - `RestYueVocalPart`（yue_abc.go:570）把 ABC 的 Vocal 声部改写为等长休止；
  - 无歌词字段动作、无负条件 style、无 cot 推导、无前缀核验。
- 谱面转换：`ConvertMidiToYueABC`（yue_abc.go:75）——旋律默认骑 `Ins` 声部、`Vocal` 全休止（`Z<n>|`）；和弦注解挂**旋律声部**小节；无 `% section` 时单块。
- 381 取证卷锚点（`coord/runs/GEN-YUE-WIRE-1/run_20261116_000009/` + `coord/runs/YUE-VOCAL-FORENSICS-1/REPORT.md`）：
  - 引擎 job 回显六字段 `{style, abc, duration, lm_seed, seed, output_format}`（engine_supervisor_stderr.log 13-20/121-128 行）；`[Prompt] cot=full, songs=1, variations=1, 1 tracks` + `[VAE] Track 1/1`——休止 Vocal **不构成引擎侧静音轨约束**；
  - 两次请求 `lyrics_chars: 0`；v1 渲染出成段人声主旋律（SheetSage2 W9 转写 27-55s 段），v2 仅零星——人声来自**无歌词约束下的自由生成**，英文词 `instrumental` 在 style 透传通道上无抑制证据。

---

## 二、官方 instrumental.py 工作流对照（2026-11-16 主干抓取）

来源（官方 repo `multimodal-art-projection/YuE` 主干，raw.githubusercontent.com，本日抓取）：
`skills/yue2-music/instrumental/SKILL.md`、`scripts/instrumental.py`、`assets/release.json`、`references/setup.md`、`assets/example-ins.abc`、`assets/example-style.txt`、`examples/instrumental_clip.py`。

### 2a. 渲染入口与配置（官方）

- 入口：`scripts/instrumental.py run`（或 `cover/prepare/generate/verify/share` 子命令）；推理栈 pin `yue2-infer==0.1.5` wheel（release.json），模型 **`m-a-p/YuE2-3B`（rev `14fc6c6f`，BF16 safetensors 7,261,441,640 B）+ `m-a-p/YuE2-Vae`（rev `9a94e1d0`，FP32 530,512,720 B）**。
- 渲染配置：**默认采样、BF16 AR/NAR、FP32 VAE（m-a-p/YuE2-Vae 作听核解码）、32 步**，每 GPU 顺序运行（SKILL.md）；环境不足时解决资源，不暗中降设置。
- **无 LoRA**：官方 repo 全树零 lora 路径（GitHub tree API 检索亲证）；instrumental.py 无任何 LoRA 加载/选择面。**卡面「双 LoRA 选择」假设不成立**——官方 instrumental 与歌曲模式共用同一对模型，差异全在条件面。
- 前缀一致性检查（instrumental.py `generate`）：`token_prefixes(req, tokenizer)` 与落盘 `native/prefix.npy` 逐位比对 + 保存 ABC 与请求 ABC 逐字节比对 + `req.text()` 与 pin 的协议模板比对——不一致即 fail。

### 2b. 请求协议与条件构造（官方，instrumental.py `validate_request`/`prepare`）

- 请求恰好六字段：`{id, style, lyrics, abc, cot, seed}`（多一字段都拒绝）。
- **style 规范化**：不以 `instrumental` 开头 → 加 `"Instrumental, "` 前缀；再追加 `", no vocals, no singing, no choir, no spoken words."` 后缀（prepare 内逐条件查重）。
- **lyrics 纪律**：必须为空**或恰好等于**谱面 `% section` 注释生成的曲式标签串（`lyric_tags`：`% verse` → `[Verse]`，`'\n\n'` 连接）——禁止唱词正文。
- **cot 推导**：谱面 Vocal 声部有和弦注解 → `cot=full`，无 → `cot=melody`（官方形态和弦挂在 Vocal 休止行）。
- seed 默认 831001。

### 2c. 谱面形态（官方 `validate_score` + example-ins.abc）

官方 instrumental 终态谱要求：Vocal **零发声音符**、Ins 有发声音符、Ins 单旋律、Vocal 只留休止+和弦；`L` 网格足够细；整小节休止用 `Z` 形态；显式时长 1 省略；`% section` 用原生段名且不连续重复。

**与我们的谱面同构性**：`example-ins.abc` 与 `ConvertMidiToYueABC` 输出逐字段同构（X/T/M/L/Q 头、两条 `V:` 声部行（含相同 clef/name/snm 文案）、`K:`、`% section` 注释、Vocal 行=引号和弦+全小节休止、Ins 行=单旋律）。**卡面判断「我们的谱面已是官方器乐形态」成立**（381 谱面层已证）。三处细节差异：

1. **和弦注解挂载声部**：官方挂 Vocal 休止行（`"G"z32|`）、Ins 行无和弦；我们挂旋律（Ins）行（`"Dm7"…|`）、Vocal 行裸 `Z4|`。这决定官方 cot 推导结果（我们的形态按官方规则会推成 `melody`）。
2. **显式时长 1**：官方省略（`z`/`c` 而非 `z1`/`c1`）；我们的转换器恒写数字。
3. **多小节休止与 Z 形态**：双方均为行内 `Z<n>|`，一致；官方另禁 `Z…|Z` 连写与无注解全休小节用 `z32|`——我们空小节写 `Z|`，符合。

（注：本地引擎在 381 已实证接受我们形态的谱面；上述 1/2 是否影响 yue2.cpp 的 prompt 组装/渲染先验，属真栈验证腿问题，本报告不预断。）

### 2d. Vocal→Ins 迁移（官方 transfer）

官方默认工作流是「模型写谱 → Vocal 音符全部移到 Ins → 新谱作为固定输入重渲染」。我们**不需要这一步**：学生旋律本来就在 Ins、Vocal 全休止（381 谱面层已证）。迁移规则只对「用户给 Vocal 旋律」的 cover 场景才需要，不在本卡接入面。

---

## 三、差距清单 + 接入方案（最小档）+ checkpoint 盘点结论

### 3a. 停止条件检查：本地 checkpoint 是否缺 instrumental 所需件

**未触发停止条件。** 官方 instrumental 工作流所需模型件 = **与歌曲模式完全相同的一对**（YuE2-3B + YuE2-Vae），无独立器乐 checkpoint、无 LoRA、无额外件（§2a 亲证）。本地模型根（§1a）持有同权重的量化 GGUF 等价物（YuE2-3B-Q8_0 + YuE2-Vae-F32）。**不存在「仅歌曲 checkpoint、缺器乐曲」的缺口。**

真实的边界（如实声明，不改写）：本地引擎是 **yue2.cpp 量化社区移植**，官方 `instrumental.py`（pin yue2-infer wheel + BF16 safetensors + prefix.npy 断言）**无法直接驱动本地引擎**；接入只能在我们的封装层做「官方等价条件构造」。官方 style 负条件/歌词曲式标签/cot 指令前缀在 yue2.cpp 的 prompt 组装里是否同构生效，本地无既有证据（381 只证了英文词 instrumental 无抑制）。

### 3b. 差距清单（官方等价配置 vs 本地现状）

| # | 维度 | 官方 instrumental | 本地现状 | 差距定性 |
|---|---|---|---|---|
| 1 | style 构造 | `Instrumental, ` 前缀 + `, no vocals, no singing, no choir, no spoken words.` 后缀 | 仅尾部 `, instrumental` 英文词（381 证无抑制效果） | **可接**（yue_prepare.go Instrumental 分支改写） |
| 2 | lyrics | 空 或 谱面 `% section`→曲式标签串（`[Verse]\n\n…`） | 恒不发（381 两请求 lyrics_chars=0） | **可接**（YueScoreInput.Sections 已有，缺 lyric_tags 等价生成+填充）；本地引擎接受 lyrics 字段（deploy smoke1 实证），但 **abc+lyrics 组合未在本地引擎测过** |
| 3 | cot | 谱面 Vocal 和弦有无 → full/melody | 固定 `cot=full` | **可接**（extra_params 已可覆盖 cot；缺推导规则） |
| 4 | 谱面细节 | 和弦挂 Vocal 休止行；显式时长 1 省略 | 和弦挂旋律行；恒写时长数字 | **可接**（yue_abc.go 转换器小改；是否影响 yue2.cpp 渲染先验=验证腿问题） |
| 5 | steps/VAE | 32 步；m-a-p/YuE2-Vae FP32 | steps=32 已同；YuE2-Vae-F32.gguf（同权重 GGUF） | **已对齐** |
| 6 | 采样参数 | 官方默认（BF16 AR/NAR、pipeline 内置） | yue2.cpp 引擎内置（日志观测锚点：`CFG=1.00, top_k=100`）；不可经 /synth 配置 | **不可达**（引擎外部面；如实披露，不硬造） |
| 7 | 前缀一致性 | token_prefixes 逐位断言 | 无等价物；本地最强等价=引擎 job 回显+replay JSON 留档（372/381 已有） | **不可达**（不同实现；以回显+replay 作替代证据面） |
| 8 | 渲染入口 | YuE2Pipeline（Python） | POST /synth（yue2.cpp） | **已存在**，无需新入口；差距全在 body 字段内容 |

### 3c. 接入方案（最小档步骤拆解，全部在卡面文件域 `agent/internal/genapi` 内；每步标注现有/缺失）

- **S1（现有+小改）** `yue_prepare.go` Instrumental 分支：style 规范化——确保 `Instrumental, ` 开头 + 追加负条件后缀（逐条件查重，照官方 prepare 语义）；disclosure actions 同步改写。改动面：单函数。
- **S2（缺失→新增）** 曲式标签歌词通道：从谱面 `% section`（或 YueScoreInput.Sections）生成 `[Verse]\n\n…` 形态字符串，Instrumental 时填 `res.Request.Lyrics`（`--empty-lyrics` 等价=保留不发选项）。改动面：yue_prepare.go + GenerationRequest 传递链（已有 Lyrics 字段，genapi.go）。
- **S3（小改）** 谱面细节对齐：`ConvertMidiToYueABC`（a）和弦注解移挂 Vocal 休止行（官方形态，b）显式时长 1 省略。**注意回归面**：`ParseYueABC`/`RestYueVocalPart`/YUE2-ABC-ADHERENCE-1 金样与 W3 往返断言需同步；和弦挂载声部变化会影响既有测试期望。
- **S4（缺失→推导）** cot 推导：谱面 Vocal 行有和弦 → cot=full，否则 melody；经 prepare 写入请求（引擎已证消费 cot——deploy smoke1 发 cot、381 日志 `[Prompt] cot=full`）。
- **S5（缺失→验证腿干跑项）** 本地引擎两个未证接受度：①abc+lyrics 组合；②cot=melody。零生成成本即可核（POST /synth 后引擎 job 回显只读确认，不等待渲染完成即取消/或用最短 duration）；**建议真栈腿先干跑这两项再生成**。
- **S6（现有）** disclosure 面：`Limitations`（dialects.go:363-364「yue_local has no instrumental switch…」）与 `not_forwarded` 文案按接入后口径更新；`YueDispatchedSummary` 反映 lyrics/cot 实发。
- **S7（现有复用）** 验证腿装置：复用 `scripts/yue_wire_smoke.py` 腿形态（W3 谱面断言 + W9 转写旁证 + 用户人耳终审）；不需要新机制。

**预期收益的诚实边界**：官方负条件 style + 曲式标签歌词 + cot 指令在官方栈是器乐形态的构成性条件；在 yue2.cpp 上是否有同等抑制效果**无本地先验证据**（381 反例：英文词 instrumental 无抑制）。接入腿跑通 ≠ 人声消失；最终裁决=用户人耳（卡面已预置 fallback：仍出成段人声→回退「带人声+披露」口径）。

### 3d. 接入腿建议（一句话）

先 S5 干跑（abc+lyrics 组合 + cot=melody 接受度，零生成），再按 S1→S2→S4 实现（S3 视 381 谱面已被引擎正常解析的证据可降为可选），S6 收口披露，S7 真栈 1-2 次生成+用户听核。

---

## 锚点索引

- 本地封装链：`agent/internal/harness/gen_generate.go:295`、`agent/internal/genapi/engine.go:75`、`agent/internal/genapi/config.go:156`、`agent/internal/genapi/dialects.go:106,211`、`agent/internal/genapi/yue_prepare.go:57,79`、`agent/internal/genapi/yue_abc.go:75,570`
- 372/381 run 工件：`coord/runs/GEN-YUE-WIRE-1/run_20261116_000009/`（genapi_providers.json、engine_supervisor_stdout/stderr.log、wire_report.json、job_v1/v2_trail.json）、`coord/runs/YUE2-PC-DEPLOY-1/run1_20261019_201652/`（results.md、engine_hash.txt、smoke1_req.json、gen.sh）、`coord/runs/YUE-VOCAL-FORENSICS-1/REPORT.md`
- 引擎部署（只读盘点）：`E:\AI\YuE2\`（engine/runtime.json、start_engine.ps1、models/5 GGUF）
- 官方依据（2026-11-16 主干 raw）：`skills/yue2-music/instrumental/SKILL.md`、`scripts/instrumental.py`、`assets/release.json`、`references/setup.md`、`assets/example-ins.abc`、`assets/example-style.txt`、`examples/instrumental_clip.py`
