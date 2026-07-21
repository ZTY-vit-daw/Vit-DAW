# VSP v1 Foundation

VSP 是 Vit Session Protocol。它不是单一网络协议，而是 Vit Kernel、GUI、Agent、控制器、分析器、网关和扩展客户端之间的正式会话契约。

本目录用于承接 VSP v1 Foundation 阶段。该阶段的目标不是继续在旧 UDP/JSON 全量广播上补丁式修复 GUI 和 Agent，而是先把 Vit DAW 的底层通信、状态、实时数据、资产和权限模型定稳，再迁移 GUI 与 Agent。

## 文档入口

- [VSP v1 Foundation Spec](VSP_V1_FOUNDATION_SPEC.md)
  - 角色、层级、通道、传输、状态模型、实时规则、资产模型、权限、插件设备和迁移原则。
- [VSP v1 Execution Target](VSP_V1_EXECUTION_TARGET.md)
  - 可直接交给新对话流/目标模式执行的目标说明、阶段任务、禁止事项和完成定义。
- [VSP v1 Conformance and Smoke](VSP_V1_CONFORMANCE_AND_SMOKE.md)
  - 一致性测试、性能验收、GUI 手测和 Agent 烟测矩阵。
- [VSP Phase 0 Audit](VSP_PHASE0_AUDIT.md)
  - 当前 Kernel / Agent / Godot 通信链路、重 payload、GUI 卡顿模型、旧 IPC 兼容边界和 Phase 1 进入条件。
- [VSP Phase 1 Schema and Channels](VSP_PHASE1_SCHEMA_AND_CHANNELS.md)
  - VSP envelope、session/capability、Command / State / Realtime / Asset / Event 通道骨架和 legacy IPC adapter 设计。
- [VSP v1 Closeout Report](VSP_V1_CLOSEOUT_REPORT.md)
- [VSP Hub Architecture](VSP_HUB_ARCHITECTURE.md)
  - 独立 `VspHub.exe` 的长期架构、进程边界、角色接入和迁移原则。
- [VSP Hub Transports](VSP_HUB_TRANSPORTS.md)
  - `POST /vsp`、`/vsp/stream`、端口、环境变量和未来 local IPC 绑定。
- [VSP Hub Extension API](VSP_HUB_EXTENSION_API.md)
  - 第三方扩展通过 `role=extension` 接入 Hub 的 hello/register/capability 模型。
- [VSP Migration from Legacy Bridge](VSP_MIGRATION_FROM_LEGACY_BRIDGE.md)
  - 从 Go legacy bridge、VitAgent 临时 `/vsp` 到独立 VSP Hub 的迁移边界。
  - Phase 6 真实 GUI/F5 验收、legacy telemetry 降级验证、最终收口门槛和待跑回归清单。

## 当前结论

1. 当前 IPC 已证明 Vit Kernel、Godot GUI 和 Agent 可被分离，但旧链路把命令、状态、遥测、实时数据和资产引用混在一起，已经暴露出 GUI 卡顿、状态洪水、实时反馈掉帧、导入后界面负载失控等问题。
2. UDP/JSON 不再适合作为主干协议。它可以保留为兼容层或小包辅助通道，但 VSP v1 必须把不同数据类型分通道处理。
3. VSP v1 的设计必须面向未来自研内核，不能把 Tracktion Engine 的内部结构泄漏为外部长期契约。
4. Agent 插件抓手继续留在 Agent 侧。VSP 只负责提供低延迟、可批量、可验证的插件执行通道，不接管 OCR、网页检索、用户学习和 Plugin Skill 智能。
5. GUI 可定制和 Agent 可接入是 Vit 的平台能力。VSP 要把这种能力做成正式 SDK/协议，而不是靠桥接脚本和隐式 JSON 字段拼接。

## 与旧 IPC 的关系

旧文档 [../VIT_IPC_CONTRACT.md](../VIT_IPC_CONTRACT.md) 是现状契约，仍是迁移期间的兼容基线。VSP v1 不是删除旧 IPC，而是定义新架构，然后按通道迁移：

1. 先保留旧命令兼容。
2. 新增 VSP Server 和 SDK。
3. GUI/Agent 按 Command、State、Realtime、Asset、Plugin 等通道逐步迁移。
4. 旧 UDP/JSON 桥接最终降级为 legacy adapter。
