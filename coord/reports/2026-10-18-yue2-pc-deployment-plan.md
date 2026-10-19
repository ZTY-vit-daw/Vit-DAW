# YuE2 PC 本地部署方案（2026-10-18，可执行版）

- 依据：用户 2026-10-18 指示——先复查是否有更适合本地部署的生成模型；若没有则选 YuE2，作为生成能力开发的实验基础。本文件=替代模型复查结论 + 可执行部署方案。
- 性质：**部署方案（决策侧产出）**；不在本文件内执行安装/下载/训练。执行回执须另立 run 目录。
- 承：[2026-10-18 YuE2 PC 部署勘察](2026-10-18-yue2-pc-local-deploy-recon.md)、[微调/后训练评估](2026-10-18-yue2-finetune-posttrain-assessment.md)。
- 目标机器：Windows 11 家庭版 26200 / i5-13600KF / 32GB RAM / **RTX 4070 12GB**（2026-10-18 实测空闲 10.2GB，关桌面后约 11.3GB）/ 驱动 617.42（CUDA 13.4）/ Python 3.13（CPU torch）/ 无 WSL、Docker、uv；E 盘 362GB 空闲；局域网 `192.168.1.181/24`，以太网当前为 Public 类别；HF 直连不可用、hf-mirror 可用（实测 200、约 5MB/s）；GitHub Release 直连约 0.09MB/s，需走镜像（`ghfast.top` 实测约 0.57MB/s）。

## 0. 替代模型复查结论

### 0.1 复查结论（2026-10-18，候选池 11 个）

**没有"全维度更优"的单一替代**，按目标分两档：

| 目标 | 最优 | 关键事实 |
|---|---|---|
| 质量上限 + 乐谱级可控 + 中文全曲 | **YuE2** | WildSongBench 6.7316 / best-of-8 6.9632（同代开源第一）；唯一 ABC 乐谱白盒编辑；48kHz；零样本翻唱/agent editing；12GB 靠社区 GGUF；权重 CC BY-NC 4.0 |
| 12GB 稳定 + Windows 原生 + 许可不阻碍产品化 + 工程/API 成熟 | **ACE-Step 1.5** | **MIT（代码+权重）**；官方 12GB 分级、Windows 便携包、Gradio/REST/CLI/VST3；2B DiT；4070 实测 15–30s/首（2min 曲）；最长 10min、50+ 语言、cover/repaint/stems/LoRA；质量 WSB 6.0118（低于 YuE2 一档） |

其余候选（均已核实，不满足 12GB + 质量/全曲 + 许可/工程综合门槛）：
- **LeVo 2**（4B，v2-large 22/28GB，12GB 的 v2-medium 未发布，许可 NOASSERTION）；
- **HeartMuLa-3B**（Apache-2.0，但官网≈24GB、RTF≈1.0，无 12GB 验证、API 弱）；
- **MiniMax Music 3**（8B+0.6B+2.4B+VAE、权重 22GB，<24GB 需层流式且慢，社区许可含商用标识要求）；
- **Muse**（8B >12GB，学术脚本）；**DiffRhythm 2**（Apache，但无 API、12GB 未验证）；
- **SongBloom**（≤150s、固定音频 prompt 非 style 文本）；**YuE v1**（24GB 两段/80GB 全曲、WSB 4.92）；
- **MusicGen / Stable Audio Open Small**（纯器乐/SFX，不满足全曲人声）。

**判定：YuE2 作为主实验基座；ACE-Step 1.5 作为可选第二基线**（工程联调兜底、同 prompt 质量 A/B、repaint/stems/LoRA 原型），不阻塞主方案、先 YuE2 后按余力追加。注意：WSB 由 YuE2 团队发布、ACE 的 WSB variant 未标注，质量结论须以本机听感/对比复核。

### 0.2 第二基线（可选，不并入本轮）

- 形态：独立进程/端口（官方 Gradio 7860 + REST 8001，或 VST3），不与 YuE2 同时驻留显存（12GB 单卡顺序加载）；
- 用途：工程接口联调兜底、同 prompt 快速 A/B、cover/repaint/stems/LoRA 可控性原型；
- 下载：ACE-Step 官方 ModelScope 源（国内可达），Windows 便携包 + `start_api_server.bat`；
- 启用条件：YuE2 主方案 S5 验收通过后，另立 `ACE15-BASELINE-1`（可选卡），先实测中文演唱与 12GB 稳定性。

## 1. 最终选择与部署形态

**选择：YuE2-3B（m-a-p，社区 C++/GGML 实现 `yue2.cpp` 的 `yue-server`，Q8_0 GGUF）。**

```
Mac 侧 / 本机 agent                     PC（Windows 11，RTX 4070 12GB）
┌──────────────────────┐   LAN HTTP   ┌──────────────────────────────────────────┐
│ genapi provider       │  async_poll  │ yue2-shim（Go/Python，唯一对外，Bearer）   │
│ id=yue2-local         │ ───────────► │  POST /generate → {"task_id"}             │
│ 工具 gen.generate      │ ◄─────────── │  GET /generate/{id} → 202 或 200 audio/wav │
└──────────────────────┘              │        │ 127.0.0.1:8087                    │
                                       │        ▼                                  │
                                       │ yue-server.exe（yue2.cpp，GGUF，无鉴权）   │
                                       │  POST /synth → {"id"}                     │
                                       │  GET /job?id=N → {"status"}               │
                                       │  GET /job?id=N&result=1 → multipart       │
                                       │  模型：Q8_0 + VAE；48kHz stereo；replay JSON│
                                       └──────────────────────────────────────────┘
```

为什么是这个形态（见勘察报告 §3/§4）：
- 官方 PyTorch 栈要求 Linux + 24GB 且 Windows 有未合入的 FlashAttention/GQA 补丁问题（PR #166/#222、issue #209），12GB 4070 不适合；
- `yue2.cpp` 的 Q8_0 权重约 4.4GB + KV，65s 歌峰值 5.8GB（`--max-seq 8192` 时 3.8GB），12GB 余量充足；MIT 代码；自带 WebUI/CLI/HTTP job API；结果带 replay JSON（可精确重渲染）；
- genapi 已有 `async_poll` 方言：任务 id → 轮询 202 → 200 audio 即完成（dialects.go: `pollHTTPDone`）。`yue-server` 的 multipart 结果不直接匹配，所以加一层薄 shim（**不改 agent/internal/genapi**），首版用 shim 快速打通，稳定后再评估是否加原生 dialect。

不在本轮范围：微调/训练（见评估报告：12GB 贴红线，不建议）、Mac mini 本地部署、产品化/商用（权重 CC BY-NC 4.0）。

## 2. 部署阶段（S0–S6）

### S0 预检（10 分钟）

```powershell
# 1) GPU/显存/驱动
nvidia-smi --query-gpu=name,memory.total,memory.used,memory.free,driver_version --format=csv
# 2) 磁盘
Get-PSDrive E | Select-Object Name,@{n='FreeGB';e={[math]::Round($_.Free/1GB,1)}}
# 3) 仓库状态（部署不改仓库源码，仅记录基线）
cd D:\Vit_DAW; git rev-parse HEAD; git status --short | Select-Object -First 20
# 4) 目录
New-Item -ItemType Directory -Force E:\AI\YuE2\engine,E:\AI\YuE2\models,E:\AI\YuE2\outputs,E:\AI\YuE2\logs,E:\AI\YuE2\run | Out-Null
# 5) 关闭占显存应用（Godot/浏览器/游戏客户端），目标初始 used ≤1GB
```
记录：GPU 型号/驱动、初始 used、E 盘空闲、HEAD、工作树摘要 → 写入 `coord/runs/YUE2-PC-DEPLOY-1/run1_<TS>/env.txt`。

### S1 取引擎（约 0.4GB 下载）

**首选（预构建，节省时间）**：从 YuE2-Studio v3.6.0 便携包提取 `yue-server.exe` + 后端 DLL（Studio/yue2.cpp 均 MIT；只取二进制，不运行/不依赖 Studio 应用）。

```powershell
$zip = "E:\AI\YuE2\run\YuE2-Studio-3.6.0-portable-windows-x64.zip"
$url1 = "https://ghfast.top/https://github.com/timoncool/YuE2-Studio/releases/download/v3.6.0/YuE2-Studio-3.6.0-portable-windows-x64.zip"
$url2 = "https://gh-proxy.com/https://github.com/timoncool/YuE2-Studio/releases/download/v3.6.0/YuE2-Studio-3.6.0-portable-windows-x64.zip"
curl.exe -L --retry 3 --retry-delay 5 -o $zip $url1   # 失败换 $url2
Get-FileHash $zip -Algorithm SHA256 | Format-List
Expand-Archive -Path $zip -DestinationPath "E:\AI\YuE2\run\studio-portable" -Force
# 核对包内架构：resources\yue2-cpp\ 下应有 yue-server.exe、ggml*.dll、CUDA/Vulkan/CPU 后端
Get-ChildItem "E:\AI\YuE2\run\studio-portable" -Recurse -Filter "yue-server.exe" | Select-Object FullName
Copy-Item -Recurse "<上一步目录>\*" "E:\AI\YuE2\engine\"
```
- 记录：zip 的 SHA256、引擎目录清单、`yue-server.exe` SHA256。若包内布局与 README 不符（文件缺失/只有安装器），停止并上交换“源码构建”分支。

**cuBLAS 13（CUDA 后端必须）**：`cublas64_13.dll` / `cublasLt64_13.dll`（与 yue2.cpp 的 CUDA 13 构建匹配）。

```powershell
$cu = "E:\AI\YuE2\run\libcublas-windows-x86_64-13.5.1.27-archive.zip"
curl.exe -L --retry 3 -o $cu "https://developer.download.nvidia.com/compute/cuda/redist/libcublas/windows-x86_64/libcublas-windows-x86_64-13.5.1.27-archive.zip"  # 2026-10-18 实测 HEAD 200、391MB
Expand-Archive $cu "E:\AI\YuE2\run\cublas" -Force
Copy-Item "E:\AI\YuE2\run\cublas\bin\cublas64_13.dll","E:\AI\YuE2\run\cublas\bin\cublasLt64_13.dll" "E:\AI\YuE2\engine\"
```
- 若 NVIDIA 下载中断：先改用包内 Vulkan 后端跑通冒烟（`--device Vulkan0`，速度略低），再补 cuBLAS。

**备选（源码构建，仅在预构建不可用时）**：
```powershell
git clone --recurse-submodules https://github.com/ServeurpersoCom/yue2.cpp.git E:\AI\YuE2\src\yue2.cpp
# VS2026 Developer PowerShell + CUDA Toolkit 13（本机未装 CUDA Toolkit）
cd E:\AI\YuE2\src\yue2.cpp; buildcuda.cmd
```
成本：CUDA Toolkit 下载约 3GB + 编译 20–40 分钟；只在 S1 首选失败时启用。

### S2 取模型（hf-mirror，约 4.34GB）

```powershell
$base = "https://hf-mirror.com/Serveurperso/YuE2-GGUF/resolve/main"
curl.exe -L --retry 3 --retry-delay 5 -o "E:\AI\YuE2\models\YuE2-3B-Q8_0.gguf" "$base/YuE2-3B-Q8_0.gguf"
curl.exe -L --retry 3 --retry-delay 5 -o "E:\AI\YuE2\models\YuE2-Vae-F32.gguf" "$base/YuE2-Vae-F32.gguf"
Get-Item "E:\AI\YuE2\models\*" | Select-Object Name,Length
Get-FileHash "E:\AI\YuE2\models\YuE2-3B-Q8_0.gguf","E:\AI\YuE2\models\YuE2-Vae-F32.gguf" -Algorithm SHA256 | Format-List
```
期望字节数（2026-10-18 hf-mirror HEAD 实测）：
- `YuE2-3B-Q8_0.gguf` = **3,810,232,064 B**
- `YuE2-Vae-F32.gguf` = **530,497,344 B**
- 记录模型 revision（Studio 侧 pin `Serveurperso/YuE2-GGUF@e630f2b`）；体积/哈希不符即停。
可选（翻唱/转谱，首版不装）：`SheetSage2-Q8_0.gguf`(106MB) + `MERT-v2-FullSong-Q8_0.gguf`(902MB) + 音频 tokenizer(47MB)。

### S3 起引擎 + 本机冒烟

```powershell
cd E:\AI\YuE2\engine
.\yue-server.exe `
  --model E:\AI\YuE2\models\YuE2-3B-Q8_0.gguf `
  --vae   E:\AI\YuE2\models\YuE2-Vae-F32.gguf `
  --device CUDA0 --host 127.0.0.1 --port 8087 --max-seq 8192 `
  *> E:\AI\YuE2\logs\engine.log
```
- 断言 `GET http://127.0.0.1:8087/health` = `{"status":"ok"}`、`/props` 返回版本/路径/48kHz/24576 context；
- 冒烟请求（先 60s，再 180s；`--max-seq 8192` 时请求必须带 `duration`，见 yue2.cpp 文档）：

```json
{
  "style": "Mandarin city pop, female vocal, warm electric piano, bass and drums, 108 BPM",
  "lyrics": "[Verse]\n夜色落在窗台\n城市在呼吸\n[Chorus]\n把这一刻写成歌",
  "cot": "full",
  "duration": 60,
  "seed": 1234,
  "steps": 32,
  "output_format": "wav16"
}
```
- 流程：`POST /synth`（拿 `{"id"}`）→ 每 3s `GET /job?id=N`（running/done）→ `GET /job?id=N&result=1` 保存 multipart → 解析出 `application/json` replay + `audio/wav` 音频；
- `ffprobe -v error -show_entries stream=codec_name,sample_rate,channels -show_entries format=duration -of json song.wav` 断言 48kHz/2ch；
- 显存采样：另开 PowerShell `while($true){ nvidia-smi --query-gpu=memory.used --format=csv,noheader,nounits; Start-Sleep 2 }` 落日志，记录峰值；
- 连续 3 次 + 180s 一次；记录耗时、峰值显存、replay JSON 的 seed/truncated 标志、输出哈希；
- 先关预览/桌面占用；若 OOM：降 `--max-seq`（并同步缩短 duration）或改用 Vulkan 后端，问题如实记录。

### S4 shim（唯一对外面）+ genapi 配置

**shim 契约**（建议 Go stdlib 单文件，零依赖；也可 Python FastAPI 临时代替）：

| 端点 | 行为 |
|---|---|
| `POST /generate` | 校验 Bearer token；读 genapi 通用 body（`prompt`/`style`/`lyrics`/`duration`/`count` + `extra_params`）；组装 Yue2Request（`style` 空时取 `prompt`；`cot/steps/output_format` 从 extra_params，默认 full/32/wav16）；转发引擎 `POST /synth`；返回 `{"task_id":"<id>"}` |
| `GET /generate/{id}` | 引擎 `GET /job?id=`：running → **202**；failed/cancelled → 200 JSON `{"status":"failed","fail_reason":...}`；done → `GET /job?id=&result=1` 解析 multipart，保存 replay JSON 到 `E:\AI\YuE2\outputs\<id>.replay.json`，返回 **200 + audio/wav 原始字节** |
| `GET /health` | 返回引擎 health 与在跑任务数 |

- 环境变量：`YUE2_ENGINE_URL=http://127.0.0.1:8087`、`YUE2_LISTEN=0.0.0.0:8000`、`YUE2_TOKEN=<32+ 随机>`、`YUE2_OUTPUT_DIR=E:\AI\YuE2\outputs`；
- 单 worker/串行锁（引擎本身 FIFO）：避免 Mac 侧并发打爆；
- 日志：请求 id、时长、状态、耗时；**不打印 token/lyrics 全文**；
- 失败分类：引擎不可达→503、token 错→401、参数错→400、provider 失败→JSON failed、结果超时→504。

**Mac 侧 genapi provider**（`~/.vit/genapi_providers.json`，token 只进用户级文件）：

```json
{
  "providers": [{
    "id": "yue2-local",
    "dialect": "async_poll",
    "base_url": "http://192.168.1.181:8000",
    "api_key": "<YUE2_TOKEN>",
    "auth": { "style": "bearer" },
    "generate_path": "/generate",
    "poll_path": "/generate/{task_id}",
    "poll_interval_seconds": 3,
    "timeout_seconds": 1800,
    "per_call_timeout_seconds": 1200,
    "param_mapping": { "style_tags": "style", "lyrics": "lyrics", "duration_seconds": "duration" },
    "extra_params": { "cot": "full", "steps": 32, "output_format": "wav16" }
  }]
}
```
- 依据 `agent/internal/genapi`：`async_poll` 的 202 = 进行中、200+audio = 完成；任务 id 字段含 `task_id`/`id`；done 状态含 `done`；上表 shim 正好落在该契约内；
- **不改 `agent/internal/genapi/**`**（零生产代码）；若后续稳定，再评估新增 `yue2_server` 原生方言（另一个卡）。

**Windows 防火墙（需管理员，一次性）**：

```powershell
Set-NetConnectionProfile -InterfaceAlias "以太网" -NetworkCategory Private
New-NetFirewallRule -DisplayName "YuE2 Shim 8000" -Direction Inbound -Protocol TCP -LocalPort 8000 -Action Allow -Profile Private -RemoteAddress <Mac-IP>
```
- 引擎 8087 只绑 `127.0.0.1`，不开放；shim 8000 仅放行 Mac IP；token 必填；不开公网/不做多租户。

### S5 验收（真栈门）

| 门 | 标准 |
|---|---|
| S0 环境 | `nvidia-smi` 正常、E 盘 ≥50GB、HEAD/工作树已记录 |
| S1 引擎 | `/health`=ok、`/props` 200、引擎日志无 cuBLAS/CUDA 错误 |
| S2 本机冒烟 | 60s 歌成功 ×3；180s 歌成功；`ffprobe` 48kHz/2ch；峰值显存 <10.5GB；重放同一 replay 得到相同音频（同 seed/同引擎） |
| S3 Mac 端到端 | Mac agent `gen.generate` → shim → 引擎 → 音频 → 既有 ingest 成 clip/take；连续 3 次；取消/超时/token 错/引擎停四类失败分类正确 |
| S4 稳定性 | 连续 5 次请求显存回到基线；服务重启后可再次加载；无残留进程 |
| S5 边界 | 5–6 分钟长歌做一次，记录 `truncated` 与耗时；如 OOM，记录边界不掩盖 |
| 工件 | `coord/runs/YUE2-PC-DEPLOY-1/run1_<TS>/`：命令/退出码/引擎日志/哈希/ffprobe/显存尖峰/耗时/Mac 回执；零 token、零密钥 |

### S6 固化与回滚

- 脚本：`scripts/yue2_pc_start.ps1`（起引擎+shim，写 PID/日志）、`scripts/yue2_pc_stop.ps1`、`scripts/yue2_pc_smoke.ps1`（健康+一次 60s 歌+ffprobe，exit 0/1）；
- 电源：交流待机已是「从不」；核对待机/睡眠即可；
- 版本：引擎 zip SHA256 + `yue-server.exe` SHA256 + 模型 SHA256/revision 入 `env.txt`；不追最新；
- 回滚：停两个进程 → 删除/重命名 `E:\AI\YuE2` → 删除防火墙规则 → 恢复网络类别（如需）；不改系统、不写注册表。

## 3. 风险与对策

| 风险 | 对策 |
|---|---|
| GitHub Release 直连极慢/断 | 用 `ghfast.top`/`gh-proxy.com` 镜像；失败则源码构建；下载后校验 SHA256 |
| HF 直连不可用 | 全程 hf-mirror；必要时 ModelScope（官方权重有镜像） |
| 12GB 显存（桌面占用 1.5–2GB） | 部署前关应用；Q8_0 + `--max-seq 8192`；首版 ≤3 分钟；长歌单独测边界 |
| cuBLAS 391MB 下载失败 | 先用 Vulkan 后端跑通，再补；或从 CUDA Toolkit 目录复制 |
| 引擎为第三方预构建二进制 | 只取二进制、固定 v3.6.0、记录 SHA256；MIT；必要时源码重构建 |
| `yue-server` 无鉴权/默认 0.0.0.0 | 强制 `--host 127.0.0.1`；对外只经 shim + token + 防火墙 |
| 单 worker、一次一首 | shim 串行；genapi `async_poll` 天然适配长任务；超时 1800s |
| 上游 6 周龄、快速变动 | 固定引擎/模型版本；升级另开卡 |
| 权重 CC BY-NC 4.0 | 只作非商用研发；不入仓/不进交付物；商用前谈授权 |
| 无官方 Windows 支持 | 走社区 C++ 路径；验收以本机真栈为准，不引用官方 Windows 承诺 |

## 4. 工期与分工（建议）

- S0–S3（引擎/模型/本机冒烟）：**0.5 天**（下载顺利约 40 分钟）
- S4–S5（shim + Mac 端到端 + 验收）：**0.5 天**
- 总计 1 天；下载失败/镜像波动另计。

建议卡：
- **YUE2-PC-DEPLOY-1**（PC 执行侧）：S0–S3 + 启动脚本 + 回执；文件域 `scripts/yue2_pc_*.ps1`、`coord/runs/YUE2-PC-DEPLOY-1/**`；不动 `agent/` 生产代码。
- **GENAPI-YUE2-LOCAL-1**（依赖上卡；PC/Mac 执行侧）：shim（`agent/cmd/yue2shim` 或 `scripts/yue2_shim`）+ Mac provider 配置 + 端到端与失败分类；零 `genapi` 改动。
- 可选后续：`YUE2-GENAPI-DIALECT-1`（原生方言）、`YUE2-LORA-SPIKE-1`（微调，见评估报告）、`YUE2-QUALITY-EVAL-1`（听感）。

## 5. 关键事实与来源（摘要）

- 官方 YuE2：Linux + Python 3.12 + 24GB 验证；权重 CC BY-NC 4.0 + 个人创作者许可；无官方 HTTP/训练代码（issue #165）——见 [官方仓库](https://github.com/multimodal-art-projection/YuE)。
- `yue2.cpp`：`POST /synth`→`{"id"}`、`GET /job?id=`→status、`GET /job?id=&result=1`→multipart（JSON replay + audio + latents）、`/health`、`/props`；Q8_0 3.81GB、VAE 530MB；VRAM 表/65s 峰值 5.8GB；MIT——[yue2.cpp](https://github.com/ServeurpersoCom/yue2.cpp)、[ARCHITECTURE.md](https://github.com/ServeurpersoCom/yue2.cpp/blob/master/docs/ARCHITECTURE.md)。
- 引擎二进制来源：YuE2-Studio v3.6.0 portable（MIT；architecture 说明 `yue-server.exe` 位于 `resources\yue2-cpp\`）——[YuE2-Studio](https://github.com/timoncool/YuE2-Studio)。
- genapi 契约：`agent/internal/genapi/dialects.go` 的 `async_poll`（202 进行中/200+audio 完成、task_id/status/done 字段）；配置 schema `agent/internal/genapi/config.go`。
- 本机网络/体积/可达性：2026-10-18 实测（hf-mirror HEAD/限速、GitHub Release 限速、NVIDIA redist HEAD）。

## 附录 A：手动下载清单（2026-10-18 实测链接与校验值）

| 文件 | 链接 | 大小 | SHA256 |
|---|---|---|---|
| YuE2-Studio v3.6.0 portable（取引擎用） | `https://github.com/timoncool/YuE2-Studio/releases/download/v3.6.0/YuE2-Studio-3.6.0-portable-windows-x64.zip`（慢）；镜像 `https://ghfast.top/<原链接>` 或 `https://gh-proxy.com/<原链接>` | 354,234,123 B | 官方未发布；下载后自算并记录 |
| cuBLAS 13 Windows redist | `https://developer.download.nvidia.com/compute/cuda/redist/libcublas/windows-x86_64/libcublas-windows-x86_64-13.5.1.27-archive.zip` | 391,055,517 B | 官方未在页面给出（下载后自算） |
| YuE2-3B-Q8_0.gguf（pin `e630f2b8f6aedf8254c3186438633c1b5eeaf80a`） | `https://hf-mirror.com/Serveurperso/YuE2-GGUF/resolve/e630f2b8f6aedf8254c3186438633c1b5eeaf80a/YuE2-3B-Q8_0.gguf` | 3,810,232,064 B | `41121ce97786d7795a325bcf123ca196956c03bb252c9e75384cfc1f2fc19e6b` |
| YuE2-Vae-F32.gguf（同 revision） | `https://hf-mirror.com/Serveurperso/YuE2-GGUF/resolve/e630f2b8f6aedf8254c3186438633c1b5eeaf80a/YuE2-Vae-F32.gguf` | 530,497,344 B | `93e49dfb1970e89ad64cacb17cf13b5d05f6bb30ef7ed3adae3050bcb728638a` |

- 直连 `huggingface.co` 在本机不可用；`hf-mirror.com` 可用（实测 200，约 5MB/s）。
- 下载后校验：`Get-FileHash <file> -Algorithm SHA256`；大小不符或哈希不符即停。
- 引擎包不含官方校验值：下载后记录 zip 与 `yue-server.exe` 的 SHA256 入 run 目录。
- Studio 包只用于取 `resources\yue2-cpp\` 下的引擎二进制；不运行 Studio 应用、不启用其自动更新/统计。
