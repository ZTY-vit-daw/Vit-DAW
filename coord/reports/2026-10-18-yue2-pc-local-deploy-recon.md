# YuE2 模型 PC 侧本地部署勘察 + YuE2 Studio 判定（2026-10-18，决策侧调查）

- 依据：用户 2026-10-18 指示——Mac 侧生成开发，发现 YuE2 可在 PC 侧本地部署做开发实验；调查合适部署方式；评估开源项目 YuE2 Studio 是否需要。
- 性质：**只读调查**。本文件不执行安装、不下载模型、不改仓库代码。所有体积/可达性均为本机实测（2026-10-18）；社区结论标注来源，未经本地验证的标【待实测】。
- 机器事实（本机 `D:\Vit_DAW`，2026-10-18 12:08 实测）：Windows 11 家庭版 26200；i5-13600KF（14C/20T）；32GB RAM；**RTX 4070 12GB（当前空闲 10.2GB）**；驱动 617.42 / CUDA UMD 13.4；Python 3.13.12（torch 2.12.1 **CPU 版**）；git 2.53 / ffmpeg 9.0.1；**无 WSL / 无 Docker / 无 uv / 无 conda**；VS Community 2026 已装（VC Tools 14.44/14.50）；非管理员会话；磁盘 C 63.8GB / D 159.4GB / **E 362.6GB** 空闲；局域网 `192.168.1.181/24`，网卡网络类别为 **Public（防火墙入站默认拦截）**。

## 0. 结论（先看）

1. **推荐部署方式：PC 侧原生 Windows + `yue2.cpp` 的 GGUF 引擎 `yue-server`（不是官方 PyTorch 栈、不是 WSL2）。** 理由：只有 GGUF 量化路线适配 12GB 显卡；官方 PyTorch 路线要求 Linux + 24GB 显存，且 Windows 原生有未合入的 FlashAttention 崩溃/慢 6 倍补丁。`yue-server` 自带异步作业 HTTP API（`POST /synth` → job id → 轮询 → multipart 音频），正好可被 Mac 侧现有 `genapi` 的 `async_poll` 方言承接。
2. **YuE2 Studio：不需要（不是必需）。** 它是同一套 `yue2.cpp` 引擎的桌面 GUI 壳（timoncool 版，850★，MIT，6GB 显存起）；生成能力全在引擎层，其对外集成面是 MCP JSON-RPC 而非我们要的 HTTP 生成接口，且带自动更新/默认匿名统计/启动联网，4 周龄快速迭代——**不作为开发依赖**。仅两点可选价值：①人工试听评估台；②其 Windows 便携包内含预构建的 `yue-server.exe`，可作引擎二进制的下载来源（同 MIT，可核对版本）。**人工试听本身用 `yue-server` 自带 WebUI（浏览器打开 `localhost:8087`）即可，无需 Studio。**
3. **对 Mac 侧的意义：** 这是当前无任何外部生成凭据条件下，给 Mac 侧生成开发提供**真实自托管生成源**的最快路径（genapi 配置即可，Mac 侧零代码改动）；可作为 REALVERIFY 类的自托管实验腿。**它不改变 2026-10-17「Suno 网页为主路线」的裁定**，也不是产品依赖（权重 CC BY-NC 4.0，商用须授权）。
4. **成本/工期估算：** 下载约 5.1GB（模型 4.34GB + 引擎包 0.35GB + cuBLAS 0.39GB），网络顺利约 40 分钟（hf-mirror 实测 ~5MB/s，GitHub Release 需走镜像）；部署+冒烟 0.5–1 人日；HTTP 服务壳 + Mac 端到端 0.5–1 人日。首版参数按 **≤3 分钟、Q8_0、cfg 1.0** 起步，实测耗时/峰值显存后再放开。
5. **待用户决策：** 是否现在派人按本方案落 PC 部署冒烟；是否同时立「genapi 自托管源对接（Mac 端到端）」卡。建议卡面见 §9。

## 1. 目标与约束

- 用户目标：Mac 侧做生成开发，PC 侧本地跑 YuE2 供开发实验（不是上线产品、不是答辩主张）。
- 硬件约束：RTX 4070 **12GB**、32GB RAM；桌面常驻约 1.5–2GB 显存，可用约 10.2–10.5GB。
- 网络约束（本机实测）：**huggingface.co 直连超时不可用**；`hf-mirror.com` 200/1.5s，8MB 限速实测约 **5MB/s**；github.com 可达但慢；**GitHub Release 直连约 0.09MB/s（8MB/90s）**，`ghfast.top` 镜像约 0.57MB/s、`gh-proxy.com` 约 0.36MB/s；pypi.org 快（0.3s）；ModelScope 200；NVIDIA `developer.download.nvidia.com` 可达（cuBLAS 13 redist zip 391MB HEAD 200）。
- 现有系统资产：`agent/internal/genapi` 已有 `sync_binary` / `async_poll` 等方言和 `~/.vit/genapi_providers.json` 配置面；`GENAPI-REALVERIFY-1`（125）因「无生成源/key」blocked。本地自托管源正好补这个缺口（作为实验源，不是 Suno 主路）。

## 2. 模型事实（官方/社区分层）

【官方确认】
- YuE2 与 YuE v1 同仓库（`multimodal-art-projection/YuE`，main 已是 YuE2，v1 在 `YuE-v1` 分支）。推理包 **0.1.6**（2026-09-09 发布），论文 arXiv:2609.33757（2026-09-27），发布仅约 6 周，仍在快速迭代。
- 单一 **YuE2-3B**（3,630,684,224 参数，28 层 AR-NAR Mixture-of-Transformers）+ 独立 VAE 解码器（`YuE2-Vae`，默认听感；`YuE2-Vae-legacy` 用于榜单复现），48kHz 立体声，25Hz 语义流，默认 32 步 flow matching。
- 官方 Quick start 原文：**Linux · Python 3.12 · 支持 BF16 的 NVIDIA GPU · 24GB 显存**，一次一首；官方 Speed 表（4090 24GB）：3.6 分钟歌生成 71s，峰值 **11.18GiB**；最大上下文测试峰值 **14.08GiB**；要求 24GB 主机内存。
- 官方只有 Python API（`plan()→generate_semantic()→synthesize()→decode()`）+ `yue2` CLI；**无官方 HTTP API、无官方 Gradio**。
- 官方明确 Windows 不在支持列表；官方 PyTorch Windows 轮子无 FlashAttention 内核，CUDA graph 直接报 `USE_FLASH_ATTENTION was not enabled for build`，修复 PR #166/#222 仍 open；NAR 的 `enable_gqa` 在 Windows 回退 MATH，社区实测慢约 6×、多占约 7GB（issue #209）。
- 许可：**代码 Apache 2.0；模型权重 CC BY-NC 4.0 + 个人创作者额外许可**（2026-09-16 版）：个人创作者可免费使用并变现生成物；学术研究/教育非商用免费；**公司商用权重须单独授权**（lauryliuyang@hkgai.org），禁止转售权重。生成物不因「由 YuE2 生成」而受 NC 限制。
- 榜单：WildSongBench 192 prompts，YuE2 6.7316 / best-of-8 6.9632 SongBench Avg，对标 Suno v5/v6（Suno v5 6.8721、v6 6.5562）；**这是自动指标，不等于人耳听感**。

【社区/第三方确认】
- 量化/GGUF 全部来自社区，官方无 GGUF：`Serveurperso/YuE2-GGUF`（yue2.cpp 官方配套）：BF16 7.17GB / Q8_0 **3.81GB** / Q6_K 2.94GB / Q5_K_M 2.62GB；VAE F32 530MB。
- `yue2.cpp`（MIT，独立 C++/GGML 实现，跑 CPU/CUDA/Vulkan/Metal）：24 项 parity 测试 CUDA0/CPU 全绿（rel RMS ~1e-3 量级），官方称 6GB 显存起；内置 `yue-server`（HTTP + WebUI）、`yue-synth`/`yue-plan`/`yue-transcribe`/`yue-tokenize` CLI；结果 multipart 里除音频还带 **replay request JSON（含语义码/乐谱/seed）**，可精确复现/重渲染——对 A/B 与回归很有价值。
- 12GB 实测参考：Pinokio 指南——Linux/Docker 12GB 可跑约 5 分钟歌（budget=12、CFG=1.0）；Windows 12GB「borderline」，建议 ≤3 分钟、max_tokens≈4500；8–11GB 明确"No"。
- 速度参考：YuE2-Studio 文档 4090 + Q8_0 3:38 歌约 **46s**；yue2.cpp 性能表 RTX PRO 6000 Blackwell native BF16 212s 歌端到端 **44s**（含模型加载）；4070 无公开实测，按算力粗估 3–4 分钟歌 **1.5–3 分钟**【待实测】。
- YuE2-Studio（timoncool）：v3.6.0（2026-10-16），850★/105 fork，MIT，Windows 原生 Tauri，自带 `yue-server.exe` + CUDA 13/12/Vulkan/CPU 后端，不写系统；12GB+ 预选 BF16 套装（9.9GB 下载），8GB+ Q8_0（5.1GB），6GB+ Q5_K_M（4.0GB）。MCP 在 `127.0.0.1:8791/mcp`，文档写明默认只允许本机 agent/窗口连接。默认勾选匿名统计（`DO_NOT_TRACK=1`/`STUDIO_TELEMETRY=0` 可关），启动即拉新闻、每小时一次；自动更新。macOS dmg 为 adhoc 签名（issue #105），Windows 安装器未见 Authenticode【推断】。
- 同名项目：`krakenunbound/yue2-studio`（34★，包官方 Python 推理，推荐 24GB 3090；**代码无授权**，LAN 6969 无密码）；`vrgamegirl19/Yue2_Studio`（80★，Apache-2.0，会改写上游安装）；`Ruach_Studio`（**AGPL-3.0**，勿并入仓库）；`joeljuvel/YuE-UI` 仅支持 v1 且停更。**用户看到的「YuE2 Studio」大概率是 timoncool 版**；无论哪个都不是我们的必需项。

## 3. 路线对比（12GB / Windows / 需要 HTTP 给 Mac 用）

| 路线 | 12GB 可行性 | 工作量 | 集成面 | 结论 |
|---|---|---|---|---|
| **A. yue2.cpp `yue-server`（GGUF，原生 Windows）** | **可行**：Q8_0 权重 AR 2195MB/NAR 1434MB（逐段换入）+ VAE 127MB + KV 2688MB/set（全上下文）；65s 歌实测峰值 5.8GB，`--max-seq 8192` 时 3.8GB | 0.5–1d | **HTTP job API + WebUI + CLI** | **推荐** |
| B. 官方 PyTorch（WSL2/Docker） | 勉强：官方 4090 峰值 11.18GiB，12GB 无余量；WSL 额外开销；Windows 原生有未合入补丁 | 0.5–1d + 需装 WSL/重启/管理员 | Python API，自建 HTTP | 仅在必须官方全精度复现时考虑 |
| C. YuE2-Studio（GUI） | 可行（同 A 引擎，12GB 预选 BF16） | 装完即用 | MCP，非生成 HTTP | 可选试听台，不作依赖 |
| D. T8 / Yeufonic 等 Windows 整合包 | 宣称 12GB 可跑【未验证】 | 快 | 部分自带 `/api/jobs` 同形接口 | 黑盒、第三方许可/更新风险，备选 |
| E. YuE2-Turbo（NOIZ，vLLM 服务） | **不可**：常驻约 18GB、仅 Linux | — | 完整 job API | 不适用 4070 |

## 4. 推荐方案（架构）

```
Mac 侧（生成开发）                        PC 侧（192.168.1.181，Windows 11 + RTX 4070 12GB）
┌─────────────────────────┐   LAN HTTP   ┌──────────────────────────────────────────────┐
│ agent genapi             │  async_poll  │ yue2-shim（FastAPI/Go，唯一对外面，带 token） │
│ provider: yue2_local     │ ───────────► │   POST /generate → {task_id}                 │
│ 工具 gen.generate 等      │ ◄─────────── │   GET /generate/{id} → 202 / 200 audio/wav    │
│ 产物走既有 ingest 链       │              │        │ 本机 127.0.0.1                    │
└─────────────────────────┘              │        ▼                                     │
                                          │ yue-server.exe（yue2.cpp，GGUF，无鉴权）       │
                                          │  POST /synth → id；GET /job?id=N&result=1     │
                                          │  模型：YuE2-3B-Q8_0.gguf + YuE2-Vae-F32.gguf  │
                                          │  输出：48kHz stereo MP3/WAV + replay JSON     │
                                          └──────────────────────────────────────────────┘
```

关键决策：
- **引擎选 `yue-server`（路径 A）**：绕开官方 Windows PyTorch 的 FlashAttention/GQA 坑；12GB 显存余量充足；MIT 代码；自带 WebUI 供人工试听；replay JSON 支持确定性重渲染。
- **对外加一层薄 shim，而不是直接暴露 `yue-server`**：`yue-server` 默认 `--host 0.0.0.0` 且**无鉴权**；shim 负责 token 鉴权、串行队列、把 `POST /synth`+轮询/multipart 映射成 genapi `async_poll` 契约、保存 replay JSON、限制访问来源。首版 shim 可用 FastAPI（约 150 行）或 Go；后续稳定后再考虑在 `agent/internal/genapi` 内新增 `yue2_server` 方言（零 shim）。
- **Mac 侧零代码**：`~/.vit/genapi_providers.json` 加一个 `async_poll` provider，`base_url=http://192.168.1.181:8000`，配 token 与 `prompt/style_tags/lyrics` 字段映射；`timeout_seconds≈1800`、`per_call_timeout_seconds≈1200`（生成一首可能 1–3 分钟，默认 60/600s 不够）。
- **网络**：PC 当前以太网为 Public 类别，需一次性（管理员）改 Private 或建入站规则并把远程地址限定为 Mac IP；PC 交流电待机已为「从不」，无需改。若 Mac 不在同一局域网，改用 Tailscale/ZeroTier 或仅 PC 本地跑。
- **安全边界**：`yue-server` 只绑 `127.0.0.1:8087`；shim 绑 `0.0.0.0:8000` 但仅防火墙放行 Mac IP + Bearer token；不开公网、不做多租户。

## 5. 部署步骤（建议写成执行卡）

**S0 预检（5 分钟）**
1. `nvidia-smi` 确认 12GB、驱动 617.42、空闲 ≥10GB；关掉不必要的浏览器/游戏客户端。
2. 确认 `E:\AI\YuE2\` 目录可写（E 盘 362GB 空闲）；后续所有模型/HF 缓存/系统临时目录都指向 E 盘，不落 C 盘。
3. 记录当前 `git rev-parse HEAD`、`git status --short`（部署动作不碰仓库源码）。

**S1 取引擎（约 0.4GB 下载）**
- 优先：下载 `YuE2-Studio-3.6.0-portable-windows-x64.zip`（354MB，GitHub Release 直连很慢——实测 0.09MB/s；用 `ghfast.top` 镜像约 0.57MB/s，或让 Studio 自己下载后取用），解出 `resources\yue2-cpp\` 下的 `yue-server.exe` + `ggml*.dll` + CUDA/Vulkan/CPU 后端；记录 zip SHA-256 与来源 URL。**只取引擎二进制，不运行/不依赖 Studio 应用。**
- 备选：用 VS 2026 + CUDA Toolkit 源码构建 `yue2.cpp`（`buildcuda.cmd`，`-DGGML_CUDA=ON`），可复现但多 ~3GB CUDA 工具链下载。
- cuBLAS：CUDA 13 后端需要 `cublas64_13.dll`/`cublasLt64_13.dll`（Studio 首次启动自动从 NVIDIA 官方 redist 下载 391MB zip）；手动从 `developer.download.nvidia.com` 同 URL 取 `bin\` 两文件放到 `yue-server.exe` 旁。若 cuBLAS 获取受阻，可先用包内 Vulkan 后端跑通（速度略低）再切 CUDA。

**S2 取模型（约 4.34GB，hf-mirror 实测 ~5MB/s）**
```
https://hf-mirror.com/Serveurperso/YuE2-GGUF/resolve/main/YuE2-3B-Q8_0.gguf   # 3,810,232,064 B
https://hf-mirror.com/Serveurperso/YuE2-GGUF/resolve/main/YuE2-Vae-F32.gguf   # 530,497,344 B
```
- Q8_0 为近无损首选；可选再下 BF16（7.17GB）做质量对比（12GB 卡可跑但更吃紧，见 §2）。
- 翻唱/可编辑乐谱若需要，再补 `SheetSage2-Q8_0.gguf`（106MB）+ `MERT-v2-FullSong-Q8_0.gguf`（902MB）+ 音频 tokenizer（47MB），**首版不用**。
- 校验 SHA-256 并固定 revision（Studio 侧 pin 的是 `Serveurperso/YuE2-GGUF` rev `e630f2b`）。

**S3 起服务 + 冒烟（PC 本机）**
```
yue-server.exe --model E:\AI\YuE2\models\YuE2-3B-Q8_0.gguf ^
  --vae E:\AI\YuE2\models\YuE2-Vae-F32.gguf --device CUDA0 ^
  --host 127.0.0.1 --port 8087 --max-seq 8192
```
- `GET /health`、`GET /props` 通过；
- 用最小请求（约 60–90s，`cot="full"`、`duration=90`、`cfg_scale=1.0`、固定 seed）`POST /synth` → 轮询 `/job?id=N` → `/job?id=N&result=1` 取 multipart；
- `ffprobe` 断言 48kHz/2ch/时长；检查 replay JSON 的 seed/语义码；`nvidia-smi -l 1` 记录峰值显存；
- 连续 3 次成功，再试 180s 歌记录耗时/显存；5–6 分钟长歌留作容量测试（可能需要 `--max-seq`/分段策略）。

**S4 服务壳 + Mac 端到端**
- shim 实现 `async_poll` 契约（`POST /generate` 202 → `{"task_id"}`；`GET /generate/{id}` 202 或 200 `audio/wav`；内部串行队列 + token 鉴权 + replay JSON 落盘）；
- Windows 防火墙放行（管理员，一次）：`New-NetFirewallRule -DisplayName "YuE2 API" -Direction Inbound -Protocol TCP -LocalPort 8000 -Action Allow -Profile Private -RemoteAddress <Mac-IP>`；网卡类别改 Private（或在 Public 上另放行并接受风险）；
- Mac 配置 genapi provider，跑 `gen.generate` → 产物进 ingest → clip/take；断言音频魔数、48kHz、时长、可寻址；
- 3 次连续真源端到端 + 1 次取消/超时/失败分类（fail-closed）探针。

**S5 固化**
- 一键脚本（`scripts/` 下 `start_yue2_pc.ps1`/`stop`）写日志与 PID，端口固定；模型/引擎/输出目录在 `E:\AI\YuE2\`，不进 git；
- 记录 run 目录、耗时、峰值显存、SHA-256、provider 配置（不含 token 明文）；
- 如果之后要长期用：再评估把 shim 逻辑并入 `agent/internal/genapi` 新方言（当前 shim 足够，不动内核/零仓库改动）。

## 6. 验收标准（建议）

| 项 | 门槛 |
|---|---|
| S0 | `/health`、`/props` 200；引擎日志无 CUDA/cuBLAS 错误 |
| S1 冒烟 | 90s 歌成功；`ffprobe` 48kHz stereo；replay JSON 完整；峰值显存 <10GB；连续 3 次成功 |
| 性能 | 180s 歌 ≤5 分钟（首版目标；实测记录后修门槛）；3 次耗时波动可解释 |
| 长歌 | 360s 歌成功且 `truncated=false`（以 replay/结果标志为准）；如 OOM 则降 `--max-seq`/缩短时长并记录边界 |
| Mac 端到端 | Mac agent 触发生成 → PC 返回音频 → ingest 成 clip/take；3 次连续；403/超时/取消分类明确 |
| 稳定性 | 显存不随重复请求增长（每次请求后回到基线）；服务重启后模型可再加载 |
| 边界声明 | 权重非商用许可、第三方引擎、Windows 未官方支持、人耳质量未评估——如实入回执 |

## 7. YuE2 Studio 判定（回答用户的问题）

**不需要。** 分三层说：
1. **能力层**：它只是 `yue2.cpp` 引擎的 GUI 壳（另有分轨/MIDI/视频/LoRA 等后处理，均非本阶段所需）；我们要的生成能力与 HTTP 接口在引擎里，不在 Studio 里。
2. **集成层**：它的对外接口是 **MCP JSON-RPC（127.0.0.1:8791）**，且默认只允许本机连接；我们的 genapi 要的是 HTTP 生成契约。以 Studio 为后端 = 把 Mac 开发绑到一个 GUI 应用的生命周期/自动更新/版本节奏上，是负债不是能力。
3. **风险层**：4 周龄项目、默认匿名统计、启动联网、自动更新、安装器未签名；作为静默依赖不合适。另两个同名项目风险更高（无代码授权/AGPL/改写上游安装）。

**可选的两种轻量用法**（都不进流水线）：
- 人工试听：`yue-server` 自带 WebUI 即可（浏览器开 `localhost:8087`），110+ 示例可从 Studio 仓库取，不必装 Studio；
- 若确实想要「带分轨/MIDI/视频的试听工作台」：用 **portable zip**（不安装、不写注册表），固定 v3.6.0、设 `DO_NOT_TRACK=1`、关闭自动更新，仅人工使用、不作为依赖、不纳入项目交付物。

## 8. 风险与边界

- **许可（硬）**：权重 CC BY-NC 4.0 + 个人创作者许可。内部非商用研发/毕业设计可用；**产品化/公司商用须先取得 M-A-P 授权**；不得把权重打进仓库/交付物。生成物个人可变现，但「公司使用权重」受限。
- **引擎来源**：yue2.cpp 为社区独立实现（MIT，parity 测试全绿），非官方 PyTorch；音质与官方是否一致需我们自己人耳 A/B（建议 3–5 首对照官方样例）。预构建二进制来自 Studio 发行包，需记录版本/哈希；也可源码自建消除来源疑虑。
- **12GB 边界**：短歌（≤3 分钟）Q8_0 余量充足；5–6 分钟长歌可能接近上限，需要 `--max-seq`/时长策略，**不要承诺任意长度**。
- **Windows 非官方支持**：官方 issue/PR 的 Windows 修复与 12GB 结论都在变；我们走 GGUF 路径正是为了绕开这些坑，但仍需自己的冒烟证据。
- **上游变动**：YuE2 发布仅 6 周，0.1.6 → 后续可能改口径；固定引擎版本 + 固定模型 revision，不跟最新。
- **网络**：HF 直连不可用，必须 hf-mirror/ModelScope；GitHub Release 必须走镜像；首次下载失败要能断点续传。
- **并发**：单 worker 串行，一次一首；shim 需队列/繁忙返回，避免 Mac 侧并发打爆。
- **法务/合规之外的边界**：不做商用承诺、不把 YuE2 写进论文主张；它只是开发实验源。

## 9. 建议卡面（待用户确认后立卡）

- **YUE2-PC-DEPLOY-1**（PC 执行侧）：按 §5 S0–S3 落 `E:\AI\YuE2\`，交付引擎/模型哈希、启动脚本、3 次冒烟 run（`coord/runs/YUE2-PC-DEPLOY-1/`）、耗时/峰值显存/48kHz 断言；写回执注明第三方引擎与 NC 许可边界。验收负责人：主管决策流。
- **GENAPI-YUE2-LOCAL-1**（依赖上卡）：shim + genapi `async_poll` provider + Mac 端到端 3 次 + 失败分类探针；不动内核，产物走既有 ingest。验收含真栈端到端与回执。
- **（可选）YUE2-QUALITY-EVAL-1**：3–5 首典型 prompt 与 Suno/官方样例 A/B 人耳评估，记录优缺点；只作选型参考，不阻塞开发线。
- **（可选）YUE2-GENAPI-DIALECT-1**：稳定后把 shim 逻辑并入 `agent/internal/genapi` 新方言，Mac 侧零外部进程。

## 10. 调查证据（本机实测 + 关键来源）

- 本机硬件/网络/工具链：本次会话 2026-10-18 实测输出（`nvidia-smi`、`Get-CimInstance`、`curl` HEAD/限速、`pip index`、`vswhere`、`powercfg`）。
- 模型体积/端点/参数：`yue2.cpp` README 与 `docs/ARCHITECTURE.md`（`POST /synth`、`GET /job`、Yue2Request 字段、VRAM 表、Q8_0 3.81GB、KV 2688MB/set、65s 峰值 5.8/3.8GB）https://github.com/ServeurpersoCom/yue2.cpp
- 官方事实/许可/速度表：https://github.com/multimodal-art-projection/YuE ；https://huggingface.co/m-a-p/YuE2-3B ；https://github.com/multimodal-art-projection/YuE/blob/main/MODEL_LICENSE
- 官方 Windows 坑：PR #166 / PR #222 / issue #209（open）；官方 Quick start 仅 Linux。
- Studio：https://github.com/timoncool/YuE2-Studio（v3.6.0、MIT、6GB、MCP、预构建引擎）；同名项目见 §2。
- 12GB 社区实测：Pinokio YuE2 指南 https://beta.pinokio.co/posts/01m2z43b44aeba7dcwme5d8603 （Linux 12GB 约 5 分钟歌；Windows borderline）。
- 本机下载可达性：hf-mirror 200/8MB/1.6s；GitHub release 直连 8MB/90s；ghfast.top 4MB/7s；NVIDIA redist 391MB HEAD 200；ModelScope 200。
