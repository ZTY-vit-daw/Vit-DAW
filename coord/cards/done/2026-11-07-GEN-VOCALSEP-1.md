# GEN-VOCALSEP-1：生成产物人声分离后处理（用户 2026-11-07 听感裁定方向）

- 发卡：GLM 主管决策流 / 2026-11-07 深夜（依据=[用户三 wav 亲听裁定](../done/2026-11-05-GEN-YUE-DIALECT-1.md 听感旁证节)——「仅人声」模式声学上仍有稀疏配器（点状节奏+低音），但旋律/节奏/速度高度统一；**加入人声分离即可直接作人声用**）
- 派发确认：已确认（用户方向明示）；验收负责人：GLM 主管决策流
- 池序 306；目标仓库=主仓 agent（生成产物后处理链）+本地分离模型；P3 / 0.5-1 天 / 依赖=278 已合入（产物链在）+三 wav 样本在盘（wt_278impl 工件目录）；L1-L2 / PC；零真栈先行（分离模型本地跑），管线集成腿可选占栈一次

## 目标

1. **分离模型选型与落地**：本地人声分离（Demucs/MDX-Net 类，GPU 在场 VRAM 预算 ≤6.3GB 线内实测）——先对 278 三 wav（run4/5/6）离线分离出 vocal stem（+可选 instrumental residual）；
2. **质量面**：分离产物听感抽查（用户耳朵为主审）+客观指标（vocal stem 残余伴奏电平估计）；如实边界：分离质量模型依赖，不承诺棚级；
3. **管线集成（裁量）**：genapi yue-local 产物→分离→vocal stem 落 artifact 的可选后处理开关（默认关）；chat 话术「只取人声」类意图接线候后续卡；
4. §8 跑前成文（分离腿本地 GPU 零 LLM；墙钟/显存如实记）。

## 约束

- 文件域=scripts 分离驱动+工件+（裁量）genapi 后处理开关±测试；模型权重不入仓（artifact 区）；不触 278 已合入链本体行为（默认关）；
- 三 wav 样本只读；禁止 git reset --hard / git clean。

## 停止条件

本地分离模型在本机跑不动（显存/依赖墙）→取证上交转部署裁定；分离质量听感不过关→如实报告不强推集成。

## 领取/回执/验收

- 领取：2026-11-07 22:41 / origin/main=9180ff89 / owner=ZCode GLM-5.3 L2 执行子代理（PC，306 领卡会话）/ 分支 port/gen-vocalsep-1 / worktree D:/Vit_DAW_wt_306vsep / 领取基线：HEAD 9180ff89，status 全净，diff --stat 空；doing 三卡（ARRLOOP-AUTHBRIDGE-1/CLOSURE-ROUTE-ADAPT-1/PLUGIN-CONTROL-EVAL-S1A-1）文件域与本卡不相交已核对；零真栈先行，307 现持栈占用（GPU 腿无栈面冲突）
- 回执：2026-11-07 23:15 / ZCode GLM-5.3 L2 子代理（续 181f94d0 领取恢复执行）/ 本次续跑基线：FF 至 origin/main 0fe253dd 后实施。**权重获取**：①官方直链 dl.fbaipublicfiles.com 速试=HTTP 403（2.9s，弃）；②落地走 **demucs 4.1.0 原生官方 HF 路径**：作者官方仓 `adefossez/HTDemucs`（htdemucs.yaml + 955717e8.safetensors，签名与官方 AWS 文件 955717e8-8726e21a.th 同源），端点 `HF_ENDPOINT=https://hf-mirror.com`（如实标注：官方仓×镜像端点，非官方直链）。955717e8.safetensors 84,025,440 B sha256=d9fa14133cfcc034a6758923bb3a8ca9f8dfd0b582134643bbf83f72c17576dd；htdemucs.yaml 21 B sha256=239c445d0b1444…；下载+哈希 29.6s。**偏差声明**：权重落 HF 标准缓存（~/.cache/huggingface/hub/models--adefossez--HTDemucs），非预期的 torch/hub/checkpoints/htdemucs.pt——demucs 4.1.0 默认优先 HF 加载路径，包原生支持位，功能等价。**功能验证**：10s 切片 CLI 分离 EXIT=0/13s，双 stem 非静音（rms 0.122/0.106）。**三 wav 分离**（源只读；驱动 scripts/gen_vocalsep_demucs.py；CPU/shifts=1；产物 48kHz 与源对齐、时长一致）：run4 9e4f26 infer 16.6s（实时系数 0.29）；run5 82984d 18.2s（0.30）；run6 46797c 17.2s（0.30）。失败如实记录：脚本首版 3 次失败（类 B 依赖假设错：torchaudio 未随本环境 demucs 安装，改用包自带 julius 后消除）；run6 首次 EXIT=127 无 traceback、同命令复跑一次即 EXIT=0→按环境中断嫌疑记录（run3_retry.txt 为证），未触发两次止损线。**客观指标**（简单代理非 SDR，方法=脚本 docstring）：vocals 能量占比 27-45%；重建残差 1.3-3.6%；vocals stem <150Hz 泄漏 ≤0.021%；人声静默帧残余伴奏比 no_vocals 低 21.9-47.3 dB。**用户试听三 vocals stem**：`D:/Vit_DAW_wt_306vsep/coord/runs/GEN-VOCALSEP-1/run_20261107_230432/stems/yue-local-9e4f260d16495bd6/vocals.wav`、`…/stems/yue-local-82984d83d18fdcce/vocals.wav`、`…/stems/yue-local-46797cf8bea5cde6/vocals.wav`（同目录 no_vocals.wav=伴奏残差）。**管线集成：未做（如实声明）**——本腿零真栈红线，genapi 生产改动须过真实栈烟测（AGENTS §5）方可交付，本腿无法合规验证；驱动已按可集成形态写（API 直调+参数化），建议后续卡（genapi 开关默认关+一次栈占用验证）。**工件**：coord/runs/GEN-VOCALSEP-1/run_20261107_230432/（RUNBOOK 跑前口径/权重清单/控制台日志/指标 JSON/stems_manifest；wav 留盘不入卷=*.wav 全局 ignore，278 先例一致）；权重不入仓。测试：三分离 EXIT=0/0/0；run ID=run_20261107_230432；零 LLM 零计费、零栈登记（本腿无栈面）。
- 验收：（主管回填）

## 状态（2026-11-07 深夜）

- 已领取（181f94d0）后**挂起（2026-11-07 深夜主管裁定：P3 非展演线依赖）**——教学轨迹录播/生成展示/混音展示三面均不依赖人声分离；**触发条件=「生成人声作为独立轨进 DAW 工程」场景入卡时再部署**（一晚工作量：demucs 包+htdemucs.pt 单模型即覆盖 vocals/drums/bass/other 四轨，无需其他拆解模型）。恢复（2026-11-07 跨零点）：**用户授权执行侧自行部署**（用户 pip 装 demucs 4.1.0 成功、权重直链 fbaipublicfiles 双方均不可达——「基于 demucs 包开一份任务，然后再去部署使用」=授权任务自行取权重）；本卡豁免 [部署归用户裁定](../../decisions/2026-11-07-deployment-tasks-to-user.md)（单卡豁免非改规则）。取权重策略：官方直链速试→不通走 HF 镜像（hf-mirror.com 可达性优先）；验证=功能验证（模型加载+分离产出可听产物）+来源/大小/sha256 如实入卷；运行环境=默认 Python 3.13（demucs 已装，CPU 推理可接受）。
- 验收（2026-11-07 深夜，主管）：**pass（分离交付+用户试听终审待）**——权重获取合规（官方仓×镜像端点如实标注+sha256 入卷+功能验证）；三首分离 0.3×实时系数+指标齐全（重建残差 ≤3.6%+静默帧伴奏压制 21.9-47.3dB）；**质量终审=用户耳朵**（三 vocals stem 路径在回执，候听）；管线集成如实未做（生产改动须栈烟测纪律正确）→集成腿候后续卡；run3 EXIT=127 环境中断嫌疑记录合规。L1 台账 48h 窗口。
- **质量终审（2026-11-07 深夜，用户耳朵）：pass「够用」**——三 stem 试听结论：效果绝对不差、当前程度 OK；更强分离模型=非技术壁垒不追。**排序观察**：run_195542（9e4f26）与 run_200252（82984d）明显比 run_213958（46797c）干净——与源 mix 配器密度的样本间差异（复验三档边界的生成随机性）传导进分离难度一致，记录为相关性旁证。管线集成腿维持候触发（「人声进工程」场景）。质量面闭环。
