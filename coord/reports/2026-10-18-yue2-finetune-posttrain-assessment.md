# YuE2 微调 / 后训练机会评估（2026-10-18，决策侧调查）

- 依据：用户 2026-10-18 追问——YuE2 开源状态下，"微调和后训练"有没有机会与余地。
- 性质：**只读调查**（未安装/未训练/未下载模型、未改仓库代码）。承 [2026-10-18 YuE2 PC 部署勘察](2026-10-18-yue2-pc-local-deploy-recon.md)；推理结论仍为 PC 侧 yue2.cpp GGUF（RTX 4070 12GB，可用约 10.2GB）。
- 事实分层：【官方】官方仓库/许可/论文；【社区实证】可复现的第三方实现与报告；【推断】基于证据的判断。所有链接见 §9。

## 0. 结论（先看）

1. **有微调机会，但只在"适配器/LoRA"这一层；后训练（DPO/RLHF/继续预训练）没有现实余地。**
   - 官方 YuE2（main / v0.1.6）**只有推理代码**，没有训练代码、数据管线或 SFT/DPO/RL 脚本，也没有发布音频→语义 token 编码器（官方 issue #165 仍 open，Discord 截图称计划发布）。官方唯一训练代码在 `YuE-v1` 分支的 `finetune/`（Megatron + DeepSpeed ZeRO-2 + xcodec），**架构不同，不能直接用于 YuE2**。
   - 社区已在 2026-09 之后把链路补齐：Mothersuperior 自举出 real-audio semantic tokenizer v4；HOT-Step-CPP 实现 **AR+NAR 联合 LoKr 训练**；YuE2-Studio 把同一训练器包装成 Windows 一键（声明 ≥11GB 显存）。
2. **12GB 本地训练是"贴红线"而非舒适区。** 4070 可用约 10.2GB，低于 Studio 声明的 11GB 门槛（关掉桌面/浏览器可回到 ~11.3GB）。可行配置只能是：**量化基座 + 60s decoder crop + planner crop + 1 首/update + 关预览 + 训练时停推理**，无 12GB 跑 joint 的公开实证；NAR-only 有 4070S 12GB 跑 22 首约 20 分钟的社区实证。
3. **许可把"研究"和"产品"切得很清楚**：非商用研究/毕业设计免费；个人创作者可卖生成物；但**衍生权重（LoRA/adapter）继续 CC BY-NC 4.0 + 署名**，不能商业销售；公司商用权重须向 m-a-p 申请商业许可。训练数据版权自负。
4. **对我们最有价值的不是"后训练"，而是三件更便宜的事**：①符号乐谱（ABC）编辑 + replay 的推理时适配；②数据管线 + 评估栈（WildSongBench/ASR-PER/AudioBox/CLAP + 盲听 + ABC lint）；③小规模 LoRA spike。三者即使训练失败也有可交付成果。
5. **建议**：如果只是"开发实验"，做 **2–3 天 LoRA spike**（8–12 首同风格歌，先 Studio/HOT-Step，失败即止损）值得；**不立项**任何 DPO/RLHF/继续预训练。若目标是论文创新点，诚实说：LoRA 迁移本身创新性有限，真正有差异的是"DAW-in-the-loop 的可控生成/评估"，需另立研究问题，且与本轮答辩时间线冲突。

## 1. 官方训练面（事实）

| 项 | 事实 | 档位 |
|---|---|---|
| main / v0.1.6 目录 | 只有 `src/yue2/` 推理代码（pipeline/modeling/nar/sampling/quantization/tokenization 加载器）+ tests + CLI；**无 train/、configs/、数据管线、SFT/DPO/RL** | 官方不支持训练 |
| `YuE-v1` 分支 | 有完整 `finetune/`：`finetune/README.md`、`core/preprocess_data_conditional_xcodec*.py`、`scripts/train_lora.py`、`run_finetune.sh`、`ds_config_zero2.json`、Megatron 数据管线 | 仅 v1 历史代码（对 YuE2 不可直接用） |
| 音频→语义 token 编码器 | 未发布；`YuE2VAE.encode()` 明确不是它；issue #165（2026-09-11 开）仍在请求；官方仅一张 Discord 截图称计划发布【未文字承诺】 | 官方缺口 |
| 技术报告 | arXiv:2609.33757：披露 AR next-token CE + NAR flow matching、ABC/语义码/latent 三表征、28 层 MoT，附录 C 有训练数据/任务/优化描述；**训练数据与代码不公开** | 方法可读，不可复现 |
| 公开预训练规模 | 官方/社区口径为数十卡 H800 级、数十万小时数据、多阶段训练【报告口径，未逐字复核】 | 与本地不可比 |

**判断**：官方把 YuE2 定位为"发布权重 + 推理"，训练面是**社区补的**，不是官方支持的开发路径。等待官方发布 tokenizer/训练代码可以关注，但不能作为近期排程前提。

## 2. 许可边界（CC BY-NC 4.0 + 2026-09-16 附加许可，原文）

| 问题 | 结论 | 依据 |
|---|---|---|
| 能否造衍生模型（LoRA/微调权重） | 可以，**仅非商业** | CC BY-NC §2(a)(1)(B)"Adapted Material for NonCommercial purposes only" |
| 衍生权重是否继续受限 | 是：CC BY-NC 4.0 + **署名**；adapter 许可不得加额外限制 | "attribution requirements for sharing model weights or **adapted weights**"；CC §3(a)(4) |
| 毕业设计/非商用研究 | 免费、明确允许 | "Students, educators, researchers and academic institutions may use YuE2 free of charge for non-commercial research and education" |
| 个人创作者变现生成物 | 允许（卖生成物/接客户/上商业平台）；**不覆盖公司商用权重** | 个人创作者附加许可原文 |
| 衍生模型生成的产物是否同样可个人变现 | **推断：是**（条款针对"生成并使用生成物"，未区分基座/衍生）；**官方未逐字点名**，正式商业化前建议书面确认 | 同上 |
| 再分发/公开发布 adapter | 可以（CC BY-NC + 署名 + 同许可），**不可商业销售权重** | "does not extend to commercial redistribution or sale of the model weights" |
| 公司商用 | 谈商业许可（ryuanab@connect.ust.hk / lauryliuyang@hkgai.org） | MODEL_LICENSE |
| 训练数据 | 自己负责权利；公开歌词库/歌曲版权风险自负 | 责任使用条款 + 常理 |

**对 Vit-DAW**：毕业设计=非商用，合规；**不得**把权重/adapter 打进商业交付物；公开发布 adapter 必须 CC BY-NC + 署名。若未来公司化，先谈授权。

## 3. 社区训练栈（可用性与能力边界）

| 项目 | 训练技术 / 可训半边 | 数据与准备 | 硬件门槛 | 代码许可 | 成熟度 | 对我们的适用性 |
|---|---|---|---|---|---|---|
| **HOT-Step-CPP** Training Studio | **Joint Training：AR+NAR 各一个 adapter（同 run）**；默认 LoKr dim128（pair≈213MB）；另有 rank64 LoRA 预设；旧配方 Prodigy+KL 锚 | 歌曲文件夹 → Essentia BPM/key + 歌词库（LRCLIB/QQ/酷狗）+ MOSS/云 LLM 一句话 caption `.yue2.txt` + latent cache + semantic codes + SheetSage2 乐谱 + MMS-FA 歌词对齐；-14 LUFS | 训练器≈14GB；整曲 decoder≈12GB；预览并行需 28GB；**量化 GGUF 基座可降 VRAM（Q4_K_M≈2GB）**；16GB 卡需停引擎 | MIT（部分插件 GPLv3，发布包整体需满足 GPLv3——集成时要评估） | 175★，push 2026-10-17，30 open issues，最活跃；配方明确"按技术报告反推"，学习率/丢弃率是猜测 | **首选训练器**；Windows 便携、CUDA 13 构建、数据管线完整 |
| **YuE2-Studio（timoncool）** | 调 HOT-Step `ace-train`（LoKr + Prodigy/歌词对齐，三步向导，断点续训） | 丢文件夹；歌词 LRCLIB/QQ/酷狗；MOSS 写风格；Beat This! 测速 | 官方声明 **NVIDIA ≥11GB VRAM** + 约 8GB 额外磁盘；CUDA 13（驱动≥580） | MIT（权重 CC BY-NC） | 851★，push 2026-10-18，17 open；4 周龄快速迭代；默认匿名统计+自动更新 | Windows 最省事的入口；但项目新、自动更新/统计需关闭，且 11GB 门槛贴红线 |
| **ComfyUI-YuE2-Trainer** | **仅 NAR** LoRA rank32/α32，EMA；AR 冻结；目标函数从推理代码反推 | 音频文件夹 + 可选 caption + trigger word；10s clip → latent 缓存；输出 ComfyUI 原生 LoRA | **24GB 推荐**（5090 laptop 实测）；4090 约 1–3s/step，1000 步 25–50min；可降 clip 6–8s | MIT | 72★，最后 push 2026-09-14（停滞）；作者承认 experimental，**voice cloning 仍不 work** | 仅作 NAR 烟测/对照，不作主路线 |
| **Ruach Studio** | 自研 trainer（bf16、AR/NAR 双曲线） | 自制 latent cache；GTSinger 声音数据 | **rank16 bf16 实测 12GB / rank32+42 首歌 18GB**；Linux 为主 | **AGPL-3.0**（集成传染，禁止并入仓库） | 45★，created 2026-10-05，较新 | 仅参考其 12GB 数据点，不采用 |
| **Mothersuperior tokenizer v4 + AR LoRA 脚本** | 音频→语义 tokenizer + NAR LoRA + AR LoRA 训练脚本；用 4,765 首 YuE2 自生成歌自举 | 每首 flac + 完整歌词 + caption；`prep_real.py`/`cursor_prep.py`/`ar_lora_cursor.py`；minted regularizer 防 token grammar 崩塌 | **24GB GPU**（14–18GB 含 gradient checkpointing）；Python 3.12 + torch 2.10 cu128 | 权重 CC BY-NC；脚本随模型卡 | 研究级但公开、可读；留出集 top-1 16.1%（NAR 回环耳测≈95%） | 研究/复现参考；12GB 跑不动其 Python 栈 |
| **HuggingFace adapter 存量** | 约 28 个 LoRA/adapter（风格/音色/语言；Mothersuperior 4 个、becausereasons 8 个等） | — | 仅推理 | CC BY-NC | 有生态但很浅；**未见中文 LoRA（按名搜索，推断为空白）** | 可作推理时 adapter 玩法的基础；中文方向可能是机会 |

**关键解锁**：在 Mothersuperior 的 real-audio tokenizer v4 出现前，"用自己音乐训 AR"不成立；现在目标是**自举的代理 tokenizer**（非官方），AR 目标有质量上限，训练时要配 minted regularizer，否则 token grammar 会崩。

## 4. 12GB 本地可行性（核心约束）

- 名义 12GB，实际可用约 **10.2GB**（桌面/浏览器占用 1.5–2GB）；关掉可回到 ~11.3GB。Studio 声明 11GB 是"最低门槛"，不是舒适区。
- HOT-Step 训练器本身约 **14GB**（bf16 基座）；60s decoder crop、整曲 decoder ≈12GB；并行预览需 28GB。**量化 GGUF 基座**（Q4_K_M≈2GB、Q8_0≈3.5GB）可显著降 VRAM，官方称与 bf16 首步 loss 差约 1%，但"未做耳测"。
- 12GB 可尝试配置：量化基座 + 1 首/update + `--nar-crop-frames 1500`（60s）+ planner crop + 预览关 + 训练时停引擎/停推理；预计峰值 9–11GB。**无公开 12GB joint 实证**（只有 4070S 12GB NAR-only 22 首≈20min；16GB joint 很紧）。
- 数据规模：**5–30 首同风格/同歌手**（<10 首工具警告），30–45 分钟音频；社区高质量案例 20–23 首。数据准备（4070 推断）约 20–40 分钟：歌词库/Whisper、SheetSage2 乐谱、MMS-FA 对齐、latent/codes 缓存（约 40MiB/专辑）。
- 训练时长（4070 推断，官方 5090 数据外推，误差可能 ±50%）：
  - Legacy 750 步/1 首/60s：5090 20–25min → 4070 **1–1.5h**
  - Balanced 200 updates×4 首：5090≈1h → 4070 **2.5–4h**
  - AI-Toolkit joint 650 步 rank32：5090 46min → 4070 缩 crop 后 **1.5–3h**
  - ComfyUI NAR-only 3000 步 10s clip：4090 1–3s/step → 4070 可跑；4070S 22 首≈20min
- 下载量：HOT-Step 便携 ~252MB（CUDA13.1）+ YuE2 Joint Training Pack 11.1GB + ConvRot 基座包 8.5GB（或已有 GGUF）+ AR 训练包 3.9GB + 推理模型 ≈ **20GB+**；E 盘够，hf-mirror 需约 1h+。
- 质量：**NAR-only 学风格/音色底色，学不到特定歌手**；AR+NAR joint 才带身份；基座中文 PER 8.44%（公开模型里较好），但过训练先坏咬字/结尾（garble、runaway outro）→ 必须人耳 + ASR-PER 硬闸门。
- 推理侧兼容：yue2.cpp 的 `--adapters` 支持 **LoRA（B@A）**、diff、全替换，兼容 AI Toolkit/ComfyUI/native YuE2 格式；**LoKr（w1/w2）未列支持**。因此若想训练产物直接被 yue-server 热插拔，应训 **LoRA**（HOT-Approved rank64 或 ComfyUI NAR LoRA），或在 Studio/HOT-Step 内做推理；**这是 spike 的必验项**。

## 5. 后训练可行性（DPO / RLHF / 继续预训练 / 蒸馏）

| 方法 | 先例 | 门槛 | 12GB 单卡 | 结论 |
|---|---|---|---|---|
| 继续预训练 / 全参 SFT | 官方预训练为数十卡 H800 级、数十万小时数据；训练代码未发布 | 64×H800 级【报告口径】 | 不可行 | 不做 |
| DPO / 多偏好优化 | LeVo（~2B LM，20k 提示/60k 偏好对，DPO+DNI，NeurIPS'25）、MR-FlowDPO、Tango2、MusicRL 等 | 策略+参考双份 bf16 ≈ 2×7.2GB，仅权重就 >12GB；4bit 需自研 AR/NAR+flow DPO | **不可行** | 研究级；云 8×A100-80G×3d ≈ ¥3.4k 量级，且需 2–4 周自研管线，不值 |
| RLHF / GRPO | MusicRL 等 | 采样 + 奖励模型 + 多卡；4070 每首 2–3min | 不可行 | 不做 |
| 蒸馏 / QAT | 无合适教师；int8 ConvRot 已是量化基座训练（QLoRA 式） | — | 不适用 | 不做 |
| 奖励/评估模型（只评估） | AudioBox Aesthetics、LAION music-aesthetics、CLAP/MERT/MuQ-MuLan、SongEval、Qwen3-Omni | 12GB 可跑 | **可行** | 值得做（评估栈） |

**判断**：在 12GB 单卡上做"有意义的后训练"=不可行；即使租云卡，投入产出比也差。**写成未来工作**比立项更诚实。

## 6. 评估栈（如果做 LoRA，必须有）

- 对外：WildSongBench（SongBench Avg / PER / Q3O / MuLan / AllMusicCaps / AudioBox 生产质量）。
- 本地轻量：ASR-PER（中文咬字）、AudioBox Aesthetics、CLAP/MuLan 相似度、ABC 结构 lint（段落/小节/跑飞结尾）、盲听 1–5 分（至少 2–3 位听者）。
- 社区明说：自动指标测不出 garble，人耳 + lint 必须；训练后必须验证**非触发 prompt 不退化**（基础能力保持）。
- 与 Vit-DAW 的既有资产可对接：audition A/B、资料库、工程快照；但本轮不新增用户手测义务。

## 7. 对 Vit-DAW 的机会分层（建议的"余地"）

| 层级 | 内容 | 成本 | 可行性 | 建议 |
|---|---|---|---|---|
| **L0 推理时适配（零训练）** | ABC 乐谱编辑/重渲染、style prompt、replay 精确复现、叠加 HF adapter（LoRA） | ≈0 | **高** | **优先**：与 DAW 白盒控制最契合，且不触训练/许可风险 |
| **L1 风格/音色 LoRA（本地）** | HOT-Step/Studio Joint Training；8–12 首同风格；产出 LoRA/LoKr pair，推理时热插拔 | 2–3 天 spike + ~20GB 下载 + 1–4h 训练 | **中**（10.2GB 贴线） | 若要有"个性化/中文风格"demo，做 spike；否则缓 |
| **L2 偏好/后训练（DPO/RL）** | 用 DAW A/B 判断做偏好数据、奖励模型、DPO | 数周 + 云卡 | **低** | 不立项，写未来工作 |
| **L3 继续预训练/全量微调** | 复现官方训练 | 数十卡 | **不可行** | 不做 |
| **差异化机会** | 中文 LoRA（HF 未见）、"乐谱可控 + 个性化 adapter + DAW 集成"的组合、数据/评估管线 | — | 中 | 可作为论文/演示的"未来工作 + 小实验"素材，不作为答辩主主张 |

## 8. 风险与止损

- **许可**：衍生权重 CC BY-NC；公开 adapter 需署名；商用需授权；训练数据版权自负。不要把权重/adapter 入仓或交付。
- **显存**：10.2GB < 11GB 声明；无 12GB joint 实证；OOM/换页/长时间训练是主要失败模式。先做 NAR-only 20 分钟烟测，再谈 joint。
- **工具成熟度**：HOT-Step 4 周龄、joint 新；ComfyUI trainer 停滞；Ruach AGPL；Studio 自动更新/统计/快速迭代。固定版本 + 隔离目录。
- **质量方差**：best checkpoint 常在中途（300–650 步），过训练坏咬字/结构；没有自动指标，必须盲听。
- **时间线**：答辩/论文当前有三轮改稿与主线开发；2–3 天 spike 可以，2–3 周后训练不可以。
- **止损线**：D2 结束仍无法在 10.2–11.3GB 内完成一次完整 run（试 3 组配置）→ 停本地训练，最多租 4090（AutoDL ¥1.9–2.7/h，4h≈¥10）验证；D3 质量不达标（盲听 <65% 偏好或中文 PER 退化 >2pp）→ 结论"本地只保留数据/评估链路，不训练"。

## 9. 证据链接

- 官方仓库/许可：https://github.com/multimodal-art-projection/YuE ；MODEL_LICENSE；issue #165（semantic tokenizer）、#81（v1 训练代码承诺）、#141（v1 finetune 讨论）
- v1 微调代码：https://github.com/multimodal-art-projection/YuE/tree/YuE-v1/finetune
- 技术报告：https://arxiv.org/abs/2609.33757
- HOT-Step-CPP：https://github.com/scragnog/HOT-Step-CPP （训练文档 `docs/user/training/yue2.md`、`docs/user/studios/training-studio.md`、`docs/dev/training-internals.md`；release v1.4.1.1 Windows CUDA13.1 252MB）
- YuE2-Studio：https://github.com/timoncool/YuE2-Studio （训练声明 11GB/8GB）
- ComfyUI-YuE2-Trainer：https://github.com/Starnodes2024/ComfyUI-YuE2-Trainer （24GB 推荐、NAR-only）
- Ruach Studio：https://github.com/igrbible/Ruach_Studio （AGPL，12GB rank16 实测）
- Mothersuperior tokenizer/AR LoRA：https://huggingface.co/Mothersuperior/yue2-mothersuperior-realaudio-tokenizer-v4 ；https://huggingface.co/Mothersuperior/YuE2-instrumental-cot-full-loras
- 评估：https://huggingface.co/datasets/m-a-p/WildSongBench ；AudioBox Aesthetics；CLAP/MERT
- 社区 12GB 数据点：ComfyUI-YuE2-Trainer issue #1（4070S 12GB NAR-only 22 首≈20min）、Reddit 16GB joint 讨论

## 10. 补充：Mac mini 32GB（Apple Silicon）能不能部署/微调（2026-10-18 追加）

**结论：部署（推理）可以；微调（训练）不实际，只存在实验路径。**

### 10.1 部署：可以，32GB 内存绰绰有余

| 路径 | 形态 | 适配性 | 已知数据/问题 |
|---|---|---|---|
| **mlx-Yue**（vanch007/mlx-Yue） | 原生 MLX + CLI/Python API；权重 HF 预转换（AR 8bit 2.66GB / BF16 4.33GB、NAR BF16 2.93GB、48k VAE） | **Mac 首选**（M1–M5；8bit 峰值约 10.2–11GiB，32GB 无压力） | M3 Max：AR 8bit 83.1 tok/s；NAR 32 步 170s 音频=187s；8 步模式宣称 <1.0 RTF【官方自述】；质量需自听 8 步折损 |
| **HOT-Step-CPP macOS-arm64** | 预编译便携包 103MB（Metal），GUI+推理+训练一体 | 可；未签名需 `xattr -cr` 去隔离 | 无公开 Metal 基准；YuE2 推理包 4.6GB（Q8_0）/7.9GB（BF16） |
| **yue2.cpp（Metal）** | 需自建：Xcode CLT + CMake（`cmake ..` 自动 Metal；`--device MTL0`） | 功能最全（server/adapter/转谱/tokenize），**与我们 PC 的 HTTP 集成方案同构** | 无官方 Metal 性能基准；CUDA0/CPU parity 全绿；Mac「可编译可跑」【推断】 |
| **Lux In Music**（NTTLuke/lvxworld） | Mac 原生 App（audio.cpp/Metal，Q8_0+F16 VAE，浏览器 UI，带分轨/局部重生） | 可；M4 Max 实测 30s 音频 warm 15s（cot=full）≈2× 实时；安装脚本自动备 Python/uv/CMake | 原型项目；无登录不可公网暴露；无 genapi 接口 |
| **YU E2-Studio dmg** | dmg 只有 UI/服务，**引擎需自建 Metal** 并设 `YUE_ENGINE_ROOT` | 可用但繁琐；训练 NVIDIA-only | README 明说发布机无 Mac |
| 官方 PyTorch MPS | 需打补丁（issue #176 BF16 因果 prefill 静默损坏） | 不推荐 | M5 Max 补丁后 79.4s 音频端到端 367s（≈4.6× 实时） |
| ComfyUI FL-YuE2 | 要求 NVIDIA BF16 | 无 Apple Silicon 实证 | — |

**速度量级（3–4 分钟歌，推断）**：M4 Pro 32GB 用 mlx-Yue 8 步约 3.5–4.5min、32 步约 8–10min；**M4 基础版（120GB/s，约 M4 Pro 一半带宽）大致翻倍**；PC 4070（CUDA yue2.cpp）约 1.5–2.5min。32GB 统一内存不构成瓶颈：Q8_0 峰值 5.8–7GB、mlx-Yue 进程 10.2–11GiB，Apple GPU 默认可用约 75% 内存（~24GB），无需调 wired limit。

**集成含义**：Mac mini 可以直接跑 `yue2-server`（Metal）出 HTTP job API，Mac 侧 `genapi async_poll` 指向 `127.0.0.1:8087` 即可，**不需要跨机/防火墙**；但速度低于 PC 4070，PC 仍是更快的生成与训练主机。

### 10.2 微调：不实际，只有实验路径

| 方案 | Apple Silicon | 事实 |
|---|---|---|
| HOT-Step YuE2 Joint Training（Metal） | **仅实验可跑** | 默认 bf16 在 Metal 首个 backward 报 `unsupported op OUT_PROD`（issue #240 open，2026-10-17 仍在）；`YUE2_AITK_PRECISION=strict-f32` 可绕过，但 M1 Max 实测 **275.2s/step**（bf16 更慢 333.7s）；另有 #218（BF16_ROUND→f32 被拒）、#198（NAR 完成后判失败/checkpoint 不保留） |
| 训练基座 | 受限 | ConvRot int8 **CUDA-only**；Mac 只能 bf16 GGUF 基座（Q4_K_M≈2GB 可省内存，但非调参基座） |
| YuE2-Studio 训练 | 不可用 | 需 NVIDIA RTX30+、11GB，「trainer 无其他路径」 |
| ComfyUI-YuE2-Trainer | 不可用 | NVIDIA、建议 24GB、仅 NAR |
| Mothersuperior / Ruach | 不可用 | CUDA 24GB / Linux+NVIDIA；Ruach 为 AGPL |
| 耗时推算 | 不具迭代价值 | Balanced 200 updates：M1 Max 约 15h；M4 Pro 约 17–25h/专辑；Fast 100 updates 仍 8–12h |

**跨端方案成立**：训练放云 4090（HOT-Step 4090 约 1min/update×4 歌，Balanced≈1–1.5h）或 PC 4070（贴红线），adapter 拷回 Mac；`yue2.cpp` 支持 ComfyUI/AI-Toolkit/native LoRA 加载，HOT-Step 训练产物同应用跨平台可读；mlx-Yue 的 LoRA 格式未声明，需先小样验证。

### 10.3 建议

1. **只买/只用 Mac mini 做推理**：选 **M4 Pro 32GB**（若为 M4 基础版 32GB，速度约减半）；装 mlx-Yue 最快，或 HOT-Step macOS 包（要 GUI/一体化）；要接 genapi 就自建 yue2.cpp Metal server（半小时级构建）。
2. **训练不在 Mac mini 上做**：租 4090 训练 → adapter 回 Mac；本地 PC 4070 仅小步试验。
3. **采购判断**：如果 Mac mini 只是为 YuE2，PC 4070 已够且更快/更成熟；Mac mini 的价值在 macOS/DAW 前端一体、低功耗常开、32GB 内存可跑 BF16 全套；训练仍离不开 NVIDIA。
4. **更新既有前提**：2026-10-17「本地 3B 音频模型 MacBook Air 跑不动、自部署不作主路」的结论对 **32GB Mac mini 部署**不再成立（推理可本地跑），但**训练**仍不成立；Mac 侧生成开发可改为"本机推理 + 外借训练"。
