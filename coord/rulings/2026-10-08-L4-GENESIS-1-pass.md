# Ruling：L4-GENESIS-1 —— pass（2026-10-08 晚窗，决策侧）

- 执行侧：flash（用户转交）；实现 `b1388a9d` @ `port/l4-genesis-1`（worktree D:/Vit_DAW_wt_genesis，base 9e30ac5f）
- 落位：cherry-pick → main `71e78c12`；三 run 工件保全主树 coord/runs/L4-GENESIS-1/（不入库）；worktree/分支随裁定清理

## 验收亲核

1. **diff 形态**：harness.go +3 行尾挂（refresh 之后）+ 新文件 project_genesis.go/测试 + G-2 补齐 + l4_genesis 烟测场景——AppendGenesis/GenesisFacts 零改动（幂等语义保持）。
2. **投影纪律**：三事实组全走既有只读消费面（BuildProjectTOMProjection/render-profile/内核快照同源）——零扩投影接口，停止条件不触发。
3. **测试**：4 harness 测试（幂等/fail-open 三形态/路由文本化）+G-2 两测试我方 worktree 复跑全绿；getwd 失败分支 Windows 不可移植触发的实测探针+代码读核申报采信。
4. **真栈**：l4_genesis 场景三腿（fail-open 不阻塞+WARN 可见 / genesis 2 条目带链式哈希 / 再开字节级幂等）×2 轮 PASS，末轮 SCRIPT-LASTEXITCODE=0 直采；首轮 LEG1 失败=断言面选错（响应白名单）属执行偏差如实分记。全量 90 包 0 FAIL+G0 五条过（申报+我方抽查采信）。

## 上交裁定（本裁定处置）

1. **跨域账本目录解析缺陷采信并立卡 L4-LEDGER-DIR-1**：真栈 project_path 为 .vit 文件路径，装配读取面（agentloop runProjectDirFromState / chat 装配）与 retain 写入面（exit_retain.go）裸喂 ledger ProjectDir→真栈 L4 层永 absent+retain 无声失败——IMPL-D 端测边界②的原因闭合。GENESIS 挂点侧已自带解析（.vit→父目录，对齐 projectstore 语义）；修复卡统一三处解析点并顺带 REVIEW-1 R-4 余缺口（G-1/G-3/G-4）。
2. **传输面白名单**（compactHostLifecycle 剥 result 注记键）：记录不裁，WARN 走日志面已足。
3. **agent 工具开面（project.open）无 genesis 挂点**：两开面并存如实申报——host 通知开面已覆盖 Godot 启动页真实驱动面；工具开面对齐与否留决策侧后裁（当前 agent 发起的 open 不入 genesis，如实边界）。
