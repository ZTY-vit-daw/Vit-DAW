# VIS-FILM-P0-ROUTE-ENABLE：读图回路验证（VitReel 实施线 P0）

- 发卡：GLM 主管决策侧 / 2026-10-11 晚夜（依据=VIS-FILM-DESIGN-1 设计件五 卡1，用户三线同步 VitReel 归 Mac）
- 派发确认：已确认（用户已裁"即刻启用"——原规格的"用户点头"前置已满足）
- 验收负责人：GLM 主管决策流
- 池序 81；**规格全文=[设计件五 卡1](../../runs/VIS-FILM-DESIGN-1/05-impl-card-specs.md)（权威）**
- 归属：Mac 生成线（VitReel）；P1 / 0.5 天 / 无
- 模型分级：L1 / Mac 执行流

## 范围修订（主管 2026-10-11 晚）

规格①（config 启用一键）**已由主管完成**（enabled=true 已写入，用户裁定即刻启用）——本卡只做②③：真实 agent 进程内 ≥1280×720 真实截图读图回路+读图质量记录（不足则升级路由模型为决策项上报）。工件落 coord/runs/VIS-FILM-P0/。

- 领取：2026-10-11 22:05 +0800 / origin/main=563080dd / owner=GLM-5.3 Mac 执行流（ZCode 会话）@ worktree /Users/timozty/Documents/Vit-DAW-visfilm-p0 / 分支=port/vis-film-p0 / 领取时 git status 净（worktree 自 origin/main 新建，零源码改动）
- 回执：**run 工件 [coord/runs/VIS-FILM-P0/run1-20261011-2215/](../../runs/VIS-FILM-P0/run1-20261011-2215/README.md) @ port/vis-film-p0（commit 36d28dca，含 README/截图/探针源码归档/agent 原始日志/telemetry）**。摘要：①Mac 侧①复刻——主管 38532a7a 的 enabled=true 只落 PC 本机，Mac 本机 `~/.vit/config.json` 原值仍 false，本腿同款一键改 true（差异校验=仅此一处，备份在 run 目录）；②读图回路成功——真实 vitagent 进程在位（/health ok，无内核/Godot，规格允许；降级 WARN=预期）+离屏 WKWebView 产真实 webui 截图（逻辑 1280×720/物理 2560×1440 Retina 2x，无主机 GUI 触碰=AGENTS §5 合规）+一次性探针二进制以生产接线（`config.Load()`→`&llm.Client{}`→`CompleteImageUnderstanding`，与 B4 未来消费方逐字同 wiring，源码归档后即删、agent 树零 diff）调读成功：responses 端点 in:1149/out:2720、17.3s、telemetry 1 条无错、零 panic、`image_understanding_route_unavailable` 零出现、停止条件（大图 4xx/5xx）未触发；③**质量评注=足够，无需升级路由模型**（ground truth 对拍：布局准确/中文文字誊写逐条吻合/元素计数量级正确 ±2-3/不确定性诚实自报 8 条；B4 消费注意=输出含推理前言需截尾、计数近似不作硬断言、17.3s/2720tok 成本锚点）。**端测覆盖边界声明**：读图调用执行体=生产接线的独立真实进程（探针二进制），非 vitagent PID 内——生产 agent 进程内当前零调用方（消费方=B4 卡池序在后）；vitagent 真实进程在位腿覆盖启动+路由启用不破坏。退出码底账：探针 rc 未直接亲采（zsh PIPESTATUS 拼写失误），成功以带内三重证据（OK 行+err:""+telemetry 无错）+代码路径推定，诚实记录于 run README。栈占用 22:10–22:40 登记/释放（dd9ac1df/后续 res 提交），teardown 零残留。
- 验收：（裁定文件）
