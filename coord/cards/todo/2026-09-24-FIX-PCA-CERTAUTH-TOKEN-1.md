# FIX-PCA-CERTAUTH-TOKEN-1：static_eq 认证授权入口补齐——解锁 104 例（PC 57+mac 47）load gate 循环依赖

- 优先级 / 预估 / 依赖：P2（升格：双端同机制阻断）/ 0.3 天 / FIX-PCA-EQCHANNEL-1 验收上交裁定（2026-09-24）；PORT-PCA-AUTOSWEEP-MAC-1 验收上交并入（2026-09-24）；sweep 幂等续跑就绪（retry 语义），解锁即自动吸收
- 模型分级：L1-L2 / GLM-5.3（授权面改动，红测试先行；~40-60 行）
- **背景（双端取证同机制）**：harness 认证授权白名单已含 static_eq（harness.go:1956）但**无入口可铸授权**——runner 入口 processor_certification_entry.go:419 拒 static_eq；full-access 路要求先晋升=循环依赖；HTTP invoke 强制 source=http（PC 锚 server.go:1463 / mac 取证补缺省 source 锚 server.go:1456）。**PC 57 例 + mac 47 例** static_eq 主体卡在 load_gate_blocked（mac 侧执行侧自举不动点核验：47 例零主体任一族有晋升，幂等重跑无可吸收——入口不修，重跑无用）。
- 目标：
  1. token-only 认证授权入口（本地 token 文件/环境变量触发，绕开 HTTP source 强制与 full-access 先晋升要求；授权仅限认证面不扩运行时权限——fail-closed 边界与既有授权纪律一致）
  2. 红测试：入口铸 static_eq 授权→runner 可跑 EQ 认证（修前红：入口拒/循环依赖；修后绿）；非 token 路行为不变回归
  3. 附带（同卡注记级）：v1 import 收据种类扩容评估（FabFilter/TDR vendor_kind_missing 3 例——如需改收据种类枚举=最小独立 commit 列明）
  4. 解锁验证：`--phase certify --families static_eq` 幂等续跑吸收 57 例（能认证者晋升、边界拒绝者记因——例外桶不要求清零，要求全处置）
- 文件域：`agent/internal/chat/processor_certification_entry.go` + `agent/internal/harness/harness.go`（如需）+ 测试；**分支开发走决策侧合入**（agent 代码卡纪律）
- 验收：①红测试修前红/修后绿；②agent 全量+webui 绿；③57 例续跑全处置（认证数+记因数=57）；④非授权路径行为零变化回归
- 停止条件：授权入口设计需动 full-access 语义或运行时权限面 → 上交裁定（不擅自扩权）
- 领取：
- 回执：
- 验收：
