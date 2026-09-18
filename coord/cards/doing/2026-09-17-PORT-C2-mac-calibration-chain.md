# PORT-C2：mac 校准链脚本（探针→白名单草稿→PCA 重认证→冒测，23 主体口径）

- 优先级 / 预估 / 依赖：P1 / 2-3 天（PORT_AUDIT 层 C，"把移植永久降级为重校准"的关键件）/ 依赖 A5（已验收：真栈冒测与 WaveShell 枚举就位）
- 模型分级：L3 / GLM-5.3（跨探针/白名单/PCA/冒测四段的链路设计；先读 PC 侧既有模式再动工）
- 目标：PC 侧校准链（参考 `scripts/b1_2_source_calibration_agent_smoke.py` 与 `b1_2_strict_reference_calibration_agent_smoke.py` 的模式）在 mac 可执行的等价物：一键 起栈 → 插件探针（23 个 Waves 晋升主体，U2 收窄口径，A5 的 719 清单为超集源）→ 语义白名单草稿 → PCA 重认证 → 只读冒测收尾；两端可重复（脚本尽量平台中立，mac 首跑）
- 文件域：`scripts/`（新脚本，命名对齐既有惯例）；如需 agent 侧端点/最小接线 → 列明并最小化（涉 `agent/` 的改动按域外上报流程）；不改 `VitApp/Source`
- 验收标准：① mac 本机一键全链 exit 0（复用 A5 冒测脚本拉栈或其模式，运行态独立临时目录）；② 产出 23 主体的白名单草稿工件（JSON，主体名/path/探针结论对齐 PC 侧 schema——以 PC 侧现有白名单文件结构为准并附对照说明）；③ PCA 重认证在 mac 落盘（机器本地状态目录，AGENTS §10：不写源码树/真实工程）；④ 收尾只读冒测 PASS；⑤ 工件目录含 run ID、各段日志、最终白名单草稿
- 停止条件：探针发现某晋升主体实例化/加载失败（非枚举）→ 按 R2 预案记录证据转 blocked（升级 tracktion/JUCE 另开卡）；PC 侧 schema 与 mac 实测不兼容 → 上交对照证据由决策侧裁定 schema 取舍
- 领取：2026-09-18 11:04 CST（Mac 执行流，用户直令「开工 PORT-C2」，BOOTSTRAP 口令表授权）/ origin/main `ad5d2e2acea3db264c7e27949646c0d6df56d763` / 分支 `port/c2-mac-calibration-chain`（独立 worktree `/tmp/vit-c2-worktree`，PROTOCOL §3）
- 回执：
- 验收：
