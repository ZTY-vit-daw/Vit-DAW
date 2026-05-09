# Vit-DAW V1.1 跨端架构审计总结（Godot / C++ Tracktion / ZMQ）

## 审计范围与链路

- 前端：`d:\Godot\project\vit-daw-frontend`
- 中间桥：`d:\Vit_DAW\scripts\bridge_core.py`（UDP 4445/4444 ↔ ZMQ REQ/REP + PUB）
- 内核：`d:\Vit_DAW\VitApp\Source\Service\CommandDispatcher.cpp`、`ZmqGateway.cpp`、`VitHeadlessService.cpp`
- 协议基线：`d:\Vit_DAW\docs\VIT_IPC_CONTRACT.md`

核心链路为：Godot 发命令（UDP）→ Python bridge（ZMQ REQ）→ C++ CommandDispatcher；  
状态反向链路为：C++ ZMQ PUB → Python bridge UDP 转发 → Godot Telemetry/项目刷新消费。

---

## 1) IPC 契约双向校验（JSON Schema 匹配度）

### 已确认事实

- C++ 侧命令分发优先级是：`cmd` -> `action` -> `command`。
- C++ 返回错误结构统一为 `{"status":"error","message":"..."}`。
- Godot `VitIpcClient` 的基础解析也按 `status/message` 处理。
- C++ 这条链路使用 JUCE JSON（`juce::JSON`），不是 `nlohmann::json`。

### 主要风险

- **命令名不对齐**：Godot 存在发送 `transport_record_arm`、`transport_loop`，但 C++ handler 未注册同名命令，实际会落入 `Unknown command`。
- **客户端 API 断裂**：Godot 多处调用 `send_command_async_await`、`send_command_async(cmd, request_id, timeout)`，但当前 `vit_ipc_client.gd` 实现与调用不一致（方法/参数数量不匹配），会导致请求关联与错误处理不可靠。
- **文档与实现存在隐式差异**：文档主要写 `cmd/action`，实现还接受 `command`，长期会放大多端认知偏差。

### 建议修复方向

- 统一 `VitIpcClient` 为单一异步契约：支持 request_id、timeout、awaitable reply。
- 在 C++ 增补缺失 transport 命令，或前端改为现有命令集合。
- 产出一份“前端命令全集 vs C++ handler 注册全集”的自动校验表（CI/脚本化）。

---

## 2) 状态真理同步链（State Synchronization）

### 已确认事实

- C++ PUB 统一由 `ZmqGateway` 推送，桥接转发到 Godot UDP。
- Godot `telemetry_manager.gd` 消费 transport/levels/delta/tile_ready。
- `Vit_Dock` 路径普遍采用 IPC 成功后 `get_project_state` 回拉并重绘。

### 主要风险

- **单向真理被破坏（局部）**：`Vit_Graph_Rack.gd` 在连线/断线时存在乐观本地 `connect_node/disconnect_node` 再发 IPC；后端拒绝/超时时会出现短暂分裂。
- **另一条 UI 路径“本地先改”**：`vit_control_v_1.0.gd` 轨道增删存在直接改场景树行为，和 Dock 的“内核真源”策略并存，容易双系统漂移。
- **PUB/UDP 丢帧现实**：`publishMessage(dontwait)` + UDP 无确认，增量事件可能丢失，若无周期性全量回拉将积累偏差。
- **端口冲突隐患**：存在另一个脚本也绑定 4444 的风险点，可能导致遥测监听失败。

### 建议修复方向

- 机架拓扑改为“pending 视觉状态 + 以内核回拉收敛”，避免本地直接写真相。
- 统一 Track 增删入口：全部走 IPC，再由 `get_project_state` 驱动 UI 重建。
- 给关键变更（rack/track/plugin）增加失配回拉策略与降级保护。

---

## 3) 高频交互防抖 vs 调度堵塞

### 已确认事实

- ZMQ 网络线程收到命令后，真正 JSON 解析与业务执行通过 `MessageManager::callAsync` 回到 JUCE message thread。
- 不存在“在音频 processBlock 里直接解析 JSON”的证据。
- C++ telemetry 定时器 50ms 推送，叠加 delta/event，天然是高频流。
- Python bridge 对控制命令是逐包透传，无节流阀。

### 主要风险

- **消息线程压力风险**：虽然不在音频线程执行，但高频命令会压满 message thread，造成 UI/控制迟滞。
- **ZMQ 请求等待窗口**：REP 等待 message thread future，最坏会卡 5 秒再超时回复。
- **delta ring buffer 满时丢事件**：高峰突发会丢增量，若前端只吃增量且缺回拉，状态一致性受损。
- **Godot 高频发射端未完全证据化**：当前仓库中 Godot 高频滑动/拖拽节流策略并未在同一代码树完整闭环验证。

### 建议修复方向

- 前端滑块/拖拽统一改为“drag-end 提交 + 可选节流”。
- 后端高频命令分级：可合并命令尽量合并，重操作分批。
- 建立“高频压测脚本 + 超时/丢包指标”。

---

## 4) 生命周期与 Undo 撤销事务闭环

### 已确认事实

- `undo/redo` 命令存在，执行后会触发内核侧同步更新。
- 插件删除路径已显式 `beginNewTransaction("Delete plugin")`。
- `Tracktion` 层 `deleteTrack/removeFromParent` 是 ValueTree + UndoManager 语义。

### 主要风险

- **事务覆盖不均匀**：`delete_track`、`add_track`、部分 set 类命令没有统一显式事务命名，撤销粒度可能不可控。
- **Undo 后资源侧状态未必同事务回滚**：如 `TiledSpectrogramBaker::releaseTrackMappings` 这类外部清理动作不一定与 Undo 同步复原。
- **Godot 端失败处理偏静默**：某些 undo/redo 失败路径直接 return，用户层难感知。
- **旧引用悬挂风险**：删除插件后若有异步链持有裸指针/旧 ID 视图对象，可能出现空实例或 stale 访问。

### 建议修复方向

- 对 Track/Plugin/Rack 结构变更统一显式 `beginNewTransaction(...)`。
- Undo/Redo 后补充必要资源重建钩子（尤其波形映射/缓存）。
- 前端对撤销失败给出明确提示，并强制刷新关键视图。

---

## 当前结论（用于 V1.1 攻坚优先级）

### P0（先做）

- 统一 `VitIpcClient` 实际 API 与调用方契约。
- 修正 Godot->C++ 命令名不对齐（transport record/loop）。
- 统一“内核真源”策略，禁止轨道/机架核心状态本地先改后补偿。

### P1（紧随其后）

- 补齐 `delete_track/add_track/set_*` 等事务一致性与 Undo 可预期性。
- 高频交互做前端节流与后端压力观测（message thread / timeout / dropped delta）。

### P2（稳定性加固）

- 建立契约自动比对（命令名、字段、类型、成功/失败响应）。
- 增加跨端回归用例：删除/撤销/重做/连线/设备切换/高频拖动。

---

## 建议的下一步执行顺序

1. 先修 IPC 客户端契约（Godot 单点）并跑全链路烟测。  
2. 再修命令名与 handler 对齐（transport 相关）。  
3. 然后处理机架/轨道“本地先改”路径，统一为内核回拉驱动。  
4. 最后做 Undo 事务覆盖与高频性能治理。  

