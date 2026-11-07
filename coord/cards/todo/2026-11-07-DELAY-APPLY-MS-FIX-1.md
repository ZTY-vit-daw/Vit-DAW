# DELAY-APPLY-MS-FIX-1：apply_delay_controls 毫秒反演修（300 S0 取证转卡）

- 发卡：GLM 主管决策流 / 2026-11-07 晚窗（依据=[300 回执域外取证②](../done/2026-11-07-PLUGIN-CONTROL-EVAL-RUNNER-1.md)——`apply_delay_controls` value_ms=250 实落 norm 0.35065=**352ms 却报 apply_status=exact**；锚点 agent/internal/chat/plugin_compressor_apply.go:175-262+plugin_delay_apply.go:263-283；工件=300 S0 run4/5 T4 write_faces。注意：P 轮 T4 模型走 typed 路径落 249ms 正确，与该缺陷并存——缺陷在另一写面函数，非产品主路径）
- 派发确认：已确认（主管发卡）；验收负责人：GLM 主管决策流
- 池序 304；目标仓库=主仓 agent（delay apply 面）；P2 / 0.5 天 / 依赖=300 工件在库（write_faces 为夹具）；L2 / PC；零栈

## 目标

1. **反演修**：ms→normalized 换算链修（对 250ms 类目标落点正确； suspected：norm→ms 正演与反演不对称或 domain 形态误读——以 300 工件实盘蓝本钉真实机理再修）；
2. 配对单测：250ms→落点 249-251ms±宽容+apply_status 诚实（exact 仅在真精确时）+修复前 352ms 形态负锚；
3. 回归：`go test ./internal/chat -count=1` 全绿+build 0。

## 约束

- 文件域=plugin_delay_apply.go±plugin_compressor_apply.go 同链±测试；不触 typed 主路径/在飞卡域；
- 禁止 git reset --hard / git clean；零真栈。

## 停止条件

发现换算链存在在案裁定约束→锚点取证上交。

## 领取/回执/验收

- （执行侧回填）/（执行侧回填）/（主管回填）
