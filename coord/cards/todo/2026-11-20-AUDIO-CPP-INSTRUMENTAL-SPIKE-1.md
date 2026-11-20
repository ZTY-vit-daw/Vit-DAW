# AUDIO-CPP-INSTRUMENTAL-SPIKE-1：audio.cpp 器乐通路 spike（LoRA 路线）

- 池序 439；P2；PC；发卡=lead（2026-11-20）；派发确认=用户「没问题」（audio.cpp 路线批准）；验收负责人=lead（步①人耳=用户）。

## 领取（audiocpp439，2026-11-20 19:05 +0800）

- 执行会话：ZCode GLM-5.3-Flash 子代理 audiocpp439（PC，主仓 D:/Vit_DAW，main 工作树；本轮仅步①，步②条件触发不在本轮）。
- 领取基线：`git rev-parse HEAD` = `e2dbd4537e22aa4e61b37a236c927bff13b04aa1`。
- 领取时 `git status --short` 已有非本卡改动（原样保留不触碰）：`CURRENT-STATE.md`、`VitApp/Workspace/Settings/Settings.xml`、`VitApp/Workspace/default_project.xml`（M）、`coord/cards/blocked/2026-11-20-TUTOR-SIM-4-PROD-NODES.md`（D）、`tracktion_engine`（M 子模块）、若干未跟踪 docs/coord/reports/ledger/.vit_derived 文件。
- **调度变更（lead 2026-11-20 19:0x）**：三件下载改由用户终端手动执行（用户网络更稳）；本会话不自行下载，轮询等待 `E:\AI\audio-cpp\` 文件就位（≤90 分钟，60s 间隔，大小稳定判据）后继续 SHA 留档与生成。
- 部署布局（以文件就位为准）：`E:\AI\audio-cpp\engine\`（v0.9.5 windows-x64-cuda12.4 bin+cudart 解压产物）、`models\yue2-3b-q8_0.gguf`、`models\yue2-vae-f32.gguf`、`models\sidecars\{4 件}`、`models\lora\ar_lora_inst_v3abc.safetensors`（AR LoRA only；NAR LoRA 本轮不挂，stock NAR decoder）。
- 失败分类：按卡面（环境中断类 vs 概率类负结果）；生成预算恰 1 次；同形失败两次停卡。
- 背景：390 fallback 收口后新证据——官方 instrumental=LoRA 适配器方案（`Mothersuperior/YuE2-instrumental-cot-full-loras`，AR+joint NAR 对）；yue2.cpp 无 LoRA 支持=纯 conditioning 不可能到达；audio.cpp v0.8.1+ 支持 `yue2.ar_lora`/`yue2.nar_lora`，Q8 峰值 <9GB（4070 12GB 可容）。

## 目标（两步，步②条件触发）

**步①（零接入直跑，本卡主体）**：
1. 新目录部署 `E:\AI\audio-cpp\`（**不改动 E:\AI\YuE2 现有部署**，两引擎并存）。
2. 下载三件（磁盘预算 ~5-8GB；逐件记录来源 URL+SHA）：
   - audio.cpp Windows x64 **CUDA** 预编译包（github.com/0xShug0/audio.cpp releases；v0.9.x 资产列表 36 项含 CUDA 版；若最新版无 Windows CUDA 资产，回退最近含它的版本；仍无→停卡上报，不源码编译硬上）；
   - HF `audio-cpp/Yue2-3B-GGUF`（audio.cpp 原生格式 base+VAE；注意**不是**现有 yue2.cpp 的 GGUF）；
   - HF `Mothersuperior/YuE2-instrumental-cot-full-loras`（AR LoRA + joint_v1 NAR LoRA；许可 CC BY-NC 4.0 登记——非商用，毕设/展演可用）。
   - HF 直连失败用 hf-mirror.com 镜像，记录实际源。
3. CLI 直跑一次 instrumental 生成（官方 docs/models/yue2.md 口径）：`--task gen --family yue2` + `cot=full` + style 器乐标签 + lyrics 用 `[instrumental]`/section 标签 + `--session-option yue2.ar_lora=…` / `yue2.nar_lora=…`；固定 seed 记录；60s 目标时长（semantic_max_tokens≈1500 帧@25fps，或按文档口径）。
4. 产物：WAV + 引擎完整 stdout/stderr + 命令行落 `coord/runs/AUDIO-CPP-INSTRUMENTAL-SPIKE-1/run_<TS>/`；VRAM 峰值记录（nvidia-smi 采样或引擎自报）。
5. 回执后 lead 组织**用户人耳听核**：零人声=通路成立；成段人声=负结果收口。

**步②（条件触发=步①人耳判定零人声/零星）**：agent `genapi` 新 provider dialect `audiocpp_local`（HTTP server `/v1/tasks/run` 面）+ engine supervisor 配置 + 最小 headless smoke。人耳未过则不接、整卡负结果收口。

## 约束

- 不动现有 yue2.cpp 引擎部署、gen 链与 genapi 既有 dialect；不改产品代码（步②才动，届时另记文件域）。
- 真栈纪律：跑生成时登记 PC-RUNTIME-STACK（GPU 归属）；结束后核对 Vit 系进程零变化。
- 失败分类：下载源缺失/引擎起不来/CUDA 不兼容=环境中断类如实记录；生成出人声=**概率类负结果非执行失败**。

## 验收

- 步①：三件来源+SHA 留档、完整命令、引擎日志、WAV、VRAM 记录、用户听核登记。
- 停止条件：Windows CUDA 预编译不存在 / 模型下载源失效 / 引擎 CUDA 初始化失败两次→停卡上报。

## 执行与判定（步①，2026-11-20）

- 生成恰 1 次 exit 0（audiocpp439；三件下载=用户终端手动，SHA 七件对官方全匹配）：cot=full + `[instrumental]` lyrics + style 器乐标签 + `yue2.ar_lora` 挂载（projections=196 scale=1.0，stock NAR）+ seed 1234；WAV 60.0s 48kHz 立体声；VRAM 峰值 6404MiB；RTF 1.283；GPU 预留/释放登记在卷（ad4bed31）；工件 `coord/runs/AUDIO-CPP-INSTRUMENTAL-SPIKE-1/run_20261120_192419/`（0caa72bf）。
- **用户人耳判定（2026-11-20 19:4x）：成段人声 → 概率类负结果，步②不触发。**

## 已知混杂因素（未排除，lead 登记候用户裁定）

- audio.cpp 官方文档明示：**LoRA 合并进 Q8 基座会重量化，不等价于 BF16 先合并**——本次被测=Q8 基座+LoRA，LoRA 效果可能被量化合并损伤。干净判定的唯一变量隔离=BF16 基座（`yue2-3b-bf16.gguf` 同仓在库，~6.2GB）重跑一次。
- 另一未证面：Mothersuperior「joint_v1 NAR LoRA」存在性未落实（本仓文件列表仅 AR 件）。
- 处置选项：a) BF16 单发重试（下载 6.2GB + 一次生成）后终裁；b) 就此整卡负结果收口（fallback 口径不变，损失=下载流量）。
