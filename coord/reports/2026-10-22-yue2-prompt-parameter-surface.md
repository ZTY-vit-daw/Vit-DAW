# YuE2（本机 yue2.cpp 部署）提示词格式与参数面调查（2026-10-22，决策侧调查）

- 依据：用户 2026-10-22 要求——查本机部署 YuE2 的「内置/后训练所用提示词最优格式」与「可稳定执行的参数指标」，按 AI 参数 vs 音乐语义参数归类，为面向用户的套壳（提示词模板框架）提供设计输入。
- 性质：**只读调查**（短暂拉起引擎取 `/props` 一次后即关闭；无生成请求、无仓库代码改动）。
- 对象：`E:\AI\YuE2\engine\yue-server.exe`（yue2.cpp **c8516db**，2026-10-09 构建）+ YuE2-3B-Q8_0 + YuE2-Vae-F32（部署承 YUE2-PC-DEPLOY-1，2026-10-19）。
- 证据来源分层：①引擎二进制内嵌字符串（系统提示词、字段校验、日志格式串——直接提取自 exe）；②引擎 `/props` 实测默认值（2026-10-22 本机）；③引擎内嵌 WebUI 的表单/tooltip（同 exe 内嵌 HTML）；④本机已验证行为（YUE2-PC-DEPLOY-1 冒烟 + YUE2-ABC-ADHERENCE-1，29 样本）；⑤官方 demo 画廊（map-yue2.github.io，WebUI 内嵌 demo 与之同源）。

## 0. 结论（先看）

1. **「后训练提示词格式」可从引擎侧完整还原**：模型内置三条系统提示词（对应 cot 三档），用户输入面固定为 `[Tags]`（风格标签串）+ `[Lyrics]`（分段歌词）两个标记块；`cot=full`（默认）时模型先写一份「带和弦标注的 ABC 谱」再出音频语义码——这就是官方所称 symbolic planning（谱面先行的创作管线）。**最优格式 = 英文逗号风格标签串 + [段落标记] 分段歌词**，官方 109 个 demo 全部是这个形状。
2. **音乐语义控制面远不止 style 文本**：请求里可以直接喂 `abc` 字段（原生方言谱：调号/拍号/速度/和弦/双声部/段落注释），本机 29 样本实测**调性服从 100%、和弦根音 0.98、旋律轮廓 0.99**；速度是软控制（132 → 实得 ~111）。这是套壳「专业模式」的硬承重墙。
3. **参数分两层完全成立**：引擎自身的 WebUI 已经这么做——常用层（Style/Lyrics/Score/Mode/Duration/Seed）+ 「Advanced and post-processing」折叠层（采样/CFG/steps/batch/格式等）。套壳照此分层即可，且比引擎 UI 更进一步：把 `cot/steps/solver/采样` 全部固定为验证过的基线值，不暴露。
4. **稳定基线（本机已验证）**：`cot=full、steps=32、wav16、duration≤360、Q8_0、--max-seq 24576`，采样参数全用引擎默认（即官方推理默认）；同 lm_seed 重跑不逐字节确定，**replay（semantic_tokens 重放）逐字节确定**；一次生成可产最多 9 个声学变体（synth_batch_size）。

## 1. 内置提示词格式（后训练原生格式，引擎二进制提取）

### 1.1 三条内置系统提示词（按 cot 档位，原文）

| cot | 系统提示词（原文） | 含义 |
|---|---|---|
| `off` | `Generate music with codec tokens from the given conditions.` | 直出语义码，不做谱面规划 |
| `melody` | `Generate a melody-only ABC transcription without chord symbols, then generate music with codec tokens from the given conditions.` | 先写单旋律 ABC，再出音频 |
| `full`（默认） | `Generate a chord-annotated ABC transcription, then generate music with codec tokens from the given conditions.` | 先写带和弦 ABC，再出音频（官方榜单配置） |

官方页明示榜单配置为 "melody-and-chord symbolic planning"，即 **cot=full 是旗舰档**。

### 1.2 用户输入面：`[Tags]` + `[Lyrics]` 两个标记块

引擎 prompt 组装含 `[Lyrics]`、`[Tags]` 标记；喂 abc 时还有 `score of song %d as written:` 段（把外部谱当模型自己的谱喂回去——这是 replay/续写的机制基础）。

**style（Tags）串惯例**（官方 demo 实例，引擎 WebUI 内嵌）：

```
Jazz ballad, brushed drums, upright bass, warm piano, muted trumpet, intimate Mandarin male vocal, 60 BPM
Mandarin city pop, female vocal, warm electric piano, bass and drums, 108 BPM        ← 本机冒烟所用
Boogie woogie style of the 1930s, fast fun, virtuoso, piano dialogue and soft, muffled female vocals
acoustic piano, lofi pop, jazz, trap                                                  ← 本机 360s 腿所用
```

构成要素：**曲风流派、乐器编制、人声（性别/语言/质感）、情绪/年代/场景修饰、BPM**。英文逗号分隔；BPM 写在 style 里是软影响（模型自定），写进 ABC 的 `Q:` 是谱面级硬约束（但仍偏软，见 §3.2）。

**lyrics 分段标记**（官方 demo + 本机实测通用）：`[Verse]/[Verse 1]/[Chorus]/[Pre-Chorus]/[Post-Chorus]/[Bridge]/[Intro]/[Outro]/[Final Chorus]/[End]`。段名大小写不敏感（本机 `[Verse]` 与官方 `[Verse 1]` 均可）；中文歌词直接支持（UTF-8，命令行内联会报 invalid JSON，须走文件/HTTP body）；Outro 可用 `(词) (词) (词)` 括号逐词写法控制拖长尾（官方 demo 有此用法）。

### 1.3 ABC 原生方言（`abc` 字段，谱面级音乐语义面）

本机 29 样本验证过的方言规范（naive ABC 会丢旋律，corr 0.16–0.20，**必须按方言写**）：

```
X:1                                    # 曲头
T:                                    # 标题
M:4/4                                 # 拍号
L:1/32                                # 单位音长
Q:1/4=106                             # 速度（谱面级，软）
V: Vocal clef=treble name="Vocal Melody" snm="Vocal"    # 声部1：人声旋律
V: Ins clef=treble name="Ins Melody" snm="Inst."        # 声部2：器乐旋律
K:C                                   # 调号
% intro                               # 段落注释（结构标记）
V: Vocal
"Dm7"c4A4z12A4c4e4-|"G7"e4f8d12z8|    # 小节内 "和弦" 前缀 + 音符（z=休止 -=连音 '记八度）
V: Ins
Z4|                                   # Z = 多小节休止
% verse ... % chorus ... % outro
```

要点：**双声部 + 小节内引号和弦标注 + `% 段落注释`** 是方言三要素；K:/M:/Q: 承载调号/拍号/速度；段落顺序须与歌词段落对应。

### 1.4 风格词的作用机制（2026-10-22 追问补充）

问题：style 关键词如何影响音频？「LLM 理解」还是「训练标签」？

**答案：两者不对立。推理期无标签库、无检索**——style 被切成文本 token 进入同一 transformer 的上下文，全部影响发生在条件生成里；该能力的来源是训练时学到的「标签文本→谱面→语义码」条件分布（自动构造的配对数据压进权重），叠加统一文本词表带来的语言泛化。证据链：

1. GGUF 元数据（本机实测）：`YuE2ForCausalLM`、vocab **184704**、tokenizer `gpt2` 型 BPE——文本与音乐 token 同词表同模型；latent_type=vae/latent_dim=64（NAR→VAE 流程内建）。
2. 论文（arXiv:2609.33757 摘要）：单一 AR-NAR MoT「先写谱→展开成语义码→实现为全曲音频」；训练用**无对齐谱面的录音**，符号监督来自 SheetSage2 转谱、语义监督来自 MERT2——即配对靠自动转谱/标注构造，风格 caption 形态与官方 demo 标签串一致。
3. 影响通道分两跳：①AR 写 ABC 谱时（调性/速度/和声语言的**统计倾向**——解释 style 里 BPM 为何是软的：硬控制面是模型自己写的谱，style 只通过条件概率影响写什么谱）；②AR 写 25Hz 语义码时（**音色/编制/人声/混音质感在此兑现**——语义码是从真实音频蒸馏的声学语义表征）。NAR/VAE 一跳基本执行语义码的决定（replay 逐字节确定、换 noise seed 只改声学渲染佐证）。
4. 未测面（如实）：本机 29 样本全程固定同一 style，「同谱换 style」剂量实验未做——style 与 abc 的控制域边界（尤其钉死谱后 style 对音色的剩余影响力）待补验证卡（建议形态：同 abc × 3 style × 2 seed，转谱+听感对比）。

## 2. 参数全量清单（引擎 `/props` + 请求校验串提取，2026-10-22 实测）

### 2.1 引擎默认请求对象（`/props` → defaults，原文）

```json
{"style":"","lyrics":"","abc":"","cot":"full","duration":360.0,
 "lm_seed":-1,"seed":-1,"steps":32,"lm_batch_size":1,"synth_batch_size":1,
 "abc_sampling":{"temperature":0.7,"top_p":0.9,"top_k":30,"repetition_penalty":1.005,"penalty_window":100,"min_tokens":32,"max_tokens":4096},
 "semantic_sampling":{"temperature":1.0,"top_p":0.95,"top_k":100,"repetition_penalty":1.2,"penalty_window":50,"min_tokens":200,"max_tokens":9000},
 "semantic_tokens":"","solver":"midpoint","abc_continue":false,"companion_scale":1.0,"cfg_scale":-1.0,
 "output_format":"mp3","peak_clip":10,"mp3_bitrate":128,"adapters":[],
 "harmony":{"identity":"root","strength":0.0,"window":16,"hold_limit":8,"outside_bonus":0.0,"outside_limit":0.25,"section_strength":0.0,"section_open":4,"follow":[]}}
```

（本机验证请求固定 `output_format="wav16"`；seed 用具体整数。）

### 2.2 【音乐语义参数】——用户优先层（套壳第一屏）

| 参数 | 载体 | 取值/格式 | 控制强度（本机实测） |
|---|---|---|---|
| 曲风/流派 | style | 英文标签，逗号串（jazz ballad, lofi pop, trap…） | 软（模型综合解读） |
| 乐器编制 | style | 英文标签（brushed drums, upright bass…） | 软 |
| 人声：性别/语言/质感 | style | female/male vocal, Mandarin…, intimate/muffled… | 软，官方 demo 常用 |
| 情绪/年代/场景 | style | 英文修饰（1930s, fast fun, melancholic…） | 软 |
| 速度 BPM | style「N BPM」或 abc `Q:` | 60–132 实测过 | **软**：Q:106/120 达 ±8%；132→实得 ~111（响应约 65%）——套壳须按"倾向项"呈现或配检测回读 |
| 调号 | abc `K:` | C/G/D/Eb… | **硬**：精确匹配 15/15、4/4（转调腿跟随到 Eb） |
| 拍号 | abc `M:` | 4/4 等 | 硬（转谱 downbeat_meter 判定跟随） |
| 和弦进行 | abc 小节 `"Dm7"` 标注 | root+quality（Dm7/G7sus4/Cmaj7…） | **硬**：根音直接匹配均值 0.98，替换和弦被跟踪 0.88 |
| 旋律 | abc 音符流 | 双声部音符 | **硬**（方言内）：轮廓相关中位 0.99；naive ABC 0.16–0.20（方言失配） |
| 曲式结构 | 歌词 `[段落]` + abc `% 段落注释` | Verse/Chorus/Bridge/… | 结构骨架可靠（M5 目视达标；跨 seed 段落一致） |
| 歌词 | lyrics | 任意语言；UTF-8；总输入 ≤65536 字节 | 逐字入唱（PER 可测） |
| 歌词-时间对位 | `lyric_schedule`（API 级） | sections[{start_sec, lyric[c0,c1], lead_sec≤30, behind, mode bias/mask}]，1–256 段，谱序排列 | 高级项：把歌词段落钉到秒位（本机未实测） |
| 和弦开放度/保持 | `harmony`（API 级） | identity root/spelling; strength 0–64; window 1–512; hold_limit 0–64; outside_bonus 0–20; outside_limit 0–1; section_strength 0–64; section_open 1–16; follow ≤64 段名 | 高级项：谱面写作期的和弦倾向引导（默认全 0=关闭；本机未实测） |
| 目标时长 | `duration` | 60/180/360 实测过 | **上限非保证**：模型自行收尾（60→29.7s、180→99.6s、360→127.7s）；上下文预算≈260s@max-seq8192、24576 已放宽到模型满上下文 |

### 2.3 【AI 参数】——引擎/高级层（套壳默认折叠或隐藏）

| 参数 | 默认 | 范围/枚举 | 说明（引擎 tooltip 原文意） |
|---|---|---|---|
| cot | full | full / melody / off | 「AR 半边在写码之前先写什么」——谱面规划档位 |
| cfg_scale | -1（自动） | 数值；1.0=单分支省一半算力 | 阶段引导强度；>1 需无条件前缀分支 |
| steps | 32 | 正整数 | 声学 ODE 中点法步数（flow matching） |
| solver | midpoint | midpoint / ab2 | 声学 ODE 求解器 |
| abc_sampling | temp 0.7, top_p 0.9, top_k 30, rep 1.005, win 100, min 32, max 4096 | 「protocol bounds」内 | 谱面（ABC）写作期的采样参数组 |
| semantic_sampling | temp 1.0, top_p 0.95, top_k 100, rep 1.2, win 50, min 200, max 9000 | 同上 | 语义码写作期的采样参数组 |
| lm_seed | -1 随机 | 整数 | **谱/旋律/长度都由它定**（换它=换一首歌） |
| seed（noise seed） | -1 随机 | 整数 | 声学噪声种子（换它=同一首歌不同渲染） |
| lm_batch_size（songs） | 1 | ≤ --max-batch（本机部署=1） | 同 prompt 出多首歌，seed=lm_seed+index |
| synth_batch_size（variations） | 1 | **1–9** | 同谱同码最多 9 个声学变体，seed=noise seed+index |
| output_format | mp3 | mp3 / wav16 / wav24 / wav32 | WAV32=原始 IEEE float 不归一化 |
| mp3_bitrate | 128 | kbps | mp3 码率 |
| peak_clip | 10 | 数值 | 输出峰值钳制（后处理） |
| companion_scale | 1.0 | [0,2] | 常驻解码器 adapter 融合强度 |
| adapters[] | [] | {name, scale, ar_scale, nar_scale} | LoRA 热插拔（--adapters 目录扫描 safetensors） |
| abc_continue / continue_semantic_tokens | false | bool | 从谱开头续写 / 从语义码续 |
| semantic_tokens | "" | token 串（replay） | 精确重放（**逐字节确定**），A/B 与回归的正道 |
| duration 的时间语义 | — | — | duration→帧预算（AR 帧预算按 duration 钳制），prompt+score+歌长受 max-seq 约束 |

### 2.4 服务/引擎级参数（CLI，非请求字段）

`--model --vae --transcriber --tokenizer --adapters <dir> --companion <file> --host --port --max-batch --keep-loaded --max-seq --vae-core --vae-halo --no-fa --clamp-fp16`。本机启动脚本固定：127.0.0.1:8087、--max-seq 24576、transcriber(SheetSage2)+tokenizer(v4) 已挂。HTTP 端点：`/synth /job /health /props /logs /tokenize-text /tokenize /transcribe`（job id 为十六进制串；结果 multipart=JSON replay+音频）。

## 3. 可稳定执行基线（本机证据汇总）

1. **验证过的请求形态**（YUE2-PC-DEPLOY-1 + ABC-ADHERENCE-1，全部成功）：
   `{"style": <英文标签串>, "lyrics": <[段落]歌词>, "abc": <方言谱，可选>, "cot":"full", "duration":60|180|360, "seed": <int>, "steps":32, "output_format":"wav16"}`
2. **吞吐/资源**：60s 歌墙钟 ~34s（≈0.55×实时），峰值显存 ≤6.3GB（含桌面基线），任务间自动卸载回落；360s 请求不拒（max-seq 24576）。
3. **确定性模型**（重要，套壳功能设计基础）：
   - 同 lm_seed 重跑：谱面层近乎一致（方言钉死后跨 seed 谱面相关 0.77–1.00，谱面由 lm_seed 主导），声学层随 noise seed 变；
   - 同 (score, semantic_tokens) replay：**逐字节确定**（smoke5a==smoke5b）；
   - 「换版本」= 固定 semantic_tokens 换 noise seed/变体；「重抽一首」= 换 lm_seed。
4. **硬边界**：style+lyrics 合计 ≤65536 字节；synth_batch_size 1–9；songs 受 --max-batch=1 限制（本机默认 1）；duration 是天花板不是保证（最长实测 127.7s）；中文歌词必须 UTF-8 走 body/文件；权重 CC BY-NC 4.0（研发非商用）。

## 4. 套壳提示词模板框架建议（面向音乐用户）

三层结构（音乐语义在明，AI 参数在暗，全部固定为 §3 基线值）：

```
┌ 第一屏（音乐语义，用户可理解可掌握）
│ ① 风格卡：曲风（多选标签）+ 乐器 + 人声（性别/语言/质感）+ 情绪 → 编译成英文 style 标签串
│ ② 歌词编辑器：中文/英文歌词 + 段落按钮（[Verse]/[Chorus]/[Bridge]…）→ 输出分段标记文本
│ ③ 音乐设定：调号（K:）、拍号（M:）、BPM（Q:）、和弦进行（可选小节网格）
│    → 套壳按 §1.3 方言编译成 abc 填入请求（不要求用户写 ABC；高级用户可切原生 ABC 编辑器）
│ ④ 目标时长：滑条 30–360s，UI 标注「模型可能提前收尾」
│ ⑤ 一键生成 / 换个版本（noise seed）/ 重新创作（lm_seed）——seed 语义用按钮表达，不露数字
├ 第二屏（专业折叠）：原生 ABC 编辑、lyric_schedule 时间对位、harmony 和弦引导
└ 第三屏（AI 参数，默认收起+警示）：cot/steps/solver/cfg/双采样组/seed 数值/输出格式
   —— 建议默认锁定基线值；解锁需显式「专业模式」开关
```

关键设计决策依据：
- **BPM 呈现为「倾向」而非「保证」**（132→111 实证；专业模式配 SheetSage2 `/transcribe` 回读验证，测量链本机已部署）；
- **调号/和弦可直接承诺**（100%/0.98 实证）——这是套壳相对 Suno 网页版的差异化卖点；
- **「多版本试听」用 synth_batch_size（≤9）**：同曲同谱不同声学渲染，成本仅 VAE/flow 段；
- **「可复现」用 replay 链路**：保存 replay JSON 即可逐字节重放，接 Vit-DAW 的 A/B 与回归资产。

### 4.1 风格卡=策划词表（卡池选择制）裁定输入（2026-10-22 追问补充）

用户判例：「孟菲斯风格」直输可命中，但别名（mafia666）与近邻风格（lofi trap）易 miss。本日补充证据与结论：

1. **官方 demo style 形态杂**（本机 WebUI 全量提取）：纯标签串（`Pop,Rock,Country,Medium,Anticipation`）、整段叙事 caption、全中文串（`重金属,华丽,黑暗,激流,男声`）、德/西语混拼（`hammondorgel`/`Sofisticado`）均有——训练 caption 多语言杂源，模型语言泛化面宽但不可预期。
2. **memphis/phonk 在本地官方 demo 中 0 次**——用户那次命中纯靠 LLM 泛化，不可作产品依赖。
3. 两种 miss 机制：**别名 miss**（mafia666=分布外 token，BPE 碎片化、信号稀释）与**近邻风格 miss**（lofi trap=命中风格空间另一个真实点，用户无感知，更危险）。
4. **裁定建议**：主入口=策划词表（维度化：曲风子树/乐器/人声/情绪/质感/年代/语言；canonical 英文 tag+中文显示名+别名集）；种子=本地官方 demo 串+WildSongBench 192 prompts（待拉取）+探针产出；新标签须过**同谱同 seed 换 style 探针**转正（与 §1.4 未测面的剂量实验卡合流）；词表带版本、miss 回流评审。自由输入不封死，作降级通道（别名表→受约束 LLM 映射入池→低置信标记+可见可编辑）。

### 4.2 构筑系统（卡牌/卡组隐喻）架构裁定输入（2026-10-22 用户提出）

用户提案：生成功能=卡组构筑；把卡牌全交给用户实现自由构筑，在其上再建自由输入（自定义卡）、风格复制（卡组链接）等上层系统。评估：**隐喻在工程上成立，采纳为套壳产品框架**。映射与精修：

- **卡牌=探针验证过的控制项**，卡面印**效果强度**（概率卡/确定卡）：调号卡/和弦卡=确定效果（100%/0.98），BPM 卡/风格卡=概率效果（65%/软）——比设置面板诚实。
- **卡组=一次生成的完整构筑**（style 卡组+歌词+可选谱面卡）；**卡组上限**有工程必要性：标签越多注意力越稀释（官方 demo style 串均短），上限即防稀释，非纯风味。
- **卡牌全开的边界**：指音乐语义面全开；AI 参数（cot/steps/采样）= 规则引擎/机舱，不进卡池；未探针的高级面（lyric_schedule/harmony）=「未解锁卡」先锁后放。
- **两级复现**：卡组链接=复现构筑（序列化卡片+参数+谱面引用，**不承诺复现音频**——同 lm_seed 重跑非逐字节确定）；replay JSON=录像（semantic_tokens 重放逐字节确定）。技术约束在游戏语言里天然自洽。
- **seed 语义的玩家语言化**：重抽一首=换 lm_seed；同曲多渲染=synth_batch_size≤9（换手牌）；把要藏的 AI 参数翻译成构筑系统动词。
- **发牌员=受约束 LLM**：从池内发牌+协同建议（如「female vocal+Mandarin+city pop」协同、反协同警告），不发明池外卡——§4.1 的 LLM 翻译器的隐喻化收编。
- **未来卡**：参考曲拷谱（SheetSage2 `/transcribe` 已部署：参考音频→ABC 谱面卡半自动构筑）；VMT 两级架构=自动构筑器（AI 代组卡组）。
- **建设顺序**：卡牌库（数据资产：canonical tag+别名+效果强度+协同标签+探针状态+版本）→ 构筑器 UI → deck code 序列化 → 发牌员 LLM → 自定义卡通道。

### 4.3 卡面效果量的认识论修正（2026-10-22 追问补充）

用户质疑「概率触发」能否计算、影响目标是什么。复核与修正：

1. **数字自纠**：§3 与 ABC 回执的「速度走完约 65%」经 `metrics_rows.json` 复算不成立——原始数据：基准 Q:106→实测 106；扰动 Q:132→实测 110/112，自然定义剂量响应 (111−106)/(132−106)≈**19–21%**。回执 65% 属指标定义未锁定，卡面数字须先锁定义（已在本报告修正引用口径）。
2. **可计算的是两类统计量，非游戏式精确概率**：依从率（P(转谱属性=请求属性)，频率估计，N=15 的 100% 实为 ≥78% @95%CI；印 ≥95% 需 ~60 零失败样本）与剂量响应幅度（移动比例）。二者不得混称「概率」。
3. **概率空间=（lm_seed, noise seed）抽样+引擎非确定性；条件于引擎版本/量化/基准卡组**，换版本旧数作废。
4. **影响目标分层**：最终是音频，但「符合风格」非波形谓词，须在测量栈操作化——谱面层（replay JSON 直读，零噪声）/音频属性层（SheetSage2，噪声已表征）/风格相似层（CLAP 类嵌入，仅方向）/听感层（真值，主观贵）。卡效果规格=作用层+验证层+效应量+条件+N+CI+版本。
5. **新证据：卡牌效果不可加且条件于卡组**——B_tempo 腿 style 文本「108 BPM」与谱面 Q:132 冲突，输出 110–112：**风格文本压过谱面速度标记**。「Q: 卡」效果非常数，协同/冲突须基准卡组探针抽查，不可全组合枚举。
6. **卡面呈现裁定**：不印裸概率；印效果等级徽章（确定/倾向/方向已验证/未验证）+背后版本化规格书；探针两档——方向探针（每卡必过，~6 跑）与深度依从探针（仅印数字卡，30–60 跑）。

### 4.4 游戏规则层：玩法/输赢/完成条件（2026-10-22 用户提出，裁定输入）

用户追问：按「生成=游戏」的逻辑，最优先问题是玩法是什么、能否输赢、完成条件是什么——此前 §4.1–4.3 只设计了卡牌内容，未设计规则层。裁定：

1. **玩法=核心循环四动作**：组卡（构筑）→出牌（生成）→**侦察**（replay 谱面直读/SheetSage2 回读/CLAP 提示/耳朵）→裁决（接受/改构筑/弃局）。侦察腿是与老虎机型套壳的本质区别（miss 可诊断、改构筑有依据）。技能表达四来源：卡牌知识/构筑协同/侦察解读/预算管理。变体：锁定局（固定 lm_seed=纯构筑技巧）vs 开放局（换 seed=方差管理，配 synth_batch_size≤9 多手牌）。
2. **能否输赢：球场不是对手**。无对系统输赢；三种真实输法=规格未达预算耗尽/主动弃局/假胜利（机械达标审美未达）。**双裁判红线**：机械裁判（转谱链，判可验证规格，可选 par）+审美裁判（仅玩家，系统永不裁定品味——CLAP 只提示不判决，防古德哈特）。
3. **完成条件=玩家宣布接受终稿**，定格三件套（构筑快照+replay 录像+终稿音频）=战绩/分享物；弃局为合法终局且产出元进度（miss 回流入池、自定义卡探针转正）。规格可行性由卡面规格书事先告知（如 360s 卡印实测上限）。
4. **架构反推**：下一工件=游戏规则文档（循环/局内状态/预算旋钮/终局宣言/双裁判规格），优先于 UI。**游戏即评估**：规格达标局=benchmark，replay/转谱/探针机器三用（产品/评估/研究），VMT=本游戏 AI 玩家。

### 4.5 三层架构定形：语义层/引擎适配层/双入口（2026-10-22 用户裁定）

用户裁定：GUI 自行组卡生成与 agent 调用生成并存；套壳以 YuE2 为原型打造，完成后后端接入 Suno/Udio 等，实现跨引擎生成可控。定形：

1. **三层**：卡牌语义层（引擎无关）→ 引擎适配层（YuE2 原型→Suno/Udio）→ 双入口（GUI=人类玩家客户端，agent=工具调用客户端）。
2. **关键裁定：卡牌跨引擎，效果规格每引擎一份（卡×引擎矩阵）**。同一张卡在不同引擎效果等级不同（YuE2 调号卡=确定 100%；Suno 无谱面通道→降级为 style 文本软提示=倾向）。**跨引擎构筑兼容性检查必须内置**：deck 编译时明报未生效/降级的卡，禁止静默丢弃。「生成可控」=语义可控+可复现跨引擎成立，控制程度逐引擎可见。
3. **双入口=一份卡组契约、两个客户端**：deck JSON 即接口契约；agent 侧对齐既有 genapi provider/dialect 体系（套壳=genapi 语义上层，非另起炉灶）；**AI 玩家用同一本规则书**（agent 读同一份卡×引擎效果矩阵——游戏即评估在此兑现）；队列纪律继承真栈串行裁定，GUI/agent 共用队列不绕过。
4. **引擎路线**：YuE2 原型期 deck 格式即设计为引擎无关（引擎参数 cot/steps/采样不进 deck，留适配层），适配接口以 YuE2 单实现定型，不为未接入引擎提前抽象；Suno/Udio 接入期如实处理可靠性等级差（无官方 API/积分计费/条款与许可边界异于 CC BY-NC）——预算腿按引擎换货币（GPU 时间 vs 积分）、侦察腿回读按引擎降级（无 replay 录像，仅音频+元数据），差异记入每引擎规格书。

### 4.6 呈现裁定：规格内部化，诚实性走赛后战报（2026-10-22 用户裁定）

用户裁定：按 YuE2 模板建卡牌系统并复制到其他平台，产品对外**不显示**生成确定/不确定的概率与效果等级。定形：

1. **卡×引擎效果矩阵完全内部化**：探针照跑、规格照记（agent 的规则书、deck 编译器、新卡转正治理仍依赖它），但不上任何用户可见界面。无置信度 UI、无等级徽章、无百分比——构建面简化。
2. **诚实性载体从赛前预测移到赛后战报**：生成后回读面板展示「请求 vs 实测」（调性/BPM/时长等，SheetSage2 链）——**报事实不报预测**。用户通过游玩学会卡牌手感（与真实卡牌游戏一致：玩家靠打牌认识卡，不靠印在卡上的概率），营销口径「可控」由每次附带实测验证支撑，比概率声明更强且无过度承诺。
3. **唯一保留的可见标记是功能支持而非概率**：某引擎不支持/无法编译的卡在构筑器中置灰提示（「此引擎无谱面通道」），不做静默丢弃——这是能力声明，不属于概率显示范围。
4. 营销边界沿用既有红线：不逐卡承诺精确数值控制；对用户的承诺形态=「你组卡、它生成、战报可验证」。

### 4.7 自定义卡牌=素材卡（adapter 承载，回归采样工作流）（2026-10-22 用户提案，裁定输入）

用户提案：YuE2 支持微调添加风格参考（约 30 首歌可训相似风格/歌手目标），用户自建风格标签用于后续创作——不再是黑盒，无限接近传统采样玩法。评估：**架构成立且引擎请求面原生预留**（`adapters:[{name,scale,ar_scale,nar_scale}]`、`--adapters` 热插拔、companion_scale），采纳为自定义卡牌第二形态。设计：

1. **卡牌类型层**：文字卡（tag→style 串，概率性、跨引擎可移植）vs **素材卡**（LoRA 文件承载，身份携带型资产；卡背=文件+scale 强度档位；「不再是黑盒」=控制力来自素材本身而非对声音的描述）。炼卡流程=新生产职业（30 首→数据管线→训练→质量门→入池带版本）；探针照用（同 deck±adapter 配对对照+scale 扫描）。
2. **本机可行性校准**（承 2026-10-18 微调评估，非新判断）：NAR-only（风格/音色底色，无歌手身份）有 4070S 12GB 实证；AR+NAR joint（歌手身份）**无 12GB 实证=贴红线 spike**；缓解=量化基座+60s 裁剪+关预览+停引擎错峰；兜底=云端 4090（~¥10/4h）训完拷回。**格式陷阱必验**：yue-server 只吃 LoRA（B@A）不吃 LoKr，HOT-Step 默认 LoKr 不可直接热插拔。质量门预声明：盲听+ASR-PER+非触发 prompt 回归；best checkpoint 常在中途；止损线沿用评估报告。
3. **采样类比精确化**：①法务同构——采样要 clearance，素材卡要来源声明（自有/授权素材；克隆他人声线=未清权采样），衍生权重 CC BY-NC+署名；②**拥有的是分布不是采样本体**——同卡不同 seed 仍在分布内采样，确定性由 replay 层补回：素材卡=分布资产，replay=实例资产（传统采样中一体的两层在此拆开）。
4. **生态位**：素材卡=YuE2 原生壁垒（Suno/Udio 无用户可训开放 adapter）；他引擎置灰或映射 persona，卡×引擎矩阵照处理。deck 引用素材卡的分享（文件分发+署名+体积）为开放设计题，原型期先私有卡。
5. **落地**：微调评估的 2–3 天 LoRA spike（8–12 首同风格、LoRA 配方、预声明门）获得产品身份=首张素材卡可行性验证腿。

## 5. 诚实边界

- 「后训练提示词」还原的是**推理引擎组装的对话模板**（系统提示词 + Tags/Lyrics 标记 + 谱面回喂）；训练数据本身不公开（官方只发权重+推理，见微调评估报告）。官方 demo 画廊（109 例）与三条系统提示词一致，可视为「模型被喂惯了的最优格式」的强证据。
- 引擎是社区 C++ 实现（yue2.cpp c8516db，MIT），非官方 PyTorch 栈；字段/默认值以该构建为准，升级引擎须重核 `/props`。
- 采样参数的「protocol bounds」具体数值不在字符串内（数值常量），本文只给默认值；越界会被 `sampling preset outside the protocol bounds` 拒绝。
- lyric_schedule / harmony 两面在本机**未实测**（仅有字段校验证据），套壳若要用须先立验证卡。
- 速度软控制结论来自 10 跑剂量-响应（N=29 总样本、单引擎单量化单风格族），外推谨慎。

## 6. 证据工件

- 引擎字符串提取：`/tmp/yue2_strings.txt`（会话临时）；关键行：系统提示词 ×3（[Sampling] 前）、`[Lyrics]/[Tags]`、字段校验串（66960–67060 区段）。
- `/props` 实测输出：2026-10-22 本机（引擎拉起后即关，端口 8087 已释放）。
- WebUI 内嵌 demo：boogie/cyber/贝加尔湖畔/友谊地久天长等（title·Version 命名，source=map-yue2.github.io + case_id）。
- 行为证据：`coord/runs/YUE2-PC-DEPLOY-1/run1_20261019_201652/`（smoke1–5 + verify360）、`coord/runs/YUE2-ABC-ADHERENCE-1/run1_20261019_215818/`（29 样本+指标）。
- 官方页旁证：https://map-yue2.github.io/（"melody-and-chord symbolic planning"、"tags, lyrics" 输入描述）。
