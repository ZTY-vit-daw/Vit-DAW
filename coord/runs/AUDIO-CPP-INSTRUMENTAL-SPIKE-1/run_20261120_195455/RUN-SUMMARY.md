# RUN-SUMMARY — AUDIO-CPP-INSTRUMENTAL-SPIKE-1 步①-B BF16 对照轮（audiocpp439，2026-11-20）

- 对照设计：**单变量隔离**——与步① run_20261120_192419 唯一差异 = `--session-option yue2.model_gguf=yue2-3b-bf16.gguf`（Q8→BF16 基座）；style/lyrics/cot=full/semantic_max_tokens=1500/LoRA 同挂同 scale/seed 1234/f32 VAE/threads/backend 全部逐项一致（见 `command.txt` 首部 diff 注记）。
- BF16 完整性：`yue2-3b-bf16.gguf` 7,261,484,576B，本地 SHA256 `4cf3fdffbaaed2d8e30a3122ac7cd27d0c96f8921805f173da9758c685b3c7d6` 与 HF 官方 LFS 指针**精确匹配**（`sha256_bf16_local_computed.txt`）。
- 生成：恰 1 次，**exit 0**；AR LoRA 挂载亲证（projections=196 scale=1.000000）；BF16 权重缓冲 3409.96MB CUDA0（步① Q8=2149.96MB）。
- 产物：`yue2_instrumental_bf16_seed1234.wav` 11,519,788 字节，**audio_duration_ms=59998.7（60.0s，与步①精确同长）**，48kHz 双声道；内部 ABC score 2296B（步①=1229B，计划分支不同）。
- **WAV 内容差异亲证**：BF16 SHA256 `cead7a51b4f2…69a5` ≠ 步① Q8 `91c5383f7977…7898`（同字节数属时长/格式巧合，音频内容不同）。
- 性能：wall 98.8s（步① 77.0s），**RTF 1.64659**（步① 1.28345），**VRAM 采样峰值 9607 MiB**（55 样本@2s，峰值 19:57:03 NAR 段；步① 6404MiB）；引擎自报 peak_rss 11554MB / peak_footprint 12007MB（windows_commit）——12GB 卡上限内，未 OOM。
- 栈纪律：跑前 GPU 1699MiB 基线、Vit 系进程 0；teardown 后 Vit 系进程 0（before/after 零变化亲证）、engine 进程直证 0（ENGINE_PROCS_ZERO）、VRAM 回落 1898MiB。
- 工件：`command.txt`、`engine_stdout.log`、`engine_stderr.log`、`vram_before/after.txt`、`vram_samples.csv`、`vit_processes_before/after.txt`、`sha256_bf16_local_computed.txt`、`sha256_step1_files_local_computed.txt`、WAV。WAV 留工作树工件目录未入 git（§12）。
- 边界：人声判定归用户人耳（lead 组织）；听核物=两枚 WAV（Q8 与 BF16 同 seed 同条件对照）。
