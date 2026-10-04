# REGION-OP-RECON-1：「范围即操作目标」能力盘点——框选局部→拆分→单独调改的管线可行性（只读勘察，设计前置）

- 池序 13；目标仓库=D:\Vit_DAW+D:\Godot\project\vit-daw-frontend（只读）；来源=手测四号场用户方向输入（2026-10-03：「用户框选了 clip 的局部范围，想要的就是只作用于这个范围，agent 该先识别到这个范围，然后把它拆出来单独调改」）；**用户已认方向，本卡定可行性与设计**
- 优先级 / 预估 / 依赖：P2 / 0.5 天只读 / VITNOTE-REGION-TIME-1 并行不依赖（识别面先行）；产出=设计输入文件，不开实现
- 模型分级：L1 / GLM 或 flash 可接（纯勘察）
- 盘点问题（每条带代码锚点回答）：
  1. **内核 clip 拆分能力**：VSP 内核命令表有无 clip.split/切开（VspKernelReference.cpp 命令表+内核编辑面）？DAW 前端既有"选/移/裁/画"编辑里裁剪的实现路径（用户手测第 7 步确认存在 clip 编辑）能否经 agent 命令面复用？
  2. **局部处理路径三选**：(a) 拆分出子 clip→对子 clip 单独处理（物理拆）；(b) 参数自动化/时间限段处理（不拆 clip，处理带时间界——内核 DSP 面是否支持）；(c) 前端侧区域处理（段落 DSP 线 L2-2 SEG 的 marker 树/SegmentationPrimitives 衔接点——排程里 L2-2 已建基元与 marker 树）。
  3. **治理适配**：D1 自由态的"单轮一次前向变更"与范围操作（拆分+调改两步）的相容形态；A/B 试听在子段上的形态（audition 面是否时间界感知）。
  4. **辖区→主任务交接**：note 识别出的范围（含 REGION-TIME-1 的时间界）如何成为主任务 goal 的操作目标（载荷传递 vs 主流直接框选指令）。
- 产出：`coord/runs/REGION-OP-RECON-1/RECON.md`（四问各带锚点+三选路径的可行性结论+建议路径与分期），供决策侧出设计卡。
- 约束：只读（零代码改动零探针写源码树）；不动 sealed fixture。
- 验收标准：四问全答+锚点可回查+路径建议明确（不代决——用户拍板）。
- 领取：2026-10-04 / 基线 08031102b6c3fc672a066bafdbc1898e4e329f75（Windows 端执行会话；领取时工作树预存 VitApp/Workspace/Settings.xml+default_project.xml 未提交改动与若干未跟踪 coord/runs 目录，本卡不触碰）
- 回执：`coord/runs/REGION-OP-RECON-1/RECON.md`（四问全答+18 条锚点索引+三选结论+分期建议）。核心发现：clip.split 全链在位（命令表 VspKernelReference.cpp:111 / ClipService.cpp:1496-1577 带子 clip id 回传 / catalog.go:987 / 前端 TimelineInputArbitrator cut 工具）；**前端已有 range 工具框选 clip 局部（selected_clip_ranges 双时间界，vit_track_lane_2d.gd:2016-2024）且已流入主对话上下文**；缺口=意图层不消费 ranges（intent.go:248-259）+ 内核 DSP 处理命令无时间界（范围只在 render.start/l2_render_probe 与 strip_silence 面存在）+ note→主流无交接机制。三选结论：(a) 物理拆可行性最高（建议主路径）；(b) 范围界定处理 agent 消费面有两条先例但工程参数面需新内核命令；(c) SEG/marker 线是分析位非执行位。治理：普通确认制轨道不撞 D1；自由态实验轨道同轮两 mutation 违 D1-S1，两轮制/拆分定性需用户裁定（RECON §3.2）。子段 A/B 可经 render.start range + audition audio_file candidate 组合实现，零 audition 面改动。端测边界声明：纯只读勘察卡，零代码改动，不适用 AGENTS §5 端侧烟测门槛；全程未起真栈、未动 sealed fixture、未触碰工作树预存改动。
- 验收：（裁定文件）
