# GEN-VOCALSEP-1 run_20261107_230432 — 跑前口径（§8，实现前成文）

- 日期：2026-11-07 23:04 / owner：306 ZCode GLM-5.3 L2 子代理 / worktree D:/Vit_DAW_wt_306vsep @ 0fe253dd（FF 至 origin/main）
- 性质：**确定性本地 CPU 推理，零 LLM、零真实栈**（本腿无栈登记，卡面已声明）

## 运行次数与预算

| 步骤 | 次数 | 预算 |
|---|---|---|
| 权重获取（官方直链速试→HF 镜像） | 直链 1 次（30s 超时即弃，已执行=403）；HF 镜像下载 1 次 | 每文件 ≤5 min |
| 功能验证（模型加载 + 短切片分离） | 1 次 | ≤5 min |
| 全量分离（3 wav × vocals/no_vocals two-stems） | 各 1 次，共 3 次，**无重跑** | CPU 每首预计 2-6 min，总 ≤30 min |
| 客观指标计算 | 1 次（3 文件各算一遍） | ≤2 min |

## 成功条件

- demucs 4.1.0 成功加载 htdemucs（来源：HF `adefossez/HTDemucs` 官方作者仓，经 hf-mirror.com 端点）；
- 3 个源 wav 各产出 vocals stem + no_vocals residual（可听产物，采样率与源一致）；
- 权重来源 URL / 大小 / sha256 入本回执；
- 指标与三 vocals stem 路径入回执供用户试听。

## 失败分类（分开记录，不合并）

- A 网络中断（下载失败/超时/镜像不可达）
- B 依赖/加载失败（包缺失、safetensors 解析错、内存不足 OOM）
- C 音频 IO 失败（源读入/写出错、采样率异常）
- D 分离产出异常（空文件/全静音/时长不符）

## 止损线

- 同一代码路径同类失败连续 2 次 → 停止重试，取证上交（按类记录 A/B/C/D）；
- 权重全渠道不可达 → 如实上交（评估同库可达替代模型或转部署裁定）；
- 单 wav 分离墙钟 >15 min → 记录后评估是否继续剩余文件（CPU 慢不是失败）；
- 遇 1308 配额中断 → 停 + 写 PROGRESS.md。

## 红线复述

- 三 wav 只读（wt_278impl 工件目录，不改不删）；不动 YuE2 环境；权重不入源码树/不入仓（HF 标准缓存区）；wav 产物留盘不入卷（*.wav 全局 ignore，278 先例）；禁止 git reset --hard / git clean。
