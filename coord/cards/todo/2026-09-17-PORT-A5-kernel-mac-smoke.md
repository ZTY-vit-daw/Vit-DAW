# PORT-A5：内核 mac 冒测最小集（dev_agent_smoke mac 等价 + WaveShell 首验）

- 优先级 / 预估 / 依赖：P1 / 1-2 天（PORT_AUDIT 层 A 收官卡）/ 依赖 A1+A4（均已验收）
- 模型分级：L3 / GLM-5.3（真机未知面：ZMQ 命令面、插件扫描 child-process、WaveShell R2/R9）
- 目标：`scripts/dev_agent_smoke.ps1` 的 mac 等价最小集：真实 VitApp 内核 mac 起停、agent↔内核链路探活（ZMQ 命令面 + agent HTTP 面）、JUCE 插件扫描 child-process；**WaveShell 枚举并入（R2 首验：23 个 Waves 主体 @ `/Library/Audio/Plug-Ins/VST3`）**
- 输入（A1 平台发现，已采纳）：内核无优雅退出面（SIGTERM=exit 143、无 shutdown 命令）——本卡须实测并记录内核停止方式（SIGTERM+宽限 / SIGKILL / 均可）；强杀后 POSIX shm 段存留用 lsof 核查，A1 stale 自愈路径（unlink 重建）可实证兜底
- 文件域：`scripts/`（新 mac 冒测脚本，复用 C4 白名单 GET 模式与 ps1 体系既有模式，命名对齐惯例）；**不改 `VitApp/Source`**（若探明必须内核侧 shutdown 命令面 → 停止条件上交开卡）；`agent/` 如需最小接线改动须列明并最小化
- 验收标准：① mac 真实栈（内核+agent 两件，Godot 前端不在本卡）冒测脚本 exit 0：起内核 → agent 起动并探活（/health 等白名单）→ ZMQ 命令面至少一条真实命令往返（如 project/state 类只读命令）→ 停内核（停止方式与退出码记录）；② 插件扫描 child-process 跑通并产出 23 Waves 主体清单（R2 首验结论显式：WaveShell1-VST3 枚举成功/失败及形态）；③ 工件独立目录（run ID + 内核/agent 日志 + lsof 证据）；④ 失败按类型记录（环境中断 / 功能失败分开，AGENTS §8）
- 停止条件：需改内核代码才能起停/探活 → 上交开卡（本卡以验证为主）；WaveShell 枚举失败 → 保留证据转 blocked（R2 预案：升级 tracktion/JUCE 或补丁另开卡）；发现内核 mac 运行时崩溃 → 记录栈与复现条件上交
- 领取：
- 回执：
- 验收：
