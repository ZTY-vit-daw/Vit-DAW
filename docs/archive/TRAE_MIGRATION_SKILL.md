# SKILL: Vit DAW Trae Migration Executor

## Skill Intent

该 Skill 用于在 Trae 中执行 Vit DAW 迁移迭代，确保阶段目标、输出格式、验收口径统一，适合持续交付与周会同步。

---

## Inputs

### Required Inputs
- `current_phase`: `P1 | P2 | P3 | P4`
- `phase_status`: 每阶段状态拆分（Done / In Progress / Todo）
- `today_goal`: 本次最多 3 个可执行目标
- `constraints`: 当前限制（例如不做本地波形加载、不做全盘扫描）

### Optional Inputs
- `known_issues`: 已知故障清单（如无声、卡顿、拖拽失效）
- `rollback_note`: 可用回滚点或开关
- `log_snippets`: 启动日志/命令日志片段

---

## Outputs

每次运行必须产出以下结构：

1. 阶段判断（当前位于 P1/P2/P3/P4）
2. 状态看板（Done / In Progress / Todo）
3. 本次执行任务（最多 3 条）
4. 变更说明（Godot / C++ / Library / Infra）
5. 验收结果（通过/失败 + 最小复现步骤）
6. 风险与回滚
7. 下一步（必须衔接下一阶段）

---

## Execution Protocol

### Step 1: Phase Gate
- 必须先确认阶段边界，禁止跨阶段引入大型需求。
- 若输入与阶段定义冲突，优先遵循阶段目标并标注冲突点。

### Step 2: Normalize Command Contract
- 命令统一为：
  - `play`
  - `stop`
  - `return_to_zero`
  - `import_audio`
- IPC 调用统一入口：`send_command_async(cmd)`。
- C++ 路由兼容 `action/cmd`，并输出标准 JSON 响应。

### Step 3: Build Task Slice
- 将任务拆成最小可验证切片，每次最多推进 1-3 项。
- 每项必须附带可观测证据（日志、状态变化、可复现操作）。

### Step 4: Validate Before Report
- 至少执行 1 轮最小测试；失败时先产出失败原因与修复建议，再给下一步。
- 禁止只描述“理论上可行”，必须给出可复现实验步骤。

### Step 5: Publish Structured Result
- 按固定输出模板发布结果，避免自由散文导致信息缺失。
- 对 Done/In Progress/Todo 逐项更新，不允许遗漏。

---

## Phase Definitions (P1-P4)

### P1: Minimal Command Chain
**目标**：能发命令、能进内核、能看到响应。  
**范围**：
- Godot 非阻塞 IPC（异步队列/轮询）
- `ping/list_tracks` 启动异步化，超时仅提示“内核离线”
- Transport: `play/stop/return_to_zero`
- 拖拽导入：`import_audio(track_index,file_path,drop_x_pos)`
- C++ `CommandDispatcher` 支持 `action/cmd` 双路由

### P2: Stability & UX Recovery
**目标**：不卡顿、界面恢复、空轨真实。  
**范围**：
- 左侧库懒加载，禁止启动全量扫盘
- 清理 `Tree blocked > 0` 结构性同步问题
- 恢复 transport 字形/样式与 Stop->0 动画反馈
- 拖拽层不拦截点击（`MOUSE_FILTER_PASS/IGNORE`）
- 空轨不加载测试音频/波形，`tile_manager` 仅清空

### P3: Places Capability
**目标**：手动书签 + 局部懒扫描 + 可拖拽文件。  
**范围**：
- “添加文件夹”入口 + `FileDialog(FILE_MODE_OPEN_DIR)`
- 持久化 `user://vit_places.json`
- 首次展开目录才扫描当前层，不递归全盘
- 节点 `file_path metadata` + `_on_tree_get_drag_data` 输出 `file_import`

### P4: Engineering Closure
**目标**：可发布、可诊断、可替换通信实现。  
**范围**：
- UI 固定调用 `send_command_async(cmd)`
- 启动打印设备/采样率/buffer/路由
- 命令打印 `import/play/stop/rtz` 前后状态
- 修复路径大小写引用与打包卫生问题
- 保证导入后播放可复现，为后续精准定位与波形回传铺路

---

## Done/In Progress/Todo State Block (Reusable)

> 每次执行前复制并更新

- P1:
  - Done:
    - [ ]
  - In Progress:
    - [ ]
  - Todo:
    - [ ]
- P2:
  - Done:
    - [ ]
  - In Progress:
    - [ ]
  - Todo:
    - [ ]
- P3:
  - Done:
    - [ ]
  - In Progress:
    - [ ]
  - Todo:
    - [ ]
- P4:
  - Done:
    - [ ]
  - In Progress:
    - [ ]
  - Todo:
    - [ ]

---

## Definition of Done (DoD) Checklist

### P1 DoD
- [ ] UI 不因 IPC 阻塞
- [ ] 四类命令请求与响应可见
- [ ] 导入链路只发 JSON，不做本地音频加载

### P2 DoD
- [ ] 启动无全量扫盘卡顿
- [ ] transport 样式与状态反馈恢复
- [ ] 空轨无默认测试资源

### P3 DoD
- [ ] Places 可添加并重启恢复
- [ ] 目录扫描按需触发
- [ ] 拖拽负载协议稳定

### P4 DoD
- [ ] 通信抽象层边界稳定
- [ ] 关键日志可定位无声问题
- [ ] 打包路径与资源卫生通过检查

---

## Failure Handling

- 若命令无响应：
  - 检查 IPC 可达性与超时提示逻辑；
  - 回退到最近可用链路，保留日志证据。
- 若启动卡顿：
  - 禁止恢复全盘扫描；改用局部刷新和按需展开。
- 若拖拽失效：
  - 检查节点 metadata 与 `file_import` 负载格式一致性。
- 若出现无声：
  - 对照启动与命令前后日志，先判定设备/路由，再判定命令执行。

---

## Self-Check Before Finalizing

- [ ] 是否明确当前阶段与阶段边界
- [ ] 是否更新了 Done/In Progress/Todo
- [ ] 是否包含最小可复现实验步骤
- [ ] 是否给出风险与回滚方案
- [ ] 术语与命令字段是否统一
- [ ] 是否避免了与阶段目标无关的扩展需求

---

## Ready-to-Use Invocation Prompt

你现在使用 Skill《Vit DAW Trae Migration Executor》执行迁移任务。  
请先输出当前阶段判断与状态看板（Done/In Progress/Todo），再给出今天最多 3 项执行任务。  
所有任务必须匹配当前阶段目标，命令字段统一为 `play/stop/return_to_zero/import_audio`，通信入口统一为 `send_command_async(cmd)`。  
完成后输出验收步骤、风险与回滚、下一步。
